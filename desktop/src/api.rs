//! Client for the DevPulse HTTP API served by `devpulse serve`.
//!
//! The types mirror the JSON produced by `internal/http` and
//! `internal/metrics` on the Go side. The contract tests at the bottom
//! decode the Go golden files, so a field rename on either side fails a
//! test instead of the dashboard at runtime.

use std::fmt;
use std::time::Duration;

use serde::Deserialize;
use serde::de::DeserializeOwned;

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
    /// `None` while the server does not know the repo's default branch.
    pub dora: Option<Box<Dora>>,
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

    pub fn metrics(&self, repo: &Repo, from: Month, to: Month) -> Result<Report, ApiError> {
        let path = format!("/api/v1/repos/{}/{}/metrics", repo.owner, repo.name);
        self.get(&path, Some((from, to)))
    }

    pub fn monthly_metrics(
        &self,
        repo: &Repo,
        from: Month,
        to: Month,
    ) -> Result<MonthlyReport, ApiError> {
        let path = format!("/api/v1/repos/{}/{}/metrics/monthly", repo.owner, repo.name);
        self.get(&path, Some((from, to)))
    }

    fn get<T: DeserializeOwned>(
        &self,
        path: &str,
        window: Option<(Month, Month)>,
    ) -> Result<T, ApiError> {
        let mut req = self
            .agent
            .get(format!("{}{path}", self.base_url))
            .header("Authorization", format!("Bearer {}", self.token));
        if let Some((from, to)) = window {
            req = req
                .query("from", from.to_string())
                .query("to", to.to_string());
        }
        let resp = req.call().map_err(|e| ApiError::Transport(e.to_string()))?;
        read_json(resp)
    }
}

fn read_json<T: DeserializeOwned>(
    mut resp: ureq::http::Response<ureq::Body>,
) -> Result<T, ApiError> {
    let status = resp.status().as_u16();
    let body = resp
        .body_mut()
        .read_to_string()
        .map_err(|e| ApiError::Transport(e.to_string()))?;

    if (200..300).contains(&status) {
        return serde_json::from_str(&body).map_err(|e| ApiError::Decode(e.to_string()));
    }

    let msg = serde_json::from_str::<ErrorBody>(&body)
        .map(|e| e.error)
        .unwrap_or(body);
    Err(match status {
        401 => ApiError::Unauthorized,
        404 => ApiError::NotFound(msg),
        400 => ApiError::BadRequest(msg),
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
            let resp = format!(
                "HTTP/1.1 {status} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
            stream.write_all(resp.as_bytes()).expect("write");
            tx.send(head).expect("send head");
        });
        (format!("http://{addr}/"), rx)
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
        }
    }

    #[test]
    fn sends_bearer_token_and_window() {
        let (url, head) = serve_once(200, GOLDEN_METRICS);
        let client = Client::new(&url, " tok ");
        let from: Month = "2026-05".parse().unwrap();
        let report = client.metrics(&repo(), from, from.next()).expect("metrics");
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
