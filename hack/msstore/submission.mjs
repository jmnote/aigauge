import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import YAML from 'yaml';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, '..', '..');
const tempDir = path.resolve(__dirname, '..', 'temp');
const msstoreDir = __dirname;

const JSON_OUTPUT_PATH = path.join(tempDir, 'submission.json');
const YAML_OUTPUT_PATH = path.join(msstoreDir, 'submission-sample.yaml');
const APP_ID = process.env.STORE_APP_ID || '9MT65KM56P99';

function loadEnv() {
  const candidates = [
    path.join(__dirname, 'submission.env'),
    path.join(__dirname, 'msstore.env'),
    path.join(__dirname, '..', 'submission.env'),
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
  const firstBrace = raw.indexOf('{');
  const lastBrace = raw.lastIndexOf('}');
  if (firstBrace === -1 || lastBrace === -1 || lastBrace <= firstBrace) {
    console.error('Failed to find JSON payload in msstore output:\n' + raw);
    process.exit(1);
  }

  const rawJsonText = raw.slice(firstBrace, lastBrace + 1).trim() + '\n';
  fs.writeFileSync(JSON_OUTPUT_PATH, rawJsonText, 'utf8');
  console.log(`Saved Store submission JSON: ${JSON_OUTPUT_PATH}`);
}

function submissionYaml() {
  if (!fs.existsSync(JSON_OUTPUT_PATH)) {
    console.error(`Error: ${JSON_OUTPUT_PATH} not found.\nRun '.\\build.ps1 submission-get' first.`);
    process.exit(1);
  }

  if (!fs.existsSync(msstoreDir)) {
    fs.mkdirSync(msstoreDir, { recursive: true });
  }

  const rawJsonText = fs.readFileSync(JSON_OUTPUT_PATH, 'utf8');
  const data = JSON.parse(rawJsonText);

  // Mask sensitive ephemeral SAS token unless --raw is specified
  if (!process.argv.includes('--raw') && data.FileUploadUrl) {
    data.FileUploadUrl = '__REDACTED__';
  }

  const yamlContent = YAML.stringify(data);
  fs.writeFileSync(YAML_OUTPUT_PATH, yamlContent, 'utf8');
  console.log(`Saved Store submission YAML: ${YAML_OUTPUT_PATH}`);
}

function validateSubmission(sourcePath, outputPath) {
  const targetSource = sourcePath || path.join(msstoreDir, 'submission-overrides.yaml');
  if (!fs.existsSync(targetSource)) {
    console.error(`Error: ${targetSource} does not exist.`);
    process.exit(1);
  }

  const content = fs.readFileSync(targetSource, 'utf8');
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
  if (notes !== undefined && (typeof notes !== 'string' || notes.length > 2000)) {
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
        if (releaseNotes !== undefined && (typeof releaseNotes !== 'string' || releaseNotes.length > 1500)) {
          console.error('ReleaseNotes must be a string of at most 1,500 characters.');
          process.exit(1);
        }

        const features = listing.Features;
        if (features !== undefined) {
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
const command = process.argv[2] || 'all';

switch (command) {
  case 'get':
    submissionGet(process.argv[3]);
    break;
  case 'yaml':
    submissionYaml();
    break;
  case 'all':
    submissionGet(process.argv[3]);
    submissionYaml();
    break;
  case 'validate':
    validateSubmission(process.argv[3], process.argv[4]);
    break;
  default:
    console.error(`Unknown command: ${command}\nUsage: node submission.mjs [get|yaml|all|validate]`);
    process.exit(1);
}
