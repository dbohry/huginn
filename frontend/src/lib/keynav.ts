import { tick } from 'svelte';
import { reader } from './reader.svelte';
import { ui } from './ui.svelte';

// Arrow keys move around the whole reader: up and down within a pane, left and right between
// the panes (feeds, articles, the open article). j and k stay as a second way to step through articles.
type Zone = 'feeds' | 'list' | 'article';

const SCROLL_STEP = 60;
const $ = <T extends HTMLElement>(selector: string) => document.querySelector<T>(selector);
const articlePane = () => $('section.pane[aria-label="Article"]');

function zoneOf(target: HTMLElement | null): Zone | null {
  if (target?.closest('nav.sidebar')) return 'feeds';
  if (target?.closest('section.list')) return 'list';
  if (target?.closest('section.pane')) return 'article';
  return null;
}

// Controls where the arrow keys already mean something else are left alone.
function ownsArrows(el: HTMLElement) {
  return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName) || el.closest('[role="slider"],[role="menu"],[role="listbox"]') !== null;
}

const focusActiveItem = () => ($<HTMLElement>('.item.active') ?? $<HTMLElement>('section.list h1'))?.focus();
const focusActiveFeed = () => ($<HTMLElement>('nav.sidebar .row.active') ?? $<HTMLElement>('nav.sidebar .row'))?.focus();

async function focusItem(id: number | null) {
  await tick();
  (id === null ? null : document.getElementById(`item-${id}`))?.focus();
}

/** Handles an arrow key. Returns true when it did something, so the page does not also scroll. */
export function arrowNav(e: KeyboardEvent): boolean {
  const dir = { ArrowDown: 'down', ArrowUp: 'up', ArrowLeft: 'left', ArrowRight: 'right' }[e.key];
  if (!dir) return false;
  const target = e.target as HTMLElement | null;
  const onPage = !target || target === document.body || target === document.documentElement;
  if (!onPage && target && ownsArrows(target)) return false;
  // With nothing focused, the arrows act on the pane that is showing.
  const zone = zoneOf(onPage ? null : target) ?? (ui.narrow ? ({ feeds: 'feeds', list: 'list', article: 'article' } as const)[reader.pane] : reader.selectedId !== null ? 'article' : 'list');

  if (dir === 'down' || dir === 'up') {
    const delta = dir === 'down' ? 1 : -1;
    if (zone === 'feeds') {
      const rows = [...document.querySelectorAll<HTMLElement>('nav.sidebar .row')];
      const at = rows.indexOf(target as HTMLElement);
      const to = at === -1 ? rows.findIndex((r) => r.classList.contains('active')) : at + delta;
      rows[Math.min(Math.max(to === -1 ? 0 : to, 0), rows.length - 1)]?.focus();
      return true;
    }
    if (zone === 'list') {
      void reader.step(delta).then(() => focusItem(reader.selectedId));
      return true;
    }
    const pane = articlePane();
    if (!pane || pane.scrollHeight <= pane.clientHeight) return false;
    pane.scrollBy({ top: delta * SCROLL_STEP });
    return true;
  }

  if (dir === 'right') {
    if (zone === 'feeds') {
      // Choosing the feed shows its articles; the focus follows into them.
      if (target?.classList.contains('row')) target.click();
      else reader.goList();
      void tick().then(focusActiveItem);
      return true;
    }
    if (zone === 'list') {
      const opened = reader.selectedId === null ? reader.step(1) : Promise.resolve(reader.showArticle());
      void opened.then(tick).then(() => !ui.narrow && articlePane()?.focus({ preventScroll: true }));
      return true;
    }
    return false;
  }

  if (zone === 'article') {
    if (ui.narrow) reader.goList();
    else focusActiveItem();
    return true;
  }
  if (zone === 'list') {
    // On a phone the feeds are a pane of their own; beside the list they are just there.
    if (ui.narrow) reader.showFeeds();
    else focusActiveFeed();
    return true;
  }
  return false;
}
