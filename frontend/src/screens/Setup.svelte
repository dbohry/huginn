<script lang="ts">
  import AuthCard from '../components/AuthCard.svelte';
  import Field from '../components/Field.svelte';
  import { ApiError } from '../lib/api';
  import { session } from '../lib/session.svelte';

  let username = $state('');
  let password = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let first = $state<HTMLInputElement>();

  $effect(() => first?.focus());

  const usernameOk = /^[A-Za-z0-9._-]{3,32}$/;
  const mismatch = $derived(confirm !== '' && password !== confirm);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    if (!usernameOk.test(username)) {
      error = 'Username must be 3 to 32 characters: letters, numbers, dots, dashes and underscores.';
      return;
    }
    if ([...password].length < 8) {
      error = 'Password must be at least 8 characters.';
      return;
    }
    if (password !== confirm) {
      error = 'The two passwords do not match.';
      return;
    }

    busy = true;
    try {
      await session.setup(username, password);
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
      // Someone else finished setup first: send this browser to the sign-in screen.
      if (err instanceof ApiError && err.status === 409) await session.init();
    } finally {
      busy = false;
    }
  }
</script>

<AuthCard
  title="Welcome to Huginn"
  subtitle="Create the first account. It becomes the administrator and can add other users later."
  {error}
>
  <form class="form" onsubmit={submit}>
    <Field label="Username" bind:value={username} bind:input={first} autocomplete="username" />
    <Field
      label="Password"
      type="password"
      bind:value={password}
      autocomplete="new-password"
      hint="At least 8 characters."
    />
    <Field
      label="Confirm password"
      type="password"
      bind:value={confirm}
      autocomplete="new-password"
      invalid={mismatch}
    />
    <button class="button" type="submit" disabled={busy}>
      {busy ? 'Creating…' : 'Create account'}
    </button>
  </form>
  <p class="note">This screen only appears once, until the first account exists.</p>
</AuthCard>

<style>
  .note {
    margin: -8px 0 0;
    font-size: 12px;
    line-height: 1.5;
    text-align: center;
    color: var(--muted);
  }
</style>
