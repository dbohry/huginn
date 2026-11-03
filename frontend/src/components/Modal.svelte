<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    title,
    onclose,
    wide = false,
    children,
  }: { title: string; onclose: () => void; wide?: boolean; children: Snippet } = $props();

  let dialog = $state<HTMLDialogElement>();
  const titleId = $props.id();

  // The browser's own dialog traps focus, closes on Escape and marks the rest of the page inert.
  $effect(() => {
    dialog?.showModal();
    return () => dialog?.close();
  });
</script>

<dialog bind:this={dialog} class:wide aria-labelledby={titleId} {onclose} oncancel={onclose}>
  <h2 id={titleId}>{title}</h2>
  {@render children()}
</dialog>

<style>
  dialog {
    width: min(440px, calc(100vw - 32px));
    padding: 24px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: var(--bg);
    color: var(--text);
  }
  dialog.wide {
    width: min(640px, calc(100vw - 32px));
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.5);
  }
  h2 {
    margin: 0 0 16px;
    font-size: 18px;
  }
</style>
