// Locate an executable on PATH via Windows' `where.exe`, returning null
// (never throwing) when it isn't found.
import { execFileSync } from "node:child_process";

export function findExecutable(name) {
  try {
    const out = execFileSync("where.exe", [name], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    if (out) return out.split(/\r?\n/)[0].trim();
  } catch { }
  return null;
}
