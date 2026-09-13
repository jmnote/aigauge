#!/usr/bin/env node

// Command raw captures an unconverted usage response for fixture development.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { CODEX_AUTH_RELATIVE_PATH, CLAUDE_CREDENTIALS_RELATIVE_PATH } from '../lib/credential-paths.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const VERSION_PATTERN = /\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?/;
const UNSAFE_FILENAME_CHAR = /[^A-Za-z0-9.-]+/g;
const TIMEOUT_MS = 15000;

function fatalf(msg) {
  console.error(`raw: ${msg}`);
  process.exit(1);
}

function commandOutput(name, args = []) {
  const res = spawnSync(name, args, { encoding: 'utf8' });
  if (res.error) {
    throw new Error(`${name}: ${res.error.message}`);
  }
  if (res.status !== 0) {
    const err = res.stderr && res.stderr.trim() ? res.stderr.trim() : `exit code ${res.status}`;
    throw new Error(`${name}: ${err}`);
  }
  return res.stdout;
}

function cliVersion(name) {
  const output = commandOutput(name, ['--version']);
  const match = output.match(VERSION_PATTERN);
  if (!match) {
    throw new Error(`could not determine ${name} version from "${output.trim()}"`);
  }
  return match[0];
}

function readCredentials(relativePath) {
  const fullPath = path.join(os.homedir(), relativePath);
  if (!fs.existsSync(fullPath)) {
    throw new Error(`read ${fullPath}: file not found`);
  }
  const content = fs.readFileSync(fullPath, 'utf8');
  try {
    return JSON.parse(content);
  } catch (err) {
    throw new Error(`parse ${fullPath}: ${err.message}`);
  }
}

async function fetchJSON(url, headers) {
  const res = await fetch(url, {
    headers,
    signal: AbortSignal.timeout(TIMEOUT_MS),
  });
  if (!res.ok) {
    throw new Error(`usage request failed: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

async function fetchCodex() {
  const creds = readCredentials(CODEX_AUTH_RELATIVE_PATH);
  const token = creds?.tokens?.access_token;
  if (!token) {
    throw new Error('Codex credentials contain no access token');
  }
  return fetchJSON('https://chatgpt.com/backend-api/wham/usage', {
    Authorization: `Bearer ${token}`,
  });
}

async function fetchClaude() {
  const creds = readCredentials(CLAUDE_CREDENTIALS_RELATIVE_PATH);
  const token = creds?.claudeAiOauth?.accessToken;
  if (!token) {
    throw new Error('Claude credentials contain no access token');
  }
  return fetchJSON('https://api.anthropic.com/api/oauth/usage', {
    Authorization: `Bearer ${token}`,
    'anthropic-beta': 'oauth-2025-04-20',
  });
}

function fetchAntigravity() {
  const raw = commandOutput('agy', ['-p', '/usage', '--output-format', 'json']);
  try {
    return JSON.parse(raw);
  } catch (err) {
    throw new Error(`invalid JSON response: ${err.message}`);
  }
}

function formatDate(date = new Date()) {
  return date.toISOString().slice(0, 10);
}

function codexPlanType(data) {
  const plan = data?.plan_type;
  if (!plan) return 'unknown';
  const sanitized = plan.replace(UNSAFE_FILENAME_CHAR, '-');
  return sanitized || 'unknown';
}

function redactCodex(data) {
  if (!('user_id' in data) || !('email' in data)) {
    throw new Error('Codex response contains no user_id or email field to redact');
  }
  data.user_id = 'user_abc';
  data.email = 'user@example.com';
}

const PROVIDERS = {
  codex: { cli: 'codex', fetch: fetchCodex },
  claude: { cli: 'claude', fetch: fetchClaude },
  antigravity: { cli: 'agy', fetch: fetchAntigravity },
};

async function capture(provider) {
  const config = PROVIDERS[provider];
  if (!config) {
    throw new Error(`unknown provider "${provider}"`);
  }

  const version = cliVersion(config.cli);
  const data = await config.fetch();

  let filename;
  const dateStr = formatDate();
  if (provider === 'codex') {
    redactCodex(data);
    const planType = codexPlanType(data);
    filename = `${provider}-${version}_${planType}_${dateStr}.json`;
  } else {
    filename = `${provider}-${version}_${dateStr}.json`;
  }

  const outputDir = path.join(__dirname, 'raw');
  if (!fs.existsSync(outputDir)) {
    fs.mkdirSync(outputDir, { recursive: true });
  }

  const outputPath = path.join(outputDir, filename);
  fs.writeFileSync(outputPath, JSON.stringify(data, null, 2) + '\n', 'utf8');
  console.log(`Wrote ${outputPath}`);
}

async function main() {
  const target = process.argv[2];
  const validProviders = ['codex', 'claude', 'antigravity', 'all'];

  if (!target || !validProviders.includes(target) || process.argv.length !== 3) {
    fatalf('usage: node ./hack/fixtures/raw.mjs <codex|claude|antigravity|all>');
  }

  const providers = target === 'all' ? ['codex', 'claude', 'antigravity'] : [target];

  for (const provider of providers) {
    try {
      await capture(provider);
    } catch (err) {
      fatalf(`${provider}: ${err.message}`);
    }
  }
}

main();
