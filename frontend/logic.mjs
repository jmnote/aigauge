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
export const DEFAULT_REFRESH_SECONDS = 120;
export const MAX_RETRY_DELAY_SECONDS = 1800;
export const MIN_WINDOW_WIDTH = 160;
export const MAX_WINDOW_WIDTH = 600;
export const DEFAULT_WINDOW_WIDTH = 250;

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

// Keeps the user's ordering for providers it recognizes, drops ids that no
// longer exist, de-duplicates, and appends anything missing - so a config from
// a version with a different provider list still yields a complete order.
export function normalizeProviderOrder(value, providerIds) {
  const known = Array.isArray(value) ? value.filter(id => providerIds.includes(id)) : [];
  const order = [...new Set(known)];
  for (const id of providerIds) {
    if (!order.includes(id)) order.push(id);
  }
  return order;
}

export function normalizeThreshold(raw, defaultThreshold, min, max) {
  const isObj = typeof raw === 'object' && raw !== null;
  const rawInput = isObj ? raw.value : raw;
  const num = typeof rawInput === 'string' && rawInput.trim() === '' ? NaN : Number(rawInput);
  const rounded = Number.isFinite(num) ? Math.round(num) : NaN;
  const value = Number.isFinite(rounded) && rounded >= min && rounded <= max
    ? rounded : defaultThreshold.value;
  const enabled = isObj && typeof raw.enabled === 'boolean' ? raw.enabled : true;
  return { enabled, value };
}

// Turns whatever was in storage into a complete, in-range config. It never
// throws and never returns a partial object: a corrupted or hostile settings
// blob has to degrade into defaults rather than take the window down with it.
export function normalizeConfig(value, providerIds, defaultConfig) {
  const warning = normalizeThreshold(value?.thresholds?.warning, defaultConfig.thresholds.warning, 1, 100);
  const critical = normalizeThreshold(value?.thresholds?.critical, defaultConfig.thresholds.critical, 0, 99);
  if (warning.enabled && critical.enabled && critical.value >= warning.value) {
    critical.value = Math.max(0, warning.value - 1);
  }

  const theme = value?.theme === 'auto' ? 'system' : value?.theme;
  return {
    providers: Object.fromEntries(providerIds.map(id => [id, { enabled: value?.providers?.[id]?.enabled !== false }])),
    providerOrder: normalizeProviderOrder(value?.providerOrder, providerIds),
    windowWidth: normalizeWindowWidth(value?.windowWidth),
    theme: VALID_THEMES.has(theme) ? theme : defaultConfig.theme,
    refreshInterval: parseIntervalToSeconds(value?.refreshInterval),
    thresholds: { warning, critical }
  };
}

// The short badge text per backend status. Deliberately short: the badge sits
// beside the provider name in a 250px window, so the reason behind a status
// ("Credentials found", "Logged in locally") belongs in the message line.
export const STATUS_BADGES = {
  connected: 'Connected',
  not_installed: 'Not installed',
  auth_check_required: 'Check connection',
  login_required: 'Login required',
  usage_unavailable: 'Usage unavailable',
  temporary_error: 'Temporary error',
  unsupported_cli: 'Unsupported CLI',
};

// States the user resolves themselves - no CLI yet, not logged in, connection
// not checked. They are the normal shape of a machine that has not been set
// up, so they must never be treated as failures. unsupported_cli is
// deliberately NOT here - an install that regresses from a working CLI to an
// incompatible one is a real failure, not a setup step, and must count
// against the failure threshold and keep retrying like any other failure (see
// Status.NeedsUserAction in internal/providers/status.go, which this set
// mirrors and must stay in sync with).
export const EXPECTED_SETUP_STATES = new Set([
  'not_installed', 'auth_check_required', 'login_required',
]);

export const isExpectedSetupState = status => EXPECTED_SETUP_STATES.has(status);

// Counting an expected setup state as a failure is what made a clean review
// machine look broken: it drives the status dot red and backs the refresh off
// exponentially over a state that is simply waiting for the user.
export const shouldCountFailure = status =>
  Boolean(status) && status !== 'connected' && !isExpectedSetupState(status);

// Nothing polls its way out of "no CLI installed" or "not signed in", so an
// expected setup state gets no automatic retry - the user's own Check again is
// the trigger.
export const shouldScheduleRetry = status => !isExpectedSetupState(status);

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
// changes; in sample mode live timers are replaced by one sample fetch.
export function providerVisibilityAction(sampleMode, enabled, hasTimer) {
  if (sampleMode) return 'restart';
  if (!enabled) return 'stop';
  return hasTimer ? 'preserve' : 'fetch';
}

// "Ready" means the only thing left is to confirm the connection; "blocked"
// means something is actually wrong. Neither an expected setup state nor a
// ready one is an error, so neither is styled as one.
export function badgeClass(status) {
  if (status === 'connected' || status === 'auth_check_required') return 'is-ready';
  if (status === 'temporary_error' || status === 'usage_unavailable' || status === 'unsupported_cli') return 'is-blocked';
  return '';
}
