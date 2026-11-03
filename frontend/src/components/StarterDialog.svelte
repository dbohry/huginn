<script lang="ts">
  import Modal from './Modal.svelte';
  import { api, ApiError, type StarterTopic } from '../lib/api';
  import { reader } from '../lib/reader.svelte';
  import { session } from '../lib/session.svelte';

  let { onclose }: { onclose: () => void } = $props();

  // How many feeds of a topic are ticked when the topic is chosen. A few, so one topic cannot flood the list.
  const PRESET = 4;

  let topics = $state<StarterTopic[]>([]);
  let loading = $state(true);
  let error = $state('');
  let busy = $state(false);
  let chosen = $state<Record<string, boolean>>({});
  let ticked = $state<Record<number, boolean>>({});
  let finished = false;

  async function load() {
    loading = true;
    error = '';
    try {
      topics = (await api.starter()).topics;
      // Start with the first two topics chosen, so there is something to accept.
      for (const t of topics.slice(0, 2)) choose(t, true);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Something went wrong.';
    } finally {
      loading = false;
    }
  }
  $effect(() => {
    load();
  });

  function choose(topic: StarterTopic, on: boolean) {
    chosen[topic.name] = on;
    topic.feeds.forEach((f, i) => {
      if (!f.followed) ticked[f.id] = on && i < PRESET;
    });
  }

  const shown = $derived(topics.filter((t) => chosen[t.name]));
  const count = $derived(Object.values(ticked).filter(Boolean).length);
  const allTicked = (t: StarterTopic) => t.feeds.filter((f) => !f.followed).every((f) => ticked[f.id]);

  function tickAll(t: StarterTopic) {
    const on = !allTicked(t);
    for (const f of t.feeds) if (!f.followed) ticked[f.id] = on;
  }

  // Closing without choosing counts as "no thanks", so the offer is not made again at every sign-in.
  async function close() {
    if (finished) return;
    finished = true;
    if (!session.user?.starterDone) {
      session.starterDone();
      api.dismissStarter().catch(() => {});
    }
    onclose();
  }

  async function add() {
    busy = true;
    error = '';
    try {
      const ids = Object.entries(ticked).filter(([, on]) => on).map(([id]) => Number(id));
      const r = await api.subscribeStarter(ids);
      finished = true;
      session.starterDone();
      reader.notify(r.added === 1 ? 'Following 1 new feed' : `Following ${r.added} new feeds`);
      onclose();
      await Promise.all([reader.loadFeeds(), reader.loadItems()]);
      reader.reloadSoon();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Something went wrong.';
    } finally {
      busy = false;
    }
  }

  let chipsEl = $state<HTMLElement>();
  $effect(() => {
    chipsEl?.querySelector('button')?.focus();
  });
</script>

<Modal title="Start with a few feeds" onclose={close} wide>
  <p class="lead">Choose topics you like. You can unfollow any of these later, or add your own with Add feed.</p>

  {#if loading}
    <p class="note" role="status">Loading…</p>
  {:else if error && topics.length === 0}
    <div class="problem" role="alert">
      <span>{error}</span>
      <button class="button secondary" onclick={load}>Try again</button>
    </div>
  {:else if topics.length === 0}
    <p class="note">The administrator has not set up any starter feeds. Use Add feed to follow one yourself.</p>
  {:else}
    <div class="chips" role="group" aria-label="Topics" bind:this={chipsEl}>
      {#each topics as t (t.name)}
        <button
          class="chip"
          aria-pressed={!!chosen[t.name]}
          onclick={() => choose(t, !chosen[t.name])}
        >
          {t.name} <span class="n">{t.feeds.length}</span>
        </button>
      {/each}
    </div>

    <div class="list" tabindex="-1">
      {#each shown as t (t.name)}
        <div class="topic">
          <h3>{t.name}</h3>
          <button class="link" onclick={() => tickAll(t)}>{allTicked(t) ? 'Clear' : 'Select all'}</button>
        </div>
        {#each t.feeds as f (f.id)}
          <label class="item">
            {#if f.followed}
              <input type="checkbox" checked disabled />
            {:else}
              <input type="checkbox" bind:checked={ticked[f.id]} />
            {/if}
            <span class="t">
              <b>{f.title}</b>
              <span>{f.followed ? 'Already following' : new URL(f.url).hostname}</span>
            </span>
          </label>
        {/each}
      {:else}
        <p class="note">Choose a topic above to see its feeds.</p>
      {/each}
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  {/if}

  <div class="foot">
    <span class="sum" aria-live="polite"><b>{count}</b> {count === 1 ? 'feed' : 'feeds'} selected</span>
    <button class="button secondary" type="button" onclick={close}>
      {session.user?.starterDone ? 'Cancel' : 'Start empty'}
    </button>
    <button class="button" type="button" disabled={busy || count === 0} onclick={add}>
      {busy ? 'Adding…' : count ? `Add ${count} ${count === 1 ? 'feed' : 'feeds'}` : 'Add feeds'}
    </button>
  </div>
</Modal>

<style>
  .lead {
    margin: -8px 0 12px;
    color: var(--muted);
    line-height: 1.5;
  }
  .note {
    margin: 12px 0;
    color: var(--muted);
  }
  .problem {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    color: var(--bad);
  }
  .error {
    margin: 8px 0 0;
    font-size: 13px;
    color: var(--bad);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 12px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 36px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 18px;
    background: transparent;
  }
  .chip[aria-pressed='true'] {
    background: var(--selected);
    border-color: var(--accent);
    font-weight: 500;
  }
  .n {
    font-size: 12px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }
  .list {
    max-height: min(360px, 42vh);
    overflow-y: auto;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
  }
  .topic {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 4px 2px;
  }
  h3 {
    margin: 0;
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .link {
    min-height: 32px;
    padding: 0 4px;
    border: 0;
    background: transparent;
    color: var(--accent);
    font-size: 12px;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 44px;
    padding: 4px;
    border-radius: 6px;
    cursor: pointer;
  }
  .item:hover {
    background: var(--surface);
  }
  .item input {
    flex: none;
    width: 18px;
    height: 18px;
    margin: 0;
    accent-color: var(--accent);
  }
  .t {
    flex: 1;
    min-width: 0;
  }
  .t b,
  .t span {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .t b {
    font-weight: 500;
  }
  .t span {
    font-size: 12px;
    color: var(--muted);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 16px;
  }
  .sum {
    flex: 1 1 120px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }
  .sum b {
    color: var(--text);
  }
</style>
