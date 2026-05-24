-- MySQL does not support `COUNT(*) FILTER (WHERE ...)` nor `DATE_TRUNC`,
-- so a faithful port of the PostgreSQL failure_rate views needs CASE
-- expressions and DATE_FORMAT. Tracked as a follow-up; the no-op here
-- keeps schema_migrations in sync so MySQL deployments don't bail.
SELECT 1;

