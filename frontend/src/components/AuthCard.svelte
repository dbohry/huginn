<script lang="ts">
  import type { Snippet } from 'svelte';
  import Logo from './Logo.svelte';
  import { session } from '../lib/session.svelte';

  let {
    title,
    subtitle,
    error = '',
    children,
  }: { title: string; subtitle?: string; error?: string; children: Snippet } = $props();
</script>

<div class="page">
  <main>
    <header>
      <Logo size={48} />
      <h1>{title}</h1>
      {#if subtitle}<p>{subtitle}</p>{/if}
    </header>

    {#if error}
      <div class="alert" role="alert">{error}</div>
    {/if}

    {@render children()}
  </main>
  <footer>Huginn{session.version ? ` ${session.version}` : ''}</footer>
</div>

<style>
  .page {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 20px;
    padding: 32px 16px;
  }
  main {
    width: 100%;
    max-width: 400px;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding: 32px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
  }
  header {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    text-align: center;
  }
  h1 {
    margin: 0;
    font-size: 22px;
  }
  header p {
    margin: 0;
    font-size: 14px;
    line-height: 1.5;
    color: var(--muted);
  }
  .alert {
    padding: 12px 14px;
    border: 1px solid var(--bad);
    border-radius: 8px;
    font-size: 13px;
    line-height: 1.4;
  }
  footer {
    font-size: 12px;
    color: var(--muted);
  }
</style>
