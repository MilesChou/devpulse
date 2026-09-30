package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/people"
	"github.com/mileschou/devpulse/internal/x/statx"
)

// MetricsPersister runs the metric queries. Two filters apply to every
// query except DORA's:
//
//   - Excluded accounts (bots, see excluded_accounts) never count.
//   - With Scoped, only work owned by the given accounts counts.
//
// The owner of a PR is its author. The owner of a build is its PR's
// author, falling back to the commit author for builds not linked to a
// PR. Accounts compare normalized (people.NormalizeAccount), which the
// SQL mirrors with REPLACE(LOWER(x), '[bot]', ”).
type MetricsPersister struct {
	*Persister

	// accounts, when non-nil, limits metrics to work owned by these
	// normalized accounts: a member's or a team's.
	accounts []string
}

func NewMetricsPersister(p *Persister) *MetricsPersister {
	return &MetricsPersister{Persister: p}
}

// Scoped returns a copy limited to work owned by accounts. An empty,
// non-nil slice matches nothing (a member without accounts).
func (m *MetricsPersister) Scoped(accounts []string) *MetricsPersister {
	return &MetricsPersister{Persister: m.Persister, accounts: nonNil(accounts)}
}

// normalized mirrors people.NormalizeAccount in SQL.
func normalized(col string) string {
	return "REPLACE(LOWER(" + col + "), '[bot]', '')"
}

// ownerFilter returns the WHERE fragment (starting with AND) and its
// args for an owner column expression. The NULL guard matters: a NULL
// owner (build of an unknown commit author) is not a bot and must not
// be dropped by NOT IN, which yields NULL for it.
func (m *MetricsPersister) ownerFilter(col string) (string, []any) {
	var b strings.Builder
	b.WriteString(" AND (" + col + " IS NULL OR " + normalized(col) + " NOT IN (SELECT account FROM excluded_accounts))")
	if m.accounts == nil {
		return b.String(), nil
	}
	if len(m.accounts) == 0 {
		return b.String() + " AND 1 = 0", nil
	}
	args := make([]any, len(m.accounts))
	for i, a := range m.accounts {
		args[i] = a
	}
	b.WriteString(" AND " + normalized(col) + " IN (?" + strings.Repeat(", ?", len(m.accounts)-1) + ")")
	return b.String(), args
}

// repoIn returns "<col> IN (?, …)" and its args for a set of repo ids.
// Metrics over several repos pool their rows, so averages and
// percentiles are exact rather than averages of per-repo values. An
// empty set matches nothing.
func repoIn(col string, repoIDs []string) (string, []any) {
	if len(repoIDs) == 0 {
		return "1 = 0", nil
	}
	args := make([]any, len(repoIDs))
	for i, id := range repoIDs {
		args[i] = id
	}
	return col + " IN (?" + strings.Repeat(", ?", len(repoIDs)-1) + ")", args
}

// buildsFrom joins each build to its PR, to find the build's owner.
const buildsFrom = ` FROM builds b
	LEFT JOIN pull_requests pr ON pr.repo_id = b.repo_id AND pr.number = b.pr_number `

// buildOwner is the owner expression over buildsFrom.
const buildOwner = `COALESCE(pr.author_account, b.author_account)`

func (m *MetricsPersister) BuildFailureRate(ctx context.Context, repoIDs []string, from, to time.Time) (total, failed int, rate float64, err error) {
	repos, args := repoIn("b.repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter(buildOwner)
	q := `SELECT COUNT(*), COUNT(CASE WHEN b.is_failure THEN 1 END)` + buildsFrom + `
	      WHERE ` + repos + ` AND b.started_at >= ? AND b.started_at < ? AND b.is_pull_request = true` + owner

	args = append(append(args, from, to), ownerArgs...)
	if err = m.QueryRowCtx(ctx, q, args...).Scan(&total, &failed); err != nil {
		return 0, 0, 0, fmt.Errorf("build failure rate: %w", err)
	}
	if total > 0 {
		rate = float64(failed) / float64(total)
	}
	return total, failed, rate, nil
}

// AverageBuildsPerPR groups by repo and PR number: PR numbers repeat
// across repos, so #12 of one repo is not #12 of another.
func (m *MetricsPersister) AverageBuildsPerPR(ctx context.Context, repoIDs []string, from, to time.Time) (float64, error) {
	repos, args := repoIn("b.repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter(buildOwner)
	q := `SELECT AVG(cnt) FROM (
	        SELECT COUNT(*) AS cnt` + buildsFrom + `
	        WHERE ` + repos + ` AND b.started_at >= ? AND b.started_at < ?
	          AND b.pr_number IS NOT NULL AND b.pr_number > 0` + owner + `
	        GROUP BY b.repo_id, b.pr_number
	      ) AS per_pr`

	args = append(append(args, from, to), ownerArgs...)
	var avg sql.NullFloat64
	if err := m.QueryRowCtx(ctx, q, args...).Scan(&avg); err != nil {
		return 0, fmt.Errorf("avg builds per pr: %w", err)
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

func (m *MetricsPersister) PRLeadTime(ctx context.Context, repoIDs []string, from, to time.Time) (count int, avgHours, p50Hours, p90Hours float64, err error) {
	repos, args := repoIn("repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter("author_account")
	q := `SELECT pr_created_at, merged_at FROM pull_requests
	      WHERE ` + repos + ` AND status = 'merged' AND merged_at >= ? AND merged_at < ?` + owner

	rows, err := m.QueryCtx(ctx, q, append(append(args, from, to), ownerArgs...)...)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("pr lead time: %w", err)
	}
	defer rows.Close()

	var durations []float64
	for rows.Next() {
		var createdRaw, mergedRaw any
		if err := rows.Scan(&createdRaw, &mergedRaw); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("pr lead time scan: %w", err)
		}
		created, err := anyToTime(createdRaw)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("pr lead time parse created: %w", err)
		}
		merged, err := anyToTime(mergedRaw)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("pr lead time parse merged: %w", err)
		}
		durations = append(durations, merged.Sub(created).Hours())
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("pr lead time rows: %w", err)
	}

	count = len(durations)
	if count == 0 {
		return 0, 0, 0, 0, nil
	}

	sort.Float64s(durations)

	var sum float64
	for _, h := range durations {
		sum += h
	}
	avgHours = sum / float64(count)
	p50Hours = statx.Percentile(durations, 0.5)
	p90Hours = statx.Percentile(durations, 0.9)

	return count, avgHours, p50Hours, p90Hours, nil
}

func (m *MetricsPersister) PRSizeDistribution(ctx context.Context, repoIDs []string, from, to time.Time) (map[string]int, error) {
	repos, args := repoIn("repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter("author_account")
	q := `SELECT COALESCE(size_bucket, 'unknown'), COUNT(*)
	      FROM pull_requests
	      WHERE ` + repos + ` AND pr_created_at >= ? AND pr_created_at < ?` + owner + `
	      GROUP BY size_bucket`

	rows, err := m.QueryCtx(ctx, q, append(append(args, from, to), ownerArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("pr size dist: %w", err)
	}
	defer rows.Close()

	dist := make(map[string]int)
	for rows.Next() {
		var bucket string
		var cnt int
		if err := rows.Scan(&bucket, &cnt); err != nil {
			return nil, fmt.Errorf("pr size dist scan: %w", err)
		}
		dist[bucket] = cnt
	}
	return dist, rows.Err()
}

// ReviewWaitTime averages ready → first review. The first review is
// recomputed from the review rows rather than read from
// pull_requests.first_review_at, which sync folds in from every review
// including bots': a Copilot review seconds after ready would otherwise
// make every PR look reviewed instantly. Review rows are stored only
// for reviews submitted after ready (see the orchestrator), matching
// how first_review_at is defined.
func (m *MetricsPersister) ReviewWaitTime(ctx context.Context, repoIDs []string, from, to time.Time) (count int, avgHours float64, err error) {
	repos, args := repoIn("pr.repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter("pr.author_account")
	q := `SELECT pr.ready_at, MIN(rv.submitted_at)
	        FROM pull_requests pr
	        JOIN pull_request_reviews rv ON rv.pull_request_id = pr.id
	       WHERE ` + repos + ` AND pr.ready_at IS NOT NULL
	         AND pr.ready_at >= ? AND pr.ready_at < ?
	         AND rv.submitted_at >= pr.ready_at
	         AND ` + normalized("rv.reviewer_account") + ` NOT IN (SELECT account FROM excluded_accounts)` + owner + `
	       GROUP BY pr.id, pr.ready_at`

	rows, err := m.QueryCtx(ctx, q, append(append(args, from, to), ownerArgs...)...)
	if err != nil {
		return 0, 0, fmt.Errorf("review wait time: %w", err)
	}
	defer rows.Close()

	var totalHours float64
	for rows.Next() {
		var readyRaw, reviewRaw any
		if err := rows.Scan(&readyRaw, &reviewRaw); err != nil {
			return 0, 0, fmt.Errorf("review wait time scan: %w", err)
		}
		ready, err := anyToTime(readyRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("review wait time parse ready: %w", err)
		}
		review, err := anyToTime(reviewRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("review wait time parse review: %w", err)
		}
		totalHours += review.Sub(ready).Hours()
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("review wait time rows: %w", err)
	}

	if count > 0 {
		avgHours = totalHours / float64(count)
	}
	return count, avgHours, nil
}

type DayDuration struct {
	Day        string
	AvgSeconds float64
	Count      int
}

// DailyBuildDuration groups in Go rather than via SQL DATE():
// each driver encodes TIMESTAMP differently on the wire (the SQLite
// driver stores Go's time.String() form, which SQLite's DATE()
// cannot parse and silently maps to NULL), so day-bucketing on the
// raw started_at is the only rendering that works on every dialect.
func (m *MetricsPersister) DailyBuildDuration(ctx context.Context, repoIDs []string, from, to time.Time) ([]DayDuration, error) {
	repos, args := repoIn("b.repo_id", repoIDs)
	owner, ownerArgs := m.ownerFilter(buildOwner)
	q := `SELECT b.started_at, b.duration_seconds` + buildsFrom + `
	      WHERE ` + repos + ` AND b.started_at >= ? AND b.started_at < ?
	        AND b.duration_seconds IS NOT NULL` + owner

	rows, err := m.QueryCtx(ctx, q, append(append(args, from, to), ownerArgs...)...)
	if err != nil {
		return nil, fmt.Errorf("daily build duration: %w", err)
	}
	defer rows.Close()

	type agg struct {
		total float64
		count int
	}
	byDay := make(map[string]*agg)
	for rows.Next() {
		var startedRaw any
		var seconds float64
		if err := rows.Scan(&startedRaw, &seconds); err != nil {
			return nil, fmt.Errorf("daily build duration scan: %w", err)
		}
		started, err := anyToTime(startedRaw)
		if err != nil {
			return nil, fmt.Errorf("daily build duration parse: %w", err)
		}
		day := started.Format("2006-01-02")
		if byDay[day] == nil {
			byDay[day] = &agg{}
		}
		byDay[day].total += seconds
		byDay[day].count++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("daily build duration rows: %w", err)
	}

	days := make([]string, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Strings(days)

	out := make([]DayDuration, 0, len(days))
	for _, day := range days {
		a := byDay[day]
		out = append(out, DayDuration{
			Day:        day,
			AvgSeconds: a.total / float64(a.count),
			Count:      a.count,
		})
	}
	return out, nil
}

// Authors returns the normalized accounts that own PRs created or
// merged, or builds started, in the window, excluded accounts left out,
// sorted. It is the row set of a per-author breakdown.
func (m *MetricsPersister) Authors(ctx context.Context, repoIDs []string, from, to time.Time) ([]string, error) {
	prRepos, prArgs := repoIn("repo_id", repoIDs)
	buildRepos, buildArgs := repoIn("b.repo_id", repoIDs)
	prOwner, _ := (&MetricsPersister{Persister: m.Persister}).ownerFilter("author_account")
	buildOwnerF, _ := (&MetricsPersister{Persister: m.Persister}).ownerFilter(buildOwner)
	q := `SELECT author_account FROM pull_requests
	       WHERE ` + prRepos + ` AND author_account IS NOT NULL
	         AND ((pr_created_at >= ? AND pr_created_at < ?) OR (merged_at >= ? AND merged_at < ?))` + prOwner + `
	      UNION
	      SELECT ` + buildOwner + buildsFrom + `
	       WHERE ` + buildRepos + ` AND b.started_at >= ? AND b.started_at < ?
	         AND ` + buildOwner + ` IS NOT NULL` + buildOwnerF

	args := append(append(prArgs, from, to, from, to), buildArgs...)
	args = append(args, from, to)
	rows, err := m.QueryCtx(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("authors: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, fmt.Errorf("authors scan: %w", err)
		}
		if n := people.NormalizeAccount(a); n != "" {
			out = append(out, n)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authors rows: %w", err)
	}
	slices.Sort(out)
	return nonNil(slices.Compact(out)), nil
}

func anyToTime(v any) (time.Time, error) {
	switch val := v.(type) {
	case time.Time:
		return val.UTC(), nil
	case string:
		return parseDBTimestamp(val)
	case []byte:
		return parseDBTimestamp(string(val))
	default:
		return time.Time{}, fmt.Errorf("unexpected type %T for timestamp", v)
	}
}
