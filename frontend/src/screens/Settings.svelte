<script lang="ts">
  import Field from '../components/Field.svelte';
  import Logo from '../components/Logo.svelte';
  import StarterAdmin from '../components/StarterAdmin.svelte';
  import StarterDialog from '../components/StarterDialog.svelte';
  import TempPasswordDialog from '../components/TempPasswordDialog.svelte';
  import { api, ApiError, type BlockedWord, type UserSummary } from '../lib/api';
  import { appearance, type Mode } from '../lib/appearance.svelte';
  import { reader } from '../lib/reader.svelte';
  import { link } from '../lib/router.svelte';
  import { session } from '../lib/session.svelte';
  import { since } from '../lib/time';

  const messageOf = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

  let starterOpen = $state(false);

  // --- sections: only the selected one is shown ---
  type Section = 'account' | 'appearance' | 'filters' | 'subscriptions' | 'users' | 'starter' | 'about';
  let selected = $state<Section>('account');
  const general: { id: Section; label: string }[] = [
    { id: 'account', label: 'Account' },
    { id: 'appearance', label: 'Appearance' },
    { id: 'filters', label: 'Filters' },
    { id: 'subscriptions', label: 'Subscriptions' },
  ];
  const admin: { id: Section; label: string }[] = [
    { id: 'users', label: 'Users' },
    { id: 'starter', label: 'Starter feeds' },
  ];

  // --- account ---
  let current = $state('');
  let next = $state('');
  let confirmNext = $state('');
  let pwError = $state('');
  let pwDone = $state(false);
  let pwBusy = $state(false);
  let pwOpen = $state(false);
  const pwReady = $derived(current !== '' && next !== '' && confirmNext !== '');

  async function changePassword(e: SubmitEvent) {
    e.preventDefault();
    pwError = '';
    pwDone = false;
    if ([...next].length < 8) {
      pwError = 'Password must be at least 8 characters.';
      return;
    }
    if (next !== confirmNext) {
      pwError = 'The two passwords do not match.';
      return;
    }
    pwBusy = true;
    try {
      await session.changePassword(current, next);
      current = next = confirmNext = '';
      pwOpen = false;
      pwDone = true;
    } catch (err) {
      pwError = messageOf(err);
    } finally {
      pwBusy = false;
    }
  }

  function togglePassword() {
    pwOpen = !pwOpen;
    pwError = '';
    pwDone = false;
    if (!pwOpen) current = next = confirmNext = '';
  }

  // --- appearance ---
  const modes: { value: Mode; label: string }[] = [
    { value: 'system', label: 'Device' },
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
  ];
  const shortcuts: { keys: string; does: string }[] = [
    { keys: 'j', does: 'Next article' },
    { keys: 'k', does: 'Previous article' },
    { keys: 'm', does: 'Mark read or unread' },
    { keys: 'o', does: 'Open the original' },
    { keys: 'f', does: 'Load the full article' },
    { keys: 'r', does: 'Refresh all feeds' },
  ];

  // --- subscriptions ---
  let importMessage = $state('');
  let importError = $state('');

  async function importFile(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    importMessage = importError = '';
    if (!file) return;
    try {
      const r = await api.importOpml(await file.text());
      importMessage = `Added ${r.added}, already followed ${r.existing}, skipped ${r.invalid} invalid. New feeds are being fetched.`;
      reader.reloadSoon();
    } catch (err) {
      importError = messageOf(err);
    }
  }

  // --- filters ---
  let blocked = $state<BlockedWord[]>([]);
  let blockedNew = $state('');
  let blockedError = $state('');

  $effect(() => {
    api.blockedWords().then((r) => (blocked = r.words), (err) => (blockedError = messageOf(err)));
  });

  async function addBlocked(e: SubmitEvent) {
    e.preventDefault();
    blockedError = '';
    try {
      const { word } = await api.addBlockedWord(blockedNew);
      blocked = [...blocked, word].sort((a, b) => a.word.localeCompare(b.word));
      blockedNew = '';
      void reader.reloadNow();
    } catch (err) {
      blockedError = messageOf(err);
    }
  }

  async function removeBlocked(b: BlockedWord) {
    blockedError = '';
    try {
      await api.deleteBlockedWord(b.id);
      blocked = blocked.filter((x) => x.id !== b.id);
      void reader.reloadNow();
    } catch (err) {
      blockedError = messageOf(err);
    }
  }

  // --- users (administrators) ---
  let users = $state<UserSummary[]>([]);
  let usersError = $state('');
  let newName = $state('');
  let newAdmin = $state(false);
  let temp = $state<{ username: string; password: string; created: boolean } | null>(null);

  async function loadUsers() {
    try {
      users = (await api.users()).users;
    } catch (err) {
      usersError = messageOf(err);
    }
  }

  $effect(() => {
    if (session.user?.isAdmin) loadUsers();
  });

  async function addUser(e: SubmitEvent) {
    e.preventDefault();
    usersError = '';
    try {
      const r = await api.createUser(newName, newAdmin);
      temp = { username: r.user.username, password: r.temporaryPassword, created: true };
      newName = '';
      newAdmin = false;
      await loadUsers();
    } catch (err) {
      usersError = messageOf(err);
    }
  }

  async function resetUser(u: UserSummary) {
    usersError = '';
    try {
      const r = await api.resetUserPassword(u.id);
      temp = { username: u.username, password: r.temporaryPassword, created: false };
      await loadUsers();
    } catch (err) {
      usersError = messageOf(err);
    }
  }

  async function removeUser(u: UserSummary) {
    if (!confirm(`Remove ${u.username}? Their feeds and articles are deleted too.`)) return;
    usersError = '';
    try {
      await api.deleteUser(u.id);
      await loadUsers();
    } catch (err) {
      usersError = messageOf(err);
    }
  }

  const lastSeen = (u: UserSummary) => (u.lastSeenAt === null ? 'Never signed in' : since('Last seen', u.lastSeenAt));
</script>

{#snippet navButton(item: { id: Section; label: string })}
  <button type="button" class="navlink" class:on={selected === item.id} aria-current={selected === item.id ? 'true' : undefined} onclick={() => (selected = item.id)}>
    {item.label}
  </button>
{/snippet}

<main class="page">
  <header class="top">
    <a class="back" href="/" use:link>
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 6l-6 6 6 6" /></svg>
      Back to reading
    </a>
    <div class="title">
      <Logo size={32} />
      <h1>Settings</h1>
    </div>
  </header>

  <div class="layout">
    <nav class="nav" aria-label="Settings sections">
      {#each general as item (item.id)}{@render navButton(item)}{/each}
      {#if session.user?.isAdmin}
        <span class="navgroup">Administration</span>
        {#each admin as item (item.id)}{@render navButton(item)}{/each}
      {/if}
      <span class="navgap"></span>
      {@render navButton({ id: 'about', label: 'About' })}
    </nav>

    <div class="content">
      {#if selected === 'account'}
        <section aria-labelledby="account">
          <div class="head">
            <h2 id="account">Account</h2>
            <p class="sub">Your sign-in details.</p>
          </div>
          <div class="group">
            <div class="row">
              <div class="text">
                <span class="lbl">{session.user?.username}{#if session.user?.isAdmin}<span class="badge">Administrator</span>{/if}</span>
                <span class="desc">Signed in</span>
              </div>
              <button class="button secondary" type="button" onclick={() => session.logout()}>Sign out</button>
            </div>
            <div class="row">
              <div class="text">
                <span class="lbl">Password</span>
                <span class="desc">Changing it signs you out on your other devices.</span>
              </div>
              <button class="button secondary" type="button" aria-expanded={pwOpen} onclick={togglePassword}>{pwOpen ? 'Cancel' : 'Change password'}</button>
            </div>
            {#if pwDone}
              <div class="row tint"><p class="ok" role="status">Password changed. Other devices have been signed out.</p></div>
            {/if}
            {#if pwOpen}
              <form class="row tint stack" onsubmit={changePassword}>
                <div class="fields">
                  <Field label="Current password" type="password" bind:value={current} autocomplete="current-password" />
                  <Field label="New password" type="password" bind:value={next} autocomplete="new-password" hint="At least 8 characters." />
                  <Field label="Confirm new password" type="password" bind:value={confirmNext} autocomplete="new-password" />
                  {#if pwError}<p class="error" role="alert">{pwError}</p>{/if}
                  <div><button class="button" type="submit" disabled={pwBusy || !pwReady}>{pwBusy ? 'Saving…' : 'Change password'}</button></div>
                </div>
              </form>
            {/if}
          </div>
        </section>
      {:else if selected === 'appearance'}
        <section aria-labelledby="appearance">
          <div class="head">
            <h2 id="appearance">Appearance</h2>
            <p class="sub">Kept in this browser only.</p>
          </div>
          <div class="group">
            <div class="row">
              <div class="text">
                <span class="lbl">Theme</span>
                <span class="desc">Match your device, or pick one.</span>
              </div>
              <fieldset class="segments">
                <legend class="sr-only">Theme</legend>
                <div class="segmented">
                  {#each modes as m (m.value)}
                    <label class="segment">
                      <input type="radio" name="mode" value={m.value} checked={appearance.mode === m.value} onchange={() => appearance.set(m.value)} />
                      <span>{m.label}</span>
                    </label>
                  {/each}
                </div>
              </fieldset>
            </div>
            <div class="row">
              <div class="text">
                <span class="lbl">Keyboard shortcuts</span>
                <span class="desc">Single keys that work while reading. Turn them off if you would rather not press one by accident.</span>
              </div>
              <label class="switch">
                <input type="checkbox" role="switch" checked={appearance.shortcuts} onchange={(e) => appearance.setShortcuts(e.currentTarget.checked)} aria-label="Keyboard shortcuts" />
                <span class="track" aria-hidden="true"></span>
              </label>
            </div>
            <dl class="row tint keys" class:off={!appearance.shortcuts}>
              {#each shortcuts as s (s.keys)}
                <div>
                  <dt><kbd>{s.keys}</kbd></dt>
                  <dd>{s.does}</dd>
                </div>
              {/each}
            </dl>
          </div>
        </section>
      {:else if selected === 'filters'}
        <section aria-labelledby="filters">
          <div class="head">
            <h2 id="filters">Filters</h2>
            <p class="sub">Hide articles whose headline contains a word you do not want to see.</p>
          </div>
          <div class="group">
            <form class="row stack" onsubmit={addBlocked}>
              <div class="text">
                <label class="lbl" for="blocked-new">Blocked words</label>
                <span class="desc">Whole words, in any capitalization. “bitcoin” hides “Bitcoin hits a new high” but not “Bitcoiner”. A short phrase works too. Hidden articles are not counted as unread.</span>
              </div>
              <div class="addword">
                <input id="blocked-new" class="text-input" placeholder="Add a word or phrase" autocomplete="off" autocapitalize="off" spellcheck="false" bind:value={blockedNew} />
                <button class="button" type="submit" disabled={blockedNew.trim() === ''}>Add</button>
              </div>
              {#if blockedError}<p class="error" role="alert">{blockedError}</p>{/if}
              {#if blocked.length > 0}
                <ul class="chips" aria-label="Blocked words">
                  {#each blocked as b (b.id)}
                    <li class="chip">
                      <span>{b.word}</span>
                      <button type="button" class="chipx" aria-label={`Unblock ${b.word}`} onclick={() => removeBlocked(b)}>✕</button>
                    </li>
                  {/each}
                </ul>
              {/if}
            </form>
          </div>
        </section>
      {:else if selected === 'subscriptions'}
        <section aria-labelledby="subs">
          <div class="head">
            <h2 id="subs">Subscriptions</h2>
            <p class="sub">Move your feeds in or out with an OPML file, the format every feed reader understands.</p>
          </div>
          <div class="group">
            <div class="row">
              <div class="text">
                <span class="lbl">Import</span>
                <span class="desc">Add feeds from an OPML file. New feeds are fetched right away.</span>
              </div>
              <label class="button secondary file">
                Import OPML
                <input type="file" accept=".opml,.xml,text/xml,application/xml,text/x-opml" onchange={importFile} />
              </label>
            </div>
            <div class="row">
              <div class="text">
                <span class="lbl">Export</span>
                <span class="desc">Download everything you follow.</span>
              </div>
              <a class="button secondary" href="/api/opml" download="huginn-subscriptions.opml">Export OPML</a>
            </div>
            <div class="row">
              <div class="text">
                <span class="lbl">Starter feeds</span>
                <span class="desc">Browse a short list of good places to begin.</span>
              </div>
              <button class="button secondary" type="button" onclick={() => (starterOpen = true)}>Browse starter feeds</button>
            </div>
            {#if importMessage}<div class="row tint"><p class="ok" role="status">{importMessage}</p></div>{/if}
            {#if importError}<div class="row tint"><p class="error" role="alert">{importError}</p></div>{/if}
          </div>
        </section>
      {:else if selected === 'users' && session.user?.isAdmin}
        <section aria-labelledby="users">
          <div class="head">
            <h2 id="users">Users</h2>
            <p class="sub">Each person has their own feeds and read state. New users get a temporary password to change at their first sign-in.</p>
          </div>
          {#if usersError}<p class="error" role="alert">{usersError}</p>{/if}
          <div class="group">
            {#each users as u (u.id)}
              <div class="row">
                <div class="who">
                  <span class="avatar" aria-hidden="true">{u.username.slice(0, 1).toUpperCase()}</span>
                  <div class="text">
                    <span class="lbl">
                      {u.username}
                      {#if u.isAdmin}<span class="badge">Administrator</span>{/if}
                      {#if u.id === session.user?.id}<span class="badge">You</span>{/if}
                    </span>
                    <span class="desc">{u.mustChangePassword ? 'Has a temporary password' : lastSeen(u)}</span>
                  </div>
                </div>
                {#if u.id !== session.user?.id}
                  <div class="rowactions">
                    <button class="button quiet" type="button" onclick={() => resetUser(u)}>Reset password</button>
                    <button class="button quiet danger" type="button" onclick={() => removeUser(u)}>Remove</button>
                  </div>
                {/if}
              </div>
            {/each}
            <form class="row tint adduser" onsubmit={addUser}>
              <label class="sr-only" for="new-user">New username</label>
              <input id="new-user" class="text-input" placeholder="New username" autocomplete="off" autocapitalize="off" spellcheck="false" bind:value={newName} />
              <label class="check"><input type="checkbox" bind:checked={newAdmin} /> Administrator</label>
              <button class="button" type="submit" disabled={newName.trim() === ''}>Add user</button>
            </form>
          </div>
        </section>
      {:else if selected === 'starter' && session.user?.isAdmin}
        <StarterAdmin />
      {:else if selected === 'about'}
        <section aria-labelledby="about">
          <div class="head">
            <h2 id="about">About</h2>
          </div>
          <div class="group">
            <div class="row">
              <div class="text">
                <span class="lbl">Huginn {session.version || ''}</span>
                <span class="desc">A small self-hosted RSS reader.</span>
              </div>
              <a class="button secondary" href="https://github.com/dbohry/huginn" target="_blank" rel="noopener noreferrer">Source and documentation</a>
            </div>
          </div>
        </section>
      {/if}
    </div>
  </div>
</main>

{#if starterOpen}<StarterDialog onclose={() => (starterOpen = false)} />{/if}

{#if temp}
  <TempPasswordDialog username={temp.username} password={temp.password} created={temp.created} onclose={() => (temp = null)} />
{/if}

<style>
  .page {
    max-width: 920px;
    margin: 0 auto;
    padding: 8px 16px calc(64px + var(--safe-bottom));
    display: flex;
    flex-direction: column;
    gap: 24px;
  }
  .top {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .back {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-height: 44px;
    margin-left: -8px;
    padding: 0 10px 0 6px;
    border-radius: 8px;
    color: var(--muted);
    text-decoration: none;
    transition: background-color 0.12s;
  }
  .back:hover {
    background: var(--selected);
    color: var(--text);
  }
  .title {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h1 {
    margin: 0;
    font-size: 28px;
    letter-spacing: -0.02em;
  }

  .layout {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 24px 48px;
  }
  .nav {
    flex: 0 0 180px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .navlink {
    display: flex;
    align-items: center;
    min-height: 40px;
    padding: 0 12px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--muted);
    text-align: left;
    white-space: nowrap;
    transition: background-color 0.12s;
  }
  .navlink:hover {
    background: var(--selected);
    color: var(--text);
  }
  .navlink.on {
    background: var(--selected);
    color: var(--text);
    font-weight: 500;
  }
  .navgroup {
    margin: 16px 12px 4px;
    font-size: 12px;
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .navgap {
    height: 8px;
  }
  .content {
    flex: 999 1 480px;
    min-width: 0;
  }
  @media (max-width: 719px) {
    .nav {
      flex: 1 1 100%;
      flex-direction: row;
      align-items: center;
      gap: 4px;
      overflow-x: auto;
      padding-bottom: 4px;
    }
    .navlink {
      min-height: 44px;
    }
    .navgroup,
    .navgap {
      display: none;
    }
  }

  .head {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 16px;
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  .sub {
    margin: 0;
    line-height: 1.5;
    color: var(--muted);
  }

  .group {
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 12px 24px;
    margin: 0;
    padding: 16px 20px;
    border-top: 1px solid var(--border);
  }
  .row:first-child {
    border-top: 0;
  }
  .row.tint {
    background: var(--surface);
  }
  .row.stack {
    align-items: stretch;
    justify-content: flex-start;
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1 1 220px;
    min-width: 0;
  }
  .lbl {
    font-weight: 500;
  }
  .desc {
    font-size: 13px;
    line-height: 1.45;
    color: var(--muted);
  }
  .badge {
    display: inline-block;
    margin-left: 6px;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--surface);
    font-size: 12px;
    font-weight: 400;
    line-height: 20px;
    color: var(--muted);
    vertical-align: 1px;
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 16px;
    width: 100%;
    max-width: 360px;
  }
  .error {
    margin: 0;
    font-size: 13px;
    color: var(--bad);
  }
  .ok {
    margin: 0;
    font-size: 13px;
    line-height: 1.5;
  }
  .rowactions :global(.button.quiet) {
    border-color: transparent;
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

  /* Theme: a segmented control made of real radio buttons. */
  .segments {
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  .segmented {
    display: inline-flex;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--surface);
  }
  .segment {
    position: relative;
  }
  .segment input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    opacity: 0;
    cursor: pointer;
  }
  .segment span {
    display: flex;
    align-items: center;
    min-height: 36px;
    padding: 0 16px;
    border-radius: 7px;
    color: var(--muted);
    transition: background-color 0.12s;
  }
  .segment input:checked + span {
    background: var(--bg);
    color: var(--text);
    font-weight: 500;
    box-shadow: 0 0 0 1px var(--border), 0 1px 2px rgb(0 0 0 / 0.12);
  }
  .segment input:not(:checked):hover + span {
    color: var(--text);
  }
  .segment:has(input:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
    border-radius: 8px;
  }

  /* Shortcuts switch: a real checkbox drawn as a switch. */
  .switch {
    position: relative;
    flex: none;
    display: inline-block;
    width: 44px;
    height: 26px;
  }
  .switch input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    opacity: 0;
    cursor: pointer;
  }
  .track {
    position: absolute;
    inset: 0;
    border-radius: 13px;
    background: var(--muted);
    transition: background-color 0.12s;
    pointer-events: none;
  }
  .track::after {
    content: '';
    position: absolute;
    top: 3px;
    left: 3px;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--bg);
    transition: transform 0.12s;
  }
  .switch input:checked + .track {
    background: var(--accent);
  }
  .switch input:checked + .track::after {
    transform: translateX(18px);
    background: var(--on-accent);
  }
  .switch:has(input:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
    border-radius: 13px;
  }

  .keys {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 8px 16px;
  }
  .keys.off {
    opacity: 0.5;
  }
  .keys div {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  dt,
  dd {
    margin: 0;
  }
  dt {
    min-width: 28px;
    text-align: center;
  }
  kbd {
    display: inline-block;
    min-width: 24px;
    padding: 1px 6px;
    border: 1px solid var(--border);
    border-bottom-width: 2px;
    border-radius: 5px;
    background: var(--bg);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
  }

  .who {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: 1 1 220px;
    min-width: 0;
  }
  .avatar {
    flex: none;
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: var(--selected);
    display: grid;
    place-items: center;
    font-weight: 600;
  }
  .rowactions {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .adduser {
    justify-content: flex-start;
  }
  .text-input {
    flex: 1 1 200px;
    max-width: 280px;
    min-height: 44px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg);
  }
  .addword {
    display: flex;
    gap: 8px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 2px 2px 12px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--bg);
  }
  .chipx {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border: 0;
    border-radius: 50%;
    background: transparent;
    color: var(--muted);
  }
  .chipx:hover {
    background: var(--selected);
    color: var(--bad);
  }
  .adduser .button {
    margin-left: auto;
  }
  @media (pointer: coarse) {
    .segment span {
      min-height: 44px;
    }
  }
</style>
