<script lang="ts">
  import { tick } from 'svelte';
  import AddFeedDialog from '../components/AddFeedDialog.svelte';
  import ArticleList from '../components/ArticleList.svelte';
  import ArticlePane from '../components/ArticlePane.svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import StarterDialog from '../components/StarterDialog.svelte';
  import Toast from '../components/Toast.svelte';
  import { appearance } from '../lib/appearance.svelte';
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
<main class="app" data-pane={reader.pane}>
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
