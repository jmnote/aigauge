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
  STATUS_BADGES,
  badgeClass,
  formatHotkeyError,
  normalizeConfig,
  normalizeProviderOrder,
  normalizeThreshold,
  normalizeWindowWidth,
  parseIntervalToSeconds,
  providerVisibilityAction,
  retryDelay,
  shouldCountFailure,
  shouldKeepStaleData,
  shouldScheduleRetry,
} from './logic.mjs';

const providerIds = ['codex', 'claude', 'antigravity'];
const defaultConfig = {
  providers: Object.fromEntries(providerIds.map(id => [id, { enabled: false }])),
  providerOrder: providerIds.slice(),
  windowWidth: DEFAULT_WINDOW_WIDTH,
  theme: 'system',
  hotkeyShortcut: null,
  refreshInterval: DEFAULT_REFRESH_SECONDS,
  thresholds: { warning: { enabled: true, value: 30 }, critical: { enabled: true, value: 10 } },
};

test('fresh install config keeps providers disabled by default', () => {
  const fresh = normalizeConfig(defaultConfig, providerIds, defaultConfig);
  assert.equal(fresh.providers.codex.enabled, false);
  assert.equal(fresh.providers.claude.enabled, false);
  assert.equal(fresh.providers.antigravity.enabled, false);
  assert.equal(fresh.hotkeyShortcut, null);
});

test('hotkey settings normalize to the supported choices', () => {
  const primary = normalizeConfig({ hotkeyShortcut: 'Ctrl+Shift+G' }, providerIds, defaultConfig);
  assert.equal(primary.hotkeyShortcut, 'Ctrl+Shift+G');

  for (const shortcut of ['Ctrl+Shift+Q', 'Ctrl+Shift+E']) {
    const selected = normalizeConfig({ hotkeyShortcut: shortcut }, providerIds, defaultConfig);
    assert.equal(selected.hotkeyShortcut, shortcut);
  }

  const invalid = normalizeConfig({ hotkeyShortcut: 'Ctrl+Alt+X' }, providerIds, defaultConfig);
  assert.equal(invalid.hotkeyShortcut, null);
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

test('the first hotkey option is the default', () => {
  const normalized = normalizeConfig({}, providerIds, defaultConfig);
  assert.equal(normalized.hotkeyShortcut, null);
});

test('window width is restored within the supported range', () => {
  assert.equal(normalizeWindowWidth(320), 320);
  assert.equal(normalizeWindowWidth(100), 200);
  assert.equal(normalizeWindowWidth(900), 600);
  assert.equal(normalizeWindowWidth('invalid'), DEFAULT_WINDOW_WIDTH);
});

const EXPECTED = ['not_installed', 'auth_check_required', 'login_required'];
const FAILURES = ['temporary_error', 'usage_unavailable', 'unsupported_cli'];
// unsupported_cli counts as a failure like the others above, but - unlike
// them - retrying it automatically can never succeed, so it is excluded here.
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

test('an unsupported CLI counts as a failure but never auto-retries, since only a CLI update fixes it', () => {
  assert.equal(shouldCountFailure('unsupported_cli'), true);
  assert.equal(shouldScheduleRetry('unsupported_cli'), false);
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
  assert.equal(providerVisibilityAction(false, true, true), 'preserve');
  assert.equal(providerVisibilityAction(false, true, false), 'fetch');
  assert.equal(providerVisibilityAction(false, false, true), 'stop');
  assert.equal(providerVisibilityAction(true, false, true), 'restart');
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

test('provider order survives unknown, duplicated and missing ids', () => {
  assert.deepEqual(
    normalizeProviderOrder(['claude', 'gemini', 'claude'], providerIds),
    ['claude', 'codex', 'antigravity'],
  );
  assert.deepEqual(normalizeProviderOrder('not an array', providerIds), providerIds);
  assert.deepEqual(normalizeProviderOrder(undefined, providerIds), providerIds);
});

test('a threshold out of range or of the wrong type falls back to its default', () => {
  assert.deepEqual(normalizeThreshold(150, { enabled: true, value: 30 }, 1, 100), { enabled: true, value: 30 });
  assert.deepEqual(normalizeThreshold('', { enabled: true, value: 30 }, 1, 100), { enabled: true, value: 30 });
  assert.deepEqual(normalizeThreshold({ enabled: false, value: 42 }, { enabled: true, value: 30 }, 1, 100),
    { enabled: false, value: 42 });
});

test('a corrupted config normalizes into a complete, usable one', () => {
  for (const stored of [null, undefined, 42, 'nonsense', [], { providers: 'no' }, { thresholds: null }]) {
    const config = normalizeConfig(stored, providerIds, defaultConfig);
    assert.deepEqual(Object.keys(config.providers).sort(), [...providerIds].sort(), String(stored));
    assert.deepEqual([...config.providerOrder].sort(), [...providerIds].sort(), String(stored));
    assert.equal(Number.isFinite(config.refreshInterval), true, String(stored));
    assert.equal(['light', 'dark', 'system'].includes(config.theme), true, String(stored));
    assert.equal(Number.isFinite(config.thresholds.warning.value), true, String(stored));
    assert.equal(Number.isFinite(config.thresholds.critical.value), true, String(stored));
  }
});

test('the legacy "auto" theme is carried over to "system"', () => {
  assert.equal(normalizeConfig({ theme: 'auto' }, providerIds, defaultConfig).theme, 'system');
});

test('critical is pushed below warning when a stored config has them crossed', () => {
  const config = normalizeConfig({
    thresholds: { warning: { enabled: true, value: 20 }, critical: { enabled: true, value: 50 } },
  }, providerIds, defaultConfig);
  assert.equal(config.thresholds.critical.value < config.thresholds.warning.value, true);
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
  assert.equal(badgeClass('not_installed'), '');
  assert.equal(badgeClass('login_required'), '');
  assert.equal(badgeClass('temporary_error'), 'is-blocked');
  assert.equal(badgeClass('unsupported_cli'), 'is-blocked');
});
