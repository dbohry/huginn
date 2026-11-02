<script lang="ts">
  import AuthCard from '../components/AuthCard.svelte';
  import Field from '../components/Field.svelte';
  import { ApiError } from '../lib/api';
  import { session } from '../lib/session.svelte';

  let username = $state('');
  let password = $state('');
  let remember = $state(true);
  let error = $state('');
  let busy = $state(false);
  let first = $state<HTMLInputElement>();

  $effect(() => first?.focus());

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await session.login(username, password, remember);
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
      password = '';
    } finally {
      busy = false;
    }
  }
</script>

<AuthCard title="Sign in to Huginn" subtitle="Your feeds, brought back each day." {error}>
  <form class="form" onsubmit={submit}>
    <Field
      label="Username"
      bind:value={username}
      bind:input={first}
      autocomplete="username"
      invalid={!!error}
    />
    <Field
      label="Password"
      type="password"
      bind:value={password}
      autocomplete="current-password"
      invalid={!!error}
    />
    <label class="check">
      <input type="checkbox" bind:checked={remember} />
      Keep me signed in on this device
    </label>
    <button class="button" type="submit" disabled={busy}>
      {busy ? 'Signing in…' : 'Sign in'}
    </button>
  </form>
</AuthCard>
