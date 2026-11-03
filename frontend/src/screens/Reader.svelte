<script lang="ts">
  import { tick } from 'svelte';
  import AddFeedDialog from '../components/AddFeedDialog.svelte';
  import ArticleList from '../components/ArticleList.svelte';
  import ArticlePane from '../components/ArticlePane.svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import StarterDialog from '../components/StarterDialog.svelte';
  import Toast from '../components/Toast.svelte';
  import { appearance } from '../lib/appearance.svelte';
  import { arrowNav } from '../lib/keynav';
  import { reader } from '../lib/reader.svelte';
  import { session } from '../lib/session.svelte';
  import { ui } from '../lib/ui.svelte';

  $effect(() => {
    reader.init();
  });

  // A new account is offered the starter feeds once, at its first sign-in.
  $effect(() => {
    if (session.user && !session.user.starterDone) reader.starterOpen = true;
  });

  // Single-key shortcuts, only when nothing is being typed and no dialog is open.
  // Inputs that are not for typing text (a checkbox, a button) leave the shortcuts working.
  const notText = new Set(['checkbox', 'radio', 'button', 'submit', 'reset', 'file', 'range', 'color', 'image']);

  function typing(target: EventTarget | null) {
    const el = target as HTMLElement | null;
    if (!el) return false;
    if (el.isContentEditable || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') return true;
    return el.tagName === 'INPUT' && !notText.has((el as HTMLInputElement).type);
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key.startsWith('Arrow') && !e.shiftKey && !typing(e.target) && !document.querySelector('dialog[open]')) {
      if (arrowNav(e)) e.preventDefault();
      return;
    }
    if (!appearance.shortcuts || e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
    if (typing(e.target) || document.querySelector('dialog[open]')) return;
    switch (e.key) {
      case 'j':
        reader.step(1);
        break;
      case 'k':
        reader.step(-1);
        break;
      case 'm':
        reader.toggleRead();
        break;
      case 'o': {
        const url = reader.selected?.url;
        if (url) window.open(url, '_blank', 'noopener,noreferrer');
        break;
      }
      case 'f':
        reader.loadFull();
        break;
      case 'r':
        reader.refresh();
        break;
      default:
        return;
    }
    e.preventDefault();
  }

  // Swipe right on the list to open the feeds menu; swipe left on the menu to close it.
  // Only phones, only a mostly horizontal drag, and not from inside something that scrolls sideways.
  const SWIPE_MIN = 60;
  let swipe: { x: number; y: number } | null = null;

  function scrollsSideways(target: EventTarget | null) {
    for (let el = target as HTMLElement | null; el && el !== document.body; el = el.parentElement) {
      if (el.scrollWidth > el.clientWidth && /(auto|scroll)/.test(getComputedStyle(el).overflowX)) return true;
    }
    return false;
  }

  function ontouchstart(e: TouchEvent) {
    const t = e.touches[0];
    const ok = e.touches.length === 1 && ui.narrow && !document.querySelector('dialog[open]');
    swipe = ok && !scrollsSideways(e.target) ? { x: t.clientX, y: t.clientY } : null;
  }

  function ontouchend(e: TouchEvent) {
    const start = swipe;
    swipe = null;
    if (!start || !ui.narrow) return;
    const t = e.changedTouches[0];
    const dx = t.clientX - start.x;
    const dy = t.clientY - start.y;
    if (Math.abs(dx) < SWIPE_MIN || Math.abs(dx) < Math.abs(dy) * 2) return;
    if (dx > 0 && reader.pane === 'list') reader.showFeeds();
    else if (dx < 0 && reader.pane === 'feeds') reader.goList();
  }

  // On a phone only one pane is on screen, so when the pane changes move focus into the new one.
  // (The article pane moves its own focus once the article has loaded.)
  let lastPane = reader.pane;
  $effect(() => {
    const pane = reader.pane;
    if (pane === lastPane) return;
    lastPane = pane;
    if (!ui.narrow || pane === 'article') return;
    tick().then(() => {
      const target =
        pane === 'feeds'
          ? document.querySelector('.sidebar .heading')
          : (document.querySelector('.item.active') ?? document.querySelector('.list h1'));
      (target as HTMLElement | null)?.focus({ preventScroll: true });
    });
  });
</script>

<svelte:window {onkeydown} />

<!-- Three panes side by side on a wide screen; one at a time on a narrow one. -->
<main class="app" class:rail={appearance.sidebarCollapsed} data-pane={reader.pane} {ontouchstart} {ontouchend} ontouchcancel={() => (swipe = null)}>
  <Sidebar />
  <ArticleList />
  <ArticlePane />
  <p class="sr-only" role="status" aria-live="polite">{reader.announcement}</p>
</main>

{#if reader.addOpen}<AddFeedDialog onclose={() => (reader.addOpen = false)} />{/if}
{#if reader.starterOpen}<StarterDialog onclose={() => (reader.starterOpen = false)} />{/if}
<Toast />

<style>
  .app {
    /* The hidden screen-reader text inside the panes is absolutely positioned. Without a
       positioned, clipping ancestor here it would stretch the whole page, so the page
       scrolled (and left a blank area) once a long list reached its end. */
    position: relative;
    overflow: hidden;
    display: grid;
    grid-template-columns: 240px minmax(300px, 380px) minmax(0, 1fr);
    height: 100vh;
    height: 100dvh;
    width: 100%;
    max-width: 100vw;
    transition: grid-template-columns 0.2s ease;
  }
  .app :global(.sidebar) {
    overflow-x: hidden;
  }
  .app.rail {
    grid-template-columns: 56px minmax(300px, 380px) minmax(0, 1fr);
  }
  @media (max-width: 899px) {
    .app {
      grid-template-columns: minmax(0, 1fr);
      padding-bottom: var(--safe-bottom);
    }
    .app :global(.sidebar),
    .app :global(.list),
    .app :global(.pane) {
      display: none;
    }
    .app[data-pane='feeds'] :global(.sidebar),
    .app[data-pane='list'] :global(.list),
    .app[data-pane='article'] :global(.pane) {
      display: flex;
    }
  }
</style>
