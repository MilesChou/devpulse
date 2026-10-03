## 1. API client

- [x] 1.1 `Client::authed` sends no `Authorization` header when the token is empty
- [x] 1.2 Reword the 401 message (`ApiError::Unauthorized`) to cover a required-but-missing token
- [x] 1.3 Test that a request with an empty token carries no `Authorization` header

## 2. Dashboard app

- [x] 2.1 `DashboardApp::client()` returns `Client`; drop the `let Some(client) = … else { return }` guards
- [x] 2.2 `save_settings` connects with an empty token instead of stopping with `NoToken`
- [x] 2.3 Connect on start when the keychain has no token (settings panel still opens)
- [x] 2.4 "Forget token" reconnects without a token

## 3. Texts

- [x] 3.1 Remove `Notice::NoToken` and the `no_token` text in both languages
- [x] 3.2 Add the "optional" token hint in both languages
- [x] 3.3 Reword the 401 and "connected" texts in both languages

## 4. Documentation

- [x] 4.1 `desktop/README.md` and `desktop/README.zh-TW.md`: token is optional; 401 troubleshooting row
- [x] 4.2 Root `README.md` and `README.zh-TW.md`: Settings needs the URL, and the token only if required

## 5. Verification

- [x] 5.1 `make desktop-lint` and `make desktop-test` pass
- [x] 5.2 The dashboard connects to a local `devpulse serve` that has no `DEVPULSE_API_TOKEN`, with the token field empty
- [x] 5.3 Against a server started with `DEVPULSE_API_TOKEN`, an empty token field shows the 401 message in the UI
