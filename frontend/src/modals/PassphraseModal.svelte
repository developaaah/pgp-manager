<script>
  import { createEventDispatcher } from 'svelte'

  export let open = false
  export let keyLabel = ''
  export let confirmLabel = 'OK'
  export let allowEmpty = false

  const dispatch = createEventDispatcher()

  let passphrase = ''
  let inputEl
  let error = ''

  export function setError(msg) {
    error = msg
  }

  function confirm() {
    if (!allowEmpty && !passphrase) return
    dispatch('confirm', { passphrase })
  }

  function close() {
    passphrase = ''
    error = ''
    open = false
    dispatch('close')
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
    if (e.key === 'Enter' && !e.isComposing) confirm()
  }

  function dialogAction(node) {
    node.showModal()
    const onNativeClose = () => close()
    node.addEventListener('close', onNativeClose)
    return {
      destroy() {
        node.removeEventListener('close', onNativeClose)
        if (node.open) node.close()
      }
    }
  }

  $: if (open) { error = ''; passphrase = '' }
</script>

{#if open}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[360px]"
>
  <div class="px-6 py-5">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-1">
      Passphrase required
    </h2>
    {#if keyLabel}
      <p class="text-[12px] text-pgp-text-3 mb-4 truncate">{keyLabel}</p>
    {:else}
      <div class="mb-4"></div>
    {/if}

    <label
      for="passphrase-input"
      class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]"
    >
      Passphrase
    </label>
    <input
      id="passphrase-input"
      bind:this={inputEl}
      bind:value={passphrase}
      type="password"
      placeholder="Enter passphrase…"
      autocomplete="current-password"
      autofocus
      class="w-full h-9 px-3 rounded-[7px] text-[13px]
             bg-pgp-field border
             {error ? 'border-red-500/50' : 'border-pgp-field-border'}
             text-pgp-text placeholder:text-pgp-text-4
             focus:outline-none focus:border-pgp-accent/60"
    />
    {#if error}
      <p class="mt-2 text-[12px] text-red-500">{error}</p>
    {/if}
  </div>

  <div class="flex justify-end gap-2 px-6 pb-5">
    <button
      type="button"
      on:click={close}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
             bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
             hover:bg-pgp-fill transition-colors duration-75"
    >Cancel</button>
    <button
      type="button"
      on:click={confirm}
      disabled={!allowEmpty && !passphrase}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             disabled:opacity-40 disabled:cursor-not-allowed
             transition-opacity duration-75"
    >{confirmLabel}</button>
  </div>
</dialog>
{/if}
