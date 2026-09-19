// Pure rules shared by app.js and its tests.
//
// Everything here is a plain function over plain values - no DOM, no Wails, no
// localStorage - so frontend/logic.test.js can exercise it under `node --test`
// with no test framework and no browser stand-in. app.js keeps the parts that
// genuinely need a document; this file keeps the decisions that are worth
// pinning down, above all the ones that decide whether a provider counts as
// broken.

export const MIN_REFRESH_SECONDS = 1;
export const MAX_REFRESH_SECONDS = 3600;
export const DEFAULT_REFRESH_SECONDS = 180;
export const PROVIDER_REFRESH_OPTIONS = [60, 180, 300, 600, 1800, 3600];
export const MAX_RETRY_DELAY_SECONDS = 1800;
export const MIN_WINDOW_WIDTH = 200;
export const MAX_WINDOW_WIDTH = 600;
export const DEFAULT_WINDOW_WIDTH = 250;
export const HOTKEY_OPTIONS = [
  { value: 'Ctrl+Shift+G', label: 'Ctrl + Shift + G' },
  { value: 'Ctrl+Shift+Q', label: 'Ctrl + Shift + Q' },
  { value: 'Ctrl+Shift+E', label: 'Ctrl + Shift + E' },
];

export function formatHotkeyError(error, enabled = true) {
  const msg = (typeof error === 'string' ? error : error?.message || String(error || '')).trim();
  const fallback = enabled ? 'Registration failed' : 'Unregistration failed';
  if (!msg) return fallback;
  return `${fallback}: ${msg}`;
}

export function hotkeyOptionLabel(shortcut) {
  if (!shortcut) return 'Disabled';
  return HOTKEY_OPTIONS.find(option => option.value === shortcut)?.label || shortcut;
}

export const VALID_THEMES = new Set(['light', 'dark', 'system']);

const clampSeconds = seconds =>
  Math.max(MIN_REFRESH_SECONDS, Math.min(MAX_REFRESH_SECONDS, seconds));

export function normalizeWindowWidth(value) {
  const width = Number(value);
  if (!Number.isFinite(width)) return DEFAULT_WINDOW_WIDTH;
  return Math.max(MIN_WINDOW_WIDTH, Math.min(MAX_WINDOW_WIDTH, Math.round(width)));
}

// Accepts what a settings file might actually hold after hand-editing or an
// older version: a number, "90", "2m", "1m30s". Anything unreadable falls back
// to the default rather than propagating NaN into a timer.
export function parseIntervalToSeconds(val) {
  if (typeof val === 'number') {
    return Number.isFinite(val) ? clampSeconds(Math.round(val)) : DEFAULT_REFRESH_SECONDS;
  }
  if (typeof val === 'string') {
    const str = val.trim().toLowerCase();
    let total = 0;
    let matched = false;
    const mMatch = str.match(/(\d+)\s*m/);
    const sMatch = str.match(/(\d+)\s*s/);
    if (mMatch) { total += parseInt(mMatch[1], 10) * 60; matched = true; }
    if (sMatch) { total += parseInt(sMatch[1], 10); matched = true; }
    if (matched) return clampSeconds(total);
    const rawNum = Number(str);
    if (str !== '' && Number.isFinite(rawNum)) return clampSeconds(Math.round(rawNum));
  }
  return DEFAULT_REFRESH_SECONDS;
}

// The provider types AI Gauge knows how to connect to and monitor. A
// provider *instance* (below) is one user-added connection of one of these
// types; a user may add several instances of the same type (e.g. two Claude
// accounts), which is why instances are identified by their own id rather
// than by type.
export const PROVIDER_TYPES = [
  { id: 'codex', label: 'Codex' },
  { id: 'claude', label: 'Claude' },
  { id: 'antigravity', label: 'Antigravity' },
  { id: 'copilot', label: 'GitHub Copilot' },
];
export const PROVIDER_TYPE_IDS = PROVIDER_TYPES.map(t => t.id);


export function providerTypeLabel(type) {
  return PROVIDER_TYPES.find(t => t.id === type)?.label || type;
}

export function shouldShowProviderUser(providers, type, user) {
	if (!user || !['codex', 'claude'].includes(type) || !Array.isArray(providers)) return false;
  return providers.filter(instance => instance?.type === type).length > 1;
}

// Turns one raw provider entry from storage into a valid instance, or null if
// it is unsalvageable (missing id, or a type this build does not know). A
// missing/blank label falls back to its type's name rather than surfacing an
// empty row.
export function normalizeProviderInstance(raw) {
  if (!raw || typeof raw !== 'object') return null;
  if (typeof raw.id !== 'string' || !raw.id) return null;
  if (!PROVIDER_TYPE_IDS.includes(raw.type)) return null;
  const label = typeof raw.label === 'string' && raw.label.trim() ? raw.label : providerTypeLabel(raw.type);
  const refreshInterval = Number(raw.refreshInterval);
  return { id: raw.id, type: raw.type, label,
    refreshInterval: PROVIDER_REFRESH_OPTIONS.includes(refreshInterval) ? refreshInterval : DEFAULT_REFRESH_SECONDS };
}

// Normalizes a whole stored provider list: drops unsalvageable entries and
// de-duplicates by id, but otherwise keeps the given order - order *is* the
// user's chosen provider order now, there is no separate providerOrder list.
export function normalizeProviders(rawList) {
  if (!Array.isArray(rawList)) return [];
  const seen = new Set();
  const result = [];
  for (const raw of rawList) {
    const instance = normalizeProviderInstance(raw);
    if (!instance || seen.has(instance.id)) continue;
    seen.add(instance.id);
    result.push(instance);
  }
  return result;
}

export function normalizeThreshold(raw, defaultThreshold, min = 5, max = 100) {
  const isObj = typeof raw === 'object' && raw !== null;
  const rawInput = isObj ? raw.value : raw;
  const num = typeof rawInput === 'string' && rawInput.trim() === '' ? NaN : Number(rawInput);
  const rounded = Number.isFinite(num) ? Math.round(num) : NaN;
  const stepped = Number.isFinite(rounded) ? Math.round(rounded / 5) * 5 : NaN;
  const value = Number.isFinite(stepped)
    ? Math.max(min, Math.min(max, stepped)) : defaultThreshold.value;
  const enabled = isObj && typeof raw.enabled === 'boolean' ? raw.enabled : true;
  return { enabled, value };
}

// A fresh install has no provider instances at all. Providers are added from
// the Settings window via the backend's AddProviderInstance + ConnectProvider
// flow.
export const DEFAULT_CONFIG = {
  providers: [],
  windowWidth: DEFAULT_WINDOW_WIDTH,
  theme: 'system',
  hotkeyShortcut: '',
  thresholds: {
    warning: { enabled: true, value: 50 },
    critical: { enabled: true, value: 20 },
  },
};

// Turns whatever the backend returned (or, defensively, whatever a corrupted
// or hand-edited settings file held) into a complete, in-range config. It
// never throws and never returns a partial object: bad input has to degrade
// into defaults rather than take the window down with it.
export function normalizeConfig(value, defaultConfig = DEFAULT_CONFIG) {
  const effectiveDefault = defaultConfig || DEFAULT_CONFIG;
  const warning = normalizeThreshold(value?.thresholds?.warning, effectiveDefault.thresholds.warning);
  const critical = normalizeThreshold(value?.thresholds?.critical, effectiveDefault.thresholds.critical);
  const theme = value?.theme === 'auto' ? 'system' : value?.theme;
  const hotkeyShortcut = HOTKEY_OPTIONS.some(option => option.value === value?.hotkeyShortcut)
    ? value.hotkeyShortcut : '';
  if (warning.enabled && critical.enabled && critical.value > warning.value) {
    critical.value = warning.value;
  }
  return {
    providers: normalizeProviders(value?.providers),
    windowWidth: normalizeWindowWidth(value?.windowWidth),
    theme: VALID_THEMES.has(theme) ? theme : effectiveDefault.theme,
    hotkeyShortcut,
    thresholds: { warning, critical }
  };
}

// The short badge text per backend status. Deliberately short: the badge sits
// beside the provider name in a 250px window, so the reason behind a status
// ("Credentials found", "Logged in locally") belongs in the message line.
export const STATUS_BADGES = {
  connected: 'Connected',
  auth_check_required: 'Check connection',
  login_required: 'Login required',
  usage_unavailable: 'Usage unavailable',
  temporary_error: 'Temporary error',
  authenticating: 'Connecting...',
  awaiting_code: 'Awaiting code',
  not_installed: 'Not installed',
  unsupported_cli: 'Update required',
};

// States the user resolves themselves - not logged in, connection not checked.
// They are the normal shape of a machine that has not been set up, so they
// must never be treated as failures. awaiting_code (a manual-code provider,
// e.g. Claude, is waiting for the code the user pastes back in - see
// ProviderConfig.ManualCode) belongs here for the same reason authenticating
// does: it is mid-login, not broken.
export const EXPECTED_SETUP_STATES = new Set([
  'auth_check_required', 'login_required', 'authenticating', 'awaiting_code', 'not_installed',
]);

export const isExpectedSetupState = status => EXPECTED_SETUP_STATES.has(status);

// States where an automatic retry cannot fix anything on its own.
const NO_AUTO_RETRY_STATES = new Set([...EXPECTED_SETUP_STATES, 'unsupported_cli']);

// Counting an expected setup state as a failure is what made a clean review
// machine look broken: it drives the status dot red and backs the refresh off
// exponentially over a state that is simply waiting for the user.
export const shouldCountFailure = status =>
  Boolean(status) && status !== 'connected' && !isExpectedSetupState(status);

// Nothing polls its way out of "no CLI installed", "not signed in", or "this
// CLI version isn't supported", so these states get no automatic retry - the
// user's own Check again is the trigger.
export const shouldScheduleRetry = status => !NO_AUTO_RETRY_STATES.has(status);

// A temporary network or service failure is the one case that keeps whatever is
// already on screen: the numbers are stale, not wrong, and blanking the card
// throws away the only thing the user opened the app for. Every other
// non-connected state has no prior data to keep.
export const shouldKeepStaleData = (status, lastSuccessAt) =>
  status === 'temporary_error' && lastSuccessAt > 0;

export const retryDelay = (failureCount, refreshInterval) =>
  Math.min(refreshInterval * (2 ** Math.min(failureCount, 4)), MAX_RETRY_DELAY_SECONDS);

// Decides how a visibility recalculation should treat a provider's refresh
// timer. In live mode an existing timer must survive unrelated settings/order
// changes (every instance is always shown and polled - there is no
// enabled/disabled state to stop for).
export function providerVisibilityAction(hasTimer) {
  return hasTimer ? 'preserve' : 'fetch';
}

// "Ready" means the only thing left is to confirm the connection; "blocked"
// means something is actually wrong. Neither an expected setup state nor a
// ready one is an error, so neither is styled as one.
export function badgeClass(status) {
  if (status === 'connected' || status === 'auth_check_required' || status === 'authenticating' || status === 'awaiting_code') return 'is-ready';
  if (status === 'temporary_error' || status === 'usage_unavailable' || status === 'unsupported_cli') return 'is-blocked';
  return '';
}
