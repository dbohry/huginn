<script lang="ts">
  import AddFeedDialog from './AddFeedDialog.svelte';
  import Logo from './Logo.svelte';
  import { link } from '../lib/router.svelte';
  import { reader } from '../lib/reader.svelte';
  import { session } from '../lib/session.svelte';
  import { ui } from '../lib/ui.svelte';

  let adding = $state(false);
</script>

<nav class="sidebar" aria-label="Feeds">
  <div class="brand">
    <Logo size={30} />
    <span>Huginn</span>
  </div>

  <ul>
    <li>
      <button class="row" class:active={reader.feedId === null} aria-current={reader.feedId === null ? 'true' : undefined} onclick={() => reader.chooseFeed(null)}>
        <span class="name">All items</span>
        <span class="count">{reader.totalUnread}</span>
      </button>
    </li>
  </ul>

  <svelte:element this={ui.narrow ? 'h1' : 'h2'} class="heading">Feeds</svelte:element>
  <ul class="feeds">
    {#each reader.feeds as feed (feed.id)}
      <li>
        <button
          class="row"
          class:active={reader.feedId === feed.id}
          aria-current={reader.feedId === feed.id ? 'true' : undefined}
          onclick={() => reader.chooseFeed(feed.id)}
        >
          <span class="name">{feed.title}</span>
          {#if feed.error}<span class="warn" title={feed.error}><span class="sr-only">Could not be fetched. </span>!</span>{/if}
          <span class="count">{feed.unread}</span>
        </button>
      </li>
    {:else}
      <li class="empty">No feeds yet.</li>
    {/each}
  </ul>

  <div class="foot">
    <button class="button" onclick={() => (adding = true)}>Add feed</button>
    <div class="links">
      <a href="/settings" use:link>Settings</a>
      <button class="plain" onclick={() => session.logout()}>Sign out</button>
    </div>
  </div>
</nav>

{#if adding}<AddFeedDialog onclose={() => (adding = false)} />{/if}

<style>
  .sidebar {
    /* Contains the hidden screen-reader text inside, so it cannot stretch the page. */
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-height: 0;
    padding: 16px 12px;
    background: var(--surface);
    border-right: 1px solid var(--border);
    overflow-y: auto;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 8px 12px;
    font-size: 16px;
    font-weight: 600;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .heading {
    margin: 12px 0 4px;
    padding: 0 8px;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .feeds {
    flex: 1;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 40px;
    padding: 0 8px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    text-align: left;
  }
  .row:hover {
    background: var(--selected);
  }
  .row.active {
    background: var(--selected);
    font-weight: 600;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .count {
    font-size: 12px;
    color: var(--muted);
  }
  .warn {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--bad);
    color: var(--bg);
    display: grid;
    place-items: center;
    font-size: 12px;
    font-weight: 700;
  }
  .empty {
    padding: 8px;
    color: var(--muted);
  }
  .foot {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }
  .links {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .links a,
  .plain {
    display: inline-flex;
    align-items: center;
    min-height: 44px;
    padding: 0 8px;
    border: 0;
    background: transparent;
    color: var(--accent);
    text-decoration: none;
  }
  .plain:hover,
  .links a:hover {
    text-decoration: underline;
  }
</style>
