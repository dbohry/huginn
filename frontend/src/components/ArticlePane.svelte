<script lang="ts">
  import { reader } from '../lib/reader.svelte';
  import { ui } from '../lib/ui.svelte';
  import { when } from '../lib/time';

  const item = $derived(reader.selected);
</script>

<section class="pane" aria-label="Article">
  <header>
    <button class="icon mobile-only" aria-label="Back to articles" onclick={() => (reader.pane = 'list')}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 6l-6 6 6 6" /></svg>
    </button>
    <span class="grow"></span>
    {#if item}
      <button class="small" onclick={() => reader.toggleRead()}>{item.unread ? 'Mark read' : 'Mark unread'}</button>
      {#if item.url}
        <a class="button open" href={item.url} target="_blank" rel="noopener noreferrer">Open original</a>
      {/if}
    {/if}
  </header>

  {#if reader.loadingItem && !item}
    <p class="placeholder" role="status">Loading…</p>
  {:else if item}
    <article>
      <!-- On a phone this is the only pane on screen, so it needs the page heading too. -->
      {#if ui.narrow}<h1 class="sr-only">Reading: {item.feedTitle}</h1>{/if}
      <h2 class="headline">{item.title}</h2>
      <p class="byline">
        {item.feedTitle}{item.author ? ` · ${item.author}` : ''} · {when(item.publishedAt)}
      </p>
      <!-- Sanitized by the server with an allow-list before it is sent. -->
      <div class="content">{@html item.content}</div>
    </article>
  {:else}
    <p class="placeholder">Select an article to read it.</p>
  {/if}
</section>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
  }
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 60px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    background: var(--bg);
    z-index: 1;
  }
  .grow {
    flex: 1;
  }
  .icon {
    width: 44px;
    height: 44px;
    margin-left: -8px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    display: grid;
    place-items: center;
  }
  .small {
    min-height: 44px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
    font-size: 13px;
  }
  .small:hover {
    background: var(--selected);
  }
  .open {
    font-size: 13px;
  }
  article {
    width: 100%;
    max-width: 680px;
    margin: 0 auto;
    padding: 40px 24px 80px;
  }
  .headline {
    margin: 0 0 8px;
    font-size: 30px;
    line-height: 1.2;
    letter-spacing: -0.01em;
  }
  .byline {
    margin: 0 0 32px;
    font-size: 13px;
    color: var(--muted);
  }
  .content {
    font-family: Georgia, 'Iowan Old Style', 'Palatino Linotype', serif;
    font-size: 18px;
    line-height: 1.65;
    overflow-wrap: anywhere;
  }
  .content :global(img),
  .content :global(video),
  .content :global(table) {
    max-width: 100%;
    height: auto;
  }
  .content :global(pre) {
    overflow-x: auto;
    padding: 12px;
    background: var(--surface);
    border-radius: 8px;
    font-size: 14px;
  }
  .content :global(blockquote) {
    margin: 0;
    padding-left: 16px;
    border-left: 3px solid var(--border);
    color: var(--muted);
  }
  .placeholder {
    margin: 0;
    padding: 64px 24px;
    text-align: center;
    color: var(--muted);
  }
  .mobile-only {
    display: none;
  }
  @media (max-width: 899px) {
    .mobile-only {
      display: grid;
    }
    article {
      padding: 24px 16px 64px;
    }
    .headline {
      font-size: 24px;
    }
  }
</style>
