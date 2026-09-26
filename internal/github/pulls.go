package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/timex"
)

// rawPull is the slim subset of GitHub's REST PR JSON we read. Extend as
// new fields become relevant; unknown JSON keys are tolerated.
type rawPull struct {
	Number         int        `json:"number"`
	State          string     `json:"state"` // open / closed
	Draft          bool       `json:"draft"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	User           *rawUser   `json:"user"`
	Labels         []rawLabel `json:"labels"`
	Base           rawRef     `json:"base"`
	Head           rawRef     `json:"head"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	MergedAt       *time.Time `json:"merged_at"`
	ClosedAt       *time.Time `json:"closed_at"`

	// PR detail-only fields. Absent from the /pulls list endpoint.
	Additions    int `json:"additions,omitempty"`
	Deletions    int `json:"deletions,omitempty"`
	ChangedLines int `json:"changed_files,omitempty"`
}

type rawUser struct {
	Login string `json:"login"`
}

type rawLabel struct {
	Name string `json:"name"`
}

type rawRef struct {
	Ref string `json:"ref"`
}

// toDomain converts a rawPull to the domain model. repoID flows in so
// the caller can attach the PR to the right repo aggregate; repoName
// scopes the "Reverts owner/repo#N" body reference to this repo.
func (r rawPull) toDomain(repoID string, repoName repo.FullName) pullrequest.PullRequest {
	author := ""
	if r.User != nil {
		author = r.User.Login
	}

	status := pullrequest.StatusOpen
	switch {
	case r.MergedAt != nil:
		status = pullrequest.StatusMerged
	case strings.EqualFold(r.State, "closed"):
		status = pullrequest.StatusClosed
	}

	pr := pullrequest.PullRequest{
		RepoID:            repoID,
		Number:            r.Number,
		Author:            author,
		Status:            status,
		Additions:         r.Additions,
		Deletions:         r.Deletions,
		TotalChangedLines: r.Additions + r.Deletions,
		IsDraft:           r.Draft,
		CreatedAt:         r.CreatedAt.UTC(),
		MergedAt:          timex.PtrUTC(r.MergedAt),
		ClosedAt:          timex.PtrUTC(r.ClosedAt),
		Title:             r.Title,
		BaseRef:           r.Base.Ref,
		HeadRef:           r.Head.Ref,
		MergeCommitSHA:    r.MergeCommitSHA,
		RevertsNumber:     pullrequest.ParseRevertedNumber(r.Body, repoName.String()),
		SourceUpdatedAt:   r.UpdatedAt.UTC(),
	}
	for _, l := range r.Labels {
		pr.Labels = append(pr.Labels, l.Name)
	}

	// GitHub's REST does not surface ready_at directly. For non-draft PRs,
	// approximate it with CreatedAt — accurate for the common case. A
	// dedicated enrichment pass (via GraphQL timelineItems) is the correct
	// fix but stays out of v1 to limit scope.
	if !r.Draft {
		t := r.CreatedAt.UTC()
		pr.ReadyAt = &t
	}
	return pr
}

// listPullsPage returns one page of the state=all PR list ordered by
// sort ("created" or "updated"), newest first. Callers that paginate do
// so by page number; the Link header is not tracked.
func (c *Client) listPullsPage(
	ctx context.Context,
	repoName repo.FullName,
	sort string,
	page, perPage int,
) ([]rawPull, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", repoName.Owner, repoName.Name)
	q := url.Values{}
	q.Set("state", "all")
	q.Set("sort", sort)
	q.Set("direction", "desc")
	q.Set("per_page", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))

	var batch []rawPull
	if _, err := c.rest(ctx, "GET", path, q, &batch); err != nil {
		return nil, err
	}
	return batch, nil
}

// GetPullRequest fetches a single PR with detail (additions/deletions).
// Returns an error wrapping ErrNotFound when the upstream responds 404
// (the number belongs to an issue, was deleted, or never existed).
func (c *Client) GetPullRequest(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	number int,
) (pullrequest.PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repoName.Owner, repoName.Name, number)

	var raw rawPull
	if _, err := c.rest(ctx, "GET", path, nil, &raw); err != nil {
		return pullrequest.PullRequest{}, err
	}
	return raw.toDomain(repoID, repoName), nil
}

// GetLatestPRNumber returns the highest PR number currently in the repo.
// It pulls the first page of the sort=created direction=desc list and
// reads the first entry's number — one REST call regardless of repo
// size.
//
// Assumption: GitHub assigns PR numbers monotonically at creation time,
// so "newest by created_at" coincides with "highest number". This is
// not formally documented but holds in practice; if upstream ever
// changes, the by-number backfill upper bound would skip the tail of
// any window where a new PR was opened between this call and the loop
// end — that PR would still be picked up on the next sync via the
// db_max+1 cursor.
//
// Returns 0 with no error when the repo has zero PRs.
func (c *Client) GetLatestPRNumber(
	ctx context.Context,
	repoName repo.FullName,
) (int, error) {
	batch, err := c.listPullsPage(ctx, repoName, "created", 1, 1)
	if err != nil {
		return 0, fmt.Errorf("get latest pr number: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	return batch[0].Number, nil
}

// pullsPerPage is GitHub's maximum page size for the PR list.
const pullsPerPage = 100

// ListPullRequestsUpdatedSince walks the PR list most-recently-updated
// first and returns the number and updated_at of every PR whose
// updated_at is at or after since. newest is the updated_at of the most recently updated PR
// in the repo (zero when it has none); the caller stores it as the next
// since, so the watermark comes from upstream's clock and never runs
// ahead of what was actually listed.
//
// A zero since only reads newest from a single one-item page and
// returns no PRs: it never walks the full history.
//
// A PR updated while the walk is in progress moves to the head of the
// list, past the pages already read. Its new updated_at is later than
// newest, so the next call picks it up.
func (c *Client) ListPullRequestsUpdatedSince(
	ctx context.Context,
	repoName repo.FullName,
	since time.Time,
) (updated []pullrequest.Stamp, newest time.Time, err error) {
	perPage := pullsPerPage
	if since.IsZero() {
		perPage = 1
	}

	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, time.Time{}, err
		}

		batch, err := c.listPullsPage(ctx, repoName, "updated", page, perPage)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("list updated pull requests: %w", err)
		}

		for i, p := range batch {
			if page == 1 && i == 0 {
				newest = p.UpdatedAt.UTC()
			}
			if since.IsZero() || p.UpdatedAt.Before(since) {
				return updated, newest, nil
			}
			updated = append(updated, pullrequest.Stamp{Number: p.Number, UpdatedAt: p.UpdatedAt.UTC()})
		}
		if len(batch) < perPage {
			return updated, newest, nil
		}
	}
}

// prCommitsPerPage is GitHub's maximum page size for PR commits. Only
// the first page is read: see GetFirstCommitAt.
const prCommitsPerPage = 100

type rawPRCommit struct {
	Commit struct {
		Author *struct {
			Date *time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// GetFirstCommitAt returns the earliest commit author date among the
// PR's commits — the DORA lead-time start. Author date (not committer
// date) is used because it survives rebases. Only the first page (100
// commits) is inspected; for larger PRs the minimum over that page is
// returned, which is documented as a known approximation. Returns nil
// when no commit carries an author date.
func (c *Client) GetFirstCommitAt(
	ctx context.Context,
	repoName repo.FullName,
	number int,
) (*time.Time, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/commits", repoName.Owner, repoName.Name, number)
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(prCommitsPerPage))

	var commits []rawPRCommit
	if _, err := c.rest(ctx, "GET", path, q, &commits); err != nil {
		return nil, fmt.Errorf("list pr commits: %w", err)
	}

	var first *time.Time
	for _, cm := range commits {
		a := cm.Commit.Author
		if a == nil || a.Date == nil {
			continue
		}
		if first == nil || a.Date.Before(*first) {
			first = a.Date
		}
	}
	return timex.PtrUTC(first), nil
}
