package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/x/commitsha"
	"github.com/mileschou/devpulse/internal/x/timex"
)

const actionsPerPage = 100

type workflowRun struct {
	ID           int64      `json:"id"`
	HeadSHA      string     `json:"head_sha"`
	Status       string     `json:"status"`
	Conclusion   *string    `json:"conclusion"`
	Event        string     `json:"event"`
	HeadBranch   string     `json:"head_branch"`
	RunStartedAt *time.Time `json:"run_started_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
	PullRequests []struct {
		Number int `json:"number"`
	} `json:"pull_requests"`
}

type workflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

// ListActionRuns pages through GitHub Actions workflow runs newest-first
// and stops once a page reaches `since`. The boundary page is returned in
// full; callers dedupe via DB unique constraint. A zero `since` walks the
// full history (cold start).
func (c *Client) ListActionRuns(ctx context.Context, owner, name string, since time.Time) ([]build.Build, error) {
	var out []build.Build
	hasSince := !since.IsZero()

	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		runs, err := c.fetchActionRunsPage(ctx, owner, name, since, page)
		if err != nil {
			return nil, err
		}
		if len(runs) == 0 {
			return out, nil
		}

		reachedWatermark := false
		for _, run := range runs {
			b, ok := buildFromRun(run)
			if !ok {
				continue
			}
			out = append(out, b)
			if hasSince && !b.StartedAt.After(since) {
				reachedWatermark = true
			}
		}

		if reachedWatermark {
			return out, nil
		}
		if len(runs) < actionsPerPage {
			return out, nil
		}
	}
}

func (c *Client) fetchActionRunsPage(ctx context.Context, owner, name string, since time.Time, page int) ([]workflowRun, error) {
	path := fmt.Sprintf("/repos/%s/%s/actions/runs", owner, name)

	q := url.Values{}
	q.Set("per_page", strconv.Itoa(actionsPerPage))
	q.Set("page", strconv.Itoa(page))
	if !since.IsZero() {
		q.Set("created", ">"+since.Format(time.RFC3339))
	}

	c.logger.Debug("fetching actions runs page",
		slog.String("owner", owner),
		slog.String("name", name),
		slog.Int("page", page),
	)

	var resp workflowRunsResponse
	if _, err := c.rest(ctx, "GET", path, q, &resp); err != nil {
		return nil, fmt.Errorf("actions runs page %d: %w", page, err)
	}
	return resp.WorkflowRuns, nil
}

func buildFromRun(r workflowRun) (build.Build, bool) {
	if r.Status != "completed" {
		return build.Build{}, false
	}
	if r.RunStartedAt == nil {
		return build.Build{}, false
	}

	sha, err := commitsha.Parse(r.HeadSHA)
	if err != nil {
		return build.Build{}, false
	}

	var prNumber int
	if len(r.PullRequests) > 0 {
		prNumber = r.PullRequests[0].Number
	}

	// The Actions runs API has no dedicated finished_at; updated_at on
	// a completed run is its closest proxy. Only feeds build duration,
	// not a PR lead-time denominator, so the approximation is contained.
	return build.Build{
		ExternalID: strconv.FormatInt(r.ID, 10),
		PRNumber:   prNumber,
		CommitSHA:  sha,
		Branch:     r.HeadBranch,
		Status:     resolveRunStatus(r.Conclusion),
		Trigger:    resolveRunTrigger(r.Event),
		StartedAt:  r.RunStartedAt.UTC(),
		FinishedAt: timex.PtrUTC(r.UpdatedAt),
	}, true
}

func resolveRunStatus(conclusion *string) build.Status {
	if conclusion == nil {
		return build.StatusUnknown
	}
	switch *conclusion {
	case "success":
		return build.StatusPassed
	case "failure":
		return build.StatusFailed
	case "timed_out", "startup_failure":
		// Infra-level failures; must count toward the failure-rate
		// metric — see build.StatusErrored.
		return build.StatusErrored
	case "cancelled":
		return build.StatusCanceled
	default:
		// neutral / skipped / stale / action_required are not failures.
		return build.StatusUnknown
	}
}

func resolveRunTrigger(event string) build.Trigger {
	switch event {
	case "pull_request":
		return build.TriggerPullRequest
	case "push":
		return build.TriggerPush
	case "schedule":
		return build.TriggerCron
	case "workflow_dispatch":
		return build.TriggerAPI
	default:
		return build.TriggerUnknown
	}
}
