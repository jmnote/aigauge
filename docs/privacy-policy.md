# AI Gauge Privacy Policy

AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, GitHub Copilot, and Google Antigravity.

## Data access and use

For Codex and Claude Code, AI Gauge signs in through each service's own official login flow and stores the resulting access and refresh tokens locally. It can also import the relevant token from an existing local Codex or Claude Code CLI session into AI Gauge's protected credential store. For GitHub Copilot, AI Gauge signs in through GitHub's OAuth Device Flow and stores the resulting access token in the same protected credential store. For Google Antigravity, AI Gauge invokes the locally installed `agy` command-line tool rather than signing in or storing a token of its own. It uses the resulting quota, reset-time, and connection-status information only to display it in the app. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.

## Credentials

AI Gauge does not request or persist passwords or payment information. For Codex and Claude Code, it stores the OAuth access/refresh token from your own sign-in locally on your device (see Local storage), loads it into memory only when needed for direct HTTPS requests to the corresponding service, and never logs or uploads it to the developer. The same local credential store is used when a token is imported from a CLI session. For GitHub Copilot, it stores the OAuth access token from your device-flow sign-in the same way, loaded into memory only for direct HTTPS requests to GitHub's API. For Google Antigravity, AI Gauge holds no credential of its own: the locally installed `agy` command-line tool manages its own authentication and connection to Google Antigravity, and AI Gauge only reads the quota information `agy` reports back.

## Third-party services

When a connected Codex or Claude Code instance is refreshed, AI Gauge may send a direct HTTPS request to OpenAI or Anthropic to retrieve usage information. OpenAI or Anthropic receives the access token and request data needed to answer its respective request. Browser-based sign-in and Claude's manual code flow also communicate directly with the relevant provider as part of authentication. When a connected GitHub Copilot instance is refreshed, AI Gauge sends a direct HTTPS request to GitHub's API to retrieve usage information, and GitHub's OAuth Device Flow likewise communicates directly with GitHub during sign-in. When an Antigravity instance is refreshed, AI Gauge invokes the locally installed `agy` command-line tool, which requests usage information from Google Antigravity using the authentication it manages itself; AI Gauge does not read or pass those credentials. Each service processes the request under its own privacy policy.

## Local storage

AI Gauge stores local application preferences, including provider instances and order, per-instance refresh intervals, theme, thresholds, window size, the optional global hotkey, and the preferred Windows startup mode. OAuth tokens for Codex, Claude Code, and GitHub Copilot are stored locally under the app's configuration directory: Windows uses DPAPI-protected `credentials.dat`; other platforms use `credentials.json` with owner-only permissions. Usage results are kept in memory only while the app is running. AI Gauge does not maintain a remote account, analytics system, or remote database.

## Your controls, retention, and deletion

On a new installation, providers are configured as instances and are not queried until they have been connected. Selecting **Connect**, **Import**, or **Check connection** starts the relevant provider login, local credential import, or local CLI check; a successful connection enables usage refreshes. Removing a Codex, Claude Code, or GitHub Copilot instance deletes its locally stored OAuth tokens. Removing an Antigravity instance stops monitoring it; the `agy` CLI continues to manage its own authentication. Local preferences can be removed by uninstalling the app or clearing its local application data. AI Gauge does not retain usage data on a remote server.

Starting with Windows is off by default. You can choose **Off**, **Show window**, or **Start in tray**
in Settings. For Microsoft Store installations, the startup task can also be managed in Windows
Settings or Task Manager. If automatic startup is enabled, connected providers may refresh usage after
sign-in using the same local credentials and provider communication described above. Turning startup
off stops automatic launches; it does not disconnect providers or delete their credentials.

## Contact

For privacy questions, contact us at <https://github.com/jmnote/aigauge/issues>. Do not include passwords, access tokens, or other sensitive information in public issues.
