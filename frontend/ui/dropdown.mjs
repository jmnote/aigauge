// Shared open/close bookkeeping for a window's small floating panels - a
// menu, a dropdown, a popover. Pulled out because app.js has more than one
// kind (the titlebar's main menu, and every card's own status card
// "dropdown"), and without something coordinating them, opening one no
// longer closes the other and both can end up on screen at once.
//
// Each window (index.html, settings.html) loads its own copy of this module,
// so "only one panel open at a time" applies per window - which matches how
// a user actually experiences it, since there is no scenario where a panel
// in one window should visually compete with one in another.
//
// A panel calls createDropdown once, at creation, with the DOM node whose
// click should NOT count as "outside" (typically the trigger button plus its
// popup, wrapped in one positioned container) and the two callbacks that
// apply/remove whatever visible-vs-hidden state means for that panel. What
// comes back is driven from the panel's own click handler via
// toggle()/open()/close() - this module never touches the panel's markup
// itself, only when onOpen/onClose run and which one is "current".

let current = null; // the currently open panel's handle, or null

function closeCurrent() {
  if (!current) return;
  const panel = current;
  current = null;
  panel.onClose();
}

export function createDropdown(scopeEl, { onOpen, onClose }) {
  const handle = {
    scopeEl,
    onClose,
    isOpen: () => current === handle,
    open() {
      if (current === handle) return;
      closeCurrent();
      current = handle;
      onOpen();
    },
    close() {
      if (current === handle) closeCurrent();
    },
    toggle() {
      if (current === handle) handle.close();
      else handle.open();
    },
  };
  return handle;
}

// composedPath() (the elements the click actually passed through at
// dispatch time), not scopeEl.contains(event.target): a menu item's own
// click handler commonly rebuilds the menu's contents (e.g. switching from a
// root menu to a submenu) before this listener runs, which detaches
// event.target from the document - and a detached node is never "contained"
// in anything, so a plain .contains() check would misread that click as
// happening outside the menu and close it the instant it opens a submenu.
document.addEventListener('click', event => {
  if (current && !event.composedPath().includes(current.scopeEl)) closeCurrent();
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape') closeCurrent();
});
