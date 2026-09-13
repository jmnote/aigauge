import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import child_process from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

async function getResvg() {
  try {
    const mod = await import("@resvg/resvg-js");
    return mod.Resvg;
  } catch {
    const npm = process.platform === "win32" ? "npm.cmd" : "npm";
    try {
      const res = child_process.spawnSync(npm, ["ci", "--ignore-scripts", "--no-audit", "--no-fund"], {
        cwd: __dirname,
        stdio: "inherit",
      });
      if (res.status === 0) {
        const mod = await import("@resvg/resvg-js");
        return mod.Resvg;
      }
    } catch { }
    throw new Error(
      "The '@resvg/resvg-js' package is required for logo and MSIX asset generation. Run 'npm ci' in the hack/ directory."
    );
  }
}

export function resolveVersion(requested) {
  if (!requested || typeof requested !== "string" || !requested.trim()) {
    requested = "0.0.0";
  }
  let val = requested.trim().replace(/^[vV]+/, "");
  if (!/^\d+\.\d+\.\d+(\.\d+)?(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(val)) {
    throw new Error(
      `Version must be semantic version text such as 0.1.1, 0.1.1-beta1, or 0.1.1-beta1+build5. Received: ${requested}`
    );
  }
  val = val.split(/[-+]/)[0];
  const parts = val.split(".");
  while (parts.length < 4) {
    parts.push("0");
  }
  return parts.join(".");
}

export async function convertLogo(options = {}) {
  const source = path.join(repoRoot, "frontend", "logo.svg");
  const output = path.join(repoRoot, "frontend", "logo.png");
  const expectedHash = "85e14c2328a97674fdde7c896155180a24ccdf29b5f4a46d23938d44de649e59";

  const sourceContent = fs.readFileSync(source);
  const actualHash = crypto.createHash("sha256").update(sourceContent).digest("hex").toLowerCase();

  if (!options.force && fs.existsSync(output) && actualHash === expectedHash) {
    console.log("Logo PNG is up to date.");
    return;
  }

  const Resvg = await getResvg();
  const png = new Resvg(sourceContent, { fitTo: { mode: "original" } }).render().asPng();
  fs.writeFileSync(output, png);
  console.log(`Created: ${path.relative(repoRoot, output)}`);
}

function findOnPath(name) {
  try {
    const out = child_process.execFileSync("where.exe", [name], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    if (out) return out.split(/\r?\n/)[0].trim();
  } catch { }
  return null;
}

function findGoWinres() {
  const onPath = findOnPath("go-winres");
  if (onPath) return onPath;

  try {
    const gopath = child_process.execFileSync("go", ["env", "GOPATH"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    if (gopath) {
      const candidate = path.join(gopath, "bin", "go-winres.exe");
      if (fs.existsSync(candidate)) return candidate;
    }
  } catch { }

  throw new Error("go-winres was not found. Install it with: go install github.com/tc-hib/go-winres@v0.3.3");
}

export async function prepareWinres(version = "0.0.0") {
  const numericVersion = resolveVersion(version);
  const winresBin = findGoWinres();
  const iconPath = path.join(repoRoot, "frontend", "logo.png");
  const outPath = path.join(repoRoot, "rsrc");

  if (!fs.existsSync(iconPath)) {
    await convertLogo({ force: true });
  }

  child_process.execFileSync(
    winresBin,
    [
      "simply",
      "--arch", "amd64",
      "--out", outPath,
      "--manifest", "gui",
      "--icon", iconPath,
      "--product-name", "AI Gauge",
      "--file-description", "AI Gauge",
      "--original-filename", "aigauge.exe",
      "--product-version", numericVersion,
      "--file-version", numericVersion,
    ],
    { stdio: "inherit" }
  );
  console.log(`Prepared Windows resources for version ${numericVersion}`);
}

function findMakeAppx(requested) {
  if (requested) {
    if (fs.existsSync(requested)) return path.resolve(requested);
    throw new Error(`makeappx.exe was not found at: ${requested}`);
  }

  const onPath = findOnPath("makeappx.exe");
  if (onPath) return onPath;

  const programFilesX86 = process.env["ProgramFiles(x86)"];
  const programFiles = process.env["ProgramFiles"];
  const roots = [
    programFilesX86 && path.join(programFilesX86, "Windows Kits", "10", "bin"),
    programFiles && path.join(programFiles, "Windows Kits", "10", "bin"),
  ].filter(Boolean);

  const candidates = [];
  for (const root of roots) {
    if (!fs.existsSync(root)) continue;
    try {
      const entries = fs.readdirSync(root, { withFileTypes: true });
      for (const entry of entries) {
        if (!entry.isDirectory()) continue;
        for (const arch of ["x64", "x86"]) {
          const candidate = path.join(root, entry.name, arch, "makeappx.exe");
          if (fs.existsSync(candidate)) {
            candidates.push(candidate);
          }
        }
      }
    } catch { }
  }

  if (candidates.length > 0) {
    candidates.sort().reverse();
    return candidates[0];
  }

  throw new Error(
    "makeappx.exe was not found. Install the Windows 10/11 SDK (App Certification Kit / MSIX tools) or pass --makeappx C:\\path\\makeappx.exe."
  );
}

const MSIX_ASSETS = [
  { name: "Square44x44Logo.png", size: 44 },
  { name: "Square150x150Logo.png", size: 150 },
  { name: "StoreLogo.png", size: 50 },
  { name: "Square310x310Logo.png", size: 310 },
];

async function generateMsixAssets(assetsDir) {
  const svgPath = path.join(repoRoot, "frontend", "logo.svg");
  const svgContent = fs.readFileSync(svgPath);
  const Resvg = await getResvg();

  for (const asset of MSIX_ASSETS) {
    const png = new Resvg(svgContent, {
      fitTo: { mode: "width", value: asset.size },
    }).render().asPng();
    fs.writeFileSync(path.join(assetsDir, asset.name), png);
  }
}

const VALID_ARCHITECTURES = ["x64", "x86", "arm64"];

export async function packageMsix(options = {}) {
  const arch = options.arch || "x64";
  if (!VALID_ARCHITECTURES.includes(arch)) {
    throw new Error(`Invalid architecture '${arch}'. Must be one of: ${VALID_ARCHITECTURES.join(", ")}`);
  }
  const appVersion = options.version || "0.0.0";
  const msixVersion = resolveVersion(appVersion);
  const releaseArtifact = Boolean(options.release);

  const binExe = path.join(repoRoot, "dist", "bin", "aigauge.exe");
  const legacyExe = path.join(repoRoot, "aigauge.exe");
  const sourceExe = fs.existsSync(binExe) ? binExe : (fs.existsSync(legacyExe) ? legacyExe : null);

  if (!sourceExe) {
    throw new Error(`Build output not found: ${binExe}`);
  }

  const dist = path.join(repoRoot, "dist");
  const staging = path.join(dist, "staging", arch);
  const assets = path.join(staging, "Assets");

  // Reset staging directory
  if (fs.existsSync(staging)) {
    fs.rmSync(staging, { recursive: true, force: true });
  }
  fs.mkdirSync(assets, { recursive: true });
  fs.mkdirSync(dist, { recursive: true });

  // Copy binary and license
  fs.copyFileSync(sourceExe, path.join(staging, "aigauge.exe"));
  fs.copyFileSync(path.join(repoRoot, "LICENSE"), path.join(staging, "LICENSE"));

  // Process manifest
  const manifestTemplate = fs.readFileSync(path.join(repoRoot, "Package.appxmanifest"), "utf8");
  const versionReplacement = `$1${msixVersion}$2`;
  let manifest = manifestTemplate.replace(/(<Identity\b.*?\bVersion=")[^"]+(")/gs, versionReplacement);
  if (manifest === manifestTemplate) {
    throw new Error('Package.appxmanifest is missing an <Identity Version="..."> attribute to update.');
  }
  const beforeArchReplace = manifest;
  manifest = manifest.replace(/ProcessorArchitecture="[^"]+"/g, `ProcessorArchitecture="${arch}"`);
  if (manifest === beforeArchReplace) {
    throw new Error('Package.appxmanifest is missing a ProcessorArchitecture attribute to update.');
  }
  fs.writeFileSync(path.join(staging, "AppxManifest.xml"), manifest, "utf8");

  // Generate vector icon assets
  await generateMsixAssets(assets);

  // Pack MSIX
  const makeAppxPath = findMakeAppx(options.makeappx);
  const artifactSuffix = releaseArtifact ? "" : "_local";
  const output = path.join(dist, `aigauge_${msixVersion}_${arch}${artifactSuffix}.msix`);

  if (fs.existsSync(output)) {
    fs.unlinkSync(output);
  }

  child_process.execFileSync(makeAppxPath, ["pack", "/d", staging, "/p", output, "/o"], { stdio: "inherit" });
  console.log(`Created: ${output}`);
  return output;
}

function parseArgs(args) {
  const result = { command: args[0] || "help", flags: {} };
  for (let i = 1; i < args.length; i++) {
    const arg = args[i];
    if (arg.startsWith("--") || arg.startsWith("-")) {
      const key = arg.replace(/^--?/, "");
      const next = args[i + 1];
      if (next && !next.startsWith("-")) {
        result.flags[key] = next;
        i++;
      } else {
        result.flags[key] = true;
      }
    }
  }
  return result;
}

function printUsage() {
  console.log(`Usage: node hack/package.mjs <command> [options]

Commands:
  logo                     Convert frontend/logo.svg to frontend/logo.png
  winres                   Generate Windows PE binary resources (go-winres)
  msix                     Package application as MSIX
  version                  Print normalized 4-part semantic version
  all                      Run logo, winres, and msix sequentially

Options:
  --version <version>      Application semantic version (default: 0.0.0)
  --arch <x64|x86|arm64>   Target architecture for MSIX (default: x64)
  --makeappx <path>        Path to makeappx.exe
  --release                Produce release artifact (no _local suffix)
  --force                  Force regeneration (for logo)
`);
}

async function main() {
  const { command, flags } = parseArgs(process.argv.slice(2));
  const version = flags.version || flags.Version || "0.0.0";
  const arch = flags.arch || flags.Architecture || "x64";
  const makeappx = flags.makeappx || flags.MakeAppx;
  const release = Boolean(flags.release || flags.ReleaseArtifact);
  const force = Boolean(flags.force);

  switch (command) {
    case "logo":
      await convertLogo({ force });
      break;
    case "winres":
      await prepareWinres(version);
      break;
    case "msix":
      await packageMsix({ arch, version, makeappx, release });
      break;
    case "version":
      console.log(resolveVersion(version));
      break;
    case "all":
      await convertLogo({ force });
      await prepareWinres(version);
      await packageMsix({ arch, version, makeappx, release });
      break;
    case "help":
    case "--help":
    case "-h":
      printUsage();
      break;
    default:
      console.error(`Unknown command: ${command}\n`);
      printUsage();
      process.exit(1);
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
  });
}
