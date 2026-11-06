<script lang="ts">
  import { tick } from 'svelte';
  import Mascot from './Mascot.svelte';
  import { appearance } from '../lib/appearance.svelte';
  import { reader } from '../lib/reader.svelte';
  import { ui } from '../lib/ui.svelte';
  import { when } from '../lib/time';

  const item = $derived(reader.selected);
  const host = (url: string) => {
    try {
      return new URL(url).hostname;
    } catch {
      return '';
    }
  };
  let headline = $state<HTMLElement>();

  // Pictures load as they scroll into view and never tell the feed's site which page asked.
  // One that cannot load is hidden instead of leaving a broken icon. The HTML itself was
  // cleaned by the server.
  function enhance(node: HTMLElement, _content: string) {
    const run = () => {
      for (const img of node.querySelectorAll('img')) {
        img.loading = 'lazy';
        img.decoding = 'async';
        img.referrerPolicy = 'no-referrer';
        img.addEventListener('error', () => img.classList.add('broken'), { once: true });
      }
    };
    run();
    return { update: run };
  }

  // On a phone the article replaces the list, so move focus to it instead of leaving it on a hidden button.
  $effect(() => {
    if (item?.id === undefined || !ui.narrow || reader.pane !== 'article') return;
    tick().then(() => headline?.focus({ preventScroll: true }));
  });
</script>

<section class="pane" aria-label="Article" tabindex="-1">
  <header>
    <button class="icon mobile-only" aria-label="Back to articles" onclick={() => reader.goList()}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 6l-6 6 6 6" /></svg>
    </button>
    <button class="icon" aria-label="Previous article" disabled={!reader.canGoPrevious} onclick={() => reader.step(-1)}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 15l6-6 6 6" /></svg>
    </button>
    <button class="icon" aria-label="Next article" disabled={!reader.canGoNext} onclick={() => reader.step(1)}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 9l6 6 6-6" /></svg>
    </button>
    <span class="grow"></span>
    {#if item}
      <button class="small" onclick={() => reader.toggleRead()}>{item.unread ? 'Mark read' : 'Mark unread'}</button>
      {#if item.url}
        <a class="button open" href={item.url} target="_blank" rel="noopener noreferrer" aria-label="Open original article">
          <span class="label">Open original</span>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M7 17 17 7" /><path d="M8 7h9v9" /></svg>
        </a>
      {/if}
    {/if}
  </header>

  {#if reader.loadingItem && !item}
    <p class="placeholder" role="status">Loading…</p>
  {:else if item}
    <article>
      <!-- On a phone this is the only pane on screen, so it needs the page heading too. -->
      {#if ui.narrow}<h1 class="sr-only">Reading: {item.feedTitle}</h1>{/if}
      <h2 class="headline" tabindex="-1" bind:this={headline}>{item.title}</h2>
      <p class="byline">
        {item.feedTitle}{item.author ? ` · ${item.author}` : ''} · {when(item.publishedAt)}
      </p>
      {#if reader.loadingFull}
        <p class="fullnote" role="status">Loading the full article…</p>
      {:else if item.fullText}
        <p class="fullnote">
          Showing the full article{host(item.url) ? ` from ${host(item.url)}` : ''}.
          <button class="linkbtn" onclick={() => reader.unloadFull()}>Unload full article</button>
        </p>
      {/if}
      <!-- Sanitized by the server with an allow-list before it is sent. -->
      <div class="content" use:enhance={item.content}>{@html item.content}</div>
      {#if !reader.loadingFull && !item.fullText}
        {#if item.summaryOnly}
          <div class="hint">
            <p>This is only the summary the feed provides.</p>
            {#if item.url}<button class="button" onclick={() => reader.loadFull()}>Load full article</button>{/if}
          </div>
        {:else if item.url}
          <button class="loadfull" onclick={() => reader.loadFull()}>Load full article</button>
        {/if}
      {/if}
    </article>
  {:else}
    {@const shortcuts = appearance.shortcuts && !ui.narrow}
    <div class="placeholder">
      <!-- The raven says the line in a speech bubble, perched on the shortcuts box when it shows. -->
      <div class="scene">
        <div class="perch" class:onbox={shortcuts}>
          <p class="bubble"><span class="body">Select an article to read it.</span></p>
          <Mascot size={63} />
        </div>
        {#if shortcuts}
          <p class="hint"><kbd>↑</kbd> <kbd>↓</kbd> (or <kbd>j</kbd> <kbd>k</kbd>) move between articles, <kbd>←</kbd> <kbd>→</kbd> between panes, <kbd>m</kbd> marks read or unread.</p>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .pane {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    overflow-y: auto;
  }
  .pane:focus {
    outline: none;
  }
  header {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 60px;
    padding: 0 16px 0 8px;
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
    flex: none;
    width: 44px;
    height: 44px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    display: grid;
    place-items: center;
    transition: background-color 0.12s;
  }
  .icon:hover:not(:disabled) {
    background: var(--selected);
  }
  .icon:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .small {
    flex: none;
    min-height: 44px;
    margin-right: 4px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
    font-size: 13px;
    transition: background-color 0.12s;
  }
  .small:hover {
    background: var(--selected);
  }
  .open {
    flex: none;
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
    text-wrap: balance;
    overflow-wrap: anywhere;
  }
  .headline:focus {
    outline: none;
  }
  .byline {
    margin: 0 0 32px;
    font-size: 13px;
    color: var(--muted);
  }
  .hint {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px 16px;
    margin: 28px 0 0;
    padding: 12px 14px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
  }
  .hint p {
    margin: 0;
    font-size: 14px;
  }
  .fullnote {
    margin: -20px 0 24px;
    font-size: 13px;
    color: var(--muted);
  }
  .linkbtn {
    margin-left: 6px;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--accent);
    font-size: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .loadfull {
    margin: 16px 0 0 -8px;
    padding: 0 8px;
    min-height: 44px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--accent);
    font-size: 13px;
    transition: background-color 0.12s;
  }
  .loadfull:hover {
    background: var(--selected);
  }
  .content {
    font-family: Georgia, 'Iowan Old Style', 'Palatino Linotype', serif;
    font-size: 18px;
    line-height: 1.65;
    overflow-wrap: anywhere;
  }
  .content :global(> :first-child) {
    margin-top: 0;
  }
  .content :global(h3),
  .content :global(h4),
  .content :global(h5),
  .content :global(h6) {
    margin: 1.6em 0 0.5em;
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
    line-height: 1.3;
  }
  .content :global(h3) {
    font-size: 1.25em;
  }
  .content :global(h4) {
    font-size: 1.1em;
  }
  .content :global(a) {
    text-underline-offset: 2px;
  }
  .content :global(img),
  .content :global(video) {
    max-width: 100%;
    height: auto;
    border-radius: 8px;
  }
  .content :global(img.broken) {
    display: none;
  }
  .content :global(figure) {
    margin: 1.4em 0;
  }
  .content :global(figcaption) {
    margin-top: 6px;
    font-family: system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif;
    font-size: 13px;
    color: var(--muted);
  }
  .content :global(table) {
    display: block;
    max-width: 100%;
    overflow-x: auto;
    border-collapse: collapse;
    font-size: 15px;
  }
  .content :global(th),
  .content :global(td) {
    padding: 6px 10px;
    border: 1px solid var(--border);
  }
  .content :global(pre) {
    overflow-x: auto;
    padding: 12px 14px;
    background: var(--surface);
    border-radius: 8px;
    font-size: 14px;
    line-height: 1.5;
  }
  .content :global(code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 0.88em;
  }
  .content :global(:not(pre) > code) {
    padding: 1px 5px;
    background: var(--surface);
    border-radius: 4px;
  }
  .content :global(blockquote) {
    margin: 1.2em 0;
    padding-left: 16px;
    border-left: 3px solid var(--border);
    color: var(--muted);
  }
  .content :global(hr) {
    margin: 2em 0;
    border: 0;
    border-top: 1px solid var(--border);
  }
  .placeholder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 80px 24px;
    text-align: center;
    color: var(--muted);
  }
  .placeholder p {
    margin: 0;
  }
  .placeholder {
    gap: 0;
  }
  /* As wide as the shortcuts box, so the raven lands on its corner. */
  .scene {
    display: flex;
    flex-direction: column;
    align-items: center;
    max-width: 100%;
  }
  .perch {
    display: flex;
    align-items: flex-start;
    gap: 14px;
  }
  /* On the box: the raven stands on its top-right corner, feet on the border. */
  .perch.onbox {
    align-self: stretch;
    justify-content: flex-end;
    padding-right: 20px;
  }
  /* Pixel art like the raven: drawn at 63px it is 3px a pixel, and so is every step here.
     The body is the border colour, clipped to stepped corners, with the fill inset one pixel. */
  .placeholder .bubble {
    --p: 3px;
    position: relative;
    isolation: isolate;
    margin-top: 6px;
    color: var(--text);
    line-height: 21px;
  }
  .bubble .body {
    position: relative;
    z-index: 0;
    display: block;
    padding: 9px 15px;
    background: var(--border);
    white-space: nowrap;
    clip-path: polygon(
      calc(var(--p) * 2) 0, calc(100% - var(--p) * 2) 0,
      calc(100% - var(--p) * 2) var(--p), calc(100% - var(--p)) var(--p),
      calc(100% - var(--p)) calc(var(--p) * 2), 100% calc(var(--p) * 2),
      100% calc(100% - var(--p) * 2), calc(100% - var(--p)) calc(100% - var(--p) * 2),
      calc(100% - var(--p)) calc(100% - var(--p)), calc(100% - var(--p) * 2) calc(100% - var(--p)),
      calc(100% - var(--p) * 2) 100%, calc(var(--p) * 2) 100%,
      calc(var(--p) * 2) calc(100% - var(--p)), var(--p) calc(100% - var(--p)),
      var(--p) calc(100% - var(--p) * 2), 0 calc(100% - var(--p) * 2),
      0 calc(var(--p) * 2), var(--p) calc(var(--p) * 2),
      var(--p) var(--p), calc(var(--p) * 2) var(--p)
    );
  }
  .bubble .body::before {
    content: '';
    position: absolute;
    inset: var(--p);
    z-index: -1;
    background: var(--surface);
    clip-path: polygon(
      var(--p) 0, calc(100% - var(--p)) 0, calc(100% - var(--p)) var(--p), 100% var(--p),
      100% calc(100% - var(--p)), calc(100% - var(--p)) calc(100% - var(--p)),
      calc(100% - var(--p)) 100%, var(--p) 100%, var(--p) calc(100% - var(--p)),
      0 calc(100% - var(--p)), 0 var(--p), var(--p) var(--p)
    );
  }
  /* The tail: a staircase pointing at the raven's beak, 4 by 5 pixels, its first column over
     the body's right border. ::before is the outline, ::after the fill that opens it. */
  .bubble::before,
  .bubble::after {
    content: '';
    position: absolute;
    top: calc(var(--p) * 4);
    right: calc(var(--p) * -3);
    width: calc(var(--p) * 4);
    height: calc(var(--p) * 5);
  }
  .bubble::before {
    z-index: -1;
    background: var(--border);
    clip-path: polygon(0 0, 50% 0, 50% 20%, 75% 20%, 75% 40%, 100% 40%, 100% 60%, 75% 60%, 75% 80%, 50% 80%, 50% 100%, 0 100%);
  }
  .bubble::after {
    z-index: 1;
    background: var(--surface);
    clip-path: polygon(0 20%, 50% 20%, 50% 40%, 75% 40%, 75% 60%, 50% 60%, 50% 80%, 0 80%);
  }
  .placeholder .hint {
    margin: 0;
    font-size: 13px;
  }
  kbd {
    padding: 1px 6px;
    border: 1px solid var(--border);
    border-bottom-width: 2px;
    border-radius: 5px;
    background: var(--surface);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
    color: var(--text);
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
    header {
      padding-right: 8px;
    }
    /* Make room for the buttons: the link keeps its name for screen readers. */
    .label {
      display: none;
    }
    .open {
      width: 44px;
      padding: 0;
    }
  }
</style>
