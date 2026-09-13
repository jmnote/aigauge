// Local credential file locations for each provider CLI, relative to the
// user's home directory. Keep these in sync with the Go-side authoritative
// copies in internal/providers/{claude,codex}.go.
import path from "node:path";

export const CODEX_AUTH_RELATIVE_PATH = path.join(".codex", "auth.json");
export const CLAUDE_CREDENTIALS_RELATIVE_PATH = path.join(".claude", ".credentials.json");
