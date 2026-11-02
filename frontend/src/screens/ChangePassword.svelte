<script lang="ts">
  import AuthCard from '../components/AuthCard.svelte';
  import Field from '../components/Field.svelte';
  import { ApiError } from '../lib/api';
  import { session } from '../lib/session.svelte';

  let current = $state('');
  let next = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);
  let first = $state<HTMLInputElement>();

  $effect(() => first?.focus());

  const mismatch = $derived(confirm !== '' && next !== confirm);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    if ([...next].length < 8) {
      error = 'Password must be at least 8 characters.';
      return;
    }
    if (next !== confirm) {
      error = 'The two passwords do not match.';
      return;
    }

    busy = true;
    try {
      await session.changePassword(current, next);
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
    } finally {
      busy = false;
    }
  }
</script>

<AuthCard
  title="Choose a new password"
  subtitle="You signed in with a temporary password. Pick one of your own to continue."
  {error}
>
  <form class="form" onsubmit={submit}>
    <Field
      label="Temporary password"
      type="password"
      bind:value={current}
      bind:input={first}
      autocomplete="current-password"
    />
    <Field
      label="New password"
      type="password"
      bind:value={next}
      autocomplete="new-password"
      hint="At least 8 characters."
    />
    <Field
      label="Confirm new password"
      type="password"
      bind:value={confirm}
      autocomplete="new-password"
      invalid={mismatch}
    />
    <button class="button" type="submit" disabled={busy}>
      {busy ? 'Saving…' : 'Save password'}
    </button>
    <button class="button secondary" type="button" onclick={() => session.logout()}>Sign out</button>
  </form>
</AuthCard>
