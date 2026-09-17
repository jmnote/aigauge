import {
  normalizeConfig, VALID_THEMES, PROVIDER_REFRESH_OPTIONS, providerTypeLabel,
  formatHotkeyError, hotkeyOptionLabel, HOTKEY_OPTIONS,
} from '/logic.mjs';
import { showToast } from '/ui/toast.mjs';

const wails = await import('/wails/runtime.js');

document.getElementById('close-settings').addEventListener('click', async () => {
  try { await rpc('CleanupPendingProviderInstances'); } catch { /* best effort cleanup */ }
  wails.Window.Close();
});

function rpc(method, ...args) {
  return wails.Call.ByName(`github.com/jmnote/aigauge/internal/app.App.${method}`, ...args);
}

// The Wails-RPC method names for each provider *type*. A provider instance
// (in config.providers) carries only its type id; this is what turns that id
// into the calls needed to diagnose and connect it.
const PROVIDER_TYPE_RPC = {
  codex: { diagnoseRpcMethod: 'DiagnoseCodex', usageRpcMethod: 'GetCodexUsage' },
  claude: { diagnoseRpcMethod: 'DiagnoseClaude', usageRpcMethod: 'GetClaudeUsage' },
  antigravity: { diagnoseRpcMethod: 'DiagnoseAntigravity', usageRpcMethod: 'GetAntigravityUsage' },
};

// Settings (the provider instance list/order/enabled state, theme, refresh
// interval, thresholds and hotkey) are owned by the Go backend rather than
// this window's localStorage - see internal/config and App.GetSettings.
// Both this window and the main window write field-level changes through
// RPCs and are kept in sync by the "aigauge:config-updated" event the backend
// emits whenever either one saves a change.
let config = normalizeConfig(await rpc('GetSettings'));

let settingsWriteQueue = Promise.resolve();
function saveSetting(method, ...args) {
  // Snapshot object arguments now: config can be replaced by an update event
  // before this queued write reaches the backend.
  const savedArgs = args.map(value => value && typeof value === 'object'
    ? JSON.parse(JSON.stringify(value)) : value);
  settingsWriteQueue = settingsWriteQueue
    .then(() => rpc(method, ...savedArgs))
    .catch(e => console.warn('Failed to save settings:', e));
  return settingsWriteQueue;
}

let settingsResizeFrame = 0;
let lastSettingsHeight = 0;
function resizeToContent() {
  if (settingsResizeFrame) cancelAnimationFrame(settingsResizeFrame);
  settingsResizeFrame = requestAnimationFrame(() => {
    settingsResizeFrame = 0;
    const shell = document.querySelector('.settings-shell');
    if (!shell) return;
    // The shell fills the current WebView, so its measured height can stay
    // pinned to the old window size when content shrinks. Temporarily measure
    // its natural height so both adding and removing providers are reflected.
    const previousHeight = shell.style.height;
    shell.style.height = 'auto';
    const height = Math.ceil(shell.scrollHeight);
    shell.style.height = previousHeight;
    if (height > 0 && Math.abs(height - lastSettingsHeight) >= 2) {
      lastSettingsHeight = height;
      rpc('SetSettingsContentHeight', height).catch(() => { });
    }
  });
}

const settingsResizeObserver = new ResizeObserver(resizeToContent);
settingsResizeObserver.observe(document.querySelector('.settings-shell'));
window.addEventListener('load', resizeToContent);

// Refetches settings from the backend and re-renders. Used after an RPC
// (Delete, Connect, Import) that changes the persisted provider list on its
// own, so this window shows exactly what was saved rather than a locally
// guessed patch that could drift from it.
async function reloadConfig() {
  try {
    config = normalizeConfig(await rpc('GetSettings'));
  } catch (e) {
    console.warn('Failed to reload settings:', e);
  }
  renderProviderList();
}

const providerState = new Map();
const addingProviders = new Set();
let pendingProviderId = null;
let pendingAuthCode = null;
let pendingLogin = null;
let providerFlowGeneration = 0;

// DOM Elements
const providerListEl = document.getElementById('provider-list');
const providerAddButtonsEl = document.getElementById('provider-add-buttons');
const providerAddDialog = document.getElementById('provider-add-dialog');
const providerAddDialogTitle = document.getElementById('provider-add-dialog-title');
const providerAddDialogStatus = document.getElementById('provider-add-dialog-status');
const providerAddDialogLogin = document.getElementById('provider-add-dialog-login');
const providerAddDialogImport = document.getElementById('provider-add-dialog-import');
const providerAddDialogCode = document.getElementById('provider-add-dialog-code');
const providerAddDialogSubmit = document.getElementById('provider-add-dialog-submit');
const providerAddDialogClose = document.getElementById('provider-add-dialog-close');
const providerAddDialogActionClose = document.getElementById('provider-add-dialog-action-close');
const confirmDialog = document.getElementById('confirm-dialog');
const confirmDialogTitle = document.getElementById('confirm-dialog-title');
const confirmDialogMessage = document.getElementById('confirm-dialog-message');
const confirmDialogCancel = document.getElementById('confirm-dialog-cancel');
const confirmDialogOk = document.getElementById('confirm-dialog-ok');
const confirmDialogClose = document.getElementById('confirm-dialog-close');
const themeButtonGroup = document.querySelector('.theme-button-group');
const warningEnabledInput = document.getElementById('warning-enabled');
const warningThresholdInput = document.getElementById('warning-threshold');
const criticalEnabledInput = document.getElementById('critical-enabled');
const criticalThresholdInput = document.getElementById('critical-threshold');
const systemTheme = matchMedia('(prefers-color-scheme: dark)');
const hotkeySelect = document.getElementById('hotkey-select');
const hotkeyStatus = document.getElementById('hotkey-status');
const hotkeyStatusText = document.getElementById('hotkey-status-text');
const hotkeyRetryBtn = document.getElementById('hotkey-retry-btn');
const versionEl = document.getElementById('version');


providerAddDialogClose.addEventListener('click', async () => {
  const closingGeneration = ++providerFlowGeneration;
  const authCode = pendingAuthCode;
  pendingAuthCode = null;
  // Wake a login flow waiting for an auth code so it can observe the
  // cancellation and exit without touching a subsequent add flow.
  authCode?.resolve();
  // Stop the backend OAuth flow and wait for its fixed-port listener to close
  // before allowing another provider login to start.
  try { await rpc('CancelAuth'); } catch { /* best effort cleanup */ }
  if (pendingLogin) {
    const { provider, instance } = pendingLogin;
    pendingLogin = null;
    pendingProviderId = null;
    addingProviders.delete(provider);
    try { await rpc('RemoveProviderInstance', instance.id); } catch { /* best effort cleanup */ }
  }
  providerAddDialog.hidden = true;
  providerAddDialogLogin.hidden = true;
  providerAddDialogImport.hidden = true;
  providerAddDialogImport.disabled = false;
  providerAddDialogLogin.disabled = false;
  providerAddDialogStatus.classList.remove('is-loading');
  providerAddDialogCode.hidden = true;
  providerAddDialogSubmit.hidden = true;
  providerAddDialogSubmit.disabled = false;
  providerAddDialogActionClose.hidden = true;
  if (providerFlowGeneration === closingGeneration) await reloadConfig();
});
providerAddDialogActionClose.addEventListener('click', () => {
  providerAddDialog.hidden = true;
  providerAddDialogActionClose.hidden = true;
});
function confirmRemove(instance) {
  confirmDialogTitle.textContent = `Remove ${instance.label}?`;
  confirmDialogMessage.textContent = instance.type === 'antigravity'
    ? 'This stops monitoring it. You can add it again later.'
    : 'This removes its saved login. You can add it again later.';
  confirmDialog.hidden = false;
  return new Promise(resolve => {
    const finish = value => { confirmDialog.hidden = true; resolve(value); };
    confirmDialogCancel.onclick = () => finish(false);
    confirmDialogClose.onclick = () => finish(false);
    confirmDialogOk.onclick = () => finish(true);
  });
}
providerAddDialogSubmit.addEventListener('click', async () => {
  if (!pendingAuthCode) return;
  providerAddDialogSubmit.disabled = true;
  try {
    const result = await rpc('SubmitAuthCode', pendingAuthCode.id, providerAddDialogCode.value.trim());
    if (result?.status !== 'connected') throw new Error(result?.message || 'Authorization code was rejected');
    pendingAuthCode.resolve();
  } catch (error) {
    providerAddDialogStatus.textContent = error?.message || String(error);
    providerAddDialogSubmit.disabled = false;
  }
});

providerAddDialogImport.addEventListener('click', async () => {
  if (!pendingLogin) return;
  const { provider, instance, generation } = pendingLogin;
  providerAddDialogImport.hidden = true;
  providerAddDialogImport.disabled = true;
  providerAddDialogLogin.hidden = true;
  providerAddDialogStatus.classList.add('is-loading');
  providerAddDialogStatus.textContent = `Using existing ${providerTypeLabel(provider)} credentials…`;
  try {
    const connection = await rpc('ImportProvider', instance.id);
    if (generation !== providerFlowGeneration || pendingLogin?.instance.id !== instance.id) return;
    if (connection?.status !== 'connected') {
      throw new Error(connection?.message || 'The existing credentials could not be used.');
    }
    providerAddDialogStatus.textContent = `Adding ${instance.label}…`;
    pendingProviderId = null;
    addingProviders.delete(provider);
    pendingLogin = null;
    await rpc('CommitProviderInstance', instance.id);
    providerAddDialog.hidden = true;
    showToast(`${instance.label} added successfully.`);
  } catch (error) {
    if (generation !== providerFlowGeneration || pendingLogin?.instance.id !== instance.id) return;
    console.warn(`Failed to import ${provider}:`, error);
    try { await rpc('RemoveProviderInstance', instance.id); } catch { /* best effort cleanup */ }
    pendingProviderId = null;
    pendingLogin = null;
    providerAddDialogStatus.textContent = `Could not use existing credentials: ${error?.message || error}`;
  } finally {
    if (generation !== providerFlowGeneration) return;
    providerAddDialogStatus.classList.remove('is-loading');
    providerAddDialogImport.disabled = false;
    providerAddDialogLogin.disabled = false;
    addingProviders.delete(provider);
    await reloadConfig();
  }
});

providerAddDialogLogin.addEventListener('click', async () => {
  if (!pendingLogin) return;
  const { provider, instance, generation } = pendingLogin;
  providerAddDialogLogin.hidden = true;
  providerAddDialogImport.hidden = true;
  providerAddDialogLogin.disabled = true;
  providerAddDialogStatus.classList.add('is-loading');
  providerAddDialogStatus.textContent = `Connecting to ${instance.label}…`;
  try {
    const connection = await rpc('ConnectProvider', instance.id);
    if (generation !== providerFlowGeneration || pendingLogin?.instance.id !== instance.id) return;
    let connected = connection?.status === 'connected';
    if (connection?.status === 'awaiting_code') {
      providerAddDialogStatus.classList.remove('is-loading');
      providerAddDialogStatus.textContent = connection.message || 'Paste the code#state from Claude here.';
      providerAddDialogCode.hidden = false;
      providerAddDialogSubmit.hidden = false;
      providerAddDialogCode.focus();
      await new Promise(resolve => { pendingAuthCode = { id: instance.id, resolve }; });
      pendingAuthCode = null;
      if (generation !== providerFlowGeneration || pendingLogin?.instance.id !== instance.id) return;
      connected = true;
      providerAddDialogCode.hidden = true;
      providerAddDialogSubmit.hidden = true;
      providerAddDialogSubmit.disabled = false;
    }
    if (!connected) {
      throw new Error(connection?.message || 'Authentication could not be completed');
    }
    pendingProviderId = null;
    addingProviders.delete(provider);
    pendingLogin = null;
    // Clear the temporary markers before broadcasting the config update so
    // Settings renders the newly committed provider without a brief gap.
    await rpc('CommitProviderInstance', instance.id);
    providerAddDialog.hidden = true;
    providerAddDialogLogin.hidden = true;
    showToast(`${instance.label} added successfully.`);
  } catch (error) {
    if (generation !== providerFlowGeneration || pendingLogin?.instance.id !== instance.id) return;
    console.warn(`Failed to add ${provider}:`, error);
    try {
      await rpc('RemoveProviderInstance', instance.id);
      pendingProviderId = null;
    } catch { /* best effort cleanup */ }
    pendingLogin = null;
    providerAddDialogStatus.textContent = `Could not add provider: ${error?.message || error}`;
  } finally {
    if (generation !== providerFlowGeneration) return;
    providerAddDialogStatus.classList.remove('is-loading');
    addingProviders.delete(provider);
    providerAddDialogClose.disabled = false;
    providerAddDialogLogin.disabled = false;
    await reloadConfig();
  }
});

let hotkeyError = '';
let hotkeyPendingSettings = null;
let hotkeyBusy = false;

// Populate hotkey choices
for (const option of [{ value: '', label: 'Disabled' }, ...HOTKEY_OPTIONS]) {
  hotkeySelect.add(new Option(option.label, option.value));
}

// ---------------------------------------------------------------------------
// Theme Management
// ---------------------------------------------------------------------------

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
    config.theme = theme;
    saveSetting('SetTheme', theme);
  }
}

if (themeButtonGroup) {
  themeButtonGroup.addEventListener('click', event => {
    const button = event.target.closest('.theme-option-btn');
    if (button) applyTheme(button.dataset.theme, true);
  });
}

systemTheme.addEventListener('change', () => {
  if (document.documentElement.dataset.theme === 'system') applyTheme('system', false);
});

let forcedTheme = '';
try {
  forcedTheme = await rpc('GetThemeOverride');
} catch (error) {
  console.warn('Unable to read the theme override:', error);
}
const activeTheme = forcedTheme || config.theme || 'system';
applyTheme(VALID_THEMES.has(activeTheme) ? activeTheme : 'system', !forcedTheme);

// ---------------------------------------------------------------------------
// Thresholds
// ---------------------------------------------------------------------------

warningEnabledInput.checked = config.thresholds.warning.enabled;
warningThresholdInput.value = config.thresholds.warning.value;
warningThresholdInput.disabled = !config.thresholds.warning.enabled;

criticalEnabledInput.checked = config.thresholds.critical.enabled;
criticalThresholdInput.value = config.thresholds.critical.value;
criticalThresholdInput.disabled = !config.thresholds.critical.enabled;

warningEnabledInput.addEventListener('change', () => {
  config.thresholds.warning.enabled = warningEnabledInput.checked;
  warningThresholdInput.disabled = !warningEnabledInput.checked;
  saveSetting('SetThresholds', config.thresholds);
});

warningThresholdInput.addEventListener('change', () => {
  let val = parseInt(warningThresholdInput.value, 10);
  if (isNaN(val)) val = config.thresholds.warning.value;
  val = Math.max(1, Math.min(100, val));
  config.thresholds.warning.value = val;
  warningThresholdInput.value = val;
  saveSetting('SetThresholds', config.thresholds);
});

criticalEnabledInput.addEventListener('change', () => {
  config.thresholds.critical.enabled = criticalEnabledInput.checked;
  criticalThresholdInput.disabled = !criticalEnabledInput.checked;
  saveSetting('SetThresholds', config.thresholds);
});

criticalThresholdInput.addEventListener('change', () => {
  let val = parseInt(criticalThresholdInput.value, 10);
  if (isNaN(val)) val = config.thresholds.critical.value;
  val = Math.max(0, Math.min(99, val));
  config.thresholds.critical.value = val;
  criticalThresholdInput.value = val;
  saveSetting('SetThresholds', config.thresholds);
});

// ---------------------------------------------------------------------------
// Global Hotkey
// ---------------------------------------------------------------------------

function updateHotkeyUI() {
  const displayedShortcut = hotkeyError
    ? config.hotkeyShortcut
    : (hotkeyPendingSettings ? hotkeyPendingSettings.shortcut : config.hotkeyShortcut);
  hotkeySelect.value = displayedShortcut || '';
  hotkeySelect.disabled = hotkeyBusy;
  hotkeyRetryBtn.disabled = hotkeyBusy;
  hotkeyRetryBtn.textContent = hotkeyBusy ? '...' : 'Retry';

  if (hotkeyError && hotkeyPendingSettings) {
    const target = hotkeyOptionLabel(hotkeyPendingSettings.shortcut);
    hotkeyStatusText.textContent = `Target: ${target}. ${formatHotkeyError(hotkeyError, hotkeyPendingSettings.shortcut !== null)}`;
    hotkeyStatusText.title = hotkeyError;
    hotkeyRetryBtn.title = `Retry ${target}`;
    hotkeyStatus.hidden = false;
  } else {
    hotkeyStatus.hidden = true;
    hotkeyStatusText.textContent = '';
    hotkeyStatusText.removeAttribute('title');
    hotkeyRetryBtn.removeAttribute('title');
  }
}

async function applyHotkeySettings(settings) {
  if (hotkeyBusy) return;

  hotkeyPendingSettings = { ...settings };
  hotkeyError = '';
  hotkeyBusy = true;
  updateHotkeyUI();

  try {
    await rpc('SetGlobalHotkey', settings.shortcut !== null, settings.shortcut || '');
    config.hotkeyShortcut = settings.shortcut || '';
    await rpc('SetHotkeyShortcut', config.hotkeyShortcut);
    hotkeyError = '';
    hotkeyPendingSettings = null;
  } catch (error) {
    hotkeyError = error?.message || String(error);
    console.warn(`Unable to ${settings.shortcut !== null ? 'register' : 'unregister'} global hotkey:`, error);
  } finally {
    hotkeyBusy = false;
    updateHotkeyUI();
  }
}

hotkeySelect.addEventListener('change', () => {
  applyHotkeySettings({ shortcut: hotkeySelect.value || null });
});
hotkeyRetryBtn.addEventListener('click', () => {
  if (hotkeyPendingSettings && !hotkeyBusy) applyHotkeySettings({ ...hotkeyPendingSettings });
});
updateHotkeyUI();

// ---------------------------------------------------------------------------
// Provider List & Diagnosis
//
// Adding a provider instance happens from this window. Existing rows manage
// instances that already exist: reorder and delete. Reordering
// (▲▼) still moves entries directly in config.providers, since array order
// *is* the provider order now (there is no separate providerOrder list).
// ---------------------------------------------------------------------------

function renderProviderList() {
  providerListEl.replaceChildren();
  if (config.providers.length === 0) {
    const emptyMessage = document.createElement('p');
    emptyMessage.className = 'provider-empty-message';
    emptyMessage.textContent = 'Click a provider below to add it.';
    providerListEl.append(emptyMessage);
  }
  // Only the instance currently being authenticated is temporary. Existing
  // persistent instances of the same provider type must remain visible while
  // another one is being added.
  config.providers.filter(instance => instance.id !== pendingProviderId).forEach((instance, index, visibleProviders) => {
    if (!providerState.has(instance.id)) providerState.set(instance.id, { status: 'unknown' });

    const row = document.createElement('div');
    row.className = 'provider-row';
    row.dataset.provider = instance.id;

    // Presence in this list is now the only "is it shown" signal - a row
    // here always has a card in the main window too, and deleting it (the
    // row's trash button) is the only way to remove either. Connecting only
    // happens from that card, which has its own Connect/paste-code UI.
    const nameLabel = document.createElement('span');
    nameLabel.className = 'provider-setting';
    nameLabel.textContent = instance.label;

    const rightWrap = document.createElement('span');
    rightWrap.className = 'provider-row-actions';

    const refreshSelect = document.createElement('select');
    refreshSelect.className = 'provider-refresh-select';
    refreshSelect.title = `Refresh interval for ${instance.label}`;
    for (const value of PROVIDER_REFRESH_OPTIONS) {
      refreshSelect.add(new Option(`${value / 60}m`, value));
    }
    refreshSelect.value = String(instance.refreshInterval);
    refreshSelect.addEventListener('change', () => {
      instance.refreshInterval = Number(refreshSelect.value);
      saveSetting('SetProviderRefreshInterval', instance.id, instance.refreshInterval);
    });
    rightWrap.append(refreshSelect);

    const moveButtons = document.createElement('span');
    moveButtons.className = 'provider-move-buttons';
    const upBtn = document.createElement('button');
    upBtn.type = 'button';
    upBtn.className = 'provider-move-btn';
    upBtn.dataset.action = 'move-up';
    upBtn.textContent = '▲';
    upBtn.title = `Move ${instance.label} up`;
    upBtn.setAttribute('aria-label', `Move ${instance.label} up`);
    upBtn.disabled = index === 0;

    const downBtn = document.createElement('button');
    downBtn.type = 'button';
    downBtn.className = 'provider-move-btn';
    downBtn.dataset.action = 'move-down';
    downBtn.textContent = '▼';
    downBtn.title = `Move ${instance.label} down`;
    downBtn.setAttribute('aria-label', `Move ${instance.label} down`);
    downBtn.disabled = index === visibleProviders.length - 1;
    moveButtons.append(upBtn, downBtn);
    rightWrap.append(moveButtons);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className = 'provider-delete-btn';
    deleteBtn.dataset.action = 'delete';
    deleteBtn.title = `Remove ${instance.label}`;
    deleteBtn.setAttribute('aria-label', `Remove ${instance.label}`);
    deleteBtn.append(Object.assign(document.createElement('span'), { className: 'icon icon-trash' }));
    rightWrap.append(deleteBtn);

    row.append(nameLabel, rightWrap);
    providerListEl.append(row);
  });

  providerAddButtonsEl.replaceChildren();
  for (const provider of Object.keys(PROVIDER_TYPE_RPC)) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'provider-add-button';
    button.title = `Add ${providerTypeLabel(provider)}`;
    button.setAttribute('aria-label', `Add ${providerTypeLabel(provider)}`);
    if (provider === 'codex') {
      const icon = document.createElement('span');
      icon.className = 'icon icon-codex';
      icon.setAttribute('aria-hidden', 'true');
      button.append(icon);
    } else {
      const image = document.createElement('img');
      image.src = new URL(`../images/${provider}.png`, import.meta.url).href;
      image.alt = '';
      image.setAttribute('aria-hidden', 'true');
      button.append(image);
    }
    button.disabled = addingProviders.size > 0;
    button.addEventListener('click', async () => {
      if (addingProviders.size > 0) return;
      if (provider === 'antigravity' && config.providers.some(instance => instance.type === provider)) {
        providerAddDialog.hidden = false;
        providerAddDialogTitle.textContent = `Add ${providerTypeLabel(provider)}`;
        providerAddDialogStatus.classList.remove('is-loading');
        providerAddDialogStatus.textContent = "Multiple Antigravity entries aren't supported. AI Gauge uses the session from the installed agy CLI, so additional entries would use the same session.";
        providerAddDialogLogin.hidden = true;
        providerAddDialogImport.hidden = true;
        providerAddDialogCode.hidden = true;
        providerAddDialogSubmit.hidden = true;
        providerAddDialogActionClose.hidden = false;
        providerAddDialogClose.disabled = false;
        return;
      }
      addingProviders.add(provider);
      const generation = ++providerFlowGeneration;
        providerAddDialog.hidden = false;
        providerAddDialogTitle.textContent = `Add ${providerTypeLabel(provider)}`;
        providerAddDialogStatus.textContent = `Adding ${providerTypeLabel(provider)}…`;
        providerAddDialogClose.disabled = true;
        providerAddDialogLogin.hidden = true;
        providerAddDialogImport.hidden = true;
        providerAddDialogActionClose.hidden = true;
        renderProviderList();
        try {
          const instance = await rpc('AddProviderInstance', provider);
          if (generation !== providerFlowGeneration) {
            try { await rpc('RemoveProviderInstance', instance.id); } catch { /* best effort cleanup */ }
            addingProviders.delete(provider);
            await reloadConfig();
            return;
          }
          pendingProviderId = instance.id;
          pendingLogin = { provider, instance, generation };
          renderProviderList();
          providerAddDialogStatus.textContent = 'Choose how to connect this account.';
          providerAddDialogLogin.textContent = provider === 'antigravity' ? 'Start monitoring with agy CLI' : 'Sign in with browser';
          if (provider === 'antigravity') {
            providerAddDialogStatus.textContent = 'AI Gauge will track your quota usage through the installed agy CLI.';
          }
          providerAddDialogLogin.hidden = false;
          if (provider !== 'antigravity') {
            try {
              const diagnosis = await rpc(PROVIDER_TYPE_RPC[provider].diagnoseRpcMethod, instance.id);
              const canImport = diagnosis?.canImport === true;
              providerAddDialogImport.hidden = !canImport;
              if (canImport) {
                providerAddDialogStatus.textContent = 'An existing credential file was found. You can try it, or sign in with a different account in your browser.';
              }
            } catch {
              providerAddDialogImport.hidden = true;
            }
          }
          providerAddDialogClose.disabled = false;
        } catch (error) {
          console.warn(`Failed to add ${provider}:`, error);
          pendingLogin = null;
          providerAddDialogStatus.textContent = `Could not add provider: ${error?.message || error}`;
          addingProviders.delete(provider);
          providerAddDialogClose.disabled = false;
          await reloadConfig();
        }
    });
    providerAddButtonsEl.append(button);
  }
  resizeToContent();
}


providerListEl.addEventListener('click', async event => {
  const button = event.target.closest('button[data-action]');
  if (!button) return;
  const id = button.closest('[data-provider]').dataset.provider;
  const action = button.dataset.action;
  const instance = config.providers.find(p => p.id === id);

  if (action === 'delete') {
    if (!instance) return;
    const confirmed = await confirmRemove(instance);
    if (!confirmed) return;

    button.disabled = true;
    try {
      await rpc('RemoveProviderInstance', id);
      providerState.delete(id);
    } catch (error) {
      console.warn('Failed to remove provider instance:', error);
    } finally {
      await reloadConfig();
    }
    return;
  }

  if (!instance) return;

  if (action === 'move-up' || action === 'move-down') {
    const order = config.providers;
    const index = order.indexOf(instance);
    const swapWith = action === 'move-up' ? index - 1 : index + 1;
    if (swapWith < 0 || swapWith >= order.length) return;
    [order[index], order[swapWith]] = [order[swapWith], order[index]];
    renderProviderList();
    saveSetting('SetProviderOrder', order.map(providerInstance => providerInstance.id));
  }
});

async function diagnoseAllProviders() {
  for (const instance of config.providers) {
    try {
      const diag = await rpc(PROVIDER_TYPE_RPC[instance.type].diagnoseRpcMethod, instance.id);
      providerState.set(instance.id, { status: diag.status, canImport: diag.canImport });
    } catch {
      providerState.set(instance.id, { status: 'unknown', canImport: false });
    }
  }
  renderProviderList();
}

renderProviderList();
diagnoseAllProviders();
resizeToContent();

// ---------------------------------------------------------------------------
// Version
// ---------------------------------------------------------------------------

try {
  const ver = await rpc('GetVersion');
  if (versionEl) versionEl.textContent = ver;
} catch {
  // ignore
}

// ---------------------------------------------------------------------------
// External Sync (Wails events pushed by the backend)
// ---------------------------------------------------------------------------

wails.Events.On('aigauge:config-updated', event => {
  const data = event?.data || event;
  if (!data) return;
  config = normalizeConfig(data);
  applyTheme(config.theme, false);
  renderProviderList();
});
