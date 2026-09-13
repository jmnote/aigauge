import fs from "node:fs";
import path from "node:path";
import os from "node:os";
import child_process from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const manifestDir = path.join(__dirname, "temp");
const manifestPath = path.join(manifestDir, ".ai-credentials-backup.local.json");
const legacyManifestPath = path.join(repoRoot, ".ai-credentials-backup.local.json");

const VALID_PROVIDERS = ["all", "antigravity", "claude", "codex"];

function normalizeProvider(provider) {
  const p = (provider || "all").toLowerCase();
  const normalized = p === "agy" ? "antigravity" : p;
  if (!VALID_PROVIDERS.includes(normalized)) {
    console.error(`Unknown provider '${provider}'. Valid providers: ${VALID_PROVIDERS.join(", ")}, agy`);
    process.exit(1);
  }
  return normalized;
}

function getProviderFromLabel(label) {
  if (/claude/i.test(label)) return "claude";
  if (/codex/i.test(label)) return "codex";
  if (/agy|antigravity/i.test(label)) return "antigravity";
  return "unknown";
}

function findExecutable(name) {
  try {
    const out = child_process.execFileSync("where.exe", [name], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    if (out) return out.split(/\r?\n/)[0].trim();
  } catch { }
  return null;
}

function getAiBackupTargets(targetProvider = "all") {
  const targets = [];
  const p = normalizeProvider(targetProvider);

  if (p === "all" || p === "claude") {
    targets.push({
      provider: "claude",
      label: "Claude credentials",
      livePath: path.join(os.homedir(), ".claude", ".credentials.json"),
    });
    const claudeBin = findExecutable("claude");
    if (claudeBin) {
      targets.push({
        provider: "claude",
        label: "claude executable",
        livePath: claudeBin,
      });
    }
  }

  if (p === "all" || p === "codex") {
    targets.push({
      provider: "codex",
      label: "Codex credentials",
      livePath: path.join(os.homedir(), ".codex", "auth.json"),
    });
    const codexBin = findExecutable("codex");
    if (codexBin) {
      targets.push({
        provider: "codex",
        label: "codex executable",
        livePath: codexBin,
      });
    }
  }

  if (p === "all" || p === "antigravity") {
    const agyBin = findExecutable("agy");
    if (agyBin) {
      targets.push({
        provider: "antigravity",
        label: "agy executable",
        livePath: agyBin,
      });
    }
    const localAppData = process.env.LOCALAPPDATA || "";
    if (localAppData) {
      const agyFallback = path.join(localAppData, "agy", "bin", "agy.exe");
      if (fs.existsSync(agyFallback) && !targets.some((t) => t.livePath.toLowerCase() === agyFallback.toLowerCase())) {
        targets.push({
          provider: "antigravity",
          label: "agy executable (fallback path)",
          livePath: agyFallback,
        });
      }
    }
  }

  return targets;
}

function normalizeManifestItem(item) {
  if (!item || typeof item !== "object") return null;
  const livePath = item.livePath || item.LivePath;
  const backupPath = item.backupPath || item.BackupPath;
  if (!livePath || !backupPath) return null;
  const label = item.label || item.Label || path.basename(livePath);
  const provider = item.provider || item.Provider || getProviderFromLabel(label);
  return {
    provider,
    label,
    livePath,
    backupPath,
  };
}

function loadManifest() {
  let raw = [];
  const activePath = fs.existsSync(manifestPath) ? manifestPath : (fs.existsSync(legacyManifestPath) ? legacyManifestPath : null);
  if (activePath) {
    try {
      raw = JSON.parse(fs.readFileSync(activePath, "utf8"));
    } catch (err) {
      console.warn(`Warning: could not parse backup manifest at ${activePath} (${err.message}).`);
      console.warn("Treating it as empty - if credentials still seem backed up, check for stray *.bak files and restore them manually.");
      raw = [];
    }
  }
  if (!Array.isArray(raw)) return [];
  return raw.map(normalizeManifestItem).filter(Boolean);
}

function saveManifest(items) {
  if (items.length === 0) {
    if (fs.existsSync(manifestPath)) {
      fs.unlinkSync(manifestPath);
    }
    if (fs.existsSync(legacyManifestPath)) {
      fs.unlinkSync(legacyManifestPath);
    }
    if (fs.existsSync(manifestDir)) {
      try {
        const remaining = fs.readdirSync(manifestDir);
        if (remaining.length === 0) {
          fs.rmdirSync(manifestDir);
        }
      } catch { }
    }
    return;
  }

  if (!fs.existsSync(manifestDir)) {
    fs.mkdirSync(manifestDir, { recursive: true });
  }
  fs.writeFileSync(manifestPath, JSON.stringify(items, null, 2), "utf8");
}

export function backup(targetProvider = "all") {
  const p = normalizeProvider(targetProvider);
  const existingMoved = loadManifest();
  const targets = getAiBackupTargets(p);
  const newMoved = [];

  for (const target of targets) {
    const livePath = target.livePath;
    const backupPath = `${livePath}.bak`;

    const alreadyTracked = existingMoved.some(
      (m) =>
        (m.backupPath && m.backupPath.toLowerCase() === backupPath.toLowerCase()) ||
        (m.livePath && m.livePath.toLowerCase() === livePath.toLowerCase())
    );
    if (alreadyTracked) continue;

    if (!fs.existsSync(livePath)) continue;

    if (fs.existsSync(backupPath)) {
      console.warn(`Skipping ${target.label}: ${backupPath} already exists. Not touching it.`);
      continue;
    }

    try {
      fs.renameSync(livePath, backupPath);
      console.log(`Backed up: ${target.label} -> ${backupPath}`);
      newMoved.push({
        provider: target.provider,
        label: target.label,
        livePath,
        backupPath,
      });
    } catch (err) {
      console.warn(`Could not back up ${target.label} (${livePath}): ${err.message}. Still in use by a running process?`);
    }
  }

  if (newMoved.length === 0) {
    const matchingExisting = existingMoved.filter((item) => {
      const prov = item.provider || getProviderFromLabel(item.label);
      return p === "all" || prov === p;
    });
    if (matchingExisting.length > 0) {
      const provLabel = p === "all" ? "All requested providers" : `Provider '${p}'`;
      console.log(`${provLabel} are already backed up (${matchingExisting.length} item(s) in manifest). Run '.\\build.ps1 ai-restore' when done testing.`);
    } else {
      console.log(`Nothing to back up for '${p}' - no live credential or executable files were found.`);
    }
    return;
  }

  const allMoved = [...existingMoved, ...newMoved];
  saveManifest(allMoved);
  console.log("");
  const restoreHint = p === "all" ? ".\\build.ps1 ai-restore" : `.\\build.ps1 ai-restore ${p}`;
  console.log(`Backed up ${newMoved.length} new item(s) for '${p}' (${allMoved.length} total item(s) currently backed up). Run '${restoreHint}' when done testing.`);
}

export function restore(targetProvider = "all") {
  const p = normalizeProvider(targetProvider);
  const allMoved = loadManifest();

  if (allMoved.length === 0) {
    console.log(`Nothing to restore - no backup manifest found at ${manifestPath}.`);
    return;
  }

  const toRestore = [];
  const remaining = [];
  for (const item of allMoved) {
    const prov = item.provider || getProviderFromLabel(item.label);
    if (p === "all" || prov === p) {
      toRestore.push(item);
    } else {
      remaining.push(item);
    }
  }

  if (toRestore.length === 0) {
    console.log(`Nothing to restore - no backed-up items found for '${p}' in ${manifestPath}.`);
    return;
  }

  let allRestored = true;
  const notRestored = [];

  for (const item of toRestore) {
    if (!item.livePath || !item.backupPath) {
      console.warn(`Skipping invalid backup item: ${item.label}`);
      continue;
    }

    const liveExists = fs.existsSync(item.livePath);
    const backupExists = fs.existsSync(item.backupPath);

    if (liveExists && !backupExists) {
      console.log(`Already restored: ${item.label} -> ${item.livePath}`);
      continue;
    }

    if (!backupExists) {
      console.warn(`Skipping ${item.label}: backup file ${item.backupPath} is gone.`);
      continue;
    }

    if (liveExists && backupExists) {
      console.warn(`Skipping ${item.label}: ${item.livePath} already exists again. Resolve manually - both it and ${item.backupPath} are left in place.`);
      allRestored = false;
      notRestored.push(item);
      continue;
    }

    try {
      fs.renameSync(item.backupPath, item.livePath);
      console.log(`Restored: ${item.label} -> ${item.livePath}`);
    } catch (err) {
      console.warn(`Could not restore ${item.label} (${item.backupPath}): ${err.message}`);
      allRestored = false;
      notRestored.push(item);
    }
  }

  const finalRemaining = [...remaining, ...notRestored];
  saveManifest(finalRemaining);
  if (!allRestored) {
    console.warn(`Some items were not restored; kept in ${manifestPath} so a re-run of 'ai-restore' can retry them.`);
  }
}

function parseArgs(args) {
  let action = "backup";
  let provider = "all";

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === "restore" || arg === "--restore" || arg === "-Restore") {
      action = "restore";
    } else if (arg === "backup" || arg === "--backup") {
      action = "backup";
    } else if (arg === "--provider" || arg === "-Provider") {
      provider = args[++i] || "all";
    } else if (!arg.startsWith("-")) {
      if (["restore", "backup"].includes(arg)) {
        action = arg;
      } else {
        provider = arg;
      }
    }
  }

  return { action, provider };
}

function main() {
  const { action, provider } = parseArgs(process.argv.slice(2));
  if (action === "restore") {
    restore(provider);
  } else {
    backup(provider);
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  main();
}

