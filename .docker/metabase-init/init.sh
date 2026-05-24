#!/bin/sh
# Metabase bootstrap script.
#
# Runs once per `docker compose up` against the `metabase` service that
# the compose overlay started. Two phases — each is idempotent on its
# own, so re-running the script in any state converges to the same end.
#
#   Phase 1: ensure admin user.
#     - has-user-setup=false → POST /api/setup with admin + database.
#       This is the fast path on a fresh install: one round-trip
#       creates the admin AND attempts to create the data source.
#       /api/setup's database payload is best-effort; phase 2 verifies.
#     - has-user-setup=true  → log in as the hardcoded admin to mint a
#       session token. The admin must have been created previously by
#       this script (or by someone who used the same credentials);
#       any other authentication failure is fatal so the failure is
#       observable — silent half-bootstraps are worse than a crash.
#
#   Phase 2 (always): ensure the DevPulse data source exists.
#     Empirically observed in this stack: when Postgres is still
#     booting (Metabase healthy but PG not yet accepting connections),
#     /api/setup's `database` payload can succeed at the HTTP level
#     (admin user created) yet never persist the data source. Phase 2
#     lists databases via the freshly minted session and POSTs
#     /api/database if "DevPulse" is missing. Subsequent runs find the
#     source and no-op.
#
# Design rules:
#   - All JSON parsing goes through jq. Earlier versions used grep +
#     sed to extract fields and accumulated a string of edge-case
#     bugs (id ordering, escape handling, empty-grep silent failures).
#   - All JSON payloads are built with jq, so env-var overrides
#     containing quotes, backslashes, or `$` are escaped automatically.
#   - All failure paths exit non-zero. WARN-then-success is invisible
#     to `docker compose up -d --wait`; if you want to allow a failure
#     mode, document it in README, not in a silent exit-0 branch.
set -eu

METABASE_URL="${METABASE_URL:-http://metabase:3000}"

# Defaults baked in for the local dev stack. Override via env on the
# `metabase-init` service in docker-compose.metabase.yml.
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@devpulse.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme1!}"
ADMIN_FIRST_NAME="${ADMIN_FIRST_NAME:-Dev}"
ADMIN_LAST_NAME="${ADMIN_LAST_NAME:-Admin}"
SITE_NAME="${SITE_NAME:-DevPulse}"

# Where Metabase reaches the application data — service name on the
# devpulse docker network, not host.docker.internal.
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-devpulse}"
DB_USER="${DB_USER:-devpulse}"
DB_PASS="${DB_PASS:-devpulse}"

DATA_SOURCE_NAME="${DATA_SOURCE_NAME:-DevPulse}"

# curl is given a small retry budget so a single transient hiccup
# (PG slow to accept the first connection, Metabase still finishing
# its startup schema migration, ...) doesn't kill the whole bootstrap.
# --max-time bounds the request; --retry retries non-2xx and network
# errors; --retry-delay waits between attempts.
CURL_OPTS="--max-time 30 --retry 3 --retry-delay 2"

cleanup() {
  rm -f /tmp/setup-response /tmp/login-response /tmp/db-response /tmp/db-list
}
trap cleanup EXIT

# login_admin POSTs /api/session with the hardcoded admin credentials
# and echoes the session id on success, or exits the script non-zero
# on any failure. Used by both phase 1 (after /api/setup) and phase 2
# (already-setup path) so the session id has exactly one source of
# truth — the /api/session endpoint — independent of /api/setup's
# response shape, which has surprised us before.
login_admin() {
  local payload status session
  payload=$(jq -n \
    --arg user "$ADMIN_EMAIL" \
    --arg pass "$ADMIN_PASSWORD" \
    '{username: $user, password: $pass}')

  status=$(
    curl $CURL_OPTS -sS -o /tmp/login-response -w '%{http_code}' \
      -X POST "$METABASE_URL/api/session" \
      -H 'Content-Type: application/json' \
      -d "$payload"
  )

  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "[metabase-init] FATAL: cannot log in as $ADMIN_EMAIL (HTTP $status)" >&2
    echo "[metabase-init] The admin was likely created with different credentials" >&2
    echo "[metabase-init] (manual UI wizard with a different email, or password rotated)." >&2
    echo "[metabase-init] Fix one of the following and rerun:" >&2
    echo "[metabase-init]   - reset Metabase metadata: docker volume rm devpulse_metabase-data" >&2
    echo "[metabase-init]   - or set ADMIN_EMAIL/ADMIN_PASSWORD env on the metabase-init service" >&2
    exit 1
  fi

  session=$(jq -r '.id // empty' /tmp/login-response)
  if [ -z "$session" ]; then
    echo "[metabase-init] FATAL: /api/session returned $status but body had no .id" >&2
    cat /tmp/login-response >&2
    exit 1
  fi
  echo "$session"
}

SESSION=""

# ---------------------------------------------------------------------
# Phase 1: ensure admin user exists.
# ---------------------------------------------------------------------

echo "[metabase-init] probing $METABASE_URL/api/session/properties"
PROPS=$(curl $CURL_OPTS -fsS "$METABASE_URL/api/session/properties")

HAS_USER=$(printf '%s' "$PROPS" | jq -r '."has-user-setup"')

# Anything other than the two booleans means the API contract changed
# or Metabase served an error page; refuse to guess.
case "$HAS_USER" in
  true|false) ;;
  *)
    echo "[metabase-init] FATAL: has-user-setup is '$HAS_USER' (expected 'true' or 'false')"
    echo "[metabase-init] /api/session/properties returned:"
    printf '%s\n' "$PROPS"
    exit 1
    ;;
esac

if [ "$HAS_USER" = "false" ]; then
  TOKEN=$(printf '%s' "$PROPS" | jq -r '."setup-token" // empty')
  if [ -z "$TOKEN" ]; then
    echo "[metabase-init] FATAL: has-user-setup=false but setup-token missing"
    exit 1
  fi

  echo "[metabase-init] no admin exists, bootstrapping admin via /api/setup"

  # Build the payload with jq so any env-supplied values are escaped
  # safely. The `database` block here is best-effort; phase 2 verifies.
  SETUP_PAYLOAD=$(jq -n \
    --arg token "$TOKEN" \
    --arg first "$ADMIN_FIRST_NAME" \
    --arg last  "$ADMIN_LAST_NAME" \
    --arg email "$ADMIN_EMAIL" \
    --arg pass  "$ADMIN_PASSWORD" \
    --arg site  "$SITE_NAME" \
    --arg ds    "$DATA_SOURCE_NAME" \
    --arg host  "$DB_HOST" \
    --argjson port "$DB_PORT" \
    --arg dbname "$DB_NAME" \
    --arg dbuser "$DB_USER" \
    --arg dbpass "$DB_PASS" \
    '{
      token: $token,
      user: {
        first_name: $first,
        last_name:  $last,
        email:      $email,
        password:   $pass,
        site_name:  $site
      },
      database: {
        engine: "postgres",
        name:   $ds,
        details: {
          host:     $host,
          port:     $port,
          dbname:   $dbname,
          user:     $dbuser,
          password: $dbpass,
          ssl:      false,
          "tunnel-enabled":   false,
          "advanced-options": false
        }
      },
      prefs: {
        site_name: $site,
        allow_tracking: false
      }
    }')

  HTTP_STATUS=$(
    curl $CURL_OPTS -sS -o /tmp/setup-response -w '%{http_code}' \
      -X POST "$METABASE_URL/api/setup" \
      -H 'Content-Type: application/json' \
      -d "$SETUP_PAYLOAD"
  )

  if [ "$HTTP_STATUS" -lt 200 ] || [ "$HTTP_STATUS" -ge 300 ]; then
    echo "[metabase-init] /api/setup FAILED with HTTP $HTTP_STATUS"
    echo "[metabase-init] response body:"
    cat /tmp/setup-response
    exit 1
  fi

  echo "[metabase-init] /api/setup OK ($HTTP_STATUS)"
  echo "[metabase-init]   admin email    = $ADMIN_EMAIL"
  echo "[metabase-init]   admin password = (hardcoded default; see docker-compose.metabase.yml for value)"
fi

# Get an admin session via /api/session. We do this whether we just
# ran /api/setup or detected an existing admin — it's the single
# source of truth for the session id and removes any dependency on
# the /api/setup response shape.
echo "[metabase-init] logging in to obtain admin session"
SESSION=$(login_admin)

# ---------------------------------------------------------------------
# Phase 2: ensure the DevPulse data source exists.
# ---------------------------------------------------------------------

echo "[metabase-init] checking for '$DATA_SOURCE_NAME' data source"

curl $CURL_OPTS -fsS "$METABASE_URL/api/database" \
  -H "X-Metabase-Session: $SESSION" \
  -o /tmp/db-list

# `.data` on newer Metabase, top-level array on older. Accept either,
# then check whether any element has name == DATA_SOURCE_NAME.
EXISTS=$(jq --arg name "$DATA_SOURCE_NAME" \
  '[(.data // .) | .[] | select(.name == $name)] | length' /tmp/db-list)

if [ "$EXISTS" -gt 0 ]; then
  echo "[metabase-init] data source '$DATA_SOURCE_NAME' already present, nothing to do"
  exit 0
fi

echo "[metabase-init] data source '$DATA_SOURCE_NAME' missing, creating via /api/database"

DB_PAYLOAD=$(jq -n \
  --arg ds    "$DATA_SOURCE_NAME" \
  --arg host  "$DB_HOST" \
  --argjson port "$DB_PORT" \
  --arg dbname "$DB_NAME" \
  --arg dbuser "$DB_USER" \
  --arg dbpass "$DB_PASS" \
  '{
    engine: "postgres",
    name:   $ds,
    details: {
      host:     $host,
      port:     $port,
      dbname:   $dbname,
      user:     $dbuser,
      password: $dbpass,
      ssl:      false,
      "tunnel-enabled":   false,
      "advanced-options": false
    },
    is_full_sync: true,
    is_on_demand: false
  }')

DB_STATUS=$(
  curl $CURL_OPTS -sS -o /tmp/db-response -w '%{http_code}' \
    -X POST "$METABASE_URL/api/database" \
    -H 'Content-Type: application/json' \
    -H "X-Metabase-Session: $SESSION" \
    -d "$DB_PAYLOAD"
)

if [ "$DB_STATUS" -lt 200 ] || [ "$DB_STATUS" -ge 300 ]; then
  echo "[metabase-init] /api/database FAILED with HTTP $DB_STATUS"
  echo "[metabase-init] response body:"
  cat /tmp/db-response
  exit 1
fi

echo "[metabase-init] data source '$DATA_SOURCE_NAME' created (HTTP $DB_STATUS)"

