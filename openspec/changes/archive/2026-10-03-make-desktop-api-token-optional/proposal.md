## Why

`devpulse serve` runs without `DEVPULSE_API_TOKEN` on a loopback address, which is how the tool is tried out locally. The desktop dashboard still refused to connect until a token was typed, so a first run against a tokenless server could not add a repo: the Repos page only said "Connect to a server in Settings". Whether a token is needed is the server's decision, not the dashboard's.

## What Changes

- The dashboard connects whenever it has a server URL. An empty token field is no longer an error.
- With no token, requests carry no `Authorization` header (previously an empty `Bearer ` value would have been sent).
- On start with no stored token, the settings panel still opens, and the dashboard also connects straight away.
- Removing the stored token reconnects without one instead of dropping to the disconnected state.
- A 401 now reads "The server rejected the API token, or requires one (401)", since it also covers a server that wants a token when none was sent.
- The "No API token for this server." notice is removed.
- No server change: `requireToken` already skips the check when no token is configured and answers 401 otherwise.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `desktop-dashboard`: the "Connect with a server URL and an API token only" requirement makes the token optional and leaves the decision to the server.

## Impact

- `desktop/src/api.rs`: `Client` omits the `Authorization` header for an empty token; 401 wording.
- `desktop/src/app.rs`: `client()` always returns a client; the no-token early returns go away.
- `desktop/src/notice.rs`, `desktop/src/i18n.rs`: `NoToken` removed, an "optional" hint added, 401 and "connected" texts reworded in both languages.
- `README.md`, `README.zh-TW.md`, `desktop/README.md`, `desktop/README.zh-TW.md`: the token is described as optional.
- HTTP API, CLI, database: unchanged.
