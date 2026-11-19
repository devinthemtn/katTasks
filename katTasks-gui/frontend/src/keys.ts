// Turns browser key events into the key names Bubble Tea uses ("j", "G",
// "ctrl+d", "shift+tab", "alt+b", "enter", " "), which the task list's
// [keys] settings and key handling are written in.

export interface KeyLike {
  key: string;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
  isComposing?: boolean;
}

const named: Record<string, string> = {
  Enter: 'enter', Escape: 'esc', Tab: 'tab', Backspace: 'backspace', Delete: 'delete', Insert: 'insert',
  ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right',
  Home: 'home', End: 'end', PageUp: 'pgup', PageDown: 'pgdown',
};

// Keys Bubble Tea also knows with ctrl+ or shift+ in front.
const withCtrl = new Set(['up', 'down', 'left', 'right', 'home', 'end', 'pgup', 'pgdown']);
const withShift = new Set(['up', 'down', 'left', 'right', 'home', 'end']);

/** teaKey returns the Bubble Tea name of a key press, or null to ignore it. */
export function teaKey(e: KeyLike): string | null {
  if (e.isComposing || e.metaKey) return null;
  const n = named[e.key];
  if (n) {
    if (n === 'tab') return e.shiftKey ? 'shift+tab' : 'tab';
    if (n === 'backspace' && (e.ctrlKey || e.altKey)) return 'alt+backspace'; // delete a word
    if (e.ctrlKey && withCtrl.has(n)) return 'ctrl+' + n;
    if (e.altKey) return 'alt+' + n;
    if (e.shiftKey && withShift.has(n)) return 'shift+' + n;
    return n;
  }
  if (/^F([1-9]|1[0-9]|20)$/.test(e.key)) return e.key.toLowerCase();
  if ([...e.key].length !== 1) return null; // Shift, Control, Dead, Unidentified…

  // AltGr arrives as ctrl+alt on Windows; it types a character (@, €…).
  if (e.ctrlKey && e.altKey) return e.key;
  if (e.ctrlKey) {
    if (/^[a-z]$/i.test(e.key)) return 'ctrl+' + e.key.toLowerCase();
    if (e.key === ' ') return 'ctrl+@';
    if (e.key === '\\') return 'ctrl+\\';
    if (e.key === ']') return 'ctrl+]';
    if (e.key === '^') return 'ctrl+^';
    if (e.key === '_') return 'ctrl+_';
    return null;
  }
  if (e.altKey) return 'alt+' + e.key;
  return e.key;
}

/** isPaste reports a paste shortcut: ctrl+v, cmd+v or shift+insert. */
export function isPaste(e: KeyLike): boolean {
  return ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === 'v') ||
    (e.shiftKey && e.key === 'Insert');
}

/** zoomStep is +1, -1 or 0 (reset) for ctrl+= / ctrl+- / ctrl+0, else null. */
export function zoomStep(e: KeyLike): number | null {
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return null;
  switch (e.key) {
    case '=': case '+': return 1;
    case '-': case '_': return -1;
    case '0': return 0;
  }
  return null;
}
