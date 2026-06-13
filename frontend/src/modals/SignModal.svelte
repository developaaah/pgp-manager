<script>
  import { createEventDispatcher } from 'svelte'
  import { ListKeys } from '../../wailsjs/go/main/App'

  export let open = false

  const dispatch = createEventDispatcher()

  let keys = []
  let loading = false
  let error = ''
  let signingFp = ''

  $: privateKeys = keys.filter(k => k.IsPrivate)
  $: canConfirm = !!signingFp

  async function load() {
    loading = true; error = ''
    try {
      keys = await ListKeys()
      if (!signingFp) {
        const first = keys.find(k => k.IsPrivate)
        if (first) signingFp = first.Fingerprint
      }
    } catch { error = 'Could not load keys.' }
    finally { loading = false }
  }

  function confirm() {
    if (!canConfirm) return
    const key = privateKeys.find(k => k.Fingerprint === signingFp)
    dispatch('confirm', { signingFp, keyLabel: key?.Email || key?.PrimaryUID || '' })
    reset()
  }

  function close() {
    reset()
    dispatch('close')
  }

  function reset() {
    open = false
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
    if (e.key === 'Enter' && canConfirm) { e.preventDefault(); confirm() }
  }

  $: if (open) load()

  function dialogAction(node) {
    node.showModal()
    node.addEventListener('close', close)
    return {
      destroy() {
        node.removeEventListener('close', close)
        if (node.open) node.close()
      }
    }
  }

  function shortFp(fp) { return fp ? fp.slice(-8).toUpperCase() : '' }
</script>

{#if open}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[480px] flex flex-col"
  style="height: min(320px, 88vh);"
>
  <div class="px-5 pt-5 pb-4 border-b border-pgp-border flex-shrink-0">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em]">Sign</h2>
    <p class="text-[12px] text-pgp-text-3 mt-[2px]">Choose the key to sign with</p>
  </div>

  <div class="flex-1 overflow-y-auto min-h-0">
    <p class="px-5 pt-4 pb-[6px] text-[11px] font-bold text-pgp-text-4 uppercase tracking-wider">
      Signing key
    </p>

    {#if loading}
      <p class="px-5 pb-3 text-[13px] text-pgp-text-3">Loading keys…</p>
    {:else if error}
      <p class="px-5 pb-3 text-[13px] text-red-500">{error}</p>
    {:else if privateKeys.length === 0}
      <p class="px-5 pb-3 text-[13px] text-pgp-text-3">No private keys found.</p>
    {:else}
      <div role="radiogroup" aria-label="Select signing key">
        {#each privateKeys as key (key.Fingerprint)}
          <div
            role="radio"
            aria-checked={signingFp === key.Fingerprint}
            tabindex="0"
            class="flex items-center gap-3 px-5 py-[9px] cursor-pointer
                   hover:bg-pgp-fill transition-colors duration-75
                   {signingFp === key.Fingerprint ? 'bg-pgp-accent-bg' : ''}"
            on:click={() => (signingFp = key.Fingerprint)}
            on:keydown={(e) => (e.key === ' ' || e.key === 'Enter') && (e.preventDefault(), signingFp = key.Fingerprint)}
          >
            <div class="w-4 h-4 rounded-full flex items-center justify-center flex-shrink-0
                        border transition-colors duration-75
                        {signingFp === key.Fingerprint ? 'border-pgp-accent' : 'border-pgp-border-strong'}">
              {#if signingFp === key.Fingerprint}
                <div class="w-2 h-2 rounded-full bg-pgp-accent"></div>
              {/if}
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-[13px] text-pgp-text truncate">{key.PrimaryUID || key.Email}</p>
              <p class="text-[11px] text-pgp-text-3 mt-[1px] truncate">
                {key.PrimaryUID && key.Email ? key.Email + ' · ' : ''}<span class="font-mono">…{shortFp(key.Fingerprint)}</span>
              </p>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <div class="flex justify-end gap-2 px-5 py-4 border-t border-pgp-border flex-shrink-0">
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
      disabled={!canConfirm}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             disabled:opacity-40 disabled:cursor-not-allowed
             transition-opacity duration-75"
    >Continue</button>
  </div>
</dialog>
{/if}
