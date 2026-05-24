-- `DROP VIEW IF EXISTS` is portable across PostgreSQL, MySQL 8+, and
-- SQLite, so the generic down works for every dialect (MySQL/SQLite
-- never created these views, but IF EXISTS makes the drop safe).
DROP VIEW IF EXISTS v_failure_rate_daily_member_repo;
DROP VIEW IF EXISTS v_failure_rate_daily_repo;
DROP VIEW IF EXISTS v_attributable_builds;

