# GitHub OAuth App

Registration values for AI Gauge's GitHub OAuth App, used by the Copilot
provider's device flow (see [internal/auth/types.go](../../internal/auth/types.go)).

| Field | Value |
|---|---|
| Application name | `AI Gauge` |
| Homepage URL | `https://github.com/jmnote/aigauge` |
| Application description | Windows tray widget that shows your GitHub Copilot request quota and credit usage at a glance. Reads only your Copilot usage/quota data via the device flow — never stores your GitHub password and requests no repository access. |
| Redirect URI | `http://127.0.0.1:1456/auth/callback` |
| Allow wildcard matching | ❌ |
| Enable Device Flow | ✅ |
| Expire user access tokens | ❌ |
| Client ID | `Ov23li770eIXd5MyhFer` |

## Why wildcard matching and token expiry are unchecked

- **Allow wildcard matching**: there is only one exact Redirect URI above, so
  wildcard matching would only widen the accepted callback URLs without any
  benefit.
- **Expire user access tokens**: refreshing an expiring token requires
  `client_secret` on GitHub's refresh endpoint, but the `copilot`
  `ProviderConfig` in [internal/auth/types.go](../../internal/auth/types.go)
  has no `ClientSecret` — AI Gauge is a public desktop app and deliberately
  never embeds one (this is also why it uses Device Flow instead of the
  redirect-based grant). Enabling this would make tokens expire every few
  hours with no way for AI Gauge to refresh them, forcing frequent
  re-authentication.
