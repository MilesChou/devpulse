## Context

The server's `requireToken` middleware (`internal/http/routes.go`) disables the bearer check when `DEVPULSE_API_TOKEN` is empty, and `CheckBind` only allows that on a loopback address. The dashboard did not match: `DashboardApp::client()` returned `None` without a token, every API call started with `let Some(client) = self.client() else { return }`, and `save_settings` stopped with a `NoToken` notice. A tokenless local server was therefore unreachable from the dashboard unless the user typed a throwaway token, which was then written to the OS keychain.

## Goals / Non-Goals

**Goals:**

- The dashboard never blocks a connection because the token is empty.
- The server stays the only place that decides whether a token is required, and its 401 reaches the user as a clear message.

**Non-Goals:**

- Changing server authentication, `CheckBind`, or the 401 status.
- Detecting in advance whether a server requires a token.
- Changing how tokens are stored (keychain, one entry per URL) or the `DEVPULSE_API_TOKEN` override.

## Decisions

**`client()` returns `Client`, not `Option<Client>`.** The token is the only thing that made it optional; the base URL always has a default. Returning `Some` unconditionally would leave a dead `else` branch at every call site, so the type changes and the call sites lose their guard. Alternative considered: a sentinel token such as `"-"` when the field is empty. Rejected: it sends a meaningless credential and would be saved to the keychain.

**An empty token sends no `Authorization` header.** `Client::authed` returns the request untouched when the token is empty. Sending `Authorization: Bearer ` with nothing after it happens to pass a tokenless server today, but it is a malformed credential and a reverse proxy in front of the server may reject it.

**Connect on start even without a stored token.** The settings panel still opens, because a first run may need a different URL, but `load_repos` runs too. Against the default loopback URL this makes a tokenless local server work with no input at all. The cost is that a server which requires a token shows a 401 in the repo list on first launch instead of the neutral "Connect to a server in Settings"; the reworded 401 text says a token may be required, so the next step is still obvious.

**"Forget token" reconnects without a token.** It used to reset the repo list to idle. With an optional token there is no "disconnected because tokenless" state left to return to, so it reloads and lets the server answer.

**One message for both 401 causes.** The dashboard cannot tell "wrong token" from "token required" by status alone, and when it sent no token only the second applies. A single text covering both avoids a second `ApiError` variant that would only differ by what the client happened to send.

## Risks / Trade-offs

- [A user forgets the token for a server that requires one and sees a 401 instead of a prompt] → The 401 text names the missing token as a cause, and the token field's hint says it is optional only when the server has none.
- [`Loadable::Idle` for the repo list is now rarely reached, so "Connect to a server in Settings" is mostly dead text] → Left in place: it is still the correct text for the idle state and costs nothing.
- [The 401-with-no-token path is covered by unit tests on `Client`, not by a UI test] → Manual check against a server started with `DEVPULSE_API_TOKEN` set.
