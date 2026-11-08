<script lang="ts">
  import Logo from './Logo.svelte';
  import { link } from '../lib/router.svelte';
  import { reader } from '../lib/reader.svelte';
  import { appearance } from '../lib/appearance.svelte';
  import { session } from '../lib/session.svelte';
  import { ui } from '../lib/ui.svelte';
  import { updated } from '../lib/time';
  import type { Feed } from '../lib/api';

  // What a refresh is doing with a feed right now, if anything.
  const busy = (f: Feed) => (f.refresh === 'fetching' || reader.refreshingFeedId === f.id ? 'fetching' : f.refresh);

  // Folded to a strip on a wide screen; a phone always shows the full menu.
  const collapsed = $derived(appearance.sidebarCollapsed && !ui.narrow);

  // Sections by topic. Feeds without a topic come last, under "Other" once any feed has one.
  const hasTopics = $derived(reader.feeds.some((f) => f.category !== ''));
  const grouped = $derived(appearance.groupByTopic && hasTopics);
  const sections = $derived.by(() => {
    if (!grouped) return [{ name: '', feeds: reader.feeds }];
    const by = new Map<string, typeof reader.feeds>();
    for (const f of reader.feeds) by.set(f.category, [...(by.get(f.category) ?? []), f]);
    return [...by.entries()]
      .sort(([a], [b]) => (a === '' ? 1 : b === '' ? -1 : a.localeCompare(b)))
      .map(([name, feeds]) => ({ name: name || 'Other', feeds }));
  });
</script>

<nav class="sidebar" class:collapsed aria-label="Feeds">
  <div class="brand">
    <Logo size={30} />
    {#if !collapsed}<span>Huginn</span>{/if}
    {#if !ui.narrow}
      <button
        class="icon fold"
        type="button"
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        aria-expanded={!collapsed}
        title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        onclick={() => appearance.setSidebarCollapsed(!collapsed)}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <rect x="3" y="4" width="18" height="16" rx="2" /><path d="M9 4v16" />
        </svg>
      </button>
    {/if}
  </div>

  {#if collapsed}
    <div class="foot">
      <button class="icon" type="button" aria-label="Add feed" title="Add feed" onclick={() => (reader.addOpen = true)}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
      </button>
      <a class="icon" href="/settings" use:link aria-label="Settings" title="Settings">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" /></svg>
      </a>
      <button class="icon" type="button" aria-label="Sign out" title="Sign out" onclick={() => session.logout()}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" /></svg>
      </button>
    </div>
  {:else}

  <ul>
    <li>
      <button class="row" class:active={reader.feedId === null} aria-current={reader.feedId === null ? 'true' : undefined} onclick={() => reader.chooseFeed(null)}>
        <span class="name">All items</span>
        {#if reader.totalUnread > 0}<span class="count">{reader.totalUnread}</span>{/if}
      </button>
    </li>
  </ul>

  <div class="headrow">
    <svelte:element this={ui.narrow ? 'h1' : 'h2'} class="heading" tabindex="-1">Feeds</svelte:element>
    {#if hasTopics}
      <button class="group" aria-pressed={appearance.groupByTopic} onclick={() => appearance.setGroupByTopic(!appearance.groupByTopic)}>
        Group by topic
      </button>
    {/if}
  </div>
  {#each sections as section (section.name)}
    {#if section.name}<svelte:element this={ui.narrow ? 'h2' : 'h3'} class="topic">{section.name}</svelte:element>{/if}
    <ul class="feeds">
      {#each section.feeds as feed (feed.id)}
        <li>
          <button
            class="row"
            class:active={reader.feedId === feed.id}
            aria-current={reader.feedId === feed.id ? 'true' : undefined}
            title={`${feed.title}\n${updated(feed.lastFetchedAt, ui.now)}`}
            onclick={() => reader.chooseFeed(feed.id)}
          >
            <span class="name">{feed.title}</span>
            {#if busy(feed) === 'fetching'}
              <svg class="state spin" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-6.2-8.56" /></svg>
              <span class="sr-only">Refreshing.</span>
            {:else if busy(feed) === 'queued'}
              <svg class="state queued" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-dasharray="2 4.3" aria-hidden="true"><circle cx="12" cy="12" r="9" /></svg>
              <span class="sr-only">Waiting to be refreshed.</span>
            {/if}
            {#if feed.error}<span class="warn" title="The last fetch failed: {feed.error}"><span class="sr-only">Could not be fetched. </span>!</span>{/if}
            {#if feed.unread > 0}<span class="count">{feed.unread}</span>{/if}
          </button>
        </li>
      {:else}
        <li class="empty">No feeds yet.</li>
      {/each}
    </ul>
  {/each}

  <div class="foot">
    <button class="button add" onclick={() => (reader.addOpen = true)}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
      Add feed
    </button>
    <div class="account">
      <span class="avatar" aria-hidden="true">{(session.user?.username ?? '').slice(0, 1).toUpperCase()}</span>
      <div class="who">
        <span class="user">{session.user?.username}</span>
        {#if session.user?.isAdmin}<span class="role">Administrator</span>{/if}
      </div>
      <a class="icon" href="/settings" use:link aria-label="Settings" title="Settings">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" /></svg>
      </a>
      <button class="icon" type="button" aria-label="Sign out" title="Sign out" onclick={() => session.logout()}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" /></svg>
      </button>
    </div>
  </div>
  {/if}
</nav>

<style>
  .sidebar {
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
  .sidebar.collapsed {
    align-items: center;
    padding: 16px 8px;
  }
  .sidebar.collapsed .brand {
    flex-direction: column;
    padding: 0 0 12px;
  }
  .sidebar.collapsed .foot {
    align-items: center;
    gap: 4px;
    width: 100%;
  }
  .fold {
    margin-left: auto;
  }
  .sidebar.collapsed .fold {
    margin-left: 0;
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
  .headrow {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    margin-top: 12px;
  }
  .headrow .heading {
    margin: 0 0 4px;
  }
  .group {
    min-height: 28px;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: transparent;
    font-size: 12px;
    color: var(--muted);
  }
  .group[aria-pressed='true'] {
    background: var(--selected);
    border-color: var(--accent);
    color: var(--text);
  }
  .topic {
    margin: 10px 8px 2px;
    font-size: 12px;
    font-weight: 600;
    color: var(--muted);
  }
  .heading:focus {
    outline: none;
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
    transition: background-color 0.12s;
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
    min-width: 20px;
    padding: 0 6px;
    border-radius: 10px;
    background: var(--bg);
    font-size: 12px;
    font-weight: 500;
    line-height: 20px;
    text-align: center;
    color: var(--muted);
  }
  .row.active .count {
    background: var(--accent);
    color: var(--on-accent);
  }
  .state {
    flex: none;
    color: var(--accent);
  }
  .state.queued {
    color: var(--muted);
  }
  /* Eases in and out of every turn, like the refresh button. */
  .spin {
    animation: spin 0.95s cubic-bezier(0.45, 0.05, 0.55, 0.95) infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .spin {
      animation-duration: 3s;
    }
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
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }
  .add {
    min-height: 40px;
  }
  .account {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-left: 4px;
  }
  .avatar {
    flex: none;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--selected);
    display: grid;
    place-items: center;
    font-weight: 600;
  }
  .who {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    line-height: 1.25;
  }
  .user {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
  }
  .role {
    font-size: 12px;
    color: var(--muted);
  }
  .icon {
    flex: none;
    display: inline-grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    transition: background-color 0.12s;
  }
  .icon:hover {
    background: var(--selected);
    color: var(--text);
  }
  @media (pointer: coarse) {
    .row {
      min-height: 44px;
    }
    .add {
      min-height: 44px;
    }
    .icon {
      width: 44px;
      height: 44px;
    }
  }
</style>
