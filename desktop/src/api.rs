//! Client for the DevPulse HTTP API served by `devpulse serve`.
//!
//! The types mirror the JSON produced by `internal/http` and
//! `internal/metrics` on the Go side. The contract tests at the bottom
//! decode the Go golden files, so a field rename on either side fails a
//! test instead of the dashboard at runtime.

use std::fmt;
use std::time::Duration;

use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};

use crate::i18n::Texts;
use crate::month::Month;

/// A tracked repository, as listed by `GET /api/v1/repos`.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Repo {
    pub id: String,
    pub full_name: String,
    pub owner: String,
    pub name: String,
    pub provider: String,
    pub description: Option<String>,
    pub default_branch: String,
    pub disabled: bool,
    /// Operator settings (`devpulse repo config`). Defaulted so an older
    /// server that does not send them still decodes.
    #[serde(default = "default_pr_start")]
    pub pr_start: u32,
    #[serde(default)]
    pub incident_label: String,
    #[serde(default)]
    pub hotfix_label: String,
}

fn default_pr_start() -> u32 {
    1
}

/// Answer to registering a repo.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Registration {
    pub repo: Repo,
    /// False when the repo was already tracked.
    pub created: bool,
    /// Why GitHub metadata could not be fetched; the repo is registered
    /// anyway.
    pub metadata_error: Option<String>,
}

/// A partial settings update; `None` fields are left unchanged.
#[derive(Debug, Clone, Default, PartialEq, Serialize)]
pub struct RepoPatch {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pr_start: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub incident_label: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hotfix_label: Option<String>,
}

/// State of the server's background sync.
#[derive(Debug, Clone, Default, PartialEq, Deserialize)]
pub struct SyncStatus {
    /// The repo being synced; empty when idle.
    pub running: String,
    pub started_at: Option<String>,
    pub last_repo: String,
    pub last_finished_at: Option<String>,
    pub last_error: String,
}

#[derive(Debug, Deserialize)]
struct ReposResponse {
    repos: Vec<Repo>,
}

/// Every metric for one repo over a `[from, to)` month window.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Report {
    pub repo: String,
    pub from: String,
    pub to: String,
    pub build_failure: BuildFailure,
    pub avg_builds_per_pr: f64,
    pub pr_lead_time: HoursSummary,
    pub review_wait: ReviewWait,
    pub pr_size_distribution: Vec<SizeBucketCount>,
    pub daily_build_duration: Vec<DayBuildDuration>,
    /// `None` while the server does not know the repo's default branch,
    /// and for a report limited to a member or team.
    pub dora: Option<Box<Dora>>,
    /// Whose work the report covers; `None` means everyone.
    #[serde(default)]
    pub scope: Option<Scope>,
}

/// The member, team or unmapped account a report is limited to.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Scope {
    /// "member", "team", or "account".
    pub kind: String,
    pub id: String,
    pub name: String,
    pub accounts: Vec<String>,
}

/// A person, shown by display name, acting through GitHub accounts.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Member {
    pub id: String,
    pub display_name: String,
    pub accounts: Vec<String>,
    pub team_ids: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Team {
    pub id: String,
    pub name: String,
    pub member_ids: Vec<String>,
}

/// One line of the per-member breakdown: a member, or an active account
/// nobody has mapped yet (`member_id` is `None`, `name` is the account).
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Row {
    pub member_id: Option<String>,
    pub name: String,
    pub accounts: Vec<String>,
    pub report: Report,
}

#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct ByMember {
    pub repo: String,
    pub from: String,
    pub to: String,
    pub rows: Vec<Row>,
}

/// Whose work to limit metrics to.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub enum ScopeParam {
    #[default]
    Everyone,
    Member(String),
    Team(String),
}

impl ScopeParam {
    fn query(&self) -> Option<(&'static str, &str)> {
        match self {
            Self::Everyone => None,
            Self::Member(id) => Some(("member", id)),
            Self::Team(id) => Some(("team", id)),
        }
    }
}

#[derive(Debug, Deserialize)]
struct MembersResponse {
    members: Vec<Member>,
}

#[derive(Debug, Deserialize)]
struct TeamsResponse {
    teams: Vec<Team>,
}

#[derive(Debug, Deserialize)]
struct AccountsResponse {
    accounts: Vec<String>,
}

#[derive(Serialize)]
struct MemberBody<'a> {
    display_name: &'a str,
    accounts: &'a [String],
}

#[derive(Serialize)]
struct TeamBody<'a> {
    name: &'a str,
    member_ids: &'a [String],
}

/// CI failure rate over PR-triggered builds. `rate` is 0..=1.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct BuildFailure {
    pub total: u64,
    pub failed: u64,
    pub rate: f64,
}

/// Avg / p50 / p90 of a sample of durations, in hours. `count == 0`
/// means no data.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct HoursSummary {
    pub count: u64,
    pub avg_hours: f64,
    pub p50_hours: f64,
    pub p90_hours: f64,
}

/// The four DORA metrics. A deployment is a PR merged into
/// `default_branch`.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct Dora {
    pub default_branch: String,
    pub hotfix_label: String,
    pub incident_label: String,
    pub deployments: u64,
    pub per_week: f64,
    pub deploy_days: u64,
    pub lead_time: HoursSummary,
    pub reverts: u64,
    pub hotfixes: u64,
    /// `None` when there were no deployments (0/0 is not a perfect score).
    pub change_failure_rate: Option<f64>,
    pub recovery: HoursSummary,
    pub recovery_from_reverts: u64,
    pub recovery_from_incidents: u64,
}

/// PR ready → first review duration, in hours.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct ReviewWait {
    pub count: u64,
    pub avg_hours: f64,
}

/// One bar of the PR size distribution, in ascending size order.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct SizeBucketCount {
    pub bucket: String,
    pub count: u64,
}

/// Average build duration of one UTC day.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct DayBuildDuration {
    pub day: String,
    pub avg_seconds: f64,
    pub count: u64,
}

/// One report per month, oldest first.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct MonthlyReport {
    pub repo: String,
    pub from: String,
    pub to: String,
    pub months: Vec<Report>,
}

#[derive(Debug, Deserialize)]
struct ErrorBody {
    error: String,
}

/// Why an API call failed, classified so the UI can say what to fix.
#[derive(Debug, Clone, PartialEq)]
pub enum ApiError {
    /// The server rejected the token (401).
    Unauthorized,
    /// The repo is not tracked, or the path does not exist (404).
    NotFound(String),
    /// The request was malformed, e.g. a bad month window (400).
    BadRequest(String),
    /// The server is busy with conflicting work, e.g. another sync (409).
    Conflict(String),
    /// The server cannot do this, e.g. sync without GITHUB_TOKEN (503).
    Unavailable(String),
    /// Any other non-2xx status.
    Status(u16, String),
    /// The server could not be reached.
    Transport(String),
    /// The body was not the JSON shape this client expects.
    Decode(String),
}

impl fmt::Display for ApiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Unauthorized => write!(f, "API token was rejected (401)"),
            Self::NotFound(msg) => write!(f, "not found: {msg}"),
            Self::BadRequest(msg) => write!(f, "bad request: {msg}"),
            Self::Conflict(msg) | Self::Unavailable(msg) => write!(f, "{msg}"),
            Self::Status(code, msg) => write!(f, "server error {code}: {msg}"),
            Self::Transport(msg) => write!(f, "cannot reach server: {msg}"),
            Self::Decode(msg) => write!(f, "unexpected response: {msg}"),
        }
    }
}

impl std::error::Error for ApiError {}

impl ApiError {
    /// The message the dashboard shows, in the UI language. `Display`
    /// stays English for logs. The server's own detail text is passed
    /// through untranslated.
    pub fn describe(&self, t: &Texts) -> String {
        match self {
            Self::Unauthorized => t.err_unauthorized.to_string(),
            Self::NotFound(msg) => (t.err_not_found)(msg),
            Self::BadRequest(msg) => (t.err_bad_request)(msg),
            Self::Conflict(msg) => (t.err_conflict)(msg),
            Self::Unavailable(msg) => (t.err_unavailable)(msg),
            Self::Status(code, msg) => (t.err_status)(*code, msg),
            Self::Transport(msg) => (t.err_transport)(msg),
            Self::Decode(msg) => (t.err_decode)(msg),
        }
    }
}

/// Blocking API client. Calls run on worker threads, never on the UI
/// thread, so a blocking client keeps the dashboard free of an async
/// runtime.
#[derive(Clone)]
pub struct Client {
    base_url: String,
    token: String,
    agent: ureq::Agent,
}

impl Client {
    pub fn new(base_url: &str, token: &str) -> Self {
        let agent = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(60)))
            // Read error bodies ourselves so the server's message reaches
            // the UI instead of a bare status code.
            .http_status_as_error(false)
            .build()
            .into();
        Self {
            base_url: base_url.trim().trim_end_matches('/').to_string(),
            token: token.trim().to_string(),
            agent,
        }
    }

    /// `GET /healthz`, which needs no token.
    pub fn health(&self) -> Result<(), ApiError> {
        let resp = self
            .agent
            .get(format!("{}/healthz", self.base_url))
            .call()
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json::<serde_json::Value>(resp).map(|_| ())
    }

    pub fn list_repos(&self) -> Result<Vec<Repo>, ApiError> {
        self.get::<ReposResponse>("/api/v1/repos", None)
            .map(|r| r.repos)
    }

    pub fn metrics(
        &self,
        repo: &Repo,
        from: Month,
        to: Month,
        scope: &ScopeParam,
    ) -> Result<Report, ApiError> {
        let path = format!("/api/v1/repos/{}/{}/metrics", repo.owner, repo.name);
        self.get_scoped(&path, Some((from, to)), scope)
    }

    pub fn monthly_metrics(
        &self,
        repo: &Repo,
        from: Month,
        to: Month,
        scope: &ScopeParam,
    ) -> Result<MonthlyReport, ApiError> {
        let path = format!("/api/v1/repos/{}/{}/metrics/monthly", repo.owner, repo.name);
        self.get_scoped(&path, Some((from, to)), scope)
    }

    pub fn metrics_by_member(
        &self,
        repo: &Repo,
        from: Month,
        to: Month,
    ) -> Result<ByMember, ApiError> {
        let path = format!(
            "/api/v1/repos/{}/{}/metrics/by-member",
            repo.owner, repo.name
        );
        self.get(&path, Some((from, to)))
    }

    pub fn list_members(&self) -> Result<Vec<Member>, ApiError> {
        self.get::<MembersResponse>("/api/v1/members", None)
            .map(|r| r.members)
    }

    /// Creates a member, or updates it when `id` is given.
    pub fn save_member(
        &self,
        id: Option<&str>,
        display_name: &str,
        accounts: &[String],
    ) -> Result<Member, ApiError> {
        let body = MemberBody {
            display_name,
            accounts,
        };
        let req = match id {
            None => self.agent.post(self.url("/api/v1/members")),
            Some(id) => self.agent.put(self.url(&format!("/api/v1/members/{id}"))),
        };
        let resp = self
            .authed(req)
            .send_json(body)
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }

    pub fn delete_member(&self, id: &str) -> Result<(), ApiError> {
        self.delete(&format!("/api/v1/members/{id}"))
    }

    pub fn list_teams(&self) -> Result<Vec<Team>, ApiError> {
        self.get::<TeamsResponse>("/api/v1/teams", None)
            .map(|r| r.teams)
    }

    /// Creates a team, or updates it when `id` is given.
    pub fn save_team(
        &self,
        id: Option<&str>,
        name: &str,
        member_ids: &[String],
    ) -> Result<Team, ApiError> {
        let body = TeamBody { name, member_ids };
        let req = match id {
            None => self.agent.post(self.url("/api/v1/teams")),
            Some(id) => self.agent.put(self.url(&format!("/api/v1/teams/{id}"))),
        };
        let resp = self
            .authed(req)
            .send_json(body)
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }

    pub fn delete_team(&self, id: &str) -> Result<(), ApiError> {
        self.delete(&format!("/api/v1/teams/{id}"))
    }

    pub fn excluded_accounts(&self) -> Result<Vec<String>, ApiError> {
        self.get::<AccountsResponse>("/api/v1/excluded-accounts", None)
            .map(|r| r.accounts)
    }

    /// Replaces the excluded set; returns it as the server normalized it.
    pub fn replace_excluded_accounts(&self, accounts: &[String]) -> Result<Vec<String>, ApiError> {
        #[derive(Serialize)]
        struct Body<'a> {
            accounts: &'a [String],
        }
        let resp = self
            .authed(self.agent.put(self.url("/api/v1/excluded-accounts")))
            .send_json(Body { accounts })
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json::<AccountsResponse>(resp).map(|r| r.accounts)
    }

    fn delete(&self, path: &str) -> Result<(), ApiError> {
        let resp = self
            .authed(self.agent.delete(self.url(path)))
            .call()
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_empty(resp)
    }

    /// Registers `full_name` (`owner/name`) for tracking.
    pub fn register_repo(&self, full_name: &str) -> Result<Registration, ApiError> {
        #[derive(Serialize)]
        struct Body<'a> {
            full_name: &'a str,
        }
        let resp = self
            .authed(self.agent.post(self.url("/api/v1/repos")))
            .send_json(Body {
                full_name: full_name.trim(),
            })
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }

    pub fn update_repo(&self, repo: &Repo, patch: &RepoPatch) -> Result<Repo, ApiError> {
        let resp = self
            .authed(self.agent.patch(self.repo_url(repo, "")))
            .send_json(patch)
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }

    /// Stops tracking the repo; the server deletes its synced data.
    pub fn remove_repo(&self, repo: &Repo) -> Result<(), ApiError> {
        let resp = self
            .authed(self.agent.delete(self.repo_url(repo, "")))
            .call()
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_empty(resp)
    }

    pub fn start_sync(&self, repo: &Repo) -> Result<SyncStatus, ApiError> {
        let resp = self
            .authed(self.agent.post(self.repo_url(repo, "/sync")))
            .send_empty()
            .map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }

    pub fn sync_status(&self) -> Result<SyncStatus, ApiError> {
        self.get("/api/v1/sync", None)
    }

    fn url(&self, path: &str) -> String {
        format!("{}{path}", self.base_url)
    }

    fn repo_url(&self, repo: &Repo, suffix: &str) -> String {
        self.url(&format!(
            "/api/v1/repos/{}/{}{suffix}",
            repo.owner, repo.name
        ))
    }

    fn authed<B>(&self, req: ureq::RequestBuilder<B>) -> ureq::RequestBuilder<B> {
        req.header("Authorization", format!("Bearer {}", self.token))
    }

    fn get<T: DeserializeOwned>(
        &self,
        path: &str,
        window: Option<(Month, Month)>,
    ) -> Result<T, ApiError> {
        self.get_scoped(path, window, &ScopeParam::Everyone)
    }

    fn get_scoped<T: DeserializeOwned>(
        &self,
        path: &str,
        window: Option<(Month, Month)>,
        scope: &ScopeParam,
    ) -> Result<T, ApiError> {
        let mut req = self.authed(self.agent.get(self.url(path)));
        if let Some((key, value)) = scope.query() {
            req = req.query(key, value);
        }
        if let Some((from, to)) = window {
            req = req
                .query("from", from.to_string())
                .query("to", to.to_string());
        }
        let resp = req.call().map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }
}

fn read_json<T: DeserializeOwned>(resp: ureq::http::Response<ureq::Body>) -> Result<T, ApiError> {
    let body = read_ok(resp)?;
    serde_json::from_str(&body).map_err(|e| ApiError::Decode(e.to_string()))
}

/// For responses without a body (204).
fn read_empty(resp: ureq::http::Response<ureq::Body>) -> Result<(), ApiError> {
    read_ok(resp).map(|_| ())
}

/// Returns the body of a 2xx response, or the classified error.
fn read_ok(mut resp: ureq::http::Response<ureq::Body>) -> Result<String, ApiError> {
    let status = resp.status().as_u16();
    let body = resp
        .body_mut()
        .read_to_string()
        .map_err(|e| ApiError::Transport(e.to_string()))?;

    if (200..300).contains(&status) {
        return Ok(body);
    }

    let msg = serde_json::from_str::<ErrorBody>(&body)
        .map(|e| e.error)
        .unwrap_or(body);
    Err(match status {
        401 => ApiError::Unauthorized,
        404 => ApiError::NotFound(msg),
        400 => ApiError::BadRequest(msg),
        409 => ApiError::Conflict(msg),
        503 => ApiError::Unavailable(msg),
        _ => ApiError::Status(status, msg),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{BufRead, BufReader, Write};
    use std::net::TcpListener;
    use std::sync::mpsc;
    use std::thread;

    const GOLDEN_METRICS: &str = include_str!("../../internal/http/testdata/metrics.json");
    const GOLDEN_MONTHLY: &str = include_str!("../../internal/http/testdata/metrics_monthly.json");

    #[test]
    fn decodes_go_metrics_golden() {
        let r: Report = serde_json::from_str(GOLDEN_METRICS).expect("decode metrics golden");
        assert_eq!(r.repo, "MilesChou/devpulse");
        assert_eq!((r.from.as_str(), r.to.as_str()), ("2026-05", "2026-06"));
        assert_eq!((r.build_failure.failed, r.build_failure.total), (2, 3));
        assert!((r.avg_builds_per_pr - 1.5).abs() < 1e-9);
        assert!((r.pr_lead_time.p90_hours - 28.0).abs() < 1e-9);
        let buckets: Vec<_> = r
            .pr_size_distribution
            .iter()
            .map(|b| b.bucket.as_str())
            .collect();
        assert_eq!(buckets, ["XS", "S", "M", "L", "XL"]);
        assert_eq!(r.daily_build_duration.len(), 2);

        let d = r.dora.expect("dora section");
        assert_eq!(d.default_branch, "main");
        assert_eq!((d.deployments, d.reverts, d.hotfixes), (3, 1, 0));
        assert!((d.change_failure_rate.unwrap() - 1.0 / 3.0).abs() < 1e-9);
        assert!((d.lead_time.p90_hours - 30.0).abs() < 1e-9);
        assert_eq!((d.recovery_from_reverts, d.recovery_from_incidents), (1, 1));
    }

    #[test]
    fn decodes_null_dora_and_null_failure_rate() {
        let mut v: serde_json::Value = serde_json::from_str(GOLDEN_METRICS).unwrap();
        v["dora"] = serde_json::Value::Null;
        let r: Report = serde_json::from_value(v).expect("null dora");
        assert!(r.dora.is_none());

        let m: MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        let april = m.months[0].dora.as_ref().expect("april dora");
        assert_eq!(april.deployments, 0);
        assert_eq!(april.change_failure_rate, None);
    }

    #[test]
    fn decodes_go_monthly_golden() {
        let m: MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).expect("decode monthly golden");
        let months: Vec<_> = m.months.iter().map(|r| r.from.as_str()).collect();
        assert_eq!(months, ["2026-04", "2026-05"]);
        assert!(m.months[0].daily_build_duration.is_empty());
    }

    /// Serves one canned HTTP response and reports the request head it
    /// received, so tests can assert on the path and headers.
    fn serve_once(status: u16, body: &'static str) -> (String, mpsc::Receiver<String>) {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind");
        let addr = listener.local_addr().expect("addr");
        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept");
            let mut head = String::new();
            let mut reader = BufReader::new(stream.try_clone().expect("clone"));
            loop {
                let mut line = String::new();
                reader.read_line(&mut line).expect("read");
                if line == "\r\n" || line.is_empty() {
                    break;
                }
                head.push_str(&line);
            }
            // Append the request body, if any, so tests can check it.
            let len = head
                .lines()
                .find_map(|l| {
                    let (k, v) = l.split_once(':')?;
                    k.eq_ignore_ascii_case("content-length")
                        .then(|| v.trim().parse::<usize>().ok())?
                })
                .unwrap_or(0);
            if len > 0 {
                let mut buf = vec![0; len];
                std::io::Read::read_exact(&mut reader, &mut buf).expect("read body");
                head.push('\n');
                head.push_str(&String::from_utf8_lossy(&buf));
            }
            let resp = format!(
                "HTTP/1.1 {status} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
            stream.write_all(resp.as_bytes()).expect("write");
            tx.send(head).expect("send head");
        });
        (format!("http://{addr}/"), rx)
    }

    /// The JSON body serve_once appended after the request headers.
    fn body_json(head: &str) -> serde_json::Value {
        let (_, body) = head.split_once("\r\n\n").expect("request has a body");
        serde_json::from_str(body).expect("body is JSON")
    }

    fn repo() -> Repo {
        Repo {
            id: "01J".into(),
            full_name: "MilesChou/devpulse".into(),
            owner: "MilesChou".into(),
            name: "devpulse".into(),
            provider: "github".into(),
            description: None,
            default_branch: "main".into(),
            disabled: false,
            pr_start: 1,
            incident_label: "incident".into(),
            hotfix_label: "hotfix".into(),
        }
    }

    #[test]
    fn sends_bearer_token_and_window() {
        let (url, head) = serve_once(200, GOLDEN_METRICS);
        let client = Client::new(&url, " tok ");
        let from: Month = "2026-05".parse().unwrap();
        let report = client
            .metrics(&repo(), from, from.next(), &ScopeParam::Everyone)
            .expect("metrics");
        assert_eq!(report.repo, "MilesChou/devpulse");

        let head = head.recv().unwrap();
        assert!(
            head.starts_with(
                "GET /api/v1/repos/MilesChou/devpulse/metrics?from=2026-05&to=2026-06 "
            ),
            "request line: {head}"
        );
        assert!(
            head.to_ascii_lowercase()
                .contains("authorization: bearer tok\r\n"),
            "headers: {head}"
        );
    }

    #[test]
    fn maps_error_statuses() {
        let (url, _) = serve_once(401, r#"{"error":"missing or invalid bearer token"}"#);
        assert_eq!(
            Client::new(&url, "x").list_repos(),
            Err(ApiError::Unauthorized)
        );

        let (url, _) = serve_once(404, r#"{"error":"repo acme/x is not tracked"}"#);
        let err = Client::new(&url, "x")
            .monthly_metrics(
                &repo(),
                "2026-01".parse().unwrap(),
                "2026-02".parse().unwrap(),
                &ScopeParam::Everyone,
            )
            .unwrap_err();
        assert_eq!(err, ApiError::NotFound("repo acme/x is not tracked".into()));

        let (url, _) = serve_once(500, "boom");
        assert_eq!(
            Client::new(&url, "x").list_repos(),
            Err(ApiError::Status(500, "boom".into()))
        );
    }

    #[test]
    fn register_posts_the_name() {
        let (url, head) = serve_once(
            201,
            r#"{"repo":{"id":"1","full_name":"acme/web","owner":"acme","name":"web","provider":"github","description":null,"default_branch":"","disabled":false,"pr_start":1,"incident_label":"incident","hotfix_label":"hotfix"},"created":true,"metadata_error":"fetch github metadata: 404"}"#,
        );
        let reg = Client::new(&url, "t")
            .register_repo(" acme/web ")
            .expect("register");
        assert!(reg.created);
        assert_eq!(reg.repo.full_name, "acme/web");
        assert_eq!(
            reg.metadata_error.as_deref(),
            Some("fetch github metadata: 404")
        );

        let head = head.recv().unwrap();
        assert!(head.starts_with("POST /api/v1/repos "), "{head}");
        assert_eq!(
            body_json(&head),
            serde_json::json!({"full_name": "acme/web"})
        );
    }

    #[test]
    fn patch_sends_only_changed_fields() {
        let (url, head) = serve_once(
            200,
            r#"{"id":"01J","full_name":"MilesChou/devpulse","owner":"MilesChou","name":"devpulse","provider":"github","description":null,"default_branch":"main","disabled":false,"pr_start":500,"incident_label":"incident","hotfix_label":"hotfix"}"#,
        );
        let patch = RepoPatch {
            pr_start: Some(500),
            ..Default::default()
        };
        let updated = Client::new(&url, "t")
            .update_repo(&repo(), &patch)
            .expect("patch");
        assert_eq!(updated.pr_start, 500);

        let head = head.recv().unwrap();
        assert!(
            head.starts_with("PATCH /api/v1/repos/MilesChou/devpulse "),
            "{head}"
        );
        assert_eq!(body_json(&head), serde_json::json!({"pr_start": 500}));
    }

    #[test]
    fn remove_accepts_no_content() {
        let (url, head) = serve_once(204, "");
        Client::new(&url, "t").remove_repo(&repo()).expect("remove");
        assert!(
            head.recv()
                .unwrap()
                .starts_with("DELETE /api/v1/repos/MilesChou/devpulse ")
        );
    }

    #[test]
    fn sync_errors_are_classified() {
        let (url, _) = serve_once(
            503,
            r#"{"error":"sync is unavailable: the server has no GITHUB_TOKEN"}"#,
        );
        assert_eq!(
            Client::new(&url, "t").start_sync(&repo()),
            Err(ApiError::Unavailable(
                "sync is unavailable: the server has no GITHUB_TOKEN".into()
            ))
        );
        let (url, _) = serve_once(409, r#"{"error":"another sync is running: acme/api"}"#);
        assert_eq!(
            Client::new(&url, "t").start_sync(&repo()),
            Err(ApiError::Conflict(
                "another sync is running: acme/api".into()
            ))
        );
    }

    #[test]
    fn repo_without_settings_fields_decodes() {
        // An older server sends no operator settings.
        let r: Repo = serde_json::from_str(
            r#"{"id":"1","full_name":"a/b","owner":"a","name":"b","provider":"github","description":null,"default_branch":"main","disabled":false}"#,
        )
        .unwrap();
        assert_eq!((r.pr_start, r.hotfix_label.as_str()), (1, ""));
    }

    #[test]
    fn scope_goes_into_the_query() {
        let (url, head) = serve_once(200, GOLDEN_METRICS);
        let from: Month = "2026-05".parse().unwrap();
        Client::new(&url, "t")
            .metrics(&repo(), from, from.next(), &ScopeParam::Team("01T".into()))
            .expect("metrics");
        let head = head.recv().unwrap();
        assert!(
            head.starts_with(
                "GET /api/v1/repos/MilesChou/devpulse/metrics?team=01T&from=2026-05&to=2026-06 "
            ),
            "{head}"
        );
    }

    #[test]
    fn saves_member_with_put_when_editing() {
        let (url, head) = serve_once(
            200,
            r#"{"id":"01M","display_name":"Alice","accounts":["alice"],"team_ids":[]}"#,
        );
        let m = Client::new(&url, "t")
            .save_member(Some("01M"), "Alice", &["alice".into()])
            .expect("save");
        assert_eq!(m.display_name, "Alice");
        let head = head.recv().unwrap();
        assert!(head.starts_with("PUT /api/v1/members/01M "), "{head}");
        assert_eq!(
            body_json(&head),
            serde_json::json!({"display_name": "Alice", "accounts": ["alice"]})
        );
    }

    #[test]
    fn decodes_by_member_rows() {
        let body = format!(
            r#"{{"repo":"a/b","from":"2026-05","to":"2026-06","rows":[{{"member_id":null,"name":"alice","accounts":["alice"],"report":{GOLDEN_METRICS}}}]}}"#
        );
        let b: ByMember = serde_json::from_str(&body).expect("decode");
        assert_eq!(b.rows[0].member_id, None);
        assert_eq!(b.rows[0].report.build_failure.total, 3);
    }

    #[test]
    fn unreachable_server_is_transport_error() {
        // Bind then drop, so the port is very likely closed.
        let addr = TcpListener::bind("127.0.0.1:0")
            .unwrap()
            .local_addr()
            .unwrap();
        let err = Client::new(&format!("http://{addr}"), "x")
            .health()
            .unwrap_err();
        assert!(matches!(err, ApiError::Transport(_)), "{err:?}");
    }

    #[test]
    fn describes_errors_per_language_keeping_server_detail() {
        use crate::i18n::{EN, ZH_TW};
        let e = ApiError::Status(503, "db down".into());
        assert_eq!(e.describe(&EN), e.to_string());
        assert_eq!(e.describe(&ZH_TW), "伺服器錯誤 503：db down");
        assert_eq!(
            ApiError::Unauthorized.describe(&ZH_TW),
            "API token 被拒絕（401）"
        );
        assert_eq!(
            ApiError::Transport("refused".into()).describe(&ZH_TW),
            "無法連上伺服器：refused"
        );
    }
}
