import {
  parseIntervalToSeconds, normalizeConfig, VALID_THEMES, STATUS_BADGES,
  shouldCountFailure, shouldScheduleRetry,
  shouldKeepStaleData, retryDelay, badgeClass,
} from '/logic.mjs';

const wails = await import('/wails/runtime.js');
if (globalThis.__AIGAUGE_LIVE__) {
  const watchLiveResource = url => {
    let snapshot = '';
    setInterval(async () => {
      const response = await fetch(url, { cache: 'no-store' });
      const current = await response.text();
      if (snapshot && current !== snapshot) location.reload();
      snapshot = current;
    }, 1000);
  };
  watchLiveResource('/__live-version');
}

const settingsStorageKey = 'aigauge-settings-v1';

function saveCurrentConfig() {
  try {
    localStorage.setItem(settingsStorageKey, JSON.stringify(config));
  } catch (e) {
    console.warn('Failed to save settings:', e);
  }
}

// Render functions are declared later in this file as `function` statements,
// so they're already hoisted by the time this array literal runs. Listed
// reverse-alphabetically by label - that order also seeds the default
// providerOrder (below) and the initial card layout.
const PROVIDERS = [
  {
    id: 'codex', label: 'Codex', rpcMethod: 'GetCodexUsage',
    sampleRpcMethod: 'GetSampleCodexUsage', diagnoseRpcMethod: 'DiagnoseCodex',
    cardId: 'codex-card', groupsId: 'codex-groups',
    dotId: 'codex-dot', tooltipId: 'codex-tooltip', errorId: 'codex-error',
    render: renderUsage
  },
  {
    id: 'claude', label: 'Claude', rpcMethod: 'GetClaudeUsage',
    sampleRpcMethod: 'GetSampleClaudeUsage', diagnoseRpcMethod: 'DiagnoseClaude',
    cardId: 'claude-card', groupsId: 'claude-groups',
    dotId: 'claude-dot', tooltipId: 'claude-tooltip', errorId: 'claude-error',
    render: renderClaude
  },
  {
    id: 'antigravity', label: 'Antigravity', rpcMethod: 'GetAntigravityUsage',
    sampleRpcMethod: 'GetSampleAntigravityUsage', diagnoseRpcMethod: 'DiagnoseAntigravity',
    cardId: 'agy-card', groupsId: 'agy-groups',
    dotId: 'agy-dot', tooltipId: 'agy-tooltip', errorId: 'agy-error',
    render: renderAntigravity
  }
];
const PROVIDERS_BY_ID = new Map(PROVIDERS.map(p => [p.id, p]));

// The short badge text for each backend status code. Deliberately short: the
// badge has to stay readable beside the provider name in a 250px window, so
// the reason behind a status ("Credentials found", "Signed in locally") goes
// in the message line underneath rather than into the badge.
const rpc = method => wails.Call.ByName(`github.com/jmnote/aigauge/internal/app.App.${method}`);

// 'live' shows the user's real providers; 'sample' shows the bundled preview.
// The preview is a screen, not a mode: nothing below writes config while it is
// open, so entering and leaving it leaves every provider setting untouched.
let viewMode = 'live';
const providerIds = PROVIDERS.map(p => p.id);

const defaultConfig = {
  // Providers start disabled so a fresh install never probes for local CLI
  // tools/credentials on its own - the first screen is "No providers
  // enabled" with "Open Settings" and "Demo" buttons instead of raw
  // not-found errors. This only affects genuinely first runs: normalizeConfig
  // (below) falls back to enabled once localStorage holds any saved config.
  providers: Object.fromEntries(providerIds.map(id => [id, { enabled: false }])),
  providerOrder: providerIds.slice(),
  theme: 'system',
  refreshInterval: 120,
  thresholds: {
    warning: { enabled: true, value: 50 },
    critical: { enabled: true, value: 20 }
  }
};

let config = normalizeConfig(defaultConfig, providerIds, defaultConfig);

try {
  const storedSettings = localStorage.getItem(settingsStorageKey);
  if (storedSettings !== null) {
    config = normalizeConfig(JSON.parse(storedSettings), providerIds, defaultConfig);
  }
} catch (e) {
  console.warn('Failed to load settings:', e);
  config = normalizeConfig(defaultConfig, providerIds, defaultConfig);
}

function applyLimitState(barElement, remaining) {
  const { warning, critical } = config.thresholds;
  const state = (critical.enabled && remaining <= critical.value) ? 'critical'
    : (warning.enabled && remaining <= warning.value) ? 'warning' : '';
  barElement.classList.remove('warning', 'critical');
  if (state) barElement.classList.add(state);
}

function refreshLimitStates() {
  document.querySelectorAll('.fill[data-remaining]').forEach(bar => {
    applyLimitState(bar, Number(bar.dataset.remaining));
  });
}

const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

const formatClockTime = targetDate => [targetDate.getHours(), targetDate.getMinutes()]
  .map(value => String(value).padStart(2, '0'))
  .join(':');

const formatTimeRemaining = (seconds, targetDate) => {
  if (!seconds || seconds <= 0 || !targetDate || Number.isNaN(targetDate.getTime())) return '';
  const withinTwentyFourHours = seconds < 24 * 60 * 60;
  return withinTwentyFourHours
    ? formatClockTime(targetDate)
    : `${MONTH_NAMES[targetDate.getMonth()]} ${targetDate.getDate()}`;
};

// "Now", for reset-time math, is the moment this usage was fetched rather
// than whenever it happens to be rendered - every provider's payload carries
// a `fetchedAt` (see hack/fixtures/gen-json and internal/providers), set right
// before that fetch went out. For a live fetch the two are milliseconds
// apart (network latency, basically), so this changes nothing normal users
// would notice. It matters for the live-server preview (hack/live-server.ps1),
// which serves whatever `.\build.ps1 fixtures-json` last saved as-is, with no
// correction - that can be arbitrarily old by the time it's viewed: without
// anchoring to fetchedAt, a Codex reset (reported as a relative "seconds
// from now") would silently push further into the future every reload, and
// an absolute reset time (Claude/Antigravity) could drift into the past and
// render as already-elapsed. (The native app's own sample preview, --sample-preview
// - see hack/screenshot.ps1 - doesn't depend on this: internal/app.App already
// re-anchors fetchedAt and every resetTime to "now" server-side before the
// frontend ever sees them.)
function referenceNow(usage) {
  const fetchedAt = new Date(usage?.fetchedAt).getTime();
  return Number.isNaN(fetchedAt) ? Date.now() : fetchedAt;
}

const formatResetAt = (resetTime, nowMs) => {
  if (!resetTime) return '';
  const targetDate = new Date(resetTime);
  if (Number.isNaN(targetDate.getTime())) return '';
  const seconds = Math.max(0, Math.round((targetDate.getTime() - nowMs) / 1000));
  return formatTimeRemaining(seconds, targetDate);
};

// Full date and time for hovering the reset-time text, e.g. "resets Sep 9 12:34".
const formatResetHover = resetTime => {
  if (!resetTime) return '';
  const targetDate = new Date(resetTime);
  if (Number.isNaN(targetDate.getTime())) return '';
  const month = MONTH_NAMES[targetDate.getMonth()];
  const day = targetDate.getDate();
  return `resets ${month} ${day} ${formatClockTime(targetDate)}`;
};

let refreshInterval = parseIntervalToSeconds(config.refreshInterval);
// One record per provider instead of a parallel `let` per provider per field -
// adding a provider only means adding an entry to PROVIDERS above.
const providerState = new Map(providerIds.map(id => [id, {
  nextRefreshAt: Date.now() + refreshInterval * 1000,
  timerId: null,
  fetching: false,
  failureCount: 0,
  lastSuccessAt: 0,
  lastError: '',
  status: '',
  plan: ''
}]));

function updateStatus(dotId, tooltipId, failureCount, lastSuccessAt, nextRefreshAt, lastError, plan) {
  const dot = document.getElementById(dotId);
  dot.classList.remove('connected', 'warning');
  if (lastSuccessAt && failureCount < 3) dot.classList.add('connected');
  else if (lastSuccessAt && failureCount < 6) dot.classList.add('warning');

  const tooltip = document.getElementById(tooltipId);
  const successValue = lastSuccessAt ? formatAgo(lastSuccessAt) : 'None';
  const nextRefreshText = formatUntil(nextRefreshAt);
  tooltip.replaceChildren(
    ...(plan ? [createTooltipRow('Plan', plan)] : []),
    createTooltipRow('Fails', `${failureCount}`),
    createTooltipRow('Last fetch', successValue),
    ...(lastError ? [createTooltipRow('Last error', lastError)] : []),
    createTooltipRow('Next fetch', nextRefreshText),
  );
}

function updateProviderStatus(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  updateStatus(meta.dotId, meta.tooltipId, state.failureCount, state.lastSuccessAt, state.nextRefreshAt, state.lastError, state.plan);
}

function createTooltipRow(labelText, valueText) {
  const row = document.createElement('div');
  row.className = 'status-tooltip-row';
  const label = document.createElement('span');
  label.className = 'status-tooltip-label';
  label.textContent = labelText;
  const value = document.createElement('span');
  value.className = 'status-tooltip-value';
  value.textContent = valueText;
  row.append(label, value);
  return row;
}

function formatAgo(timestamp) {
  const elapsed = Math.max(0, Math.floor((Date.now() - timestamp) / 1000));
  return `${elapsed}s ago`;
}

function formatUntil(timestamp) {
  const seconds = Math.max(0, Math.ceil((timestamp - Date.now()) / 1000));
  return `in ${seconds}s`;
}

function scheduleProvider(id) {
  const state = providerState.get(id);
  clearTimeout(state.timerId);
  state.timerId = null;
  // The sample preview is static, so there is nothing to poll for.
  if (viewMode !== 'live') return;
  if (!config.providers[id].enabled) return;
  if (!shouldScheduleRetry(state.status)) return;
  const delay = retryDelay(state.failureCount, refreshInterval);
  state.nextRefreshAt = Date.now() + delay * 1000;
  state.timerId = setTimeout(() => fetchProvider(id), delay * 1000);
}

function setLoading(dotId, loading) {
  document.getElementById(dotId).classList.toggle('loading', loading);
}

function clearLoadingText(cardId) {
  document.querySelectorAll(`#${cardId} .loading-text`).forEach(element => element.classList.remove('loading-text'));
}

function renderFormattedMessage(element, text) {
  element.replaceChildren();
  if (!text) return;
  const parts = text.split(/(<code>.*?<\/code>|<a\s+[^>]*>.*?<\/a>)/g);
  for (const part of parts) {
    if (part.startsWith('<code>') && part.endsWith('</code>')) {
      const code = document.createElement('code');
      code.textContent = part.slice(6, -7);
      element.appendChild(code);
    } else if (part.startsWith('<a ') && part.endsWith('</a>')) {
      const match = part.match(/^<a\s+href="([^"]*)">(.*?)<\/a>$/);
      if (match) {
        const a = document.createElement('a');
        const href = match[1];
        a.href = href;
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.textContent = match[2];
        a.addEventListener('click', (e) => {
          e.preventDefault();
          if (wails && wails.Browser && typeof wails.Browser.OpenURL === 'function') {
            wails.Browser.OpenURL(href).catch(err => console.error('Failed to open URL:', err));
          } else {
            window.open(href, '_blank', 'noopener,noreferrer');
          }
        });
        element.appendChild(a);
      } else {
        element.appendChild(document.createTextNode(part));
      }
    } else if (part) {
      element.appendChild(document.createTextNode(part));
    }
  }
}

function createDiagnosisActions(diagnosis, onCheck) {
  const actions = document.createElement('div');
  actions.className = 'setup-provider-actions';
  const primary = document.createElement('button');
  primary.type = 'button';
  if (diagnosis.status === 'auth_check_required') {
    primary.textContent = 'Check connection';
  } else if (diagnosis.status === 'not_installed' || diagnosis.status === 'unsupported_cli') {
    primary.textContent = 'Check CLI';
  } else {
    primary.textContent = 'Check again';
  }
  primary.addEventListener('click', onCheck);
  actions.append(primary);

  if (diagnosis.details) {
    const detailsBtn = document.createElement('button');
    detailsBtn.type = 'button';
    detailsBtn.textContent = 'Details';
    detailsBtn.setAttribute('aria-expanded', 'false');
    const details = document.createElement('p');
    details.className = 'setup-provider-details';
    details.textContent = diagnosis.details;
    details.hidden = true;
    detailsBtn.addEventListener('click', () => {
      details.hidden = !details.hidden;
      detailsBtn.setAttribute('aria-expanded', String(!details.hidden));
      requestWindowResize();
    });
    actions.append(detailsBtn);
    return [actions, details];
  }
  return [actions];
}

function showProviderError(errorId, message, diagnosis, onCheck) {
  const element = document.getElementById(errorId);
  if (!element) return;
  element.replaceChildren();
  if (!message) {
    element.hidden = true;
    return;
  }
  element.hidden = false;
  const p = document.createElement('p');
  p.className = 'provider-error-message';
  renderFormattedMessage(p, message);
  element.append(p);

  if (diagnosis && diagnosis.status && onCheck) {
    element.append(...createDiagnosisActions(diagnosis, onCheck));
  }
}

// Draws a provider that came back with something other than usable numbers.
//
// A temporary network or service failure is the one case that keeps whatever
// was already on screen: the data is stale, not wrong, and blanking the card
// throws away the only thing the user opened the app to see. It is labelled
// with its age so nobody reads month-old numbers as current. Every other
// non-connected state (no CLI, not signed in, connection not checked) has no
// prior data to keep, so its card clears to the guidance instead.
function renderNonUsageState(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const message = String(usage.message || usage.error || '');
  const onCheck = () => fetchProvider(id);
  if (shouldKeepStaleData(usage.status, state.lastSuccessAt)) {
    showProviderError(meta.errorId, `${message} Showing data from ${formatAgo(state.lastSuccessAt)}.`, usage, onCheck);
  } else {
    document.getElementById(meta.groupsId).replaceChildren();
    showProviderError(meta.errorId, message, usage, onCheck);
  }
  updateProviderStatus(id);
  requestWindowResize();
}

function renderUsage(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const groups = document.getElementById(meta.groupsId);
  if (usage.error) {
    renderNonUsageState(id, usage);
    return;
  }
  showProviderError(meta.errorId, '');
  state.plan = usage.plan || '';
  updateProviderStatus(id);
  groups.replaceChildren();
  const nowMs = referenceNow(usage);
  // Codex reports resets as seconds-from-now rather than an absolute
  // timestamp; convert so renderBuckets can use the same absolute-time
  // formatting as every other provider. Anchored to nowMs (not a fresh
  // Date.now() here) so a frozen live-preview "now" applies to this
  // conversion too, not just to the formatting below.
  const toResetTime = seconds => (seconds > 0 ? new Date(nowMs + seconds * 1000).toISOString() : '');
  renderBuckets(groups, [
    { label: '5h', remaining: 100 - usage.fiveHour, resetTime: toResetTime(usage.fiveHourResetIn) },
    { label: '7d', remaining: 100 - usage.sevenDay, resetTime: toResetTime(usage.sevenDayResetIn) },
  ], nowMs);
  requestWindowResize();
}

// Each row gets its own tooltip (a sibling of .inline-reset within .limit,
// not a child of it - .inline-reset has overflow:hidden for text truncation,
// which would clip a tooltip nested inside it) showing just that row's own
// full reset date-time, e.g. "Jan 1 (Fri) 00:00".
function renderBucketRow(container, label, remaining, resetTime, nowMs) {
  const clamped = Math.max(0, Math.min(100, remaining));
  const limit = document.createElement('div');
  limit.className = 'limit';

  const info = document.createElement('div');
  info.className = 'limit-info';

  const meta = document.createElement('div');
  meta.className = 'limit-meta';

  const labelText = document.createElement('span');
  labelText.className = 'limit-label-text';
  labelText.textContent = label;

  const defaultReset = formatResetAt(resetTime, nowMs);
  const hoverReset = formatResetHover(resetTime);

  const reset = document.createElement('span');
  reset.className = 'inline-reset';
  reset.textContent = defaultReset;
  if (hoverReset && hoverReset !== defaultReset) {
    reset.dataset.defaultText = defaultReset;
    reset.dataset.hoverText = hoverReset;
  }

  meta.append(labelText, reset);

  const value = document.createElement('span');
  value.className = 'limit-value';
  value.textContent = `${Math.round(clamped)}%`;

  info.append(meta, value);

  const bar = document.createElement('div');
  bar.className = 'bar';
  const fill = document.createElement('div');
  fill.className = 'fill';
  fill.style.width = `${clamped}%`;
  fill.dataset.remaining = clamped;
  applyLimitState(fill, clamped);
  bar.append(fill);

  limit.append(info, bar);
  container.append(limit);
}

// Renders a set of bucket rows (e.g. 5h/7d, or an Antigravity model group's
// own 5h/weekly pair) into `container`.
function renderBuckets(container, buckets, nowMs) {
  for (const bucket of buckets) {
    renderBucketRow(container, bucket.label, bucket.remaining, bucket.resetTime, nowMs);
  }
}

const antigravityWindowOrder = { '5h': 0, '24h': 1, weekly: 2 };
const antigravityWindowLabels = { '5h': '5h', weekly: '7d' };

function renderAntigravity(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const groups = document.getElementById(meta.groupsId);
  if (usage.error) {
    renderNonUsageState(id, usage);
    return;
  }
  showProviderError(meta.errorId, '');
  state.plan = usage.plan || '';
  updateProviderStatus(id);
  groups.replaceChildren();
  if (!usage.groups?.length) {
    requestWindowResize();
    return;
  }
  const nowMs = referenceNow(usage);
  for (const group of usage.groups) {
    const groupElement = document.createElement('div');
    groupElement.className = 'agy-group';
    const title = document.createElement('div');
    title.className = 'agy-group-title';
    title.textContent = group.name;
    groupElement.append(title);
    const buckets = [...(group.buckets || [])].sort((a, b) =>
      (antigravityWindowOrder[a.window] ?? 3) - (antigravityWindowOrder[b.window] ?? 3));
    renderBuckets(groupElement, buckets.map(bucket => ({
      label: antigravityWindowLabels[bucket.window] ?? bucket.name,
      remaining: bucket.remaining,
      resetTime: bucket.resetTime,
    })), nowMs);
    groups.append(groupElement);
  }
  requestWindowResize();
}

function renderClaude(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const groups = document.getElementById(meta.groupsId);
  if (usage.error) {
    renderNonUsageState(id, usage);
    return;
  }
  showProviderError(meta.errorId, '');
  state.plan = usage.plan || '';
  updateProviderStatus(id);
  groups.replaceChildren();
  if (!usage.buckets?.length) {
    requestWindowResize();
    return;
  }
  renderBuckets(groups, usage.buckets.map(bucket => ({
    label: bucket.name,
    remaining: bucket.remaining,
    resetTime: bucket.resetTime,
  })), referenceNow(usage));
  requestWindowResize();
}

async function fetchProvider(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const sampleMode = viewMode === 'sample';
  if (state.fetching) return;
  if (!sampleMode && !config.providers[id].enabled) return;
  state.fetching = true;
  setLoading(meta.dotId, true);
  try {
    const usage = await rpc(sampleMode ? meta.sampleRpcMethod : meta.rpcMethod);
    clearLoadingText(meta.cardId);
    state.status = usage.status || '';
    if (state.status && state.status !== 'connected') {
      state.lastError = String(usage.message || usage.error || '').slice(0, 160);
      // Only a genuine failure moves the counter - see EXPECTED_SETUP_STATES.
      if (shouldCountFailure(state.status)) state.failureCount += 1;
    } else if (usage.error) {
      state.failureCount += 1;
      state.lastError = String(usage.error).slice(0, 160);
    } else {
      state.failureCount = 0;
      state.lastSuccessAt = Date.now();
      state.lastError = '';
    }
    meta.render(id, usage);
  } catch (error) {
    clearLoadingText(meta.cardId);
    state.status = '';
    state.failureCount += 1;
    state.lastError = `Frontend call failed: ${error}`.slice(0, 160);
    meta.render(id, { error: state.lastError });
  } finally {
    state.fetching = false;
    setLoading(meta.dotId, false);
    scheduleProvider(id);
  }
}

document.querySelectorAll('.provider-refresh-btn').forEach(button => {
  button.addEventListener('click', () => fetchProvider(button.dataset.providerId));
});

const settingsDialog = document.getElementById('settings-dialog');
const providerListEl = document.getElementById('provider-list');
const pinWindowBtn = document.getElementById('pin-window');
const themeButtonGroup = document.querySelector('.theme-button-group');
const refreshPresetSelect = document.getElementById('refresh-preset-select');
const refreshIntervalInput = document.getElementById('refresh-interval-input');
const warningEnabledInput = document.getElementById('warning-enabled');
const warningThresholdInput = document.getElementById('warning-threshold');
const criticalEnabledInput = document.getElementById('critical-enabled');
const criticalThresholdInput = document.getElementById('critical-threshold');
const systemTheme = matchMedia('(prefers-color-scheme: dark)');

let isAlwaysOnTop = false;
// True while the first-run screen's diagnosis rows are already rendered, so
// reopening Settings does not re-probe every provider.
let setupRendered = false;

function updateAlwaysOnTopUI(isTop) {
  pinWindowBtn.classList.toggle('active', isTop);
  pinWindowBtn.title = isTop ? 'Always on top (Enabled)' : 'Always on top (Disabled)';
  pinWindowBtn.setAttribute('aria-pressed', isTop ? 'true' : 'false');
}

function toggleAlwaysOnTop() {
  isAlwaysOnTop = !isAlwaysOnTop;
  updateAlwaysOnTopUI(isAlwaysOnTop);
  wails.Window.SetAlwaysOnTop(isAlwaysOnTop);
  wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetAlwaysOnTop', isAlwaysOnTop).catch(() => { });
}

pinWindowBtn.addEventListener('click', toggleAlwaysOnTop);
updateAlwaysOnTopUI(isAlwaysOnTop);

function updateProvidersVisibility() {
  const sampleMode = viewMode === 'sample';
  let anyEnabled = false;

  for (const provider of PROVIDERS) {
    const enabled = config.providers[provider.id].enabled !== false;
    anyEnabled = anyEnabled || enabled;
    // The preview shows all three cards regardless of what the user has
    // enabled - and without consulting that setting for anything else.
    document.getElementById(provider.cardId).style.display = (sampleMode || enabled) ? '' : 'none';

    const state = providerState.get(provider.id);
    clearTimeout(state.timerId);
    if (sampleMode || enabled) {
      if (sampleMode || !state.timerId) fetchProvider(provider.id);
    } else {
      state.timerId = null;
    }
  }

  const showSetup = !sampleMode && !anyEnabled;
  document.getElementById('setup-screen').hidden = !showSetup;
  document.getElementById('sample-bar').hidden = !sampleMode;
  if (showSetup) {
    // Re-run the diagnoses only on the way into the screen, not on every
    // visibility recalculation, so opening Settings does not re-probe.
    if (!setupRendered) {
      setupRendered = true;
      renderSetupProviders();
    }
  } else {
    setupRendered = false;
  }

  const visibleCards = config.providerOrder
    .map(id => document.getElementById(PROVIDERS_BY_ID.get(id).cardId))
    .filter(card => card.style.display !== 'none');
  visibleCards.forEach((card, index) => {
    card.classList.toggle('card-divider', index > 0);
  });

  requestWindowResize();
}

function applyProviderOrder() {
  const usageSections = document.querySelector('.usage-sections');
  for (const id of config.providerOrder) {
    usageSections.append(document.getElementById(PROVIDERS_BY_ID.get(id).cardId));
  }
  updateProvidersVisibility();
}

function renderProviderList() {
  providerListEl.replaceChildren();
  config.providerOrder.forEach((id, index) => {
    const meta = PROVIDERS_BY_ID.get(id);
    const row = document.createElement('div');
    row.className = 'provider-row';
    row.dataset.provider = id;

    const checkboxLabel = document.createElement('label');
    checkboxLabel.className = 'provider-setting';
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.dataset.action = 'toggle';
    checkbox.checked = config.providers[id].enabled;
    checkboxLabel.append(checkbox, document.createTextNode(` ${meta.label}`));

    const moveButtons = document.createElement('span');
    moveButtons.className = 'provider-move-buttons';
    const upBtn = document.createElement('button');
    upBtn.type = 'button';
    upBtn.className = 'provider-move-btn';
    upBtn.dataset.action = 'move-up';
    upBtn.textContent = '▲';
    upBtn.title = `Move ${meta.label} up`;
    upBtn.setAttribute('aria-label', `Move ${meta.label} up`);
    upBtn.disabled = index === 0;
    const downBtn = document.createElement('button');
    downBtn.type = 'button';
    downBtn.className = 'provider-move-btn';
    downBtn.dataset.action = 'move-down';
    downBtn.textContent = '▼';
    downBtn.title = `Move ${meta.label} down`;
    downBtn.setAttribute('aria-label', `Move ${meta.label} down`);
    downBtn.disabled = index === config.providerOrder.length - 1;
    moveButtons.append(upBtn, downBtn);

    row.append(checkboxLabel, moveButtons);
    providerListEl.append(row);
  });
}

providerListEl.addEventListener('change', event => {
  const checkbox = event.target.closest('input[data-action="toggle"]');
  if (!checkbox) return;
  const id = checkbox.closest('[data-provider]').dataset.provider;
  config.providers[id].enabled = checkbox.checked;
  updateProvidersVisibility();
  saveCurrentConfig();
});

providerListEl.addEventListener('click', event => {
  const button = event.target.closest('button[data-action]');
  if (!button) return;
  const id = button.closest('[data-provider]').dataset.provider;
  const order = config.providerOrder;
  const index = order.indexOf(id);
  const swapWith = button.dataset.action === 'move-up' ? index - 1 : index + 1;
  if (swapWith < 0 || swapWith >= order.length) return;
  [order[index], order[swapWith]] = [order[swapWith], order[index]];
  renderProviderList();
  applyProviderOrder();
  saveCurrentConfig();
});

function updateThemeButtonsUI(theme) {
  if (!themeButtonGroup) return;
  themeButtonGroup.querySelectorAll('.theme-option-btn').forEach(button => {
    const isActive = button.dataset.theme === theme;
    button.classList.toggle('active', isActive);
    button.setAttribute('aria-pressed', isActive ? 'true' : 'false');
  });
}

function applyTheme(theme, persist = true) {
  const resolved = theme === 'system'
    ? (systemTheme.matches ? 'dark' : 'light')
    : (theme === 'dark' ? 'dark' : 'light');
  document.documentElement.dataset.theme = theme;
  document.documentElement.dataset.resolvedTheme = resolved;
  updateThemeButtonsUI(theme);
  if (persist) {
    // The selected mode, not the resolved light/dark - otherwise "System"
    // could never be persisted (it would immediately collapse to whatever
    // it resolved to right now) and would silently stop following the OS
    // theme after the next launch.
    config.theme = theme;
    saveCurrentConfig();
  }
}

if (themeButtonGroup) {
  themeButtonGroup.addEventListener('click', event => {
    const button = event.target.closest('.theme-option-btn');
    if (button) applyTheme(button.dataset.theme, true);
  });
}

let forcedTheme = '';
try {
  forcedTheme = await wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.GetThemeOverride');
} catch (error) {
  console.warn('Unable to read the theme override:', error);
}
const activeTheme = forcedTheme || config.theme || 'system';
applyTheme(VALID_THEMES.has(activeTheme) ? activeTheme : 'system', !forcedTheme);

systemTheme.addEventListener('change', () => {
  if (document.documentElement.dataset.theme === 'system') applyTheme('system', false);
});

function updateRefreshPresetSelect(seconds) {
  const matchingOption = refreshPresetSelect.querySelector(`option[value="${seconds}"]`);
  refreshPresetSelect.value = matchingOption ? String(seconds) : 'custom';
}

function setRefreshInterval(val) {
  const seconds = typeof val === 'string' && val.trim() === '' ? config.refreshInterval : parseIntervalToSeconds(val);
  refreshInterval = seconds;
  config.refreshInterval = refreshInterval;
  refreshIntervalInput.value = refreshInterval;
  updateRefreshPresetSelect(refreshInterval);
  saveCurrentConfig();
  providerIds.forEach(scheduleProvider);
}

function openSettings() {
  renderProviderList();
  refreshIntervalInput.value = refreshInterval;
  updateRefreshPresetSelect(refreshInterval);
  warningEnabledInput.checked = config.thresholds.warning.enabled;
  warningThresholdInput.value = config.thresholds.warning.value;
  warningThresholdInput.disabled = !config.thresholds.warning.enabled;
  criticalEnabledInput.checked = config.thresholds.critical.enabled;
  criticalThresholdInput.value = config.thresholds.critical.value;
  criticalThresholdInput.disabled = !config.thresholds.critical.enabled;
  settingsDialog.showModal();
  requestWindowResize();
}

document.getElementById('settings').addEventListener('click', openSettings);
document.getElementById('open-settings-btn').addEventListener('click', openSettings);

// ---------------------------------------------------------------------------
// First-run screen
//
// Replaces the old bare "No providers enabled" message. It explains what the
// app does and what it needs, then shows each provider's readiness with the
// one action that moves it forward. Everything it calls is local-only: the
// Diagnose* RPCs read installed files and run no-network status commands, so
// opening AI Gauge on a machine that was never configured contacts nobody.
// ---------------------------------------------------------------------------

function setupRow(provider) {
  return document.querySelector(`.setup-provider[data-provider-id="${provider.id}"]`);
}

function buildSetupRow(provider, diagnosis) {
  const row = document.createElement('div');
  row.className = 'setup-provider';
  row.dataset.providerId = provider.id;

  const head = document.createElement('div');
  head.className = 'setup-provider-head';
  const name = document.createElement('span');
  name.className = 'setup-provider-name';
  name.textContent = provider.label;
  const badge = document.createElement('span');
  badge.className = `setup-provider-badge ${badgeClass(diagnosis.status)}`.trim();
  badge.textContent = STATUS_BADGES[diagnosis.status] || 'Checking...';
  head.append(name, badge);

  const message = document.createElement('p');
  message.className = 'setup-provider-message';
  renderFormattedMessage(message, diagnosis.message || '');
  row.append(head, message);

  if (!diagnosis.status) return row; // still checking: no actions to offer yet

  const onCheck = diagnosis.status === 'auth_check_required'
    ? () => checkConnection(provider)
    : () => diagnoseProvider(provider);

  row.append(...createDiagnosisActions(diagnosis, onCheck));
  return row;
}

function showSetupRow(provider, diagnosis) {
  const existing = setupRow(provider);
  if (!existing) return;
  existing.replaceWith(buildSetupRow(provider, diagnosis));
  requestWindowResize();
}

function diagnoseProvider(provider) {
  showSetupRow(provider, { status: '', message: 'Checking...' });
  return rpc(provider.diagnoseRpcMethod)
    .then(diagnosis => showSetupRow(provider, diagnosis || {}))
    .catch(() => showSetupRow(provider, {
      status: 'temporary_error',
      message: 'Could not check this provider. Try again.',
    }));
}

// Each provider resolves independently so one slow diagnosis never holds up
// the others - or the sample preview, which stays clickable throughout.
function renderSetupProviders() {
  const container = document.getElementById('setup-providers');
  container.replaceChildren(...PROVIDERS.map(provider =>
    buildSetupRow(provider, { status: '', message: 'Checking...' })));
  requestWindowResize();
  for (const provider of PROVIDERS) diagnoseProvider(provider);
}

// The one place the first-run screen is allowed to reach the network, and only
// because the user just asked it to. A provider that answers with real usage is
// enabled and the window switches to the live dashboard; anything else just
// updates that provider's row and leaves the configuration alone.
async function checkConnection(provider) {
  showSetupRow(provider, { status: '', message: 'Checking connection...' });
  try {
    const usage = await rpc(provider.rpcMethod);
    if (usage.status === 'connected') {
      config.providers[provider.id].enabled = true;
      saveCurrentConfig();
      renderProviderList();
      updateProvidersVisibility();
      return;
    }
    showSetupRow(provider, usage || {});
  } catch (error) {
    showSetupRow(provider, {
      status: 'temporary_error',
      message: 'Could not reach this provider. Retry in a moment.',
    });
  }
}

// ---------------------------------------------------------------------------
// Sample-data preview
//
// Lets anyone - a Store reviewer on a clean machine, or a curious new user -
// see the gauges, countdowns, thresholds and themes working without installing
// a CLI or signing in to anything. It is a separate screen fed by separate
// RPCs, not a mode layered over the real providers: nothing here reads or
// writes the provider settings, so leaving the preview restores exactly the
// setup the user came from.
// ---------------------------------------------------------------------------

function resetProviderRuntimeState() {
  for (const id of providerIds) {
    const state = providerState.get(id);
    clearTimeout(state.timerId);
    state.timerId = null;
    state.failureCount = 0;
    state.lastError = '';
    state.lastSuccessAt = 0;
    state.status = '';
    state.plan = '';
  }
}

function openSamplePreview() {
  viewMode = 'sample';
  resetProviderRuntimeState();
  updateProvidersVisibility();
}

function closeSamplePreview() {
  viewMode = 'live';
  resetProviderRuntimeState();
  updateProvidersVisibility();
}

document.getElementById('sample-preview-btn').addEventListener('click', openSamplePreview);
document.getElementById('back-to-setup-btn').addEventListener('click', closeSamplePreview);

document.querySelectorAll('.dialog-close').forEach(button => {
  button.addEventListener('click', () => button.closest('dialog').close('cancel'));
});

settingsDialog.addEventListener('click', event => {
  if (event.target === settingsDialog) settingsDialog.close();
});
settingsDialog.addEventListener('close', () => requestWindowResize());

document.getElementById('hide-window').addEventListener('click', () => {
  wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.HideToTray').catch(() => wails.Window.Hide());
});

function parseThresholdInput(inputEl, fallback, min, max) {
  const str = inputEl.value.trim();
  if (str === '') return fallback;
  const num = Number(str);
  if (!Number.isFinite(num)) return fallback;
  return Math.max(min, Math.min(max, Math.round(num)));
}

function saveThresholds() {
  const warningEnabled = warningEnabledInput.checked;
  const criticalEnabled = criticalEnabledInput.checked;

  warningThresholdInput.disabled = !warningEnabled;
  criticalThresholdInput.disabled = !criticalEnabled;

  let warningValue = parseThresholdInput(warningThresholdInput, config.thresholds.warning.value, 1, 100);
  let criticalValue = parseThresholdInput(criticalThresholdInput, config.thresholds.critical.value, 0, 99);

  if (warningEnabled && criticalEnabled && criticalValue >= warningValue) {
    criticalValue = Math.max(0, warningValue - 1);
  }

  warningThresholdInput.value = warningValue;
  criticalThresholdInput.value = criticalValue;

  config.thresholds = {
    warning: { enabled: warningEnabled, value: warningValue },
    critical: { enabled: criticalEnabled, value: criticalValue }
  };
  saveCurrentConfig();
  refreshLimitStates();
}

warningEnabledInput.addEventListener('change', saveThresholds);
criticalEnabledInput.addEventListener('change', saveThresholds);
warningThresholdInput.addEventListener('change', saveThresholds);
criticalThresholdInput.addEventListener('change', saveThresholds);

refreshPresetSelect.addEventListener('change', () => {
  if (refreshPresetSelect.value === 'custom') {
    refreshIntervalInput.focus();
    refreshIntervalInput.select();
    return;
  }
  setRefreshInterval(Number(refreshPresetSelect.value));
});

refreshIntervalInput.addEventListener('change', () => setRefreshInterval(refreshIntervalInput.value));

wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.GetVersion').then(version => {
  document.getElementById('version').textContent = version || 'v—';
});

// Covers launching with --sample-preview (see hack/screenshot.ps1), which
// opens the window straight into the preview so a screenshot run needs no
// live account. It picks a screen and nothing else - the saved provider
// settings are read and written exactly as they would be without the flag.
rpc('GetSamplePreviewAtStartup').then(open => {
  if (open) openSamplePreview();
}).catch(() => { });

let lastReportedHeight = 0;
let resizeTimer = null;
function requestWindowResize() {
  if (resizeTimer) cancelAnimationFrame(resizeTimer);
  resizeTimer = requestAnimationFrame(() => {
    const shell = document.querySelector('.shell');
    if (!shell) return;
    // .shell is height:100% of the window (so the SetContentHeight call
    // below can grow it later) - which means its own scrollHeight can never
    // report less than the window's current height, only more. Content that
    // shrinks (e.g. disabling a provider) would then never take effect: the
    // measurement stays pinned to the old, larger height forever. Force the
    // box to its natural content height for this one measurement, then
    // restore the CSS-declared height immediately after.
    const previousHeight = shell.style.height;
    shell.style.height = 'auto';
    let height = Math.ceil(shell.scrollHeight);
    shell.style.height = previousHeight;
    if (settingsDialog?.open) {
      // settingsDialog.scrollHeight, not getBoundingClientRect().height: the
      // dialog's own box is clamped by the browser to fit whatever room the
      // window currently has (that's the clipping this whole branch exists
      // to prevent), so its rendered height shrinks right along with that -
      // scrollHeight is the one measurement that still reports its true,
      // unclamped content height regardless. +2 for its own top/bottom
      // border (outside scrollHeight); +31/+12 is the top/bottom space
      // .settings-dialog reserves in style.css (clear of the main window's
      // .titlebar, plus a little breathing room at the bottom) - the window
      // needs to be at least that much taller than the dialog's natural
      // height for it to fit without being clipped on either side.
      const dialogHeight = Math.ceil(settingsDialog.scrollHeight) + 2 + 31 + 12;
      if (dialogHeight > height) height = dialogHeight;
    }
    if (height > 0 && Math.abs(height - lastReportedHeight) >= 2) {
      lastReportedHeight = height;
      wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetContentHeight', height).catch(() => { });
    }
  });
}

const resizeObserver = new ResizeObserver(() => {
  requestWindowResize();
});
resizeObserver.observe(document.querySelector('.shell'));

applyProviderOrder();

function refreshStatusTooltips() {
  for (const provider of PROVIDERS) {
    if (config.providers[provider.id].enabled) updateProviderStatus(provider.id);
  }
}

// Hover and focus are independent CSS states, so without this a mouse-hover
// tooltip and a Tab-focused tooltip could both be visible at once. Entering
// any trigger suppresses every other tooltip via a class that outranks the
// plain hover/focus display rules (see .tooltip-suppressed below); leaving
// lifts the suppression so whatever's still legitimately hovered/focused (if
// anything) can show again on its own.
function setActiveTooltip(tooltip) {
  document.querySelectorAll('.status-tooltip').forEach(element => {
    element.classList.toggle('tooltip-suppressed', element !== tooltip);
  });
}

function clearActiveTooltip() {
  document.querySelectorAll('.status-tooltip').forEach(element => {
    element.classList.remove('tooltip-suppressed');
  });
}

// A tooltip always opens downward from its anchor (never flips above) - the
// window instead grows to fit it, via the same requestWindowResize() call
// every other content change already uses. A tooltip is position:absolute,
// so it never changes .shell's own box size on its own (the ResizeObserver
// below only reacts to that), but requestWindowResize()'s own measurement
// picks up its extent regardless, so each show/hide needs its own explicit
// call, same as every other content change.
//
// The status tooltip is triggered by the whole provider heading. Tooltips
// ignore pointer events, so moving away from the heading closes them even
// when the pointer moves directly onto the tooltip box.
//
// Horizontally the cursor is the tooltip's right edge - it hangs down and to
// the left of the pointer, like a native tooltip would - clamped so it never
// slides past the heading's own edges. When there isn't enough room left of
// the cursor for the tooltip's full width (e.g. the cursor is near the
// heading's own left edge), the clamp just holds it against that edge
// instead; it can no longer track the cursor exactly, but that's fine here.
// .status-tooltip is sized to its own content (width: max-content, capped by
// max-width), so its width is measured live off the element rather than
// assumed - it differs per provider depending on how long that row's
// label/value text is. A keyboard-focused tooltip has no cursor position to
// follow, so it's left alone to keep the CSS default of dead-center
// (left: 50%; transform: translateX(-50%)) - positionTooltip overrides both
// of those inline, and mouseleave clears the overrides so a later
// keyboard-triggered show isn't left pinned at the last mouse position.
//
// The heading itself sits flush against the window's edges (see the
// negative-margin comment on .heading above), and .shell clips overflow-x -
// so the clamp also leaves EDGE_MARGIN of clearance beyond the tooltip's own
// box on each side, room for its box-shadow/backdrop-filter blur to render
// without getting cut off when the tooltip is pinned all the way to one end.
const EDGE_MARGIN = 12;
function positionTooltip(heading, tooltip, clientX) {
  const rect = heading.getBoundingClientRect();
  const width = tooltip.offsetWidth || rect.width;
  const desiredLeft = clientX - rect.left - width;
  const minLeft = EDGE_MARGIN;
  const maxLeft = Math.max(rect.width - width - EDGE_MARGIN, minLeft);
  tooltip.style.left = `${Math.min(Math.max(desiredLeft, minLeft), maxLeft)}px`;
  tooltip.style.transform = 'none';
}

document.querySelectorAll('.heading').forEach(element => {
  const tooltip = element.querySelector('.status-tooltip');
  element.addEventListener('mouseenter', event => {
    refreshStatusTooltips();
    setActiveTooltip(tooltip);
    positionTooltip(element, tooltip, event.clientX);
    requestWindowResize();
  });
  element.addEventListener('mousemove', event => {
    positionTooltip(element, tooltip, event.clientX);
  });
  element.addEventListener('mouseleave', () => {
    clearActiveTooltip();
    tooltip.style.left = '';
    tooltip.style.transform = '';
    requestWindowResize();
  });
});

document.querySelectorAll('.status-area').forEach(element => {
  element.addEventListener('focusin', () => {
    refreshStatusTooltips();
    const tooltip = element.querySelector('.status-tooltip');
    setActiveTooltip(tooltip);
    requestWindowResize();
  });
  element.addEventListener('focusout', event => {
    if (element.contains(event.relatedTarget)) return;
    clearActiveTooltip();
    requestWindowResize();
  });
});

// .inline-reset elements swap their text content in place to the full date
// on mouse hover, and revert back on mouse out.
document.addEventListener('mouseover', event => {
  const reset = event.target.closest?.('.inline-reset');
  if (reset && !reset.contains(event.relatedTarget) && reset.dataset.hoverText) {
    reset.textContent = reset.dataset.hoverText;
  }
});
document.addEventListener('mouseout', event => {
  const reset = event.target.closest?.('.inline-reset');
  if (reset && !reset.contains(event.relatedTarget) && reset.dataset.defaultText) {
    reset.textContent = reset.dataset.defaultText;
  }
});

setInterval(() => {
  if (document.visibilityState === 'visible' && document.querySelector('.heading:hover, .status-area :focus-visible')) {
    refreshStatusTooltips();
  }
}, 1000);
