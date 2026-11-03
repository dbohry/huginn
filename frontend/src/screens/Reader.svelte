<script lang="ts">
  import ArticleList from '../components/ArticleList.svelte';
  import ArticlePane from '../components/ArticlePane.svelte';
  import Sidebar from '../components/Sidebar.svelte';
  import { reader } from '../lib/reader.svelte';

  $effect(() => {
    reader.init();
  });
</script>

<!-- Three panes side by side on a wide screen; one at a time on a narrow one. -->
<main class="app" data-pane={reader.pane}>
  <Sidebar />
  <ArticleList />
  <ArticlePane />
  <p class="sr-only" role="status" aria-live="polite">{reader.announcement}</p>
</main>

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
