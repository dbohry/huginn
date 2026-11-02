<script lang="ts">
  import { router } from './lib/router.svelte';
  import { session } from './lib/session.svelte';
  import ChangePassword from './screens/ChangePassword.svelte';
  import Login from './screens/Login.svelte';
  import Reader from './screens/Reader.svelte';
  import Settings from './screens/Settings.svelte';
  import Setup from './screens/Setup.svelte';

  let failed = $state(false);

  $effect(() => {
    session.init().catch(() => (failed = true));
  });

  // The screen follows who is using the app. The address bar follows the screen.
  const screen = $derived.by(() => {
    switch (session.phase) {
      case 'setup':
        return 'setup';
      case 'anonymous':
        return 'login';
      case 'authed':
        return session.user?.mustChangePassword ? 'change-password' : 'home';
      default:
        return 'loading';
    }
  });

  const paths = { setup: '/setup', login: '/login', 'change-password': '/change-password' };

  $effect(() => {
    if (screen === 'loading') return;
    if (screen === 'home') {
      // Signed in: the reader at /, settings at /settings. Anything else goes to /.
      if (router.path !== '/' && router.path !== '/settings') router.navigate('/', { replace: true });
    } else {
      router.navigate(paths[screen], { replace: true });
    }
  });
</script>

{#if failed}
  <p class="status" role="alert">Cannot reach the server.</p>
{:else if screen === 'setup'}
  <Setup />
{:else if screen === 'login'}
  <Login />
{:else if screen === 'change-password'}
  <ChangePassword />
{:else if screen === 'home'}
  {#if router.path === '/settings'}
    <Settings />
  {:else}
    <Reader />
  {/if}
{:else}
  <p class="status" role="status">Loading…</p>
{/if}

<style>
  .status {
    min-height: 100vh;
    display: grid;
    place-items: center;
    margin: 0;
    color: var(--muted);
    font-size: 14px;
  }
</style>
