// Regression tests for the rules in logic.mjs. Run with `.\build.ps1 test`
// (or `node --test frontend`). No test framework and no browser stand-in: these
// are the decisions that can be stated as plain functions, which is exactly why
// they were pulled out of app.js.

import test from 'node:test';
import assert from 'node:assert/strict';

import {
  DEFAULT_REFRESH_SECONDS,
  DEFAULT_WINDOW_WIDTH,
  MAX_REFRESH_SECONDS,
  MAX_RETRY_DELAY_SECONDS,
  MIN_REFRESH_SECONDS,
  PROVIDER_REFRESH_OPTIONS,
  PROVIDER_TYPE_IDS,
  STATUS_BADGES,
  badgeClass,
  formatHotkeyError,
  formatRefreshOption,
  hotkeyOptionLabel,
  normalizeConfig,
  normalizeProviderInstance,
  normalizeProviders,
  normalizeThreshold,
  normalizeWindowWidth,
  parseIntervalToSeconds,
  providerTypeLabel,
  providerVisibilityAction,
  retryDelay,
  shouldCountFailure,
  shouldShowProviderUser,
  shouldKeepStaleData,
  shouldScheduleRetry,
} from './logic.mjs';

const defaultConfig = {
  providers: [],
  windowWidth: DEFAULT_WINDOW_WIDTH,
  theme: 'system',
  hotkeyShortcut: '',
  thresholds: { warning: { enabled: true, value: 30 }, critical: { enabled: true, value: 10 } },
};

test('fresh install config has no provider instances', () => {
  const fresh = normalizeConfig(defaultConfig, defaultConfig);
  assert.deepEqual(fresh.providers, []);
  assert.equal(fresh.hotkeyShortcut, '');
});

test('hotkey settings normalize to the supported choices', () => {
  const primary = normalizeConfig({ hotkeyShortcut: 'Ctrl+Shift+G' }, defaultConfig);
  assert.equal(primary.hotkeyShortcut, 'Ctrl+Shift+G');

  for (const shortcut of ['Ctrl+Shift+Q', 'Ctrl+Shift+E']) {
    const selected = normalizeConfig({ hotkeyShortcut: shortcut }, defaultConfig);
    assert.equal(selected.hotkeyShortcut, shortcut);
  }

  const invalid = normalizeConfig({ hotkeyShortcut: 'Ctrl+Alt+X' }, defaultConfig);
  assert.equal(invalid.hotkeyShortcut, '');
});

test('formatHotkeyError formats messages with informative fallback', () => {
  assert.equal(formatHotkeyError('failed to register global shortcut: hotkey already registered'), 'Registration failed: failed to register global shortcut: hotkey already registered');
  assert.equal(formatHotkeyError('The hotkey is already registered'), 'Registration failed: The hotkey is already registered');
  assert.equal(formatHotkeyError('Unable to claim shortcut'), 'Registration failed: Unable to claim shortcut');
  assert.equal(formatHotkeyError('Access is denied'), 'Registration failed: Access is denied');
  assert.equal(formatHotkeyError(new Error('Access is denied')), 'Registration failed: Access is denied');
  assert.equal(formatHotkeyError(''), 'Registration failed');
  assert.equal(formatHotkeyError(null), 'Registration failed');
  assert.equal(formatHotkeyError('Access is denied', false), 'Unregistration failed: Access is denied');
  assert.equal(formatHotkeyError('', false), 'Unregistration failed');
});

test('hotkeyOptionLabel describes configured and disabled shortcuts', () => {
  assert.equal(hotkeyOptionLabel(null), 'Disabled');
  assert.equal(hotkeyOptionLabel('Ctrl+Shift+G'), 'Ctrl + Shift + G');
  assert.equal(hotkeyOptionLabel('Ctrl+Alt+X'), 'Ctrl+Alt+X');
});

test('the first hotkey option is the default', () => {
  const normalized = normalizeConfig({}, defaultConfig);
  assert.equal(normalized.hotkeyShortcut, '');
});

test('normalizeConfig falls back to DEFAULT_CONFIG when defaultConfig is omitted', () => {
  const normalized = normalizeConfig({});
  assert.equal(normalized.hotkeyShortcut, '');
  assert.equal(normalized.theme, 'system');
  assert.deepEqual(normalized.providers, []);
  assert.equal(normalized.thresholds.warning.value, 50);
  assert.equal(normalized.thresholds.critical.value, 20);
});

test('window width is restored within the supported range', () => {
  assert.equal(normalizeWindowWidth(320), 320);
  assert.equal(normalizeWindowWidth(100), 200);
  assert.equal(normalizeWindowWidth(900), 600);
  assert.equal(normalizeWindowWidth('invalid'), DEFAULT_WINDOW_WIDTH);
});

const EXPECTED = ['auth_check_required', 'login_required', 'authenticating', 'awaiting_code', 'not_installed'];
const FAILURES = ['temporary_error', 'usage_unavailable'];
const RETRIED = ['temporary_error', 'usage_unavailable'];

// --- the rule the certification failure came down to -----------------------

test('an expected setup state is never counted as a failure', () => {
  for (const status of EXPECTED) {
    assert.equal(shouldCountFailure(status), false, status);
  }
});

test('a real failure is counted', () => {
  for (const status of FAILURES) {
    assert.equal(shouldCountFailure(status), true, status);
  }
});

test('a connected provider and an absent status are not failures', () => {
  assert.equal(shouldCountFailure('connected'), false);
  assert.equal(shouldCountFailure(''), false);
  assert.equal(shouldCountFailure(undefined), false);
});

test('an expected setup state schedules no automatic retry', () => {
  for (const status of EXPECTED) {
    assert.equal(shouldScheduleRetry(status), false, status);
  }
});

test('unsupported CLI is blocked but does not auto-retry', () => {
  assert.equal(shouldCountFailure('unsupported_cli'), true);
  assert.equal(shouldScheduleRetry('unsupported_cli'), false);
  assert.equal(badgeClass('unsupported_cli'), 'is-blocked');
});

test('a recoverable state keeps its automatic retry', () => {
  for (const status of [...RETRIED, 'connected', '']) {
    assert.equal(shouldScheduleRetry(status), true, status);
  }
});

// --- stale data on a temporary failure -------------------------------------

test('a temporary failure keeps previously fetched usage on screen', () => {
  assert.equal(shouldKeepStaleData('temporary_error', 1_700_000_000_000), true);
});

test('a temporary failure with nothing fetched yet has nothing to keep', () => {
  assert.equal(shouldKeepStaleData('temporary_error', 0), false);
});

test('states other than a temporary failure never keep stale usage', () => {
  for (const status of [...EXPECTED, 'usage_unavailable', 'unsupported_cli', 'connected']) {
    assert.equal(shouldKeepStaleData(status, 1_700_000_000_000), false, status);
  }
});

// --- backoff ---------------------------------------------------------------

test('retry delay backs off from the refresh interval and stays capped', () => {
  assert.equal(retryDelay(0, 60), 60);
  assert.equal(retryDelay(1, 60), 120);
  assert.equal(retryDelay(4, 60), 960);
  assert.equal(retryDelay(99, 60), 960, 'the exponent stops growing after 4 failures');
  assert.equal(retryDelay(4, 1800), MAX_RETRY_DELAY_SECONDS, 'and the delay itself is capped');
});

test('visibility changes preserve an existing live refresh timer', () => {
  assert.equal(providerVisibilityAction(true), 'preserve');
  assert.equal(providerVisibilityAction(false), 'fetch');
});

// --- corrupted or foreign settings -----------------------------------------

test('a refresh interval is read from every shape a settings file may hold', () => {
  assert.equal(parseIntervalToSeconds(90), 90);
  assert.equal(parseIntervalToSeconds('90'), 90);
  assert.equal(parseIntervalToSeconds('2m'), 120);
  assert.equal(parseIntervalToSeconds('1m30s'), 90);
  assert.equal(parseIntervalToSeconds(' 45s '), 45);
});

test('an unreadable refresh interval falls back instead of yielding NaN', () => {
  for (const value of [undefined, null, '', 'soon', {}, [], NaN, Infinity]) {
    const seconds = parseIntervalToSeconds(value);
    assert.equal(Number.isFinite(seconds), true, `${String(value)} produced ${seconds}`);
    assert.equal(seconds, DEFAULT_REFRESH_SECONDS, String(value));
  }
});

test('a refresh interval is clamped into range', () => {
  assert.equal(parseIntervalToSeconds(0), MIN_REFRESH_SECONDS);
  assert.equal(parseIntervalToSeconds(-5), MIN_REFRESH_SECONDS);
  assert.equal(parseIntervalToSeconds(99999), MAX_REFRESH_SECONDS);
});

test('refresh interval options format seconds and minutes correctly', () => {
  assert.equal(formatRefreshOption(30), '30s');
  assert.equal(formatRefreshOption(60), '1m');
  assert.equal(formatRefreshOption(180), '3m');
  assert.ok(PROVIDER_REFRESH_OPTIONS.includes(30));
});

test('a provider instance needs a valid id and a recognized type', () => {
  assert.deepEqual(
    normalizeProviderInstance({ id: 'abc123', type: 'claude' }),
    { id: 'abc123', type: 'claude', label: 'Claude', refreshInterval: DEFAULT_REFRESH_SECONDS },
  );
  assert.deepEqual(
    normalizeProviderInstance({ id: 'abc123', type: 'claude', refreshInterval: 30 }),
    { id: 'abc123', type: 'claude', label: 'Claude', refreshInterval: 30 },
  );
  assert.equal(normalizeProviderInstance({ id: 'abc123', type: 'gemini' }), null, 'unknown type');
  assert.equal(normalizeProviderInstance({ type: 'claude' }), null, 'missing id');
  assert.equal(normalizeProviderInstance(null), null);
  assert.equal(normalizeProviderInstance('claude'), null);
});

test('a blank or missing label falls back to the provider type name', () => {
  assert.equal(normalizeProviderInstance({ id: 'a', type: 'codex', label: '' }).label, 'Codex');
  assert.equal(normalizeProviderInstance({ id: 'a', type: 'codex', label: '   ' }).label, 'Codex');
});

test('providerTypeLabel names every known type and echoes back an unknown one', () => {
  assert.equal(providerTypeLabel('codex'), 'Codex');
  assert.equal(providerTypeLabel('claude'), 'Claude');
  assert.equal(providerTypeLabel('antigravity'), 'Antigravity');
  assert.equal(providerTypeLabel('copilot'), 'GitHub Copilot');
  assert.equal(PROVIDER_TYPE_IDS.length, 4);
  assert.equal(providerTypeLabel('gemini'), 'gemini');
});

test('account identifiers appear only when the same supported provider is registered more than once', () => {
  const providers = [
    { id: 'c1', type: 'codex' },
    { id: 'c2', type: 'codex' },
    { id: 'a1', type: 'antigravity' },
  ];
  assert.equal(shouldShowProviderUser(providers, 'codex', 'alex'), true);
  assert.equal(shouldShowProviderUser(providers, 'claude', 'ea24'), false);
  assert.equal(shouldShowProviderUser(providers, 'codex', ''), false);
  assert.equal(shouldShowProviderUser([...providers, { id: 'a2', type: 'antigravity' }], 'antigravity', 'abcd'), false);
});

test('a provider list keeps order, drops bad entries, and de-duplicates by id', () => {
  assert.deepEqual(
    normalizeProviders([
      { id: 'c1', type: 'claude' },
      { id: 'bad', type: 'gemini' },
      { id: 'x1', type: 'codex' },
      { id: 'c1', type: 'claude', label: 'duplicate id, ignored' },
    ]),
    [
      { id: 'c1', type: 'claude', label: 'Claude', refreshInterval: DEFAULT_REFRESH_SECONDS },
      { id: 'x1', type: 'codex', label: 'Codex', refreshInterval: DEFAULT_REFRESH_SECONDS },
    ],
  );
  assert.deepEqual(normalizeProviders('not an array'), []);
  assert.deepEqual(normalizeProviders(undefined), []);
});

test('thresholds clamp numeric values and default unreadable values', () => {
  assert.deepEqual(normalizeThreshold(150, { enabled: true, value: 30 }), { enabled: true, value: 100 });
  assert.deepEqual(normalizeThreshold('', { enabled: true, value: 30 }, 1, 100), { enabled: true, value: 30 });
  assert.deepEqual(normalizeThreshold({ enabled: false, value: 42 }, { enabled: true, value: 30 }, 1, 100),
    { enabled: false, value: 40 });
});

test('a corrupted config normalizes into a complete, usable one', () => {
  for (const stored of [null, undefined, 42, 'nonsense', [], { providers: 'no' }, { thresholds: null }]) {
    const config = normalizeConfig(stored, defaultConfig);
    assert.deepEqual(config.providers, [], String(stored));
    assert.equal(['light', 'dark', 'system'].includes(config.theme), true, String(stored));
    assert.equal(Number.isFinite(config.thresholds.warning.value), true, String(stored));
    assert.equal(Number.isFinite(config.thresholds.critical.value), true, String(stored));
  }
});

test('the legacy "auto" theme is carried over to "system"', () => {
  assert.equal(normalizeConfig({ theme: 'auto' }, defaultConfig).theme, 'system');
});

test('critical threshold is normalized below warning threshold', () => {
  const config = normalizeConfig({
    thresholds: { warning: { enabled: true, value: 20 }, critical: { enabled: true, value: 50 } },
  }, defaultConfig);
  assert.equal(config.thresholds.warning.value, 20);
  assert.equal(config.thresholds.critical.value, 20);
});

// --- presentation ----------------------------------------------------------

test('every status the backend can return has badge text', () => {
  for (const status of [...EXPECTED, ...FAILURES, 'connected']) {
    assert.equal(typeof STATUS_BADGES[status], 'string', status);
    assert.notEqual(STATUS_BADGES[status], '', status);
  }
});

test('badge text stays short enough for a 250px window', () => {
  for (const [status, text] of Object.entries(STATUS_BADGES)) {
    assert.ok(text.length <= 18, `${status} badge "${text}" is ${text.length} chars`);
  }
});

test('waiting-for-setup states are not styled as errors', () => {
  assert.equal(badgeClass('auth_check_required'), 'is-ready');
  assert.equal(badgeClass('connected'), 'is-ready');
  assert.equal(badgeClass('login_required'), '');
  assert.equal(badgeClass('temporary_error'), 'is-blocked');
});

const thresholdFixtures = [
  {
    name: 'legacy endpoints',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: true, value: 1 }, critical: { enabled: true, value: 99 },
      }
    },
    expected: { warning: { enabled: true, value: 5 }, critical: { enabled: true, value: 5 } },
  },
  {
    name: 'rounded upper bound',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: true, value: 42 }, critical: { enabled: true, value: 98 },
      }
    },
    expected: { warning: { enabled: true, value: 40 }, critical: { enabled: true, value: 40 } },
  },
  {
    name: 'legacy disabled zero',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: true, value: 50 }, critical: { enabled: false, value: 0 },
      }
    },
    expected: { warning: { enabled: true, value: 50 }, critical: { enabled: false, value: 5 } },
  },
  {
    name: 'legacy enabled zero',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: true, value: 50 }, critical: { enabled: true, value: 0 },
      }
    },
    expected: { warning: { enabled: true, value: 50 }, critical: { enabled: true, value: 5 } },
  },
  {
    name: 'current maximum',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: true, value: 100 }, critical: { enabled: true, value: 100 },
      }
    },
    expected: { warning: { enabled: true, value: 100 }, critical: { enabled: true, value: 100 } },
  },
  {
    name: 'outside range',
    settings: {
      theme: 'dark', startupMode: 'tray', thresholds: {
        warning: { enabled: false, value: -10 }, critical: { enabled: true, value: 150 },
      }
    },
    expected: { warning: { enabled: false, value: 5 }, critical: { enabled: true, value: 100 } },
  },
];
for (const fixture of thresholdFixtures) {
  test('threshold migration: ' + fixture.name, () => {
    const config = normalizeConfig(fixture.settings);
    assert.deepEqual(config.thresholds, fixture.expected);
    assert.deepEqual(normalizeConfig(config).thresholds, fixture.expected);
  });
}
