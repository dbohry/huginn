<script lang="ts">
  import { ago, updated } from '../lib/time';
  import { reader } from '../lib/reader.svelte';

  async function unfollow() {
    const feed = reader.currentFeed;
    if (feed && confirm(`Stop following ${feed.title}? Its articles are deleted too.`)) await reader.removeFeed(feed.id);
  }

  const emptyText = $derived(
    reader.unreadOnly ? 'Nothing unread. You are all caught up.' : reader.feeds.length === 0 ? 'Add a feed to get started.' : 'No articles yet.',
  );
</script>

<section class="list" aria-label="Articles">
  <header>
    <button class="icon mobile-only" aria-label="Show feeds" onclick={() => (reader.pane = 'feeds')}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
    </button>
    <h1>{reader.title}</h1>
    <button class="icon" aria-label="Refresh feeds" disabled={reader.refreshing} onclick={() => reader.refresh()}>
      <svg class:spin={reader.refreshing} width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-3-6.7" /><path d="M21 4v5h-5" /></svg>
    </button>
  </header>

  <div class="tools">
    <label class="check">
      <input type="checkbox" checked={reader.unreadOnly} onchange={(e) => reader.setUnreadOnly(e.currentTarget.checked)} />
      Unread only
    </label>
    <button class="small" onclick={() => reader.markAllRead()}>Mark all read</button>
  </div>

  {#if reader.currentFeed}
    <div class="feedbar">
      {#if reader.currentFeed.error}
        <p class="feederror">Could not be fetched: {reader.currentFeed.error}</p>
      {/if}
      <p class="updated">{updated(reader.currentFeed.lastFetchedAt)}</p>
      <div class="feedlinks">
        <a href={reader.currentFeed.siteUrl || reader.currentFeed.url} target="_blank" rel="noopener noreferrer">Visit site</a>
        <button class="plain" disabled={reader.refreshingFeedId !== null} onclick={() => reader.refreshFeed(reader.currentFeed!.id)}>
          {reader.refreshingFeedId === reader.currentFeed.id ? 'Refreshing…' : 'Refresh'}
        </button>
        <button class="plain" onclick={unfollow}>Unfollow</button>
      </div>
    </div>
  {/if}

  {#if reader.error}<p class="error" role="alert">{reader.error}</p>{/if}

  <ul>
    {#each reader.items as item (item.id)}
      <li>
        <button
          class="item"
          class:active={reader.selectedId === item.id}
          aria-current={reader.selectedId === item.id ? 'true' : undefined}
          onclick={() => reader.open(item)}
        >
          <span class="meta">
            <span class="dot" class:unread={item.unread} aria-hidden="true"></span>
            <span class="sr-only">{item.unread ? 'Unread. ' : ''}</span>
            <span class="feed">{item.feedTitle}</span>
            <span>{ago(item.publishedAt)}</span>
          </span>
          <span class="title" class:unread={item.unread}>{item.title}</span>
          {#if item.snippet}<span class="snippet">{item.snippet}</span>{/if}
        </button>
      </li>
    {/each}
  </ul>

  {#if !reader.loadingList && reader.items.length === 0}
    <p class="empty">{emptyText}</p>
  {/if}
  {#if reader.loadingList}<p class="empty" role="status">Loading…</p>{/if}

  {#if reader.next}
    <div class="more">
      <button class="button secondary" disabled={reader.loadingMore} onclick={() => reader.more()}>
        {reader.loadingMore ? 'Loading…' : 'Load more'}
      </button>
    </div>
  {/if}
</section>

<style>
  .list {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    border-right: 1px solid var(--border);
    overflow-y: auto;
  }
  header {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 60px;
    padding: 0 8px 0 16px;
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    background: var(--bg);
    z-index: 1;
  }
  h1 {
    flex: 1;
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .icon {
    width: 44px;
    height: 44px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    display: grid;
    place-items: center;
  }
  .icon:hover:not(:disabled) {
    background: var(--selected);
  }
  .spin {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation: none;
    }
  }
  .tools {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px 0 16px;
    border-bottom: 1px solid var(--border);
  }
  .tools .check {
    min-height: 44px;
    font-size: 13px;
  }
  .small {
    min-height: 36px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
    font-size: 13px;
  }
  .small:hover {
    background: var(--selected);
  }
  .feedbar {
    padding: 4px 8px 4px 16px;
    border-bottom: 1px solid var(--border);
  }
  .feederror {
    margin: 8px 0 4px;
    font-size: 13px;
    color: var(--bad);
    overflow-wrap: anywhere;
  }
  .updated {
    margin: 8px 0 0;
    font-size: 12px;
    color: var(--muted);
  }
  .feedlinks {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .feedlinks a,
  .plain {
    display: inline-flex;
    align-items: center;
    min-height: 44px;
    padding: 0 8px 0 0;
    border: 0;
    background: transparent;
    color: var(--accent);
    font-size: 13px;
    text-decoration: none;
  }
  .plain {
    padding: 0 8px;
  }
  .plain:last-child {
    padding-right: 0;
  }
  .plain:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .feedlinks a:hover,
  .plain:hover {
    text-decoration: underline;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .item {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 100%;
    padding: 14px 16px;
    border: 0;
    border-bottom: 1px solid var(--border);
    background: transparent;
    text-align: left;
  }
  .item:hover,
  .item.active {
    background: var(--selected);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--muted);
  }
  .feed {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex: none;
  }
  .dot.unread {
    background: var(--accent);
  }
  .title {
    font-size: 14px;
    line-height: 1.35;
  }
  .title.unread {
    font-weight: 600;
  }
  .snippet {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    font-size: 13px;
    line-height: 1.45;
    color: var(--muted);
  }
  .empty {
    margin: 0;
    padding: 32px 16px;
    text-align: center;
    color: var(--muted);
  }
  .error {
    margin: 12px 16px 0;
    padding: 10px 12px;
    border: 1px solid var(--bad);
    border-radius: 8px;
    font-size: 13px;
  }
  .more {
    display: flex;
    justify-content: center;
    padding: 16px;
  }
  .mobile-only {
    display: none;
  }
  @media (max-width: 899px) {
    .list {
      border-right: 0;
    }
    .mobile-only {
      display: grid;
    }
  }
</style>
