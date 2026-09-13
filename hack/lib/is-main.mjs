// CommonJS's `require.main === module` has no direct ESM equivalent; this
// is the standard workaround, centralized so each CLI script doesn't repeat
// its own copy of the comparison.
import path from "node:path";
import { fileURLToPath } from "node:url";

export function isMain(importMetaUrl) {
  return Boolean(process.argv[1]) && fileURLToPath(importMetaUrl) === path.resolve(process.argv[1]);
}
