<script lang="ts">
  import Modal from './Modal.svelte';
  import { copyText } from '../lib/clipboard';

  type Props = {
    username: string;
    password: string;
    /** True for a new account, false for a reset. */
    created: boolean;
    onclose: () => void;
  };

  let { username, password, created, onclose }: Props = $props();

  let field = $state<HTMLInputElement>();
  let copied = $state<'yes' | 'no' | ''>('');

  async function copy() {
    const ok = await copyText(password);
    copied = ok ? 'yes' : 'no';
    // If copying did not work, leave the password selected so it can be copied by hand.
    if (!ok) field?.select();
  }
</script>

<Modal title={created ? 'Account created' : 'Password reset'} {onclose}>
  <div class="body">
    <p>
      {#if created}
        Give <strong>{username}</strong> this temporary password. They will be asked to choose their own the first time they sign in.
      {:else}
        <strong>{username}</strong> has been signed out everywhere. Give them this temporary password. They will be asked to choose their own when they sign in.
      {/if}
    </p>

    <div class="secret">
      <label for="temp-password">Temporary password</label>
      <div class="row">
        <input
          id="temp-password"
          bind:this={field}
          readonly
          value={password}
          onfocus={(e) => e.currentTarget.select()}
          spellcheck="false"
          autocomplete="off"
        />
        <button class="button secondary" type="button" onclick={copy}>Copy</button>
      </div>
      <span class="status" role="status">
        {#if copied === 'yes'}Copied.{:else if copied === 'no'}Could not copy. Select it and copy by hand.{/if}
      </span>
    </div>

    <p class="warning">This is the only time it is shown. Closing this window will not let you see it again.</p>

    <div class="buttons">
      <button class="button" type="button" onclick={onclose}>Done</button>
    </div>
  </div>
</Modal>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  p {
    margin: 0;
    line-height: 1.5;
  }
  .secret {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  label {
    font-size: 13px;
    font-weight: 500;
  }
  .row {
    display: flex;
    gap: 8px;
  }
  input {
    flex: 1;
    min-width: 0;
    min-height: 44px;
    padding: 0 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 15px;
  }
  .status {
    min-height: 1.4em;
    font-size: 12px;
    color: var(--muted);
  }
  .warning {
    font-size: 13px;
    color: var(--muted);
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
  }
</style>
