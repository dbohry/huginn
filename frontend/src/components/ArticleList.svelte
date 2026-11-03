<script lang="ts">
  import { link } from '../lib/router.svelte';
  import { ago, updated } from '../lib/time';
  import { reader } from '../lib/reader.svelte';
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';

  const feed = $derived(reader.currentFeed);
  const firstRun = $derived(reader.feeds.length === 0 && !reader.loadingList);

  async function unfollow() {
    if (feed && confirm(`Stop following ${feed.title}? Its articles are deleted too.`)) await reader.removeFeed(feed.id);
  }

  const emptyText = $derived(
    reader.unreadOnly ? 'Nothing unread. You are all caught up.' : feed ? 'No articles in this feed yet.' : 'No articles yet.',
  );

  // Unread articles in what is shown: the open feed, or every feed.
  const unreadCount = $derived(feed ? feed.unread : reader.totalUnread);

  const reduceMotion = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

  // A row that leaves the Unread list fades while the rows below glide up. A whole new list
  // (a filter change, a refresh) replaces rows too, and that must not animate row by row.
  function leave(node: HTMLElement) {
    if (reader.loadingList || reduceMotion()) return { duration: 0 };
    const height = node.offsetHeight;
    return {
      duration: 300,
      easing: cubicOut,
      css: (t: number) => `height:${t * height}px;opacity:${t};overflow:hidden;`,
    };
  }

  // Bumped once the list for a new filter has arrived, so the rows ease in then, not before.
  let swap = $state(0);
  async function choose(unreadOnly: boolean) {
    if (unreadOnly === reader.unreadOnly) return;
    await reader.setUnreadOnly(unreadOnly);
    swap++;
  }

  // Loads the next page when the end of the list scrolls close, so there is no button to hunt for.
  // The sentinel is recreated after every page, so a page that is still short of the end loads the next.
  function nearEnd(node: HTMLElement) {
    if (typeof IntersectionObserver === 'undefined') return;
    const watcher = new IntersectionObserver((entries) => entries[0]?.isIntersecting && reader.more(), {
      root: node.closest('.list'),
      rootMargin: '600px',
    });
    watcher.observe(node);
    return { destroy: () => watcher.disconnect() };
  }

  // Keeps the open article in view while moving through the list with the keyboard.
  $effect(() => {
    const id = reader.selectedId;
    if (id !== null) document.getElementById(`item-${id}`)?.scrollIntoView({ block: 'nearest' });
  });
</script>

<section class="list" aria-label="Articles" aria-busy={reader.loadingList}>
  <header>
    <button class="icon mobile-only" aria-label="Show feeds" onclick={() => reader.showFeeds()}>
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
    </button>
    <h1 tabindex="-1">{reader.title}</h1>
    <button class="icon" aria-label="Refresh all feeds" disabled={reader.refreshing || firstRun} onclick={() => reader.refresh()}>
      <svg class:spin={reader.refreshing} width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-3-6.7" /><path d="M21 4v5h-5" /></svg>
    </button>
  </header>

  {#if !firstRun}
    <div class="tools">
      <div class="seg" role="group" aria-label="Show" class:all={!reader.unreadOnly}>
        <span class="thumb" aria-hidden="true"></span>
        <button type="button" aria-pressed={reader.unreadOnly} onclick={() => choose(true)}>
          Unread
          {#if unreadCount > 0}
            {#key unreadCount}<span class="n" in:fly={{ y: 8, duration: reduceMotion() ? 0 : 250 }}>{unreadCount}</span>{/key}
          {/if}
        </button>
        <button type="button" aria-pressed={!reader.unreadOnly} onclick={() => choose(false)}>All</button>
      </div>
      <button class="small markall" aria-label={feed ? `Mark all read in ${feed.title}` : undefined} onclick={() => reader.markAllRead()}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12l5 5L18 6" /><path d="M12 17l1 1L22 7" /></svg>
        Mark all read
      </button>
    </div>
  {/if}

  {#if feed}
    <div class="feedbar">
      <p class="updated">
        {updated(feed.lastFetchedAt)}
        {#if feed.error}<span class="feederror">Could not be fetched: {feed.error}</span>{/if}
      </p>
      <div class="feedactions">
        <button class="act" disabled={reader.refreshingFeedId !== null} onclick={() => reader.refreshFeed(feed.id)}>
          <svg class:spin={reader.refreshingFeedId === feed.id} width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-3-6.7" /><path d="M21 4v5h-5" /></svg>
          {reader.refreshingFeedId === feed.id ? 'Refreshing…' : 'Refresh'}
        </button>
        <a class="act" href={feed.siteUrl || feed.url} target="_blank" rel="noopener noreferrer">Visit site</a>
        <span class="grow"></span>
        <button class="act danger" onclick={unfollow}>Unfollow</button>
      </div>
      <div class="topicrow">
        <label for="feed-topic">Topic</label>
        {#key feed.id}
          <input
            id="feed-topic"
            list="feed-topics"
            maxlength="40"
            value={feed.category}
            placeholder="None"
            onchange={(e) => reader.setFeedCategory(feed.id, e.currentTarget.value)}
          />
        {/key}
        <datalist id="feed-topics">
          {#each reader.topics as t (t)}<option value={t}></option>{/each}
        </datalist>
      </div>
      <label class="check auto">
        <input type="checkbox" checked={feed.fullText} onchange={(e) => reader.setFeedFullText(feed.id, e.currentTarget.checked)} />
        Load full articles automatically
      </label>
    </div>
  {/if}

  {#if reader.listError}
    <div class="error" role="alert">
      <span>{reader.listError}</span>
      <button class="small" onclick={() => reader.retry()}>Try again</button>
    </div>
  {/if}

  {#if firstRun && !reader.listError}
    <div class="first">
      <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M4 11a9 9 0 0 1 9 9" /><path d="M4 4a16 16 0 0 1 16 16" /><circle cx="5" cy="19" r="1.2" fill="var(--accent)" /></svg>
      <h2>Follow your first feed</h2>
      <p>Paste the address of a feed or of a website, and Huginn finds its feed. Coming from another reader? Import its OPML file.</p>
      <div class="firstactions">
        <button class="button" onclick={() => (reader.addOpen = true)}>Add feed</button>
        <button class="button secondary" onclick={() => (reader.starterOpen = true)}>Browse starter feeds</button>
        <a class="button secondary" href="/settings" use:link>Import OPML</a>
      </div>
    </div>
  {:else}
    {#key swap}
    <ul class:swap={swap > 0}>
      {#each reader.items as item (item.id)}
        <li out:leave>
          <button
            id="item-{item.id}"
            class="item"
            class:active={reader.selectedId === item.id}
            aria-current={reader.selectedId === item.id ? 'true' : undefined}
            onclick={() => reader.open(item)}
          >
            <span class="dot" class:unread={item.unread} aria-hidden="true"></span>
            <span class="sr-only">{item.unread ? 'Unread. ' : ''}</span>
            <span class="meta">
              <span class="feed">{item.feedTitle}</span>
              <span>{ago(item.publishedAt)}</span>
            </span>
            <span class="title" class:unread={item.unread}>{item.title}</span>
            {#if item.snippet}<span class="snippet">{item.snippet}</span>{/if}
          </button>
        </li>
      {/each}
    </ul>
    {/key}

    {#if reader.loadingList && reader.items.length === 0}
      <p class="empty" role="status">Loading…</p>
    {:else if reader.items.length === 0 && !reader.listError}
      <p class="empty">{emptyText}</p>
    {/if}

    {#if reader.next}
      {#key reader.items.length}
        <div class="more" use:nearEnd>
          <button class="button secondary" disabled={reader.loadingMore} onclick={() => reader.more()}>
            {reader.loadingMore ? 'Loading…' : 'Load more'}
          </button>
        </div>
      {/key}
    {/if}
  {/if}
</section>

<style>
  .list {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    border-right: 1px solid var(--border);
    overflow-x: hidden;
    overflow-y: auto;
  }
  /* Nothing inside may push the list wider than its pane. On iOS Safari a single over-wide
     row makes the whole page pan sideways. */
  .list > :global(*) {
    min-width: 0;
    max-width: 100%;
  }
  li {
    min-width: 0;
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
  h1:focus {
    outline: none;
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
    transition: background-color 0.12s;
  }
  .icon:hover:not(:disabled) {
    background: var(--selected);
  }
  .icon:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .spin {
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .tools {
    flex-wrap: wrap;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 8px 6px 16px;
    border-bottom: 1px solid var(--border);
  }
  /* Two-way switch. The thumb slides between the halves. */
  .seg {
    position: relative;
    display: grid;
    grid-template-columns: 1fr 1fr;
    padding: 3px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
  }
  .seg button {
    position: relative;
    z-index: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: 34px;
    padding: 0 14px;
    border: 0;
    border-radius: 7px;
    background: transparent;
    color: var(--muted);
    font-size: 13px;
    font-weight: 500;
    transition: color 0.2s;
  }
  .seg button[aria-pressed='true'] {
    color: var(--text);
  }
  .thumb {
    position: absolute;
    top: 3px;
    bottom: 3px;
    left: 3px;
    width: calc(50% - 3px);
    border-radius: 7px;
    background: var(--bg);
    box-shadow: 0 0 0 1px var(--border);
    transition: transform 0.25s cubic-bezier(0.3, 0.9, 0.3, 1);
  }
  .seg.all .thumb {
    transform: translateX(100%);
  }
  .n {
    display: inline-block;
    font-size: 12px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--accent);
  }
  .markall {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border-color: transparent;
    color: var(--accent);
    font-weight: 500;
  }
  .swap {
    animation: swap 0.28s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  @keyframes swap {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .thumb,
    .seg button {
      transition: none;
    }
    .swap {
      animation: none;
    }
  }
  .small {
    min-height: 36px;
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
  .feedbar {
    padding: 8px 8px 4px 16px;
    border-bottom: 1px solid var(--border);
  }
  .updated {
    margin: 0 0 4px;
    font-size: 12px;
    color: var(--muted);
  }
  .feederror {
    display: block;
    margin-top: 2px;
    color: var(--bad);
    overflow-wrap: anywhere;
  }
  .feedactions {
    display: flex;
    align-items: center;
    gap: 2px;
    margin-left: -8px;
  }
  .grow {
    flex: 1;
  }
  .topicrow {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  .topicrow input {
    flex: 1;
    min-width: 0;
    min-height: 32px;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg);
  }
  .auto {
    min-height: 40px;
    margin-left: 0;
    font-size: 13px;
  }
  .act {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 40px;
    padding: 0 8px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--accent);
    font-size: 13px;
    text-decoration: none;
    transition: background-color 0.12s;
  }
  .act:hover:not(:disabled) {
    background: var(--selected);
  }
  .act:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .act.danger {
    color: var(--bad);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .item {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 100%;
    min-width: 0;
    max-width: 100%;
    padding: 14px 16px 14px 30px;
    border: 0;
    border-bottom: 1px solid var(--border);
    background: transparent;
    text-align: left;
    transition: background-color 0.12s;
  }
  .item:hover,
  .item.active {
    background: var(--selected);
  }
  /* The unread dot hangs in the left margin, so text lines up whether or not it shows. */
  .dot {
    position: absolute;
    left: 13px;
    top: 19px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .dot.unread {
    background: var(--accent);
  }
  .meta {
    min-width: 0;
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
  .title {
    font-size: 14px;
    line-height: 1.35;
    overflow-wrap: anywhere;
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
    overflow-wrap: anywhere;
    min-width: 0;
    max-width: 100%;
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
    display: flex;
    align-items: center;
    gap: 12px;
    justify-content: space-between;
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
  .first {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 56px 28px;
    text-align: center;
  }
  .first h2 {
    margin: 4px 0 0;
    font-size: 18px;
  }
  .first p {
    margin: 0;
    max-width: 300px;
    line-height: 1.5;
    color: var(--muted);
  }
  .firstactions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
    margin-top: 8px;
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
  @media (pointer: coarse) {
    .act {
      min-height: 44px;
    }
  }
</style>
