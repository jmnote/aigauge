import {
  parseIntervalToSeconds, normalizeConfig, VALID_THEMES,
  shouldCountFailure, shouldScheduleRetry, isExpectedSetupState,
  shouldKeepStaleData, retryDelay, providerVisibilityAction,
  normalizeWindowWidth, providerTypeLabel, shouldShowProviderUser, DEFAULT_REFRESH_SECONDS,
} from '/logic.mjs';
import { createDropdown } from '/ui/dropdown.mjs';

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

const rpc = (method, ...args) =>
  wails.Call.ByName(`github.com/jmnote/aigauge/internal/app.App.${method}`, ...args);

// Pending instances belong to the settings window's in-progress add flow.
// Keep them in the backend response so that flow can be committed or removed,
// but never create a main-window card for them.
function normalizeMainConfig(raw) {
  const visible = raw && typeof raw === 'object' && Array.isArray(raw.providers)
    ? { ...raw, providers: raw.providers.filter(instance => !instance?.pending) }
    : raw;
  return normalizeConfig(visible);
}

// The Wails-RPC method names for each provider *type*. A provider *instance*
// (an entry in config.providers) carries only its type id and its own id;
// this is what turns those into the calls used to diagnose, fetch and render
// it. Render functions are declared later in this file as `function`
// statements, so they're already hoisted by the time this literal runs.
// Every provider's usage RPC now returns the same DisplayUsage shape, so
// one renderUsage (below) draws every card - there's no per-type render
// function to look up here anymore.
const RPC_BY_TYPE = {
  codex: { rpcMethod: 'GetCodexUsage', diagnoseRpcMethod: 'DiagnoseCodex' },
  claude: { rpcMethod: 'GetClaudeUsage', diagnoseRpcMethod: 'DiagnoseClaude' },
  antigravity: { rpcMethod: 'GetAntigravityUsage', diagnoseRpcMethod: 'DiagnoseAntigravity' },
  copilot: { rpcMethod: 'GetCopilotUsage', diagnoseRpcMethod: 'DiagnoseCopilot' },
};


// Turns one provider instance from config.providers into the full set of
// per-card details the rendering code needs (RPC method names, the element
// ids the card's pieces get, which render function draws its usage).
function buildMeta(instance) {
  return {
    id: instance.id, type: instance.type, label: instance.label,
    displayLabel: providerTypeLabel(instance.type),
    ...RPC_BY_TYPE[instance.type],
    cardId: `provider-card-${instance.id}`,
    userId: `provider-user-${instance.id}`,
    creditsId: `provider-credits-${instance.id}`,
    groupsId: `provider-groups-${instance.id}`,
    dotId: `provider-dot-${instance.id}`,
    statusCardId: `provider-status-card-${instance.id}`,
    errorId: `provider-error-${instance.id}`,
  };
}

// Rebuilt every time config.providers changes (see syncProviderCards).
const PROVIDERS_BY_ID = new Map();
// The live DOM node for each provider instance's card, so syncProviderCards
// can reorder/show/hide/remove them without rebuilding one that still exists.
const cardElements = new Map();

// Settings (the provider instance list/order/enabled state, theme, refresh
// interval, thresholds and hotkey) are owned by the Go backend rather than
// this window's localStorage - see internal/config and App.GetSettings.
// Both this window and the settings window write field-level changes through
// those RPCs and are kept in sync by the "aigauge:config-updated" event the
// backend emits whenever either one saves a change (see applyExternalConfig).
let config;
try {
  config = normalizeMainConfig(await rpc('GetSettings'));
} catch (e) {
  console.warn('Failed to load settings:', e);
    config = normalizeMainConfig({});
}

let settingsWriteQueue = Promise.resolve();
function saveTheme(theme) {
  settingsWriteQueue = settingsWriteQueue
    .then(() => rpc('SetTheme', theme))
    .catch(e => console.warn('Failed to save theme:', e));
  return settingsWriteQueue;
}

// Refetches settings from the backend and re-renders from them. Used after
// an RPC (Connect or Import) that changes the persisted provider
// list on its own, so this window shows exactly what was saved rather than a
// locally guessed patch that could drift from it.
async function reloadConfigFromBackend() {
  try {
    applyExternalConfig(await rpc('GetSettings'));
  } catch (e) {
    console.warn('Failed to reload settings:', e);
  }
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
// than whenever it happens to be rendered - every DisplayUsage payload
// (internal/providers) carries a `fetchedAt`, set right before that fetch
// went out. For a live fetch the two are milliseconds apart (network
// latency, basically), so this changes nothing normal users would notice.
  // It matters for the live-server fixture browser (hack/live-server.mjs), which
// serves whatever `.\build.ps1 fixtures-usage` last captured as-is, with no
// correction - that can be arbitrarily old by the time it's viewed: without
// anchoring to fetchedAt, an absolute reset time could drift into the past
  // and render as already-elapsed.
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

// One record per provider instance instead of a parallel `let` per field.
// Populated lazily as instances appear (see ensureProviderState) rather than
// from a fixed list, since instances are added and removed at runtime.
const providerState = new Map();

function ensureProviderState(id) {
  if (!providerState.has(id)) {
    providerState.set(id, {
      nextRefreshAt: Date.now() + (findInstance(id)?.refreshInterval || DEFAULT_REFRESH_SECONDS) * 1000,
      timerId: null,
      fetching: false,
      failureCount: 0,
      lastSuccessAt: 0,
      lastError: '',
      status: '',
      user: '',
      email: '',
      displayName: '',
      plan: '',
      resetCredits: null,
    });
  }
  return providerState.get(id);
}

function findInstance(id) {
  return config.providers.find(p => p.id === id);
}

function updateStatus(id, dotId, statusCardId, status, failureCount, lastSuccessAt, nextRefreshAt, lastError, plan, resetCredits, email, displayName) {
  const dot = document.getElementById(dotId);
  dot.classList.remove('connected', 'warning');
  // An expected setup state (login_required, not_installed, ...) is never
  // "still connected", no matter how long ago the last real success was:
  // failureCount is deliberately never incremented for these states (see
  // shouldCountFailure) and lastSuccessAt is never cleared, so without this
  // gate a provider whose session expired kept a permanently green dot.
  const stale = lastSuccessAt && !isExpectedSetupState(status);
  if (stale && failureCount < 3) dot.classList.add('connected');
  else if (stale && failureCount < 6) dot.classList.add('warning');

  const statusCard = document.getElementById(statusCardId);
  const successValue = lastSuccessAt ? formatAgo(lastSuccessAt) : 'None';
  const nextRefreshText = formatUntil(nextRefreshAt);
  statusCard.replaceChildren(
    ...(email ? [createTooltipRow('Email', email)] : []),
    ...(displayName ? [createTooltipRow('Display name', displayName)] : []),
    ...(plan ? [createTooltipRow('Plan', plan)] : []),
    ...(Number.isFinite(resetCredits) ? [createTooltipRow('Reset credits', `${resetCredits}`)] : []),
    createTooltipRow('Fetch fails', `${failureCount}`),
    createTooltipRow('Last fetch', successValue),
    ...(lastError ? [createTooltipRow('Last error', lastError)] : []),
    createTooltipRow('Next fetch', nextRefreshText),
    buildStatusActions(id),
  );
}

function updateProviderStatus(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  updateStatus(id, meta.dotId, meta.statusCardId, state.status, state.failureCount, state.lastSuccessAt, state.nextRefreshAt, state.lastError, state.plan, state.resetCredits, state.email, state.displayName);
}

// The dot dropdown's refresh action.
function buildStatusActions(id) {
  const actions = document.createElement('div');
  actions.className = 'status-card-actions';

  const refreshItem = document.createElement('button');
  refreshItem.type = 'button';
  refreshItem.className = 'provider-menu-item';
  refreshItem.title = 'Refresh';
  refreshItem.setAttribute('aria-label', 'Refresh');
  const refreshIcon = Object.assign(document.createElement('span'), { className: 'icon icon-refresh' });
  refreshIcon.setAttribute('aria-hidden', 'true');
  refreshItem.append(refreshIcon);
  refreshItem.addEventListener('click', event => {
    event.stopPropagation();
    statusDropdowns.get(id)?.close();
    fetchProvider(id);
  });

  actions.append(refreshItem);
  return actions;
}

function createTooltipRow(labelText, valueText) {
  const row = document.createElement('div');
  row.className = 'status-card-row';
  const label = document.createElement('span');
  label.className = 'status-card-label';
  label.textContent = labelText;
  const value = document.createElement('span');
  value.className = 'status-card-value';
  renderFormattedMessage(value, valueText);
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
  if (!shouldScheduleRetry(state.status)) return;
  const delay = retryDelay(state.failureCount, findInstance(id)?.refreshInterval || DEFAULT_REFRESH_SECONDS);
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
          e.stopPropagation();
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

// Copies text for the "Copy" button on a details disclosure (e.g. the raw
// "HTTP 404 Not Found" behind a provider error), so it can be pasted into a
// bug report without retyping it. navigator.clipboard is preferred, but a
// WebView2 page is not guaranteed to grant it, so a hidden-textarea +
// execCommand fallback covers that case too.
async function copyTextToClipboard(text) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to the legacy fallback below.
    }
  }
  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.top = '-1000px';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const ok = document.execCommand('copy');
    textarea.remove();
    return ok;
  } catch {
    return false;
  }
}

// Icons live as files under frontend/icons/ (see the .icon-* classes in
// styles/style.css) rather than built inline here, so every icon in the app -
// titlebar buttons included - comes from the same place and picks up
// currentColor (e.g. .details-copy-btn.is-copied) the same way.
function createIcon(name) {
  const span = document.createElement('span');
  span.className = `icon icon-${name}`;
  span.setAttribute('aria-hidden', 'true');
  return span;
}

function closeOpenDetailsTooltips() {
  let changed = false;
  document.querySelectorAll('.details-wrap.details-open').forEach(el => {
    el.classList.remove('details-open');
    const btn = el.querySelector('.details-info-btn');
    if (btn) btn.setAttribute('aria-expanded', 'false');
    changed = true;
  });
  if (changed) requestWindowResize();
}

function createDiagnosisActions(diagnosis, onCheck, onConnect, onCancel, onSubmitCode) {
  const actions = document.createElement('div');
  actions.className = 'setup-provider-actions';

  if (diagnosis.status === 'authenticating') {
    const cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', async () => {
      cancelBtn.disabled = true;
      if (onCancel) await onCancel();
    });
    actions.append(cancelBtn);
    return [actions];
  }

  if (diagnosis.status === 'awaiting_code') {
    // A manual-code provider (Claude) redirects to its own hosted page
    // rather than back to AI Gauge, so instead of blocking until login
    // finishes, this collects the code the user pastes back in here and
    // sends it via onSubmitCode (SubmitAuthCode).
    const form = document.createElement('form');
    form.className = 'awaiting-code-form';

    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'awaiting-code-input';
    input.placeholder = 'Paste code here';
    input.autocomplete = 'off';
    input.spellcheck = false;

    const submitBtn = document.createElement('button');
    submitBtn.type = 'submit';
    submitBtn.className = 'connect-btn';
    submitBtn.textContent = 'Submit';

    const cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', async () => {
      cancelBtn.disabled = true;
      if (onCancel) await onCancel();
    });

    form.addEventListener('submit', async event => {
      event.preventDefault();
      const code = input.value.trim();
      if (!code) return;
      input.disabled = true;
      submitBtn.disabled = true;
      cancelBtn.disabled = true;
      if (onSubmitCode) await onSubmitCode(code);
    });

    form.append(input, submitBtn, cancelBtn);
    actions.append(form);
    return [actions];
  }

  const primary = document.createElement('button');
  primary.type = 'button';
  const isConnectAction = diagnosis.status === 'login_required';

  if (isConnectAction) {
    primary.textContent = 'Connect';
    primary.classList.add('connect-btn');
  } else if (diagnosis.status === 'auth_check_required') {
    primary.textContent = 'Check connection';
  } else {
    primary.textContent = 'Check again';
  }
  actions.append(primary);

  let detailsWrap = null;
  let detailsBtn = null;

  if (diagnosis.details) {
    detailsWrap = document.createElement('span');
    detailsWrap.className = 'details-wrap';

    detailsBtn = document.createElement('button');
    detailsBtn.type = 'button';
    detailsBtn.className = 'details-info-btn';
    detailsBtn.title = 'Details';
    detailsBtn.setAttribute('aria-label', 'Details');
    detailsBtn.setAttribute('aria-expanded', 'false');
    detailsBtn.append(createIcon('info'));

    const tooltip = document.createElement('div');
    tooltip.className = 'details-tooltip';
    tooltip.setAttribute('role', 'tooltip');
    // Details contain provider/API output and are not an application-owned
    // message format. Keep them as plain text so response bodies cannot turn
    // into clickable links or protocol-handler launches.
    tooltip.textContent = diagnosis.details;

    const copyBtn = document.createElement('button');
    copyBtn.type = 'button';
    copyBtn.className = 'details-copy-btn';
    copyBtn.title = 'Copy';
    copyBtn.setAttribute('aria-label', 'Copy details');
    copyBtn.append(createIcon('copy'));
    let copyResetTimer = null;
    copyBtn.addEventListener('click', async e => {
      e.stopPropagation();
      const ok = await copyTextToClipboard(diagnosis.details);
      copyBtn.replaceChildren(createIcon('check'));
      copyBtn.classList.toggle('is-copied', ok);
      copyBtn.title = ok ? 'Copied!' : 'Copy failed';
      clearTimeout(copyResetTimer);
      copyResetTimer = setTimeout(() => {
        copyBtn.replaceChildren(createIcon('copy'));
        copyBtn.classList.remove('is-copied');
        copyBtn.title = 'Copy';
      }, 1500);
    });
    tooltip.append(copyBtn);

    detailsBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      const wasOpen = detailsWrap.classList.contains('details-open');
      closeOpenDetailsTooltips();
      if (!wasOpen) {
        detailsWrap.classList.add('details-open');
        detailsBtn.setAttribute('aria-expanded', 'true');
      }
      requestWindowResize();
    });

    tooltip.addEventListener('click', (e) => {
      e.stopPropagation();
    });

    detailsWrap.addEventListener('mouseenter', () => requestWindowResize());
    detailsWrap.addEventListener('mouseleave', () => {
      if (!detailsWrap.classList.contains('details-open')) {
        requestWindowResize();
      }
    });

    detailsWrap.append(detailsBtn, tooltip);
    actions.append(detailsWrap);
  }

  primary.addEventListener('click', async () => {
    if (primary.disabled) return;
    primary.disabled = true;
    closeOpenDetailsTooltips();
    if (detailsWrap) {
      detailsWrap.style.display = 'none';
    }
    try {
      if (isConnectAction && onConnect) {
        await onConnect();
      } else if (onCheck) {
        await onCheck();
      }
    } finally {
      if (primary.isConnected) {
        primary.disabled = false;
        if (detailsWrap) {
          detailsWrap.style.display = '';
          detailsWrap.style.animation = 'none';
          void detailsWrap.offsetWidth;
          detailsWrap.style.animation = '';
        }
      }
    }
  });

  return [actions];
}

function showProviderError(errorId, message, diagnosis, onCheck, onConnect, onCancel, onSubmitCode) {
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

  if (diagnosis && diagnosis.status && (onCheck || onConnect || onSubmitCode)) {
    element.append(...createDiagnosisActions(diagnosis, onCheck, onConnect, onCancel, onSubmitCode));
  }
}

// Draws a provider that came back with something other than usable numbers.
//
// A temporary network or service failure is the one case that keeps whatever
// was already on screen: the data is stale, not wrong, and blanking the card
// throws away the only thing the user opened the app to see. It is labelled
// with its age so nobody reads month-old numbers as current. Every other
// non-connected state (not logged in, connection not checked) has no
// prior data to keep, so its card clears to the guidance instead.
function renderNonUsageState(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const message = String(usage.message || usage.error || '');
  const onCheck = () => fetchProvider(id);
  const onConnect = () => connectExistingInstance(id, usage);
  const onCancel = async () => {
    await rpc('CancelAuth', id);
    fetchProvider(id);
  };
  // Only reachable when usage.status is 'awaiting_code' - see
  // connectExistingInstance, which is the only place that status is ever set.
  const onSubmitCode = code => submitAuthCodeForInstance(id, code);
  if (shouldKeepStaleData(usage.status, state.lastSuccessAt)) {
    showProviderError(meta.errorId, `${message} Showing data from ${formatAgo(state.lastSuccessAt)}.`, usage, onCheck, onConnect, onCancel, onSubmitCode);
  } else {
    state.user = '';
    state.email = '';
    state.displayName = '';
    state.resetCredits = null;
    updateProviderUser(id);
    updateResetCredits(id);
    document.getElementById(meta.groupsId).replaceChildren();
    showProviderError(meta.errorId, message, usage, onCheck, onConnect, onCancel, onSubmitCode);
  }
  updateProviderStatus(id);
  requestWindowResize();
}

// Renders any provider's usage: DisplayUsage is already fully resolved on
// the Go side (absolute reset times, sorted/labeled buckets), so this just
// lays groups out - flat (no title) for a single unnamed group, as a titled
// section otherwise.
function renderUsage(id, usage) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  const container = document.getElementById(meta.groupsId);
  if (usage.error) {
    renderNonUsageState(id, usage);
    return;
  }
  state.user = typeof usage.user === 'string' ? usage.user.trim() : '';
  state.email = typeof usage.email === 'string' ? usage.email.trim() : '';
  state.displayName = typeof usage.displayName === 'string' ? usage.displayName.trim() : '';
  updateProviderUser(id);
  showProviderError(meta.errorId, '');
  state.plan = usage.plan || '';
  state.resetCredits = Number.isFinite(usage.resetCredits) ? usage.resetCredits : null;
  updateResetCredits(id);
  updateProviderStatus(id);
  container.replaceChildren();
  if (!usage.groups?.length) {
    requestWindowResize();
    return;
  }
  const nowMs = referenceNow(usage);
  const flat = usage.groups.length === 1 && !usage.groups[0].name;
  for (const group of usage.groups) {
    const target = flat ? container : appendGroupElement(container, group.name);
    renderBuckets(target, group.buckets || [], nowMs);
  }
  requestWindowResize();
}

function updateProviderUser(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const user = document.getElementById(meta?.userId);
  if (!meta || !user) return;
  const value = providerState.get(id)?.user || '';
  user.textContent = value;
  user.hidden = !shouldShowProviderUser(config.providers, meta.type, value);
}

function updateResetCredits(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const element = document.getElementById(meta?.creditsId);
  if (!meta || !element) return;
  const credits = providerState.get(id)?.resetCredits;
  const visible = Number.isFinite(credits) && credits > 0;
  element.hidden = !visible;
  if (visible) {
    element.querySelector('.reset-credits-count').textContent = `${credits}`;
  }
}

function appendGroupElement(container, name) {
  const groupElement = document.createElement('div');
  groupElement.className = 'usage-group';
  if (name) {
    const title = document.createElement('div');
    title.className = 'usage-group-title';
    title.textContent = name;
    groupElement.append(title);
  }
  container.append(groupElement);
  return groupElement;
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

async function fetchProvider(id) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  if (state.fetching) return;
  state.fetching = true;
  setLoading(meta.dotId, true);
  try {
    const usage = await rpc(meta.rpcMethod, meta.id);
    clearLoadingText(meta.cardId);
    state.status = usage.status || '';
    if (state.status && state.status !== 'connected') {
      state.lastError = String(usage.message || usage.error || '').slice(0, 120);
      // Only a genuine failure moves the counter - see EXPECTED_SETUP_STATES.
      if (shouldCountFailure(state.status)) state.failureCount += 1;
    } else if (usage.error) {
      state.failureCount += 1;
      state.lastError = String(usage.error).slice(0, 120);
    } else {
      state.failureCount = 0;
      state.lastSuccessAt = Date.now();
      state.lastError = '';
    }
    renderUsage(id, usage);
  } catch (error) {
    clearLoadingText(meta.cardId);
    state.status = '';
    state.failureCount += 1;
    state.lastError = `Frontend call failed: ${error}`.slice(0, 120);
    renderUsage(id, { error: state.lastError });
  } finally {
    state.fetching = false;
    setLoading(meta.dotId, false);
    scheduleProvider(id);
  }
}

const pinWindowBtn = document.getElementById('pin-window');
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
  wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetAlwaysOnTop', isAlwaysOnTop).catch(() => { });
}

pinWindowBtn.addEventListener('click', toggleAlwaysOnTop);
updateAlwaysOnTopUI(isAlwaysOnTop);

// One card's dot-button dropdown: status details and actions are rendered in
// the card flow directly below the provider heading.
const statusDropdowns = new Map();

// Builds the DOM for one provider instance's usage card: heading (name,
// status dot/dropdown), the status card, usage groups, and error area.
// Interaction wiring is attached here at creation time rather than through a
// delegated or one-time document-wide listener, since cards themselves are
// created and destroyed as provider instances are added and removed.
function buildProviderCard(meta) {
  const section = document.createElement('section');
  section.className = 'usage-card';
  section.id = meta.cardId;

  const providerHeader = document.createElement('div');
  providerHeader.className = 'provider-header';
  const heading = document.createElement('div');
  heading.className = 'heading';
  const nameArea = document.createElement('span');
  nameArea.className = 'provider-name-area';
  const name = document.createElement('strong');
  name.textContent = meta.displayLabel;
  const user = document.createElement('span');
  user.className = 'provider-user';
  user.id = meta.userId;
  user.title = 'Account user';
  user.hidden = true;
  nameArea.append(name, user);

  const statusArea = document.createElement('span');
  statusArea.className = 'status-area';
  const credits = document.createElement('span');
  credits.className = 'reset-credits';
  credits.id = meta.creditsId;
  credits.title = 'Reset credits';
  credits.setAttribute('aria-label', 'Reset credits');
  credits.hidden = true;
  const creditsIcon = document.createElement('span');
  creditsIcon.className = 'icon icon-bolt reset-credits-icon';
  creditsIcon.setAttribute('aria-hidden', 'true');
  const creditsCount = document.createElement('span');
  creditsCount.className = 'reset-credits-count';
  credits.append(creditsIcon, creditsCount);
  const statusWrap = document.createElement('span');
  statusWrap.className = 'status-wrap';

  const dotBtn = document.createElement('button');
  dotBtn.type = 'button';
  dotBtn.className = 'status-dot-btn';
  dotBtn.setAttribute('aria-haspopup', 'true');
  dotBtn.setAttribute('aria-expanded', 'false');
  dotBtn.setAttribute('aria-label', `${meta.label} connection status and actions`);
  const dot = document.createElement('span');
  dot.className = 'status-dot';
  dot.id = meta.dotId;
  dotBtn.append(dot);

  statusWrap.append(dotBtn);

  const statusCard = document.createElement('div');
  statusCard.className = 'provider-status-card';
  statusCard.id = meta.statusCardId;
  statusCard.hidden = true;

  const dropdown = createDropdown(providerHeader, {
    onOpen: () => {
      closeOpenDetailsTooltips();
      updateProviderStatus(meta.id);
      statusCard.hidden = false;
      dotBtn.setAttribute('aria-expanded', 'true');
      requestWindowResize();
    },
    onClose: () => {
      statusCard.hidden = true;
      dotBtn.setAttribute('aria-expanded', 'false');
      requestWindowResize();
    },
  });
  statusDropdowns.set(meta.id, dropdown);
  dotBtn.addEventListener('click', event => {
    event.stopPropagation();
    dropdown.toggle();
  });

  statusArea.append(credits, statusWrap);
  heading.append(nameArea, statusArea);
  providerHeader.append(heading, statusCard);

  const groups = document.createElement('div');
  groups.className = 'usage-groups';
  groups.id = meta.groupsId;
  const loading = document.createElement('div');
  loading.className = 'plan loading-text';
  loading.textContent = 'Loading usage...';
  groups.append(loading);

  const error = document.createElement('div');
  error.className = 'provider-error';
  error.id = meta.errorId;
  error.setAttribute('role', 'status');
  error.hidden = true;

  section.append(providerHeader, groups, error);

  return section;
}

// Reconciles the DOM with config.providers: creates a card (and its state)
// for any new instance, removes one for any instance that no longer exists,
// and reorders the surviving cards to match. Called whenever config changes
// - locally, or via the "aigauge:config-updated" event from the backend or
// the settings window - so this is the one place card lifecycle is decided.
function syncProviderCards() {
  const usageSections = document.querySelector('.usage-sections');
  const currentIds = new Set(config.providers.map(p => p.id));

  for (const [id, section] of cardElements) {
    if (currentIds.has(id)) continue;
    const state = providerState.get(id);
    if (state) clearTimeout(state.timerId);
    providerState.delete(id);
    PROVIDERS_BY_ID.delete(id);
    statusDropdowns.get(id)?.close();
    statusDropdowns.delete(id);
    section.remove();
    cardElements.delete(id);
  }

  for (const instance of config.providers) {
    const meta = buildMeta(instance);
    PROVIDERS_BY_ID.set(instance.id, meta);
    let section = cardElements.get(instance.id);
    if (!section) {
      section = buildProviderCard(meta);
      cardElements.set(instance.id, section);
      ensureProviderState(instance.id);
    }
    updateProviderUser(instance.id);
    updateResetCredits(instance.id);
    usageSections.append(section);
  }

  updateProvidersVisibility();
}

function updateProvidersVisibility() {
  // Every instance in config.providers always gets a card - presence in the
  // list is the only "is it shown" signal (removing it is the only way to
  // hide it), so there's nothing here to hide by instance.
  for (const instance of config.providers) {
    const state = providerState.get(instance.id);
    const action = providerVisibilityAction(Boolean(state.timerId));
    if (action === 'fetch') {
      fetchProvider(instance.id);
    }
  }

  const showSetup = config.providers.length === 0;
  document.getElementById('setup-screen').hidden = !showSetup;

  const visibleCards = config.providers.map(instance => cardElements.get(instance.id));
  visibleCards.forEach((card, index) => {
    card.classList.toggle('card-divider', index > 0);
  });

  requestWindowResize();
}

function applyTheme(theme, persist = true) {
  const resolved = theme === 'system'
    ? (systemTheme.matches ? 'dark' : 'light')
    : (theme === 'dark' ? 'dark' : 'light');
  document.documentElement.dataset.theme = theme;
  document.documentElement.dataset.resolvedTheme = resolved;
  if (persist) {
    config.theme = theme;
    saveTheme(theme);
  }
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

// ---------------------------------------------------------------------------
// Settings button
// ---------------------------------------------------------------------------

const settingsBtn = document.getElementById('settings-btn');
settingsBtn.addEventListener('click', event => {
  event.stopPropagation();
  rpc('OpenSettings').catch(() => { });
});

function applyExternalConfig(newConfig) {
  if (!newConfig) return;
  config = normalizeMainConfig(newConfig);
  applyTheme(config.theme, false);
  syncProviderCards();
  config.providers.forEach(instance => scheduleProvider(instance.id));
  refreshStatusTooltips();
  refreshLimitStates();
  requestWindowResize();
}

wails.Events.On('aigauge:config-updated', event => {
  applyExternalConfig(event.data || event);
});

if (config.hotkeyShortcut) {
  rpc('SetGlobalHotkey', true, config.hotkeyShortcut).catch(() => { });
}

// ---------------------------------------------------------------------------
// First-run onboarding & adding a provider
//
// Replaces the old screen that diagnosed three fixed providers. There is no
// fixed catalog any more, so onboarding is just the entry point for adding
// the first provider instance - the same flow available in Settings
// item uses (see addProviderAndConnect below) - shown as a prominent card
// instead of a menu item so a first-time user cannot miss it.
// ---------------------------------------------------------------------------

// Shared by both entry points: the onboarding card above (a brand new
// instance) and a live card's inline "Connect"/"Reconnect" action (an
// existing instance that lost its credentials) - see connectExistingInstance.
// Returns { ok, message } rather than throwing so callers can decide what "it
// didn't work" should look like in their own context instead of catching a
// generic rejection.
async function connectProviderInstance(meta, diagnosis) {
  if (diagnosis?.canImport) {
    let useLocal = false;
    try {
      const choice = await wails.Dialogs.Question({
        Title: 'AI Gauge',
        Message: `Found an existing login session for ${meta.label}.\n\nWould you like to connect using this account?\n(Select 'No' to log in via browser with a different account)`,
        Buttons: [
          { Label: 'Yes', IsDefault: true },
          { Label: 'No' },
        ],
      });
      useLocal = choice === 'Yes';
    } catch {
      useLocal = window.confirm(`Found an existing login session for ${meta.label}.\n\nConnect using this account?`);
    }

    if (useLocal) {
      try {
        const diag = await rpc('ImportProvider', meta.id);
        return diag?.status === 'connected'
          ? { ok: true }
          : { ok: false, message: diag?.message || 'Import failed.' };
      } catch (error) {
        return { ok: false, message: `Import failed: ${error?.message || error}` };
      }
    }
  }

  try {
    const diag = await rpc('ConnectProvider', meta.id);
    if (diag?.status === 'connected') return { ok: true };
    // A manual-code provider (Claude) redirects to its own hosted page rather
    // than back to AI Gauge, so ConnectProvider cannot block until login
    // completes - it hands back this status instead, and the caller collects
    // the code the user pastes back in and calls SubmitAuthCode with it.
    if (diag?.status === 'awaiting_code') {
      return { ok: false, awaitingCode: true, message: diag?.message || 'Paste the code shown in your browser.' };
    }
    return { ok: false, message: diag?.message || 'Authentication was cancelled or failed.' };
  } catch (error) {
    return { ok: false, message: `Authentication failed: ${error?.message || error}` };
  }
}

// Submits a manual-code provider's pasted code (see connectProviderInstance's
// awaitingCode branch) and reflects whatever actually happened via the same
// reloadConfigFromBackend path a normal connect uses.
async function submitAuthCodeForInstance(id, code) {
  try {
    await rpc('SubmitAuthCode', id, code);
  } catch {
    // reloadConfigFromBackend below re-diagnoses regardless of outcome.
  }
  await reloadConfigFromBackend();
}

// Reconnects a provider instance that already exists but lost its
// credentials (shown as the "Connect"/"Check connection" action on a live
// card's error state). The card's own next fetch - triggered by
// reloadConfigFromBackend below, via the same fetch-on-visibility-recalc path
// scheduleProvider/updateProvidersVisibility already use - reflects whatever
// actually happened, so there is nothing else to update here.
//
// The one exception is a manual-code provider's awaiting_code result: that is
// not something a fresh Diagnose*/GetXUsage call can reconstruct (no token is
// stored yet), so it is rendered directly into the card instead of being
// dropped by a reload, and stays there until submitAuthCodeForInstance runs.
async function connectExistingInstance(id, diagnosis) {
  const meta = PROVIDERS_BY_ID.get(id);
  const state = providerState.get(id);
  if (!meta || !state) return;
  const result = await connectProviderInstance(meta, diagnosis);
  if (result.awaitingCode) {
    // Mirrors what fetchProvider does before calling a render function: set
    // the state this card's badge/status panel reads before drawing it.
    state.status = 'awaiting_code';
    state.lastError = result.message;
    renderNonUsageState(id, { status: 'awaiting_code', message: result.message });
    return;
  }
  await reloadConfigFromBackend();
}

document.getElementById('hide-window').addEventListener('click', () => {
  wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.HideToTray').catch(() => wails.Window.Hide());
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
    if (height > 0 && Math.abs(height - lastReportedHeight) >= 2) {
      lastReportedHeight = height;
      // The backend preserves the window's current user-selected width and
      // changes only its height to fit the reflowed content.
      wails.Call.ByName('github.com/jmnote/aigauge/internal/app.App.SetContentHeight', height).catch(() => { });
    }
  });
}

const resizeObserver = new ResizeObserver(() => {
  requestWindowResize();
});
resizeObserver.observe(document.querySelector('.shell'));

// Restore width before starting provider rendering so text wraps and content
// height are measured against the user's chosen size from the outset.
try {
  await rpc('SetWindowWidth', config.windowWidth);
} catch (error) {
  console.warn('Unable to restore the saved window width:', error);
}

let widthSaveTimer = null;
let widthIndicatorTimer = null;
let lastObservedWidth = window.innerWidth;
const widthIndicator = document.getElementById('window-size-indicator');
window.addEventListener('resize', () => {
  const width = normalizeWindowWidth(window.innerWidth);
  if (width === normalizeWindowWidth(lastObservedWidth)) return;
  lastObservedWidth = window.innerWidth;
  widthIndicator.textContent = `${width} px`;
  widthIndicator.classList.add('is-visible');
  clearTimeout(widthIndicatorTimer);
  widthIndicatorTimer = setTimeout(() => widthIndicator.classList.remove('is-visible'), 700);
  if (width === config.windowWidth) return;
  config.windowWidth = width;
  clearTimeout(widthSaveTimer);
  widthSaveTimer = setTimeout(() => {
    rpc('SetSavedWindowWidth', width).catch(error => console.warn('Failed to save window width:', error));
  }, 250);
});

syncProviderCards();

function refreshStatusTooltips() {
  for (const instance of config.providers) {
    updateProviderStatus(instance.id);
  }
}

// The status panel is rendered in normal card flow, so opening or closing it
// changes the shell's measured content height naturally. The explicit resize
// request keeps the native window height synchronized immediately.

document.addEventListener('click', () => {
  closeOpenDetailsTooltips();
});

document.addEventListener('keydown', event => {
  if (event.key === 'Escape') {
    closeOpenDetailsTooltips();
  }
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
  if (document.visibilityState === 'visible' && [...statusDropdowns.values()].some(d => d.isOpen())) {
    refreshStatusTooltips();
  }
}, 1000);
