<script lang="ts">
  import Field from '../components/Field.svelte';
  import Logo from '../components/Logo.svelte';
  import TempPasswordDialog from '../components/TempPasswordDialog.svelte';
  import { api, ApiError, type UserSummary } from '../lib/api';
  import { appearance, type Mode } from '../lib/appearance.svelte';
  import { reader } from '../lib/reader.svelte';
  import { link } from '../lib/router.svelte';
  import { session } from '../lib/session.svelte';
  import { since } from '../lib/time';

  const messageOf = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

  // --- account ---
  let current = $state('');
  let next = $state('');
  let confirmNext = $state('');
  let pwError = $state('');
  let pwDone = $state(false);
  let pwBusy = $state(false);
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
      pwDone = true;
    } catch (err) {
      pwError = messageOf(err);
    } finally {
      pwBusy = false;
    }
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

  <section class="card" aria-labelledby="account">
    <div class="head">
      <h2 id="account">Account</h2>
      <p class="who">
        Signed in as <strong>{session.user?.username}</strong>
        {#if session.user?.isAdmin}<span class="badge">Administrator</span>{/if}
      </p>
    </div>
    <form class="form" onsubmit={changePassword}>
      <Field label="Current password" type="password" bind:value={current} autocomplete="current-password" />
      <Field label="New password" type="password" bind:value={next} autocomplete="new-password" hint="At least 8 characters." />
      <Field label="Confirm new password" type="password" bind:value={confirmNext} autocomplete="new-password" />
      {#if pwError}<p class="error" role="alert">{pwError}</p>{/if}
      {#if pwDone}<p class="ok" role="status">Password changed. Other devices have been signed out.</p>{/if}
      <div class="actions">
        <button class="button" type="submit" disabled={pwBusy || !pwReady}>{pwBusy ? 'Saving…' : 'Change password'}</button>
        <button class="button secondary" type="button" onclick={() => session.logout()}>Sign out</button>
      </div>
    </form>
  </section>

  <section class="card" aria-labelledby="appearance">
    <div class="head">
      <h2 id="appearance">Appearance</h2>
      <p class="sub">Kept in this browser only.</p>
    </div>
    <fieldset class="segments">
      <legend class="label">Theme</legend>
      <div class="segmented">
        {#each modes as m (m.value)}
          <label class="segment">
            <input type="radio" name="mode" value={m.value} checked={appearance.mode === m.value} onchange={() => appearance.set(m.value)} />
            <span>{m.label}</span>
          </label>
        {/each}
      </div>
    </fieldset>
  </section>

  <section class="card" aria-labelledby="keys">
    <div class="head">
      <h2 id="keys">Keyboard shortcuts</h2>
      <p class="sub">Single keys that work while reading. Turn them off if you would rather not risk pressing one by accident.</p>
    </div>
    <label class="check">
      <input type="checkbox" checked={appearance.shortcuts} onchange={(e) => appearance.setShortcuts(e.currentTarget.checked)} />
      Use keyboard shortcuts
    </label>
    <dl class="keys" class:off={!appearance.shortcuts}>
      {#each shortcuts as s (s.keys)}
        <div>
          <dt><kbd>{s.keys}</kbd></dt>
          <dd>{s.does}</dd>
        </div>
      {/each}
    </dl>
  </section>

  <section class="card" aria-labelledby="subs">
    <div class="head">
      <h2 id="subs">Subscriptions</h2>
      <p class="sub">Move your feeds in or out with an OPML file, the format every feed reader understands.</p>
    </div>
    <div class="actions">
      <a class="button secondary" href="/api/opml" download="huginn-subscriptions.opml">Export OPML</a>
      <label class="button secondary file">
        Import OPML
        <input type="file" accept=".opml,.xml,text/xml,application/xml,text/x-opml" onchange={importFile} />
      </label>
    </div>
    {#if importMessage}<p class="ok" role="status">{importMessage}</p>{/if}
    {#if importError}<p class="error" role="alert">{importError}</p>{/if}
  </section>

  {#if session.user?.isAdmin}
    <section class="card" aria-labelledby="users">
      <div class="head">
        <h2 id="users">Users</h2>
        <p class="sub">Each person has their own feeds and read state. New users get a temporary password to change at their first sign-in.</p>
      </div>
      {#if usersError}<p class="error" role="alert">{usersError}</p>{/if}
      <ul class="users">
        {#each users as u (u.id)}
          <li>
            <span class="avatar" aria-hidden="true">{u.username.slice(0, 1).toUpperCase()}</span>
            <div class="info">
              <span class="name">
                <strong>{u.username}</strong>
                {#if u.isAdmin}<span class="badge">Administrator</span>{/if}
                {#if u.id === session.user?.id}<span class="badge">You</span>{/if}
              </span>
              <span class="muted">{u.mustChangePassword ? 'Has a temporary password' : lastSeen(u)}</span>
            </div>
            {#if u.id !== session.user?.id}
              <div class="rowactions">
                <button class="button secondary" onclick={() => resetUser(u)}>Reset password</button>
                <button class="button danger" onclick={() => removeUser(u)}>Remove</button>
              </div>
            {/if}
          </li>
        {/each}
      </ul>
      <form class="adduser" onsubmit={addUser}>
        <div class="fields">
          <Field label="New username" bind:value={newName} autocomplete="off" />
          <label class="check"><input type="checkbox" bind:checked={newAdmin} /> Administrator</label>
          <div><button class="button" type="submit" disabled={newName.trim() === ''}>Add user</button></div>
        </div>
      </form>
    </section>
  {/if}

  <section class="card" aria-labelledby="about">
    <div class="head">
      <h2 id="about">About</h2>
      <p class="sub">
        Huginn {session.version || ''}, a small self-hosted RSS reader.
        <a href="https://github.com/dbohry/huginn" target="_blank" rel="noopener noreferrer">Source and documentation</a>
      </p>
    </div>
  </section>
</main>

{#if temp}
  <TempPasswordDialog username={temp.username} password={temp.password} created={temp.created} onclose={() => (temp = null)} />
{/if}

<style>
  .page {
    max-width: 680px;
    margin: 0 auto;
    padding: 8px 16px calc(64px + var(--safe-bottom));
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .top {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-bottom: 8px;
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
    text-decoration: none;
    transition: background-color 0.12s;
  }
  .back:hover {
    background: var(--selected);
  }
  .title {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h1 {
    margin: 0;
    font-size: 26px;
    letter-spacing: -0.01em;
  }
  .card {
    padding: 20px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 14px;
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
  .sub,
  .who {
    margin: 0;
    line-height: 1.5;
    color: var(--muted);
  }
  .who strong {
    color: var(--text);
  }
  .badge {
    display: inline-block;
    margin-left: 4px;
    padding: 0 8px;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg);
    font-size: 12px;
    line-height: 20px;
    color: var(--muted);
    vertical-align: 1px;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
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
  }
  .label {
    padding: 0;
    font-size: 13px;
    font-weight: 500;
    margin-bottom: 8px;
  }
  .segmented {
    display: inline-flex;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--border);
    border-radius: 11px;
    background: var(--bg);
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
    min-height: 38px;
    padding: 0 18px;
    border-radius: 8px;
    transition: background-color 0.12s;
  }
  .segment input:checked + span {
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 500;
  }
  .segment input:not(:checked):hover + span {
    background: var(--selected);
  }
  .segment:has(input:focus-visible) {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
    border-radius: 8px;
  }

  .keys {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 8px 16px;
    margin: 0;
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

  .users {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .users li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    padding: 12px 0;
    border-top: 1px solid var(--border);
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
  .info {
    flex: 1 1 160px;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .muted {
    font-size: 12px;
    color: var(--muted);
  }
  .rowactions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .adduser {
    padding-top: 16px;
    border-top: 1px solid var(--border);
  }
  .fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 320px;
  }
  @media (pointer: coarse) {
    .segment span {
      min-height: 44px;
    }
  }
</style>
