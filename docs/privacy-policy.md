# AI Gauge Privacy Policy

AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, and Google Antigravity.

## Data access and use

AI Gauge reads existing local Codex and Claude Code login sessions and invokes the locally installed `agy` command-line tool when Antigravity monitoring is enabled. It uses the resulting quota, reset-time, and connection-status information only to display it in the app. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.

## Credentials

AI Gauge does not request or persist passwords, payment information, or authentication credentials in its own storage. It uses access tokens maintained by existing Codex and Claude Code login sessions, and authentication handled by the locally installed `agy` command-line tool for Google Antigravity. Codex and Claude Code tokens are used only in memory for direct HTTPS requests to the corresponding service; they are not logged or uploaded to the developer. `agy` manages its own authentication and connection to Google Antigravity.

## Third-party services

When a provider is enabled, AI Gauge may send a request directly to OpenAI, Anthropic, or Google Antigravity to retrieve usage information. Those services receive the authentication information and request data needed to answer that request, and process it under their own privacy policies.

## Local storage

AI Gauge stores only local application preferences, including provider visibility and order, refresh interval, theme, thresholds, and window preferences. It does not store credentials or usage data as application data, and does not maintain a remote account, analytics system, or remote database.

## Your controls, retention, and deletion

Providers are disabled on a new installation. Enabling a provider initiates its usage request and enables future refreshes at the interval you select. Selecting **Check connection** on the setup screen initiates one request; a successful connection then enables future refreshes. You can disable a provider at any time to stop future requests from AI Gauge. The app retains the most recent usage result only in memory while it is running and does not retain usage data on a remote server.

Local preferences can be removed by uninstalling the app or clearing its local application data. Service credentials remain managed by their respective services or tools.

## Contact

For privacy questions, contact us at <https://github.com/jmnote/aigauge/issues>. Do not include passwords, access tokens, or other sensitive information in public issues.
