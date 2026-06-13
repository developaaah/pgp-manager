<script>
  import { createEventDispatcher, onMount } from 'svelte'
  import { ConfirmSetup, DefaultConfigDir, OpenDirectoryDialog } from '../../wailsjs/go/main/App'

  const dispatch = createEventDispatcher()

  let defaultDir = ''
  let dir = ''
  let error = ''
  let busy = false

  $: isDefault = dir === '' || dir === defaultDir

  onMount(async () => {
    try {
      defaultDir = await DefaultConfigDir()
      dir = defaultDir
    } catch {}
  })

  async function browse() {
    try {
      const chosen = await OpenDirectoryDialog()
      if (chosen) dir = chosen
    } catch {}
  }

  async function confirm() {
    busy = true
    error = ''
    try {
      await ConfirmSetup(dir)
      dispatch('done')
    } catch (e) {
      error = String(e)
    } finally {
      busy = false
    }
  }

  function dialogAction(node) {
    node.showModal()
    const onNativeClose = () => { if (!busy) node.showModal() }
    node.addEventListener('close', onNativeClose)
    return {
      destroy() {
        node.removeEventListener('close', onNativeClose)
        if (node.open) node.close()
      }
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') e.preventDefault()
  }
</script>

<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[520px] max-w-[90vw]"
>
  <div class="px-6 py-5">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-2">
      Welcome to PGP Manager
    </h2>
    <p class="text-[12.5px] text-pgp-text-2 leading-relaxed mb-4">
      No configuration was found. Choose where PGP Manager should store its
      configuration. Using the default directory makes this a one-time choice;
      any other directory runs the app standalone (config and keys live there,
      and you will be asked again on every start).
    </p>

    <div class="flex gap-2 mb-2">
      <input
        type="text"
        bind:value={dir}
        spellcheck="false"
        class="flex-1 h-8 px-3 rounded-[7px] text-[12.5px] font-mono
               bg-pgp-field border border-pgp-field-border text-pgp-text
               focus:outline-none focus:border-pgp-accent"
      />
      <button
        type="button"
        on:click={browse}
        class="h-8 px-[14px] rounded-[7px] text-[12.5px] font-medium
               bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
               hover:bg-pgp-fill transition-colors duration-75"
      >Browse…</button>
    </div>

    <p class="text-[11.5px] text-pgp-text-3 mb-1">
      {#if isDefault}
        Default location — configuration will be created here once.
      {:else}
        Standalone mode — config and keys stay in this directory.
      {/if}
    </p>

    {#if error}
      <div class="rounded-[8px] bg-red-500/10 border border-red-500/30 px-4 py-3 mt-3">
        <p class="text-[12.5px] text-red-400 whitespace-pre-wrap break-words">{error}</p>
      </div>
    {/if}
  </div>

  <div class="flex justify-end gap-2 px-6 pb-5">
    {#if !isDefault}
      <button
        type="button"
        on:click={() => { dir = defaultDir }}
        class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
               bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
               hover:bg-pgp-fill transition-colors duration-75"
      >Use Default</button>
    {/if}
    <button
      type="button"
      on:click={confirm}
      disabled={busy}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90 disabled:opacity-50
             transition-opacity duration-75"
    >{busy ? 'Setting up…' : 'Continue'}</button>
  </div>
</dialog>
