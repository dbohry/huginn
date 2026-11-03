export type Mode = 'system' | 'light' | 'dark';

const MODE_KEY = 'huginn.mode';
const SHORTCUTS_KEY = 'huginn.shortcuts';
const GROUP_KEY = 'huginn.group';
const UNREAD_KEY = 'huginn.unreadOnly';

// The browser's address bar and status bar take this colour on a phone.
const barColour = { light: '#ffffff', dark: '#141414' };

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null; // Storage can be blocked; the defaults are fine.
  }
}

function write(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // Not saved; the choice still applies until the page is closed.
  }
}

function storedMode(): Mode {
  const v = read(MODE_KEY);
  return v === 'light' || v === 'dark' ? v : 'system';
}

function apply(mode: Mode) {
  const root = document.documentElement;
  if (mode === 'system') root.removeAttribute('data-theme');
  else root.setAttribute('data-theme', mode);

  // index.html has one theme-color tag for each device setting. A saved choice overrides both.
  for (const tag of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    const own = tag.media.includes('dark') ? 'dark' : 'light';
    tag.content = barColour[mode === 'system' ? own : mode];
  }
}

/** The look and feel choices of this browser: light or dark, and keyboard shortcuts. */
class Appearance {
  mode = $state<Mode>(storedMode());
  /** Single-key shortcuts such as j and k. Off for people who would trigger them by accident. */
  shortcuts = $state(read(SHORTCUTS_KEY) !== 'off');

  /** Show the feed list in sections by topic, when feeds have topics. */
  groupByTopic = $state(read(GROUP_KEY) !== 'off');

  setGroupByTopic(on: boolean) {
    this.groupByTopic = on;
    write(GROUP_KEY, on ? null : 'off');
  }

  /** Whether the article list shows only unread articles. Remembered, so it need not be ticked every visit. */
  unreadOnly = $state(read(UNREAD_KEY) === 'on');

  setUnreadOnly(on: boolean) {
    this.unreadOnly = on;
    write(UNREAD_KEY, on ? 'on' : null);
  }

  set(mode: Mode) {
    this.mode = mode;
    apply(mode);
    write(MODE_KEY, mode === 'system' ? null : mode);
  }

  setShortcuts(on: boolean) {
    this.shortcuts = on;
    write(SHORTCUTS_KEY, on ? null : 'off');
  }
}

export const appearance = new Appearance();

/** Paints the saved theme before the first render, so there is no flash. */
export function applyCachedTheme() {
  apply(appearance.mode);
}
