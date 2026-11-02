<script lang="ts">
  import Field from './Field.svelte';
  import Modal from './Modal.svelte';
  import { ApiError } from '../lib/api';
  import { reader } from '../lib/reader.svelte';

  let { onclose }: { onclose: () => void } = $props();

  let url = $state('');
  let error = $state('');
  let busy = $state(false);
  let input = $state<HTMLInputElement>();

  $effect(() => input?.focus());

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      const feed = await reader.addFeed(url);
      await reader.chooseFeed(feed.id);
      onclose();
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Something went wrong.';
    } finally {
      busy = false;
    }
  }
</script>

<Modal title="Add a feed" {onclose}>
  <form class="form" onsubmit={submit}>
    <Field
      label="Feed address"
      type="url"
      bind:value={url}
      bind:input
      placeholder="https://example.com/feed.xml"
      hint="The address of an RSS, Atom or JSON feed."
      invalid={!!error}
    />
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="buttons">
      <button class="button secondary" type="button" onclick={onclose}>Cancel</button>
      <button class="button" type="submit" disabled={busy || url.trim() === ''}>
        {busy ? 'Adding…' : 'Add feed'}
      </button>
    </div>
  </form>
</Modal>

<style>
  .error {
    margin: 0;
    font-size: 13px;
    color: var(--bad);
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
