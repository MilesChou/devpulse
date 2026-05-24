-- Failure-rate views (PostgreSQL).
--
-- Two layers:
--   v_attributable_builds                -- attribution rule (exclude
--                                           push-to-trunk, spec rule)
--                                           plus day / week / month
--                                           columns precomputed.
--   v_failure_rate_daily_repo            -- daily aggregate per repo.
--   v_failure_rate_daily_member_repo     -- daily aggregate per
--                                           (member, repo), matches
--                                           metrics-aggregation spec
--                                           Requirement 1.
--
-- Granularity: daily is the finest meaningful grain (the source builds
-- table has sub-second timestamps but failure-rate is only stable at
-- day resolution upward). Weekly and monthly are NOT exposed as
-- separate views — every daily row carries `day`, `week`, `month`
-- columns, so a caller rolling up to either grain just GROUP BYs the
-- right column and SUMs the counts:
--
--   -- daily
--   SELECT day, builds, failures FROM v_failure_rate_daily_repo
--   WHERE repo_id = ? ORDER BY day DESC;
--
--   -- weekly
--   SELECT week, SUM(builds), SUM(failures)
--   FROM v_failure_rate_daily_repo
--   WHERE repo_id = ? GROUP BY week ORDER BY week DESC;
--
--   -- monthly
--   SELECT month, SUM(builds), SUM(failures)
--   FROM v_failure_rate_daily_repo
--   WHERE repo_id = ? GROUP BY month ORDER BY month DESC;
--
-- The views deliberately do NOT expose `fail_pct`. Failure rate is
-- non-additive — AVG(fail_pct) across days is NOT the correct weekly
-- or monthly rate, only SUM(failures) / SUM(builds) is. Forcing the
-- caller to compute the percentage from raw counts makes it
-- impossible to roll up incorrectly.
--
-- MySQL / SQLite cannot express COUNT(*) FILTER (WHERE ...) nor
-- DATE_TRUNC; their .up.sql variants are no-ops for now.

CREATE VIEW v_attributable_builds AS
SELECT
    b.id,
    b.repo_id,
    b.author_account,
    b.started_at,
    b.is_failure,
    b.trigger_event,
    DATE_TRUNC('day',   b.started_at)::date AS day,
    DATE_TRUNC('week',  b.started_at)::date AS week,
    DATE_TRUNC('month', b.started_at)::date AS month
FROM builds b
WHERE b.trigger_event != 'push';

CREATE VIEW v_failure_rate_daily_repo AS
SELECT
    repo_id,
    day,
    week,
    month,
    COUNT(*) AS builds,
    COUNT(*) FILTER (WHERE is_failure) AS failures
FROM v_attributable_builds
GROUP BY repo_id, day, week, month;

CREATE VIEW v_failure_rate_daily_member_repo AS
SELECT
    repo_id,
    author_account,
    day,
    week,
    month,
    COUNT(*) AS builds,
    COUNT(*) FILTER (WHERE is_failure) AS failures
FROM v_attributable_builds
WHERE author_account IS NOT NULL  -- skip rows whose author back-fill hasn't landed
GROUP BY repo_id, author_account, day, week, month;

