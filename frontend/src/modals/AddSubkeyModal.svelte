<script>
  import { createEventDispatcher } from 'svelte'
  import { AddSubkey } from '../../wailsjs/go/main/App'

  export let open = false
  export let keyInfo = null

  const dispatch = createEventDispatcher()

  let subkeyType = 'encrypt'
  let expiryYears = '0'
  let passphrase = ''
  let adding = false
  let error = ''
  let success = false

  $: if (open) {
    error = ''; success = false; passphrase = ''
    subkeyType = 'encrypt'; expiryYears = '0'
  }

  async function handleAdd() {
    if (adding) return
    adding = true; error = ''
    try {
      await AddSubkey(keyInfo.Fingerprint, subkeyType, expiryYears, passphrase)
      success = true
      await new Promise(r => setTimeout(r, 900))
      dispatch('added')
      close()
    } catch (e) {
      error = String(e)
    } finally {
      adding = false
    }
  }

  function close() {
    open = false
    dispatch('close')
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
    if (e.key === 'Enter' && !e.isComposing && !adding && !success) handleAdd()
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
</script>

{#if open && keyInfo}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[380px]"
>
  <div class="px-6 pt-6 pb-2">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-[3px]">
      Add subkey
    </h2>
    <p class="text-[12px] text-pgp-text-3 mb-4 leading-relaxed truncate">
      {keyInfo.PrimaryUID || keyInfo.Email}
    </p>

    <div class="mb-3">
      <p id="subkey-type-label"
        class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]">
        Type
      </p>
      <div role="radiogroup" aria-labelledby="subkey-type-label"
           class="flex rounded-[7px] overflow-hidden border border-pgp-border-strong text-[12px]">
        {#each [['encrypt','Encryption'],['sign','Signing']] as [val, label]}
          <button
            type="button"
            role="radio"
            aria-checked={subkeyType === val}
            on:click={() => (subkeyType = val)}
            class="flex-1 h-8 px-3 transition-colors duration-75 text-center
                   {subkeyType === val
                     ? 'bg-pgp-accent text-white'
                     : 'bg-pgp-fill text-pgp-text-3 hover:bg-pgp-fill-2 hover:text-pgp-text-2'}"
          >{label}</button>
        {/each}
      </div>
    </div>

    <div class="mb-3">
      <p id="subkey-expiry-label"
        class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]">
        Expires
      </p>
      <div role="radiogroup" aria-labelledby="subkey-expiry-label"
           class="flex rounded-[7px] overflow-hidden border border-pgp-border-strong text-[12px]">
        {#each [['0','Never'],['1','1 yr'],['2','2 yrs'],['3','3 yrs'],['5','5 yrs']] as [val, label]}
          <button
            type="button"
            role="radio"
            aria-checked={expiryYears === val}
            on:click={() => (expiryYears = val)}
            class="flex-1 h-8 px-1 transition-colors duration-75 text-center
                   {expiryYears === val
                     ? 'bg-pgp-accent text-white'
                     : 'bg-pgp-fill text-pgp-text-3 hover:bg-pgp-fill-2 hover:text-pgp-text-2'}"
          >{label}</button>
        {/each}
      </div>
    </div>

    <div class="mb-3">
      <label for="add-subkey-pass"
        class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[5px]">
        Primary key passphrase
        <span class="normal-case font-normal">(leave empty if key has no passphrase)</span>
      </label>
      <input
        id="add-subkey-pass"
        bind:value={passphrase}
        type="password"
        placeholder="Passphrase (optional)…"
        autocomplete="current-password"
        autofocus
        class="w-full h-9 px-3 rounded-[7px] text-[13px]
               bg-pgp-field border border-pgp-field-border
               text-pgp-text placeholder:text-pgp-text-4
               focus:outline-none focus:border-pgp-accent/60"
      />
    </div>

    {#if error}
      <p class="text-[13px] text-red-500 mb-2 leading-[1.4]">{error}</p>
    {/if}
    {#if success}
      <p class="text-[13px] text-green-500 mb-2">Subkey added.</p>
    {/if}
  </div>

  <div class="flex items-center justify-between px-6 py-4">
    <button
      type="button"
      on:click={close}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
             bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
             hover:bg-pgp-fill transition-colors duration-75"
    >Cancel</button>
    <button
      type="button"
      on:click={handleAdd}
      disabled={adding || success}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             disabled:opacity-40 disabled:cursor-not-allowed
             transition-opacity duration-75"
    >{adding ? 'Adding…' : success ? 'Added' : 'Add subkey'}</button>
  </div>
</dialog>
{/if}
