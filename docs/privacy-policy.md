# AI Gauge Privacy Policy

AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, and Google Antigravity.

## Data access and use

For Codex and Claude Code, AI Gauge signs in through each service's own official login flow and stores the resulting access token itself. For Google Antigravity, AI Gauge invokes the locally installed `agy` command-line tool rather than signing in or storing a token of its own. It uses the resulting quota, reset-time, and connection-status information only to display it in the app. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.

## Credentials

AI Gauge does not request or persist passwords or payment information. For Codex and Claude Code, it stores the OAuth access/refresh token from your own sign-in locally on your device (see Local storage), using it only in memory for direct HTTPS requests to the corresponding service; it is not logged or uploaded to the developer. For Google Antigravity, AI Gauge holds no credential of its own: the locally installed `agy` command-line tool manages its own authentication and connection to Google Antigravity, and AI Gauge only reads the quota information `agy` reports back.

## Third-party services

When Codex or Claude Code is enabled, AI Gauge may send a direct HTTPS request to OpenAI or Anthropic to retrieve usage information. OpenAI or Anthropic receives the access token and request data needed to answer its respective request. When Google Antigravity is enabled, AI Gauge invokes the locally installed `agy` command-line tool, which requests usage information from Google Antigravity using the authentication it manages itself; AI Gauge does not read or pass those credentials. Each service processes the request under its own privacy policy.

## Local storage

AI Gauge stores only local application preferences, including provider visibility and order, refresh interval, theme, thresholds, and window preferences. It does not store credentials or usage data as application data, and does not maintain a remote account, analytics system, or remote database.

## Your controls, retention, and deletion

Providers are disabled on a new installation. Enabling a provider initiates its usage request and enables future refreshes at the interval you select. Selecting **Check connection** on the setup screen initiates one request; a successful connection then enables future refreshes. You can disable a provider at any time to stop future requests from AI Gauge. The app retains the most recent usage result only in memory while it is running and does not retain usage data on a remote server.

Local preferences can be removed by uninstalling the app or clearing its local application data. Service credentials remain managed by their respective services or tools.

## Contact

For privacy questions, contact us at <https://github.com/jmnote/aigauge/issues>. Do not include passwords, access tokens, or other sensitive information in public issues.
