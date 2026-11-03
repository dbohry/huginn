<script lang="ts">
  import Field from './Field.svelte';
  import { api, ApiError, type StarterTopic } from '../lib/api';

  const messageOf = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

  let topics = $state<StarterTopic[]>([]);
  let loading = $state(true);
  let error = $state('');
  let info = $state('');

  let category = $state('');
  let title = $state('');
  let url = $state('');
  let adding = $state(false);

  const total = $derived(topics.reduce((n, t) => n + t.feeds.length, 0));
  const ready = $derived(category.trim() !== '' && url.trim() !== '');

  async function load() {
    try {
      topics = (await api.starter()).topics;
    } catch (e) {
      error = messageOf(e);
    } finally {
      loading = false;
    }
  }
  $effect(() => {
    load();
  });

  async function add(e: SubmitEvent) {
    e.preventDefault();
    error = info = '';
    adding = true;
    try {
      await api.addStarterFeed(category, title, url);
      title = url = '';
      await load();
      info = 'Added. New users will see it in the list.';
    } catch (err) {
      error = messageOf(err);
    } finally {
      adding = false;
    }
  }

  async function remove(id: number) {
    error = info = '';
    try {
      await api.deleteStarterFeed(id);
      await load();
    } catch (err) {
      error = messageOf(err);
    }
  }

  async function reset() {
    if (!confirm('Replace the whole list with the one that comes with Huginn?')) return;
    error = info = '';
    try {
      await api.resetStarter();
      await load();
      info = 'The built-in list is back.';
    } catch (err) {
      error = messageOf(err);
    }
  }

  async function importFile(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    error = info = '';
    if (!file) return;
    if (!confirm('Replace the whole list with the feeds in this file? Folders become topics.')) return;
    try {
      const r = await api.importStarterOpml(await file.text());
      await load();
      info = `The list now has ${r.feeds} feeds.${r.skipped ? ` ${r.skipped} could not be used.` : ''}`;
    } catch (err) {
      error = messageOf(err);
    }
  }
</script>

<section class="card" aria-labelledby="starter">
  <div class="head">
    <h2 id="starter">Starter feeds</h2>
    <p class="sub">
      The list new users are offered at their first sign-in, and anyone can reopen from the sidebar.
      Changing it does not touch feeds people already follow.
    </p>
  </div>

  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if info}<p class="ok" role="status">{info}</p>{/if}

  {#if loading}
    <p class="sub">Loading…</p>
  {:else if total === 0}
    <p class="sub">The list is empty, so new users see no suggestions. Add a feed below, import an OPML file, or restore the built-in list.</p>
  {:else}
    <div class="topics">
      {#each topics as t (t.name)}
        <details>
          <summary>{t.name} <span class="n">{t.feeds.length}</span></summary>
          <ul>
            {#each t.feeds as f (f.id)}
              <li>
                <span class="info">
                  <strong>{f.title}</strong>
                  <span class="muted">{f.url}</span>
                </span>
                <button class="button danger" aria-label="Remove {f.title}" onclick={() => remove(f.id)}>Remove</button>
              </li>
            {/each}
          </ul>
        </details>
      {/each}
    </div>
  {/if}

  <form class="add" onsubmit={add}>
    <h3>Add a feed</h3>
    <div class="fields">
      <div class="field">
        <label for="starter-topic">Topic</label>
        <input id="starter-topic" list="starter-topics" maxlength="40" bind:value={category} autocomplete="off" />
        <datalist id="starter-topics">
          {#each topics as t (t.name)}<option value={t.name}></option>{/each}
        </datalist>
      </div>
      <Field label="Title (optional)" bind:value={title} />
      <Field label="Feed address" type="url" bind:value={url} placeholder="https://example.com/feed.xml" />
      <div><button class="button" type="submit" disabled={adding || !ready}>{adding ? 'Adding…' : 'Add to list'}</button></div>
    </div>
  </form>

  <div class="actions">
    <a class="button secondary" href="/api/starter/opml" download="huginn-starter-feeds.opml">Export OPML</a>
    <label class="button secondary file">
      Replace from OPML
      <input type="file" accept=".opml,.xml,text/xml,application/xml,text/x-opml" onchange={importFile} />
    </label>
    <button class="button secondary" type="button" onclick={reset}>Restore built-in list</button>
  </div>
</section>

<style>
  .card {
    padding: 20px;
    border: 1px solid var(--border);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .head {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  h3 {
    margin: 0 0 8px;
    font-size: 14px;
  }
  .sub {
    margin: 0;
    line-height: 1.5;
    color: var(--muted);
  }
  .error {
    margin: 0;
    font-size: 13px;
    color: var(--bad);
  }
  .ok {
    margin: 0;
    font-size: 13px;
  }
  .topics {
    border-top: 1px solid var(--border);
  }
  details {
    border-bottom: 1px solid var(--border);
  }
  summary {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 44px;
    cursor: pointer;
    font-weight: 500;
  }
  .n {
    font-size: 12px;
    font-weight: 400;
    color: var(--muted);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0 0 8px;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    padding: 6px 0;
  }
  .info {
    flex: 1 1 200px;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .info .muted {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .muted {
    font-size: 12px;
    color: var(--muted);
  }
  li .button {
    min-height: 36px;
  }
  .add {
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 360px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .field label {
    font-size: 13px;
    font-weight: 500;
  }
  .field input {
    min-height: 44px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg);
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .file {
    position: relative;
    cursor: pointer;
  }
  .file input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }
  .file:focus-within {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
