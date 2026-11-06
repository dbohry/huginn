<script lang="ts">
  import { link } from '../lib/router.svelte';
  import { ago, updated } from '../lib/time';
  import { reader } from '../lib/reader.svelte';
  import { fade, fly } from 'svelte/transition';
  import { untrack } from 'svelte';
  import { cubicOut } from 'svelte/easing';

  const feed = $derived(reader.currentFeed);
  const firstRun = $derived(reader.feeds.length === 0 && !reader.loadingList);

  async function unfollow() {
    if (feed && confirm(`Stop following ${feed.title}? Its articles are deleted too.`)) await reader.removeFeed(feed.id);
  }

  const emptyText = $derived(
    feed ? 'No articles in this feed yet.' : 'No articles yet.',
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

  // The raven's wing strokes in three poses: raised, level and lowered. The wings beat by morphing between them.
  const WINGS = [
    'M4 6.5C7 7.5 10 10.5 12 15C14 10.5 17 7.5 20 6.5',
    'M2 11.5C5.5 10 9.5 10.5 12 13.5C14.5 10.5 18.5 10 22 11.5',
    'M4.5 17.5C6.5 13.5 9.5 11.5 12 12C14.5 11.5 17.5 13.5 19.5 17.5',
  ];
  const WINGBEAT = [WINGS[0], WINGS[1], WINGS[2], WINGS[1], WINGS[0]].join(';');

  // Articles a refresh adds to the list that is already showing. They unfold in from the top.
  // A different list (another feed or filter) shares no articles with the old one, so nothing in it counts.
  let shownIds = new Set<number>();
  let arrivals: number[] = [];
  $effect.pre(() => {
    const ids = reader.items.map((i) => i.id);
    const sameList = ids.some((id) => shownIds.has(id));
    const refreshing = untrack(() => reader.refreshing || reader.refreshingFeedId !== null);
    arrivals = refreshing && sameList ? ids.filter((id) => !shownIds.has(id)) : [];
    shownIds = new Set(ids);
  });
  function arrive(node: HTMLElement, id: number) {
    const order = arrivals.indexOf(id);
    if (order < 0 || reduceMotion()) return { duration: 0 };
    const height = node.offsetHeight;
    return {
      delay: Math.min(order, 5) * 60,
      duration: 340,
      easing: cubicOut,
      css: (t: number) => `height:${t * height}px;opacity:${t};overflow:hidden;transform:translateY(${(t - 1) * 6}px);`,
    };
  }

  // After every feed has been refreshed, the button shows a ring closing on a tick for a moment.
  let refreshed = $state(false);
  $effect(() => {
    if (!reader.refreshedAt) return;
    refreshed = true;
    const timer = setTimeout(() => (refreshed = false), 1100);
    return () => clearTimeout(timer);
  });

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
    <button
      class="icon refresh"
      class:busy={reader.refreshing}
      class:refreshed
      aria-label="Refresh all feeds"
      disabled={reader.refreshing || firstRun}
      onclick={() => reader.refresh()}
    >
      <svg class="arrow" class:spin={reader.refreshing} width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-3-6.7" /><path d="M21 4v5h-5" /></svg>
      <svg class="done" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle class="ring" cx="12" cy="12" r="9" /><path class="tk" d="M8.2 12.4l2.6 2.6L16 9.8" /></svg>
    </button>
    {#if reader.refreshing}
      <!-- Huginn, the raven, glides along the header's edge and draws the progress line behind it. -->
      <span class="glide" aria-hidden="true" transition:fade={{ duration: reduceMotion() ? 0 : 200 }}>
        <span class="run">
          <span class="trail"></span>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d={WINGS[1]}>
              <animate attributeName="d" dur="0.55s" repeatCount="indefinite" calcMode="spline" keyTimes="0;.25;.5;.75;1" keySplines=".45 0 .55 1;.45 0 .55 1;.45 0 .55 1;.45 0 .55 1" values={WINGBEAT} />
            </path>
          </svg>
        </span>
      </span>
    {/if}
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
        <li in:arrive={item.id} out:leave>
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
    {:else if reader.items.length === 0 && !reader.listError && reader.unreadOnly}
      <div class="caughtup" role="status">
        <span class="ring">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path class="tick" d="M5 12.5l4.5 4.5L19 7.5" /></svg>
        </span>
        <h2>You're all caught up</h2>
        <p>Nothing unread right now. New articles will show up here.</p>
        <button class="button secondary" onclick={() => choose(false)}>Show all items</button>
      </div>
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
  /* Eases in and out of every turn, so it reads as working rather than stuck. */
  .spin {
    animation: spin 0.95s cubic-bezier(0.45, 0.05, 0.55, 0.95) infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  /* Refresh all feeds: the arrow turns in the accent colour while the raven glides, then a ring closes on a tick. */
  .refresh > svg {
    grid-area: 1 / 1;
  }
  .refresh.busy:disabled {
    opacity: 1;
    color: var(--accent);
  }
  .refresh .arrow {
    transition:
      transform 0.32s cubic-bezier(0.2, 0.8, 0.2, 1),
      opacity 0.18s;
  }
  .refresh.refreshed .arrow {
    opacity: 0;
    transform: rotate(-140deg) scale(0.6);
  }
  .refresh .done {
    color: var(--accent);
    opacity: 0;
    transition: opacity 0.2s;
  }
  .refresh.refreshed .done {
    opacity: 1;
  }
  .done .ring {
    stroke-dasharray: 57;
    stroke-dashoffset: 57;
    transform: rotate(-90deg);
    transform-origin: 12px 12px;
  }
  .done .tk {
    stroke-dasharray: 12;
    stroke-dashoffset: 12;
  }
  .refreshed .done .ring {
    stroke-dashoffset: 0;
    transition: stroke-dashoffset 0.34s cubic-bezier(0.2, 0.8, 0.2, 1);
  }
  .refreshed .done .tk {
    stroke-dashoffset: 0;
    transition: stroke-dashoffset 0.24s cubic-bezier(0.2, 0.8, 0.2, 1) 0.2s;
  }
  .glide {
    position: absolute;
    left: 0;
    right: 0;
    bottom: -1px;
    height: 0;
    pointer-events: none;
    color: var(--accent);
  }
  .run {
    position: absolute;
    left: 0;
    top: 0;
    width: 100%;
    animation: glide 2s cubic-bezier(0.45, 0.05, 0.55, 0.95) infinite;
  }
  .trail {
    position: absolute;
    right: 100%;
    bottom: -1px;
    width: 96px;
    height: 2px;
    margin-right: -4px;
    background: linear-gradient(90deg, transparent, var(--accent));
  }
  .run svg {
    position: absolute;
    left: -2px;
    bottom: -4px;
  }
  @keyframes glide {
    from {
      transform: translateX(-24px);
    }
    to {
      transform: translateX(calc(100% + 120px));
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation: pulse 1.4s ease-in-out infinite;
    }
    .glide {
      display: none;
    }
    .refresh .arrow,
    .refresh .done,
    .done .ring,
    .done .tk {
      transition: none !important;
    }
  }
  @keyframes pulse {
    50% {
      opacity: 0.4;
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
  .caughtup {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 56px 24px;
    text-align: center;
    animation: caughtin 0.5s ease both;
  }
  .caughtup .ring {
    display: grid;
    place-items: center;
    width: 64px;
    height: 64px;
    margin-bottom: 10px;
    border-radius: 50%;
    background: var(--selected);
    color: var(--accent);
  }
  .caughtup h2 {
    margin: 0;
    font-size: 17px;
  }
  .caughtup p {
    margin: 0 0 14px;
    max-width: 260px;
    color: var(--muted);
  }
  .caughtup .tick {
    stroke-dasharray: 30;
    stroke-dashoffset: 30;
    animation: tickdraw 0.5s 0.25s ease forwards;
  }
  @keyframes tickdraw {
    to {
      stroke-dashoffset: 0;
    }
  }
  @keyframes caughtin {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .caughtup,
    .caughtup .tick {
      animation: none;
      stroke-dashoffset: 0;
    }
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
