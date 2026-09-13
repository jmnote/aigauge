// Import an optional hack/ devDependency, transparently running `npm ci`
// in hack/ once if it isn't installed yet, and throwing a clear error if it
// still can't be found afterward.
import { spawnSync } from "node:child_process";
import { hackDir } from "./paths.mjs";

export async function ensureImport(specifier, purpose) {
  try {
    return await import(specifier);
  } catch {
    const npm = process.platform === "win32" ? "npm.cmd" : "npm";
    try {
      const res = spawnSync(npm, ["ci", "--ignore-scripts", "--no-audit", "--no-fund"], {
        cwd: hackDir,
        stdio: "inherit",
      });
      if (res.status === 0) {
        return await import(specifier);
      }
    } catch { }
    const suffix = purpose ? ` for ${purpose}` : "";
    throw new Error(`The '${specifier}' package is required${suffix}. Run 'npm ci' in the hack/ directory.`);
  }
}
