<script lang="ts">
  import { reader } from '../lib/reader.svelte';
</script>

<!-- The live region is always on the page, so a message that appears in it is read out. -->
<div class="toasts" aria-live="polite">
  {#if reader.toast}
    {#key reader.toast.id}
      <div class="toast" class:error={reader.toast.kind === 'error'} role={reader.toast.kind === 'error' ? 'alert' : undefined}>
        <span class="text">{reader.toast.text}</span>
        <button type="button" class="close" aria-label="Dismiss message" onclick={() => reader.dismissToast()}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M6 6l12 12M18 6 6 18" /></svg>
        </button>
      </div>
    {/key}
  {/if}
</div>

<style>
  .toasts {
    position: fixed;
    left: 0;
    right: 0;
    bottom: calc(20px + var(--safe-bottom));
    display: flex;
    justify-content: center;
    padding: 0 16px;
    pointer-events: none;
    z-index: 50;
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 4px;
    max-width: min(480px, 100%);
    padding: 4px 4px 4px 16px;
    border-radius: 10px;
    background: var(--text);
    color: var(--bg);
    box-shadow: 0 6px 24px rgb(0 0 0 / 0.25);
    animation: rise 0.18s ease-out;
  }
  .toast.error {
    background: var(--bad);
  }
  .text {
    padding: 8px 0;
    font-size: 14px;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }
  .close {
    flex: none;
    width: 36px;
    height: 36px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    opacity: 0.8;
    display: grid;
    place-items: center;
  }
  .close:hover {
    opacity: 1;
    background: rgb(127 127 127 / 0.25);
  }
  /* Position only: a fade would leave the text low-contrast for its first moments. */
  @keyframes rise {
    from {
      transform: translateY(8px);
    }
  }
</style>
