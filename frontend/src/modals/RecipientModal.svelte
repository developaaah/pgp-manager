<script>
  import { createEventDispatcher } from 'svelte'
  import { ListKeys } from '../../wailsjs/go/main/App'

  export let open = false
  export let selectedFingerprints = []

  const dispatch = createEventDispatcher()

  let keys = []
  let loading = false
  let error = ''
  let search = ''

  $: filtered = keys.filter(k =>
    !search ||
    (k.PrimaryUID || '').toLowerCase().includes(search.toLowerCase()) ||
    (k.Email || '').toLowerCase().includes(search.toLowerCase()) ||
    k.Fingerprint.toLowerCase().includes(search.toLowerCase())
  ).sort((a, b) => {
    if (a.IsPrivate !== b.IsPrivate) return a.IsPrivate ? 1 : -1
    return (a.PrimaryUID || a.Email || '').localeCompare(b.PrimaryUID || b.Email || '')
  })

  async function load() {
    loading = true; error = ''
    try { keys = await ListKeys() }
    catch { error = 'Could not load keys.' }
    finally { loading = false }
  }

  function toggle(fp) {
    if (selectedFingerprints.includes(fp)) {
      selectedFingerprints = selectedFingerprints.filter(f => f !== fp)
    } else {
      selectedFingerprints = [...selectedFingerprints, fp]
    }
  }

  function confirm() {
    dispatch('confirm', { fingerprints: selectedFingerprints })
    close()
  }

  function close() {
    open = false
    search = ''
    dispatch('close')
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
  }

  $: if (open) { load() }

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
  style="height: min(480px, 80vh);"
>
  <div class="px-5 pt-5 pb-4 border-b border-pgp-border flex-shrink-0">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-3">
      Select Recipients
    </h2>
    <label for="recipient-search" class="sr-only">Search recipients</label>
    <input
      id="recipient-search"
      bind:value={search}
      type="search"
      placeholder="Search by name, email or fingerprint…"
      autofocus
      class="w-full h-8 px-3 rounded-[7px] text-[13px]
             bg-pgp-field border border-pgp-field-border
             text-pgp-text placeholder:text-pgp-text-4
             focus:outline-none focus:border-pgp-accent/60"
    />
  </div>

  <div class="flex-1 overflow-y-auto py-2 min-h-0">
    {#if loading}
      <p class="px-5 py-4 text-[13px] text-pgp-text-3">Loading keys…</p>
    {:else if error}
      <p class="px-5 py-4 text-[13px] text-red-500">{error}</p>
    {:else if filtered.length === 0}
      <p class="px-5 py-4 text-[13px] text-pgp-text-3">
        {search ? 'No keys match your search.' : 'No keys in your keystore.'}
      </p>
    {:else}
      {#each filtered as key (key.Fingerprint)}
        <div
          role="checkbox"
          aria-checked={selectedFingerprints.includes(key.Fingerprint)}
          tabindex="0"
          class="flex items-center gap-3 px-5 py-[10px] cursor-pointer
                 hover:bg-pgp-fill transition-colors duration-75
                 {selectedFingerprints.includes(key.Fingerprint) ? 'bg-pgp-accent-bg' : ''}"
          on:click={() => toggle(key.Fingerprint)}
          on:keydown={(e) => (e.key === ' ' || e.key === 'Enter') && (e.preventDefault(), toggle(key.Fingerprint))}
        >
          <div class="w-4 h-4 rounded flex items-center justify-center flex-shrink-0
                      border transition-colors duration-75
                      {selectedFingerprints.includes(key.Fingerprint)
                        ? 'bg-pgp-accent border-pgp-accent'
                        : 'border-pgp-border-strong bg-transparent'}">
            {#if selectedFingerprints.includes(key.Fingerprint)}
              <svg aria-hidden="true" class="w-2.5 h-2.5 text-white" viewBox="0 0 10 10" fill="none">
                <path d="M2 5l2.5 2.5L8 3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            {/if}
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-[13px] text-pgp-text truncate">{key.PrimaryUID || key.Email}</p>
            <p class="text-[11px] text-pgp-text-3 mt-[1px] truncate">
              {key.PrimaryUID && key.Email ? key.Email + ' · ' : ''}<span class="font-mono">…{shortFp(key.Fingerprint)}</span>
            </p>
          </div>
          {#if key.Status !== 'valid'}
            <span class="text-[10px] px-[6px] py-[2px] rounded-full
                         {key.Status === 'expired' ? 'bg-yellow-500/20 text-yellow-500' : 'bg-red-500/20 text-red-500'}">
              {key.Status}
            </span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>

  <div class="flex items-center justify-between px-5 py-4 border-t border-pgp-border flex-shrink-0">
    <span class="text-[12px] text-pgp-text-3">
      {selectedFingerprints.length
        ? `${selectedFingerprints.length} selected`
        : 'No recipients selected'}
    </span>
    <div class="flex gap-2">
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
        disabled={selectedFingerprints.length === 0}
        class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
               bg-pgp-accent hover:opacity-90
               disabled:opacity-40 disabled:cursor-not-allowed
               transition-opacity duration-75"
      >Encrypt</button>
    </div>
  </div>
</dialog>
{/if}
