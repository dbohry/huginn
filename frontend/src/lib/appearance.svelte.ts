export type Mode = 'system' | 'light' | 'dark';

const KEY = 'huginn.mode';

function stored(): Mode {
  try {
    const v = localStorage.getItem(KEY);
    if (v === 'light' || v === 'dark') return v;
  } catch {
    // Storage can be blocked; the device setting is a fine default.
  }
  return 'system';
}

function apply(mode: Mode) {
  const root = document.documentElement;
  if (mode === 'system') root.removeAttribute('data-theme');
  else root.setAttribute('data-theme', mode);
}

/** Light, dark or follow the device. Kept in this browser only. */
class Appearance {
  mode = $state<Mode>(stored());

  set(mode: Mode) {
    this.mode = mode;
    apply(mode);
    try {
      if (mode === 'system') localStorage.removeItem(KEY);
      else localStorage.setItem(KEY, mode);
    } catch {
      // Not saved; the choice still applies until the page is closed.
    }
  }
}

export const appearance = new Appearance();

/** Paints the saved theme before the first render, so there is no flash. */
export function applyCachedTheme() {
  apply(appearance.mode);
}
