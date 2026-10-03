## MODIFIED Requirements

### Requirement: Connect with a server URL and an API token only

The user MUST be able to use the dashboard by entering the DevPulse server URL and, when the server requires one, its API token, without installing a database driver or handing the dashboard any GitHub or CI token. The dashboard MUST NOT refuse to connect because no token was entered; whether a token is required is the server's decision.

#### Scenario: First launch

- **WHEN** the user starts the dashboard with no stored token
- **THEN** the settings panel opens with the server URL and the optional API token, and the dashboard connects to the server without a token

#### Scenario: Server without a token

- **WHEN** the user connects with the token field empty to a server that has no `DEVPULSE_API_TOKEN`
- **THEN** the dashboard sends no `Authorization` header and loads the repos

#### Scenario: Server requires a token

- **WHEN** the user connects with the token field empty to a server that requires a token
- **THEN** the dashboard shows the server's 401 as a message saying the token was rejected or is required

#### Scenario: Connection test

- **WHEN** the user presses Test
- **THEN** the dashboard reports whether the server is reachable and whether it accepts the connection, as separate failure messages
