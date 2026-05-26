package travis

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/x/commitsha"
	"github.com/mileschou/devpulse/internal/x/timex"
)

// Pushes to a trunk branch are post-merge builds; pushes to any other
// branch are treated as PR-flow builds (covers PRs merged before the
// Travis backfill catches up).
var trunkBranches = map[string]struct{}{"main": {}, "master": {}}

// rawBuild is the slim subset of Travis v3 /repo/:slug/builds JSON.
type rawBuild struct {
	ID                int        `json:"id"`
	Number            string     `json:"number"`
	State             string     `json:"state"`
	EventType         string     `json:"event_type"`
	PullRequestNumber int        `json:"pull_request_number"`
	Duration          *int       `json:"duration"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	Commit            *struct {
		SHA string `json:"sha"`
	} `json:"commit"`
	Branch *struct {
		Name string `json:"name"`
	} `json:"branch"`
	Repository *struct {
		Slug string `json:"slug"`
	} `json:"repository"`
}

type buildsResponse struct {
	Builds []rawBuild `json:"builds"`
}

// ListBuildsSince pages builds in id-desc order and stops once a page
// contains a build at-or-before `since`. The boundary page is returned
// in full so callers can rely on DB-side dedupe to absorb the overlap.
// A zero `since` walks the full history (cold start). ctx cancellation
// is checked between pages. Per-build memory footprint is ~100 bytes
// so even 100k+ histories stay under a few MB.
func (c *Client) ListBuildsSince(ctx context.Context, slug string, since time.Time) ([]build.Build, error) {
	var (
		out    []build.Build
		offset = 0
	)
	hasWatermark := !since.IsZero()

	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		batch, err := c.fetchBuildsPage(ctx, slug, offset, defaultLimit)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return out, nil
		}

		reachedWatermark := false
		for _, raw := range batch {
			b, ok := buildFromRaw(raw)
			if !ok {
				continue
			}
			out = append(out, b)
			if hasWatermark && !b.StartedAt.After(since) {
				reachedWatermark = true
			}
		}

		if reachedWatermark {
			return out, nil
		}
		if len(batch) < defaultLimit {
			return out, nil
		}
		offset += defaultLimit
	}
}

// fetchBuildsPage returns one page of builds in id-desc order.
func (c *Client) fetchBuildsPage(ctx context.Context, slug string, offset, limit int) ([]rawBuild, error) {
	path := "/repo/" + repoSlugEscaped(slug) + "/builds"

	q := url.Values{}
	q.Set("include", "build.commit,build.branch")
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("sort_by", "id:desc")

	var resp buildsResponse
	if err := c.get(ctx, path, q, &resp); err != nil {
		return nil, err
	}
	return resp.Builds, nil
}

// buildFromRaw maps a rawBuild into the domain. It returns (zero, false)
// for entries that can't be placed on a timeline (no started_at and no
// finished_at).
func buildFromRaw(r rawBuild) (build.Build, bool) {
	startRaw := r.StartedAt
	if startRaw == nil {
		startRaw = r.FinishedAt // best-effort fallback
	}
	if startRaw == nil {
		return build.Build{}, false
	}
	if r.Commit == nil || r.Commit.SHA == "" {
		return build.Build{}, false
	}

	sha, err := commitsha.Parse(r.Commit.SHA)
	if err != nil {
		return build.Build{}, false
	}

	branch := ""
	if r.Branch != nil {
		branch = r.Branch.Name
	}

	return build.Build{
		ExternalID: strconv.Itoa(r.ID),
		PRNumber:   r.PullRequestNumber,
		CommitSHA:  sha,
		Branch:     branch,
		Status:     resolveStatus(r.State),
		Trigger:    resolveTrigger(r.EventType, branch),
		StartedAt:  startRaw.UTC(),
		FinishedAt: timex.PtrUTC(r.FinishedAt),
	}, true
}

func resolveStatus(state string) build.Status {
	switch strings.ToLower(state) {
	case "passed":
		return build.StatusPassed
	case "failed":
		return build.StatusFailed
	case "errored":
		return build.StatusErrored
	case "canceled":
		return build.StatusCanceled
	default:
		return build.StatusUnknown
	}
}

func resolveTrigger(eventType, branch string) build.Trigger {
	switch strings.ToLower(eventType) {
	case "pull_request":
		return build.TriggerPullRequest
	case "push":
		if _, trunk := trunkBranches[branch]; trunk {
			return build.TriggerPush
		}
		return build.TriggerPullRequest
	case "cron":
		return build.TriggerCron
	case "api":
		return build.TriggerAPI
	default:
		return build.TriggerUnknown
	}
}
