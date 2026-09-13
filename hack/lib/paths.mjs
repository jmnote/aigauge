// Shared path constants for hack/ tooling scripts, computed once here so
// every script agrees on where the repo root and hack/temp live instead of
// each re-deriving them from its own __dirname.
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const hackDir = path.resolve(__dirname, "..");
export const repoRoot = path.resolve(hackDir, "..");
export const tempDir = path.join(hackDir, "temp");
