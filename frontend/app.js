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

const validThemes = new Set(['light', 'dark', 'system']);
const settingsStorageKey = 'aigauge-settings-v1';

function parseIntervalToSeconds(val) {
  if (typeof val === 'number') {
    return Number.isFinite(val) ? Math.max(1, Math.min(3600, Math.round(val))) : 120;
  }
  if (typeof val === 'string') {
    const str = val.trim().toLowerCase();
    let total = 0;
    let matched = false;
    const mMatch = str.match(/(\d+)\s*m/);
    const sMatch = str.match(/(\d+)\s*s/);
    if (mMatch) { total += parseInt(mMatch[1], 10) * 60; matched = true; }
    if (sMatch) { total += parseInt(sMatch[1], 10); matched = true; }
    if (matched) return Math.max(1, Math.min(3600, total));
    const rawNum = parseInt(str, 10);
    if (Number.isFinite(rawNum)) return Math.max(1, Math.min(3600, rawNum));
  }
  return 120;
}

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
    cardId: 'codex-card', groupsId: 'codex-groups',
    dotId: 'codex-dot', tooltipId: 'codex-tooltip', errorId: 'codex-error',
    render: renderUsage
  },
  {
    id: 'claude', label: 'Claude', rpcMethod: 'GetClaudeUsage',
    cardId: 'claude-card', groupsId: 'claude-groups',
    dotId: 'claude-dot', tooltipId: 'claude-tooltip', errorId: 'claude-error',
    render: renderClaude
  },
  {
    id: 'antigravity', label: 'Antigravity', rpcMethod: 'GetAntigravityUsage',
    cardId: 'agy-card', groupsId: 'agy-groups',
    dotId: 'agy-dot', tooltipId: 'agy-tooltip', errorId: 'agy-error',
    render: renderAntigravity
  }
];
const PROVIDERS_BY_ID = new Map(PROVIDERS.map(p => [p.id, p]));
const providerIds = PROVIDERS.map(p => p.id);

const defaultConfig = {
  providers: Object.fromEntries(providerIds.map(id => [id, { enabled: true }])),
  providerOrder: providerIds.slice(),
  theme: 'system',
  refreshInterval: 120,
  thresholds: {
    warning: 50,
    critical: 20
  }
};

function normalizeProviderOrder(value) {
  const known = Array.isArray(value) ? value.filter(id => providerIds.includes(id)) : [];
  const order = [...new Set(known)];
  for (const id of providerIds) {
    if (!order.includes(id)) order.push(id);
  }
  return order;
}

function normalizeConfig(value) {
  const warningValue = Number(value?.thresholds?.warning);
  const criticalValue = Number(value?.thresholds?.critical);
  const warning = Number.isFinite(warningValue) && warningValue >= 1 && warningValue <= 100
    ? warningValue : defaultConfig.thresholds.warning;
  let critical = Number.isFinite(criticalValue) && criticalValue >= 0 && criticalValue <= 99
    ? criticalValue : defaultConfig.thresholds.critical;
  if (critical >= warning) critical = Math.max(0, warning - 1);

  const theme = value?.theme === 'auto' ? 'system' : value?.theme;
  return {
    providers: Object.fromEntries(providerIds.map(id => [id, { enabled: value?.providers?.[id]?.enabled !== false }])),
    providerOrder: normalizeProviderOrder(value?.providerOrder),
    theme: validThemes.has(theme) ? theme : defaultConfig.theme,
    refreshInterval: parseIntervalToSeconds(value?.refreshInterval),
    thresholds: { warning, critical }
  };
}

let config = normalizeConfig(defaultConfig);

try {
  const storedSettings = localStorage.getItem(settingsStorageKey);
  if (storedSettings !== null) {
    config = normalizeConfig(JSON.parse(storedSettings));
  }
} catch (e) {
  console.warn('Failed to load settings:', e);
  config = normalizeConfig(defaultConfig);
}

let warningThreshold = config.thresholds.warning;
let criticalThreshold = config.thresholds.critical;

function applyLimitState(barElement, remaining) {
  const state = remaining <= criticalThreshold ? 'critical'
    : remaining <= warningThreshold ? 'warning' : '';
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
// a `fetchedAt` (see hack/gensample and internal/providers), set right
// before that fetch went out. For a live fetch the two are milliseconds
// apart (network latency, basically), so this changes nothing normal users
// would notice. It matters for fixture data - the live-server preview
// (hack/live-server.ps1) and the native app run with --fixtures= (see
// hack/screenshot.ps1) both render whatever `.\build.ps1 fixtures` last
// saved - which can be arbitrarily old by the time it's viewed: without
// anchoring to fetchedAt, a Codex reset (reported as a relative "seconds
// from now") would silently push further into the future every reload, and
// an absolute reset time (Claude/Antigravity) could drift into the past and
// render as already-elapsed.
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

function retryDelay(failureCount) {
  return Math.min(refreshInterval * (2 ** Math.min(failureCount, 4)), 1800);
}

function scheduleProvider(id) {
  const state = providerState.get(id);
  clearTimeout(state.timerId);
  if (!config.providers[id].enabled) return;
  const delay = retryDelay(state.failureCount);
  state.nextRefreshAt = Date.now() + delay * 1000;
  state.timerId = setTimeout(() => fetchProvider(id), delay * 1000);
}

function setLoading(dotId, loading) {
  document.getElementById(dotId).classList.toggle('loading', loading);
}

function clearLoadingText(cardId) {
  document.querySelectorAll(`#${cardId} .loading-text`).forEach(element => element.classList.remove('loading-text'));
}

function showProviderError(errorId, message) {
  const element = document.getElementById(errorId);
  element.textContent = message;
  element.hidden = !message;
}

function renderUsage(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const groups = document.getElementById(meta.groupsId);
  if (usage.error) {
    groups.replaceChildren();
    showProviderError(meta.errorId, usage.error);
    updateProviderStatus(id);
    requestWindowResize();
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
    groups.replaceChildren();
    showProviderError(meta.errorId, usage.error);
    updateProviderStatus(id);
    requestWindowResize();
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
    groups.replaceChildren();
    showProviderError(meta.errorId, usage.error);
    updateProviderStatus(id);
    requestWindowResize();
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
  if (!config.providers[id].enabled || state.fetching) return;
  state.fetching = true;
  setLoading(meta.dotId, true);
  try {
    const usage = await wails.Call.ByName(`github.com/jmnote/aigauge/internal/app.App.${meta.rpcMethod}`);
    clearLoadingText(meta.cardId);
    if (usage.error) {
      state.failureCount += 1;
      state.lastError = String(usage.error).slice(0, 120);
    } else {
      state.failureCount = 0;
      state.lastSuccessAt = Date.now();
      state.lastError = '';
    }
    meta.render(id, usage);
  } catch (error) {
    clearLoadingText(meta.cardId);
    state.failureCount += 1;
    state.lastError = `Frontend call failed: ${error}`.slice(0, 120);
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
const warningThresholdInput = document.getElementById('warning-threshold');
const criticalThresholdInput = document.getElementById('critical-threshold');
const systemTheme = matchMedia('(prefers-color-scheme: dark)');

let isAlwaysOnTop = false;

function updateAlwaysOnTopUI(isTop) {
  pinWindowBtn.classList.toggle('active', isTop);
  pinWindowBtn.title = isTop ? 'Always on top (Enabled)' : 'Always on top (Disabled)';
  pinWindowBtn.setAttribute('aria-pressed', isTop ? 'true' : 'false');
}

function toggleAlwaysOnTop() {
  isAlwaysOnTop = !isAlwaysOnTop;
  updateAlwaysOnTopUI(isAlwaysOnTop);
  wails.Window.SetAlwaysOnTop(isAlwaysOnTop);
  wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetAlwaysOnTop', isAlwaysOnTop).catch(() => {});
}

pinWindowBtn.addEventListener('click', toggleAlwaysOnTop);
updateAlwaysOnTopUI(isAlwaysOnTop);

function updateProvidersVisibility() {
  const noProviders = document.getElementById('no-providers');
  let anyEnabled = false;

  for (const provider of PROVIDERS) {
    const enabled = config.providers[provider.id].enabled !== false;
    anyEnabled = anyEnabled || enabled;
    document.getElementById(provider.cardId).style.display = enabled ? '' : 'none';

    const state = providerState.get(provider.id);
    if (enabled) {
      if (!state.timerId) fetchProvider(provider.id);
    } else {
      clearTimeout(state.timerId);
      state.timerId = null;
    }
  }
  noProviders.style.display = anyEnabled ? 'none' : 'flex';

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
applyTheme(validThemes.has(activeTheme) ? activeTheme : 'system', !forcedTheme);

systemTheme.addEventListener('change', () => {
  if (document.documentElement.dataset.theme === 'system') applyTheme('system', false);
});

function updateRefreshPresetSelect(seconds) {
  const matchingOption = refreshPresetSelect.querySelector(`option[value="${seconds}"]`);
  refreshPresetSelect.value = matchingOption ? String(seconds) : 'custom';
}

function setRefreshInterval(val) {
  let seconds = Number(val);
  seconds = Number.isFinite(seconds) ? Math.max(1, Math.min(3600, Math.round(seconds))) : 120;
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
  warningThresholdInput.value = warningThreshold;
  criticalThresholdInput.value = criticalThreshold;
  settingsDialog.showModal();
  requestWindowResize();
}

document.getElementById('settings').addEventListener('click', openSettings);
document.getElementById('open-settings-btn').addEventListener('click', openSettings);

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

function saveThresholds() {
  if (warningThresholdInput.value === '' || criticalThresholdInput.value === '') return;
  const warningValue = Number(warningThresholdInput.value);
  const criticalValue = Number(criticalThresholdInput.value);
  if (!Number.isFinite(warningValue) || !Number.isFinite(criticalValue)) return;
  warningThreshold = Math.max(1, Math.min(100, warningValue));
  criticalThreshold = Math.max(0, Math.min(99, criticalValue));
  if (criticalThreshold >= warningThreshold) {
    criticalThreshold = Math.max(0, warningThreshold - 1);
  }
  warningThresholdInput.value = warningThreshold;
  criticalThresholdInput.value = criticalThreshold;
  config.thresholds.warning = warningThreshold;
  config.thresholds.critical = criticalThreshold;
  saveCurrentConfig();
  refreshLimitStates();
}

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
      wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetContentHeight', height).catch(() => {});
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
