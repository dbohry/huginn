<script lang="ts">
  import Field from '../components/Field.svelte';
  import TempPasswordDialog from '../components/TempPasswordDialog.svelte';
  import { api, ApiError, type UserSummary } from '../lib/api';
  import { appearance, type Mode } from '../lib/appearance.svelte';
  import { reader } from '../lib/reader.svelte';
  import { link } from '../lib/router.svelte';
  import { session } from '../lib/session.svelte';
  import { ago } from '../lib/time';

  const messageOf = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

  // --- account ---
  let current = $state('');
  let next = $state('');
  let confirmNext = $state('');
  let pwError = $state('');
  let pwDone = $state(false);
  let pwBusy = $state(false);

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
    { value: 'system', label: 'Follow the device' },
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
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

  const lastSeen = (u: UserSummary) => (u.lastSeenAt === null ? 'Never signed in' : `Last seen ${ago(u.lastSeenAt)} ago`);
</script>

<div class="page">
  <header>
    <a href="/" use:link class="back">← Back to reading</a>
    <h1>Settings</h1>
  </header>

  <section aria-labelledby="account">
    <h2 id="account">Account</h2>
    <p class="who">Signed in as <strong>{session.user?.username}</strong>{session.user?.isAdmin ? ' (administrator)' : ''}.</p>
    <form class="form" onsubmit={changePassword}>
      <Field label="Current password" type="password" bind:value={current} autocomplete="current-password" />
      <Field label="New password" type="password" bind:value={next} autocomplete="new-password" hint="At least 8 characters." />
      <Field label="Confirm new password" type="password" bind:value={confirmNext} autocomplete="new-password" />
      {#if pwError}<p class="error" role="alert">{pwError}</p>{/if}
      {#if pwDone}<p class="ok" role="status">Password changed. Other devices have been signed out.</p>{/if}
      <div><button class="button" type="submit" disabled={pwBusy}>Change password</button></div>
    </form>
  </section>

  <section aria-labelledby="appearance">
    <h2 id="appearance">Appearance</h2>
    <fieldset>
      <legend class="sr-only">Theme</legend>
      {#each modes as m (m.value)}
        <label class="check">
          <input type="radio" name="mode" value={m.value} checked={appearance.mode === m.value} onchange={() => appearance.set(m.value)} />
          {m.label}
        </label>
      {/each}
    </fieldset>
  </section>

  <section aria-labelledby="subs">
    <h2 id="subs">Subscriptions</h2>
    <p>Move your feeds in or out with an OPML file, the format every feed reader understands.</p>
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
    <section aria-labelledby="users">
      <h2 id="users">Users</h2>
      {#if usersError}<p class="error" role="alert">{usersError}</p>{/if}
      <ul class="users">
        {#each users as u (u.id)}
          <li>
            <div class="info">
              <strong>{u.username}</strong>{u.isAdmin ? ' · administrator' : ''}
              <span class="muted">{u.mustChangePassword ? 'Has a temporary password' : lastSeen(u)}</span>
            </div>
            {#if u.id !== session.user?.id}
              <button class="button secondary" onclick={() => resetUser(u)}>Reset password</button>
              <button class="button danger" onclick={() => removeUser(u)}>Remove</button>
            {/if}
          </li>
        {/each}
      </ul>
      <form class="form add" onsubmit={addUser}>
        <Field label="New username" bind:value={newName} autocomplete="off" />
        <label class="check"><input type="checkbox" bind:checked={newAdmin} /> Administrator</label>
        <div><button class="button" type="submit" disabled={newName.trim() === ''}>Add user</button></div>
      </form>
    </section>
  {/if}
</div>

{#if temp}
  <TempPasswordDialog username={temp.username} password={temp.password} created={temp.created} onclose={() => (temp = null)} />
{/if}

<style>
  .page {
    max-width: 640px;
    margin: 0 auto;
    padding: 24px 16px 64px;
    display: flex;
    flex-direction: column;
    gap: 32px;
  }
  header {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .back {
    display: inline-flex;
    align-items: center;
    min-height: 44px;
  }
  h1 {
    margin: 0;
    font-size: 26px;
  }
  h2 {
    margin: 0 0 12px;
    font-size: 17px;
  }
  section {
    padding-bottom: 32px;
    border-bottom: 1px solid var(--border);
  }
  p {
    margin: 0 0 12px;
    line-height: 1.5;
  }
  .who {
    color: var(--muted);
  }
  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
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
  .error {
    color: var(--bad);
    font-size: 13px;
  }
  .ok {
    font-size: 13px;
  }
  .muted {
    display: block;
    font-size: 12px;
    color: var(--muted);
  }
  .users {
    list-style: none;
    margin: 0 0 16px;
    padding: 0;
  }
  .users li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 12px 0;
    border-bottom: 1px solid var(--border);
  }
  .info {
    flex: 1 1 160px;
  }
  .add {
    max-width: 320px;
  }
</style>
