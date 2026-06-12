package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

type MetricsPersister struct{ *Persister }

func NewMetricsPersister(p *Persister) *MetricsPersister {
	return &MetricsPersister{Persister: p}
}

func (m *MetricsPersister) BuildFailureRate(ctx context.Context, repoID string, from, to time.Time) (total, failed int, rate float64, err error) {
	const q = `SELECT COUNT(*), COUNT(CASE WHEN is_failure THEN 1 END)
	           FROM builds
	           WHERE repo_id = ? AND started_at >= ? AND started_at < ? AND is_pull_request = true`

	if err = m.QueryRowCtx(ctx, q, repoID, from, to).Scan(&total, &failed); err != nil {
		return 0, 0, 0, fmt.Errorf("build failure rate: %w", err)
	}
	if total > 0 {
		rate = float64(failed) / float64(total)
	}
	return total, failed, rate, nil
}

func (m *MetricsPersister) AverageBuildsPerPR(ctx context.Context, repoID string, from, to time.Time) (float64, error) {
	const q = `SELECT AVG(cnt) FROM (
	             SELECT COUNT(*) AS cnt FROM builds
	             WHERE repo_id = ? AND started_at >= ? AND started_at < ?
	               AND pr_number IS NOT NULL AND pr_number > 0
	             GROUP BY pr_number
	           ) AS per_pr`

	var avg sql.NullFloat64
	if err := m.QueryRowCtx(ctx, q, repoID, from, to).Scan(&avg); err != nil {
		return 0, fmt.Errorf("avg builds per pr: %w", err)
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

func (m *MetricsPersister) PRLeadTime(ctx context.Context, repoID string, from, to time.Time) (count int, avgHours, p50Hours, p90Hours float64, err error) {
	const q = `SELECT pr_created_at, merged_at FROM pull_requests
	           WHERE repo_id = ? AND status = 'merged' AND merged_at >= ? AND merged_at < ?`

	rows, err := m.QueryCtx(ctx, q, repoID, from, to)
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
	p50Hours = percentile(durations, 0.5)
	p90Hours = percentile(durations, 0.9)

	return count, avgHours, p50Hours, p90Hours, nil
}

func (m *MetricsPersister) PRSizeDistribution(ctx context.Context, repoID string, from, to time.Time) (map[string]int, error) {
	const q = `SELECT COALESCE(size_bucket, 'unknown'), COUNT(*)
	           FROM pull_requests
	           WHERE repo_id = ? AND pr_created_at >= ? AND pr_created_at < ?
	           GROUP BY size_bucket`

	rows, err := m.QueryCtx(ctx, q, repoID, from, to)
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

func (m *MetricsPersister) ReviewWaitTime(ctx context.Context, repoID string, from, to time.Time) (count int, avgHours float64, err error) {
	const q = `SELECT ready_at, first_review_at FROM pull_requests
	           WHERE repo_id = ? AND ready_at IS NOT NULL AND first_review_at IS NOT NULL
	             AND ready_at >= ? AND ready_at < ?`

	rows, err := m.QueryCtx(ctx, q, repoID, from, to)
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
func (m *MetricsPersister) DailyBuildDuration(ctx context.Context, repoID string, from, to time.Time) ([]DayDuration, error) {
	const q = `SELECT started_at, duration_seconds FROM builds
	           WHERE repo_id = ? AND started_at >= ? AND started_at < ?
	             AND duration_seconds IS NOT NULL`

	rows, err := m.QueryCtx(ctx, q, repoID, from, to)
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

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
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
