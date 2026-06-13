<script>
  import { createEventDispatcher } from 'svelte'
  import { ImportMultipleKeys, PreviewKeys } from '../../wailsjs/go/main/App'

  export let open = false
  export let armored = ''

  const dispatch = createEventDispatcher()

  let importing = false
  let error = ''
  let importResults = []
  let preview = []
  let previewLoading = false

  $: trimmed = armored.trimStart()
  $: isPrivate = trimmed.startsWith('-----BEGIN PGP PRIVATE KEY BLOCK')
  $: keyCount = (trimmed.match(/-----BEGIN PGP .* KEY BLOCK-----/g) || []).length
  $: isMultiple = keyCount > 1

  $: if (open && armored) {
    error = ''; importResults = []
    loadPreview()
  }

  async function loadPreview() {
    previewLoading = true
    try { preview = await PreviewKeys(armored.trim()) }
    catch { preview = [] }
    finally { previewLoading = false }
  }

  async function handleImport() {
    importing = true; error = ''
    try {
      const result = await ImportMultipleKeys(armored.trim())
      importResults = result.Entries || []
      const successCount = importResults.filter(r => !r.Error).length
      if (successCount > 0 || importResults.every(r => r.Error === 'already exists')) {
        await new Promise(r => setTimeout(r, 900))
        dispatch('imported')
        close()
      }
    } catch (e) {
      error = String(e)
    } finally {
      importing = false
    }
  }

  function close() {
    open = false; error = ''; importResults = []; preview = []
    dispatch('close')
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
    if (e.key === 'Enter' && !e.isComposing) handleImport()
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

  function shortFp(fp) { return fp ? '…' + fp.slice(-8).toUpperCase() : '' }
</script>

{#if open}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[400px]"
>
  <div class="px-6 pt-6 pb-5">
    <div class="flex items-center gap-3 mb-4">
      <div aria-hidden="true"
           class="w-10 h-10 rounded-full flex items-center justify-center flex-shrink-0
                  {isPrivate ? 'bg-purple-500/15' : 'bg-pgp-accent-bg'}">
        <svg aria-hidden="true" class="w-5 h-5 {isPrivate ? 'text-purple-400' : 'text-pgp-accent'}"
             viewBox="0 0 16 16" fill="none">
          <circle cx="6" cy="7" r="3" stroke="currentColor" stroke-width="1.25"/>
          <path d="M8.2 9.2l5.3 5.3" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
          <path d="M11.5 12l.5-.5M13 13.5l.5-.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
        </svg>
      </div>
      <div>
        <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em]">
          {isMultiple ? `Import ${keyCount} keys` : `Import ${isPrivate ? 'private' : 'public'} key`}
        </h2>
        <p class="text-[12px] text-pgp-text-3 mt-[2px]">
          {isMultiple ? 'Multiple PGP key blocks detected' : 'PGP key detected'}
        </p>
      </div>
    </div>

    {#if previewLoading}
      <div class="rounded-[8px] bg-pgp-fill border border-pgp-border px-3 py-2 mb-4">
        <p class="text-[11px] text-pgp-text-4">Parsing key…</p>
      </div>
    {:else if preview.length > 0}
      <div class="flex flex-col gap-[5px] mb-4">
        {#each preview as key}
          <div class="flex items-center gap-2 px-3 py-[7px] rounded-[8px]
                      bg-pgp-fill border border-pgp-border">
            <div class="flex-1 min-w-0">
              <p class="text-[12px] font-medium text-pgp-text truncate">
                {key.PrimaryUID || key.Email || 'Unknown'}
              </p>
              {#if key.PrimaryUID && key.Email}
                <p class="text-[11px] text-pgp-text-3 truncate">{key.Email}</p>
              {/if}
            </div>
            <span class="text-[10px] font-mono text-pgp-text-4 flex-shrink-0">
              {shortFp(key.Fingerprint)}
            </span>
            {#if key.AlreadyExists}
              <span class="text-[10px] px-[6px] py-[1px] rounded-full flex-shrink-0
                           bg-pgp-fill-2 border border-pgp-border text-pgp-text-3">
                exists
              </span>
            {:else}
              <span class="text-[10px] px-[6px] py-[1px] rounded-full flex-shrink-0
                           bg-pgp-accent/15 text-pgp-accent">
                new
              </span>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <div class="rounded-[8px] bg-pgp-fill border border-pgp-border p-3 mb-4 overflow-hidden">
        <p class="text-[11px] font-mono text-pgp-text-3 leading-relaxed break-all"
           style="max-height: 4.5em; overflow: hidden; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3;">
          {armored.trim().slice(0, 200)}{armored.length > 200 ? '…' : ''}
        </p>
      </div>
    {/if}

    {#if error}
      <p class="text-[13px] text-red-500 mb-3 leading-[1.4]">{error}</p>
    {:else if importResults.length > 0}
      <div class="flex flex-col gap-1.5 mb-3">
        {#each importResults as result}
          <div class="flex items-center gap-2 text-[12px]">
            {#if !result.Error}
              <svg aria-hidden="true" class="w-3.5 h-3.5 text-green-500 flex-shrink-0" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/>
                <path d="M5.5 8.5l2 2 3.5-4.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span class="text-pgp-text-2 truncate">{result.UID || shortFp(result.Fingerprint) || 'Key imported'}</span>
            {:else}
              <svg aria-hidden="true" class="w-3.5 h-3.5 text-red-500 flex-shrink-0" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/>
                <path d="M6 6l4 4M10 6l-4 4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
              </svg>
              <span class="text-red-500 truncate">{result.Error}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

  </div>

  <div class="flex items-center justify-between px-6 pb-5">
    <button
      type="button"
      on:click={close}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
             bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
             hover:bg-pgp-fill transition-colors duration-75"
    >Dismiss</button>
    <button
      type="button"
      on:click={handleImport}
      disabled={importing}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             {isPrivate ? 'bg-purple-500 hover:bg-purple-500/90' : 'bg-pgp-accent hover:opacity-90'}
             disabled:opacity-40 disabled:cursor-not-allowed
             transition-opacity duration-75"
    >
      {importing ? 'Importing…' : isMultiple ? `Import ${keyCount} keys` : 'Import key'}
    </button>
  </div>
</dialog>
{/if}
