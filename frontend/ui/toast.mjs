// Small, reusable transient notification for window-local UI.
let toastElement = null;
let hideTimer = null;

function getToastElement() {
  if (toastElement?.isConnected) return toastElement;
  toastElement = document.createElement('div');
  toastElement.className = 'ui-toast';
  toastElement.setAttribute('role', 'status');
  toastElement.setAttribute('aria-live', 'polite');
  toastElement.hidden = true;
  document.body.append(toastElement);
  return toastElement;
}

export function showToast(message, duration = 3000) {
  const element = getToastElement();
  element.textContent = message;
  element.hidden = false;
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => { element.hidden = true; }, duration);
}
