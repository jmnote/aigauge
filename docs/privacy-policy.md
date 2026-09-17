# AI Gauge Privacy Policy

AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, and Google Antigravity.

## Data access and use

For Codex and Claude Code, AI Gauge signs in through each service's own official login flow and stores the resulting access token itself. For Google Antigravity, AI Gauge invokes the locally installed `agy` command-line tool rather than signing in or storing a token of its own. It uses the resulting quota, reset-time, and connection-status information only to display it in the app. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.

## Credentials

AI Gauge does not request or persist passwords or payment information. For Codex and Claude Code, it stores the OAuth access/refresh token from your own sign-in locally on your device (see Local storage), using it only in memory for direct HTTPS requests to the corresponding service; it is not logged or uploaded to the developer. For Google Antigravity, AI Gauge holds no credential of its own: the locally installed `agy` command-line tool manages its own authentication and connection to Google Antigravity, and AI Gauge only reads the quota information `agy` reports back.

## Third-party services

When Codex or Claude Code is enabled, AI Gauge may send a direct HTTPS request to OpenAI or Anthropic to retrieve usage information. OpenAI or Anthropic receives the access token and request data needed to answer its respective request. When Google Antigravity is enabled, AI Gauge invokes the locally installed `agy` command-line tool, which requests usage information from Google Antigravity using the authentication it manages itself; AI Gauge does not read or pass those credentials. Each service processes the request under its own privacy policy.

## Local storage

AI Gauge stores local application preferences, including provider instances and order, refresh intervals, theme, thresholds, and window preferences. OAuth access and refresh tokens for Codex and Claude Code are stored locally under the app's configuration directory: Windows uses DPAPI-protected `credentials.dat`; other platforms use `credentials.json` with owner-only permissions. Usage results are kept in memory only while the app is running. AI Gauge does not maintain a remote account, analytics system, or remote database.

## Your controls, retention, and deletion

On a new installation, providers are configured as instances and are not queried until they have been connected. Selecting **Connect** or **Check connection** starts the relevant login or local CLI check; a successful connection enables usage refreshes. Removing a Codex or Claude Code instance deletes its locally stored OAuth tokens. Removing an Antigravity instance stops monitoring it; the `agy` CLI continues to manage its own authentication. Local preferences can be removed by uninstalling the app or clearing its local application data. AI Gauge does not retain usage data on a remote server.


## Contact

For privacy questions, contact us at <https://github.com/jmnote/aigauge/issues>. Do not include passwords, access tokens, or other sensitive information in public issues.
