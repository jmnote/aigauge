# AI Gauge Privacy Policy

AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, and Google Antigravity.

## Data access and use

AI Gauge reads existing local Codex and Claude Code login sessions and invokes the locally installed `agy` command-line tool when Antigravity monitoring is enabled. It uses the returned information only to display quotas, reset times, and connection status locally. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.

## Credentials

AI Gauge does not request or persist passwords, payment information, or authentication credentials in its own storage. It uses access tokens maintained by existing Codex and Claude Code login sessions, and authentication handled by the locally installed `agy` command-line tool for Google Antigravity. Credentials are sent only to the corresponding service endpoints as required and are not logged or uploaded to the developer.

## Third-party services

Requests may be sent directly to OpenAI, Anthropic, or Google Antigravity services to retrieve usage information. Those services process data under their own privacy policies.

## Local storage

AI Gauge stores only local application preferences, including provider visibility and order, refresh interval, theme, thresholds, and window preferences. It does not maintain a remote account, analytics system, or remote database.

## Data retention and deletion

AI Gauge does not retain usage data on a remote server. Local preferences can be removed by uninstalling the app or clearing its local application data. Service credentials remain managed by their respective services or tools.

## Contact

For privacy questions, contact us at <https://github.com/jmnote/aigauge/issues>. Do not include passwords, access tokens, or other sensitive information in public issues.
