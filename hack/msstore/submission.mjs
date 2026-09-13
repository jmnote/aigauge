import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { repoRoot, hackDir, tempDir } from '../lib/paths.mjs';
import { ensureImport } from '../lib/ensure-npm.mjs';
import { isMain } from '../lib/is-main.mjs';

const msstoreDir = path.dirname(fileURLToPath(import.meta.url));

async function getYaml() {
  const mod = await ensureImport('yaml');
  return mod.default;
}

const JSON_OUTPUT_PATH = path.join(tempDir, 'submission.json');
const YAML_OUTPUT_PATH = path.join(msstoreDir, 'submission-sample.yaml');
const APP_ID = process.env.STORE_APP_ID || '9MT65KM56P99';

function loadEnv() {
  const candidates = [
    path.join(msstoreDir, 'submission.env'),
    path.join(msstoreDir, 'msstore.env'),
    path.join(hackDir, 'submission.env'),
  ];
  for (const envPath of candidates) {
    if (fs.existsSync(envPath)) {
      try {
        process.loadEnvFile(envPath);
        break;
      } catch (err) {
        console.warn(`Warning: failed to load ${envPath}:`, err.message);
      }
    }
  }
}

function findMsStore() {
  const localApps = path.join(process.env.LOCALAPPDATA || '', 'Microsoft', 'WindowsApps', 'msstore.exe');
  if (fs.existsSync(localApps)) {
    return localApps;
  }
  return 'msstore.exe';
}

function findJsonObjectEnd(text, start) {
  let depth = 0;
  let inString = false;
  let escaped = false;
  for (let i = start; i < text.length; i++) {
    const char = text[i];
    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (char === '\\') {
        escaped = true;
      } else if (char === '"') {
        inString = false;
      }
      continue;
    }
    if (char === '"') {
      inString = true;
      continue;
    }
    if (char === '{') {
      depth++;
    } else if (char === '}') {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function extractSubmissionObject(text) {
  let searchFrom = 0;
  while (true) {
    const start = text.indexOf('{', searchFrom);
    if (start < 0) return null;
    const end = findJsonObjectEnd(text, start);
    if (end < 0) {
      searchFrom = start + 1;
      continue;
    }
    const candidate = text.slice(start, end + 1);
    try {
      const parsed = JSON.parse(candidate);
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        if (parsed.Id || parsed.Listings) {
          return parsed;
        }
      }
    } catch {
      // Not valid JSON - continue scanning from the next brace
    }
    searchFrom = start + 1;
  }
}

function submissionGet(appId = APP_ID) {
  loadEnv();

  if (!fs.existsSync(tempDir)) {
    fs.mkdirSync(tempDir, { recursive: true });
  }

  const msstore = findMsStore();

  const { AZURE_AD_TENANT_ID, SELLER_ID, AZURE_AD_APPLICATION_CLIENT_ID, AZURE_AD_APPLICATION_SECRET } = process.env;
  if (AZURE_AD_TENANT_ID && SELLER_ID && AZURE_AD_APPLICATION_CLIENT_ID && AZURE_AD_APPLICATION_SECRET) {
    const reconfig = spawnSync(
      msstore,
      [
        'reconfigure',
        '--tenantId', AZURE_AD_TENANT_ID,
        '--sellerId', SELLER_ID,
        '--clientId', AZURE_AD_APPLICATION_CLIENT_ID,
        '--clientSecret', AZURE_AD_APPLICATION_SECRET,
      ],
      { stdio: 'inherit' }
    );
    if (reconfig.status !== 0) {
      process.exit(reconfig.status ?? 1);
    }
  }

  const targetAppId = appId && !appId.startsWith('--') ? appId : APP_ID;
  const getRes = spawnSync(msstore, ['submission', 'get', targetAppId], { encoding: 'utf8' });
  if (getRes.status !== 0) {
    if (getRes.stderr) process.stderr.write(getRes.stderr);
    process.exit(getRes.status ?? 1);
  }

  const raw = getRes.stdout || '';
  const submission = extractSubmissionObject(raw);
  if (!submission) {
    console.error('Failed to extract submission JSON payload from msstore output:\n' + raw);
    process.exit(1);
  }

  const rawJsonText = JSON.stringify(submission, null, 2) + '\n';
  fs.writeFileSync(JSON_OUTPUT_PATH, rawJsonText, 'utf8');
  console.log(`Saved Store submission JSON: ${JSON_OUTPUT_PATH}`);
}

async function submissionYaml() {
  if (!fs.existsSync(JSON_OUTPUT_PATH)) {
    console.error(`Error: ${JSON_OUTPUT_PATH} not found.\nRun '.\\build.ps1 submission-get' first.`);
    process.exit(1);
  }

  if (!fs.existsSync(msstoreDir)) {
    fs.mkdirSync(msstoreDir, { recursive: true });
  }

  let data;
  try {
    const rawJsonText = fs.readFileSync(JSON_OUTPUT_PATH, 'utf8');
    data = JSON.parse(rawJsonText);
  } catch (err) {
    console.error(`Failed to parse ${JSON_OUTPUT_PATH}: ${err.message}`);
    process.exit(1);
  }

  // Mask sensitive ephemeral SAS token unless --raw is specified
  if (!process.argv.includes('--raw') && data && data.FileUploadUrl) {
    data.FileUploadUrl = '__REDACTED__';
  }

  const YAML = await getYaml();
  const yamlContent = YAML.stringify(data);
  fs.writeFileSync(YAML_OUTPUT_PATH, yamlContent, 'utf8');
  console.log(`Saved Store submission YAML: ${YAML_OUTPUT_PATH}`);
}

async function validateSubmission(sourcePath, outputPath) {
  const targetSource = sourcePath || path.join(msstoreDir, 'submission-overrides.yaml');
  if (!fs.existsSync(targetSource)) {
    console.error(`Error: ${targetSource} does not exist.`);
    process.exit(1);
  }

  const content = fs.readFileSync(targetSource, 'utf8');
  const YAML = await getYaml();
  let submission;
  try {
    submission = YAML.parse(content);
  } catch (err) {
    console.error(`Invalid YAML in ${targetSource}: ${err.message}`);
    process.exit(1);
  }

  if (!submission || typeof submission !== 'object' || Array.isArray(submission)) {
    console.error('Store submission overrides must be a YAML object.');
    process.exit(1);
  }

  const notes = submission.NotesForCertification;
  if (notes != null && (typeof notes !== 'string' || notes.length > 2000)) {
    console.error('NotesForCertification must be a string of at most 2,000 characters.');
    process.exit(1);
  }

  if ('Listings' in submission) {
    const listings = submission.Listings;
    if (!listings || typeof listings !== 'object' || Array.isArray(listings)) {
      console.error('Listings must be a YAML object.');
      process.exit(1);
    }

    if ('en-us' in listings) {
      const enUs = listings['en-us'];
      if (!enUs || typeof enUs !== 'object' || Array.isArray(enUs)) {
        console.error('Listings.en-us must be a YAML object.');
        process.exit(1);
      }

      const listing = enUs.BaseListing;
      if (listing) {
        if (typeof listing !== 'object' || Array.isArray(listing)) {
          console.error('Listings.en-us.BaseListing must be a YAML object.');
          process.exit(1);
        }

        const releaseNotes = listing.ReleaseNotes;
        if (releaseNotes != null && (typeof releaseNotes !== 'string' || releaseNotes.length > 1500)) {
          console.error('ReleaseNotes must be a string of at most 1,500 characters.');
          process.exit(1);
        }

        const features = listing.Features;
        if (features != null) {
          if (!Array.isArray(features) || features.length > 20) {
            console.error('Features must be a list with at most 20 entries.');
            process.exit(1);
          }
          for (const feature of features) {
            if (typeof feature !== 'string' || feature.length > 200) {
              console.error('Each Features entry must be a string of at most 200 characters.');
              process.exit(1);
            }
          }
        }
      }
    }
  }

  if (outputPath) {
    fs.writeFileSync(outputPath, JSON.stringify(submission), 'utf8');
  }
  console.log(`Validated Store submission overrides: ${targetSource}`);
}

// CLI entrypoint
async function main() {
  const command = process.argv[2] || 'all';

  switch (command) {
    case 'get':
      submissionGet(process.argv[3]);
      break;
    case 'yaml':
      await submissionYaml();
      break;
    case 'all':
      submissionGet(process.argv[3]);
      await submissionYaml();
      break;
    case 'validate':
      await validateSubmission(process.argv[3], process.argv[4]);
      break;
    default:
      console.error(`Unknown command: ${command}\nUsage: node submission.mjs [get|yaml|all|validate]`);
      process.exit(1);
  }
}

if (isMain(import.meta.url)) {
  main().catch((err) => {
    console.error(err.message || err);
    process.exit(1);
  });
}
