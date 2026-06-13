<script>
  import { onDestroy } from 'svelte'
  import { ListKeys, EncryptFile, DecryptFile, OpenFileDialog, FindDecryptionKeyFromFile, GetCachedPassphrase, HasCachedPassphrase, CachePassphrase, OpenFolder } from '../../../wailsjs/go/main/App'
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import RecipientModal from '../../modals/RecipientModal.svelte'
  import PassphraseModal from '../../modals/PassphraseModal.svelte'

  let state = 'idle'
  let filePath = ''
  let fileName = ''
  let error = ''
  let outputPath = ''
  let dragOver = false

  let selectedFingerprints = []
  let showRecipientModal = false
  let showPassphraseModal = false
  let passphraseKeyLabel = ''
  let passphraseResolve = null

  $: isPGPFile = /\.(pgp|asc|gpg)$/i.test(filePath)
  $: fileExt = fileName.includes('.') ? fileName.split('.').pop().toLowerCase() : ''

  const unsubDrop = EventsOn('file-drop', (path) => {
    if (path) setFile(path)
  })
  onDestroy(() => { unsubDrop?.() })

  function setFile(path) {
    filePath = path
    const parts = path.replace(/\\/g, '/').split('/')
    fileName = parts[parts.length - 1] || path
    error = ''
    outputPath = ''
    state = 'ready'
  }

  async function chooseFile() {
    try {
      const path = await OpenFileDialog()
      if (path) setFile(path)
    } catch (e) { error = String(e) }
  }

  function handleDragOver(e) { e.preventDefault(); dragOver = true }
  function handleDragLeave() { dragOver = false }
  function handleDrop(e) {
    e.preventDefault()
    dragOver = false
  }

  function handleEncrypt() { showRecipientModal = true }

  async function doEncrypt() {
    state = 'working'; error = ''
    try {
      const result = await EncryptFile(filePath, selectedFingerprints, '')
      if (result.Error) { error = result.Error; state = 'ready' }
      else { outputPath = result.OutputPath; state = 'done' }
    } catch (e) { error = String(e); state = 'ready' }
  }

  async function handleDecrypt() {
    try {
      const matchFp = await FindDecryptionKeyFromFile(filePath)
      if (!matchFp) { error = 'No matching private key found for this file.'; return }

      const keys = await ListKeys()
      const key = keys.find(k => k.Fingerprint === matchFp)
      if (!key) { error = 'No matching private key found for this file.'; return }

      let pp
      if (await HasCachedPassphrase(matchFp)) {
        pp = await GetCachedPassphrase(matchFp)
      } else {
        pp = await askPassphrase(key.Email || key.PrimaryUID)
        if (pp === null) return
      }

      state = 'working'; error = ''
      const result = await DecryptFile(filePath, matchFp, pp)
      if (result.Error) { error = result.Error; state = 'ready'; return }
      await CachePassphrase(matchFp, pp)
      outputPath = result.OutputPath
      state = 'done'
    } catch (e) { error = String(e); state = 'ready' }
  }

  function reset() {
    state = 'idle'; filePath = ''; fileName = ''
    error = ''; outputPath = ''; selectedFingerprints = []
  }

  async function copyOutputPath() {
    try { await navigator.clipboard.writeText(outputPath) } catch {}
  }

  function askPassphrase(label) {
    return new Promise(resolve => {
      passphraseKeyLabel = label
      passphraseResolve = resolve
      showPassphraseModal = true
    })
  }

  function onRecipientsConfirm(e) {
    selectedFingerprints = e.detail.fingerprints
    showRecipientModal = false
    if (filePath && selectedFingerprints.length) doEncrypt()
  }

  function onPassphraseConfirm(e) {
    showPassphraseModal = false
    passphraseResolve?.(e.detail.passphrase)
    passphraseResolve = null
  }

  function onPassphraseClose() {
    showPassphraseModal = false
    passphraseResolve?.(null)
    passphraseResolve = null
  }
</script>

<div class="px-7 pt-6 pb-5 flex-shrink-0 border-b border-pgp-border">
  <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">Files</h1>
  <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
    Encrypt or decrypt files — drop a file or click to choose
  </p>
</div>

<div class="flex flex-col flex-1 overflow-hidden min-h-0 p-6">

  {#if state === 'idle'}
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      on:dragover={handleDragOver}
      on:dragleave={handleDragLeave}
      on:drop={handleDrop}
      style="--wails-drop-target: drop;"
      class="flex-1 flex flex-col items-center justify-center rounded-field border-2 border-dashed
             transition-colors duration-100 cursor-pointer select-none
             {dragOver
               ? 'border-pgp-accent/60 bg-pgp-accent/[0.04]'
               : 'border-pgp-border hover:border-pgp-border-strong hover:bg-pgp-fill/40'}"
    >
      <svg class="w-12 h-12 mb-4 text-pgp-text-4" viewBox="0 0 48 48" fill="none">
        <rect x="10" y="6" width="28" height="36" rx="3" stroke="currentColor" stroke-width="1.5"/>
        <path d="M30 6v10h8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        <path d="M17 22h14M17 28h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>

      <p class="text-[15px] text-pgp-text-3 mb-1">
        {dragOver ? 'Release to select' : 'Drop a file here'}
      </p>
      <p class="text-[13px] text-pgp-text-4 mb-5">or</p>

      <button
        type="button"
        on:click={chooseFile}
        class="h-9 px-6 rounded-btn text-[13px] font-medium
               bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
               hover:bg-pgp-fill transition-colors duration-75"
      >Choose File…</button>

      <p class="mt-5 text-[12px] text-pgp-text-4 text-center max-w-[260px] leading-relaxed">
        Files ending in .pgp, .asc, .gpg will be decrypted.<br>All other files will be encrypted.
      </p>
    </div>

  {:else if state === 'ready'}
    <div class="flex flex-col gap-4 flex-1">

      <div class="flex items-center gap-4 px-4 py-4 rounded-field
                  bg-pgp-field border border-pgp-field-border">

        <div class="w-11 h-11 rounded-[8px] flex items-center justify-center flex-shrink-0
                    {isPGPFile ? 'bg-pgp-accent/10' : 'bg-pgp-border/60'}">
          {#if isPGPFile}
            <svg class="w-5 h-5 text-pgp-accent" viewBox="0 0 16 16" fill="none">
              <rect x="3" y="7" width="10" height="8" rx="1.5" stroke="currentColor" stroke-width="1.25"/>
              <path d="M5 7V5.5a3 3 0 016 0V7" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
              <circle cx="8" cy="11" r="1" fill="currentColor"/>
            </svg>
          {:else}
            <svg class="w-5 h-5 text-pgp-text-3" viewBox="0 0 16 16" fill="none">
              <rect x="3" y="1.5" width="10" height="13" rx="1.5" stroke="currentColor" stroke-width="1.25"/>
              <path d="M10 1.5v4h3" stroke="currentColor" stroke-width="1.25" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M5 8h6M5 11h4" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
            </svg>
          {/if}
        </div>

        <div class="flex-1 min-w-0">
          <p class="text-[14px] text-pgp-text font-medium truncate">{fileName}</p>
          <p class="text-[12px] text-pgp-text-3 font-mono mt-[2px] truncate">{filePath}</p>
        </div>

        <button
          type="button"
          on:click={reset}
          class="text-[12px] text-pgp-text-4 hover:text-pgp-text-3 transition-colors flex-shrink-0"
          title="Choose a different file"
        >Change</button>
      </div>

      {#if error}
        <p class="text-[13px] text-red-500 px-1">{error}</p>
      {/if}

      <div class="flex items-center gap-[6px]">
        {#if isPGPFile}
          <button
            type="button"
            on:click={handleDecrypt}
            class="h-[30px] px-5 rounded-btn text-[13px] font-medium text-white
                   bg-pgp-accent hover:opacity-90 transition-opacity duration-75"
          >Decrypt</button>
          <button
            type="button"
            on:click={handleEncrypt}
            class="h-[30px] px-4 rounded-btn text-[13px] font-medium
                   bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                   hover:bg-pgp-fill transition-colors duration-75"
          >Encrypt</button>
        {:else}
          <button
            type="button"
            on:click={handleEncrypt}
            class="h-[30px] px-5 rounded-btn text-[13px] font-medium text-white
                   bg-pgp-accent hover:opacity-90 transition-opacity duration-75"
          >Encrypt</button>
          <button
            type="button"
            on:click={handleDecrypt}
            class="h-[30px] px-4 rounded-btn text-[13px] font-medium
                   bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                   hover:bg-pgp-fill transition-colors duration-75"
          >Decrypt</button>
        {/if}
      </div>

    </div>

  {:else if state === 'working'}
    <div class="flex-1 flex items-center justify-center">
      <p class="text-[14px] text-pgp-text-3">Working…</p>
    </div>

  {:else if state === 'done'}
    <div class="flex flex-col items-center justify-center flex-1 gap-4">

      <div class="w-12 h-12 rounded-full bg-green-500/10 flex items-center justify-center">
        <svg class="w-6 h-6 text-green-600 dark:text-green-400" viewBox="0 0 24 24" fill="none">
          <path d="M5 12l4.5 4.5L19 7" stroke="currentColor" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </div>

      <div class="text-center">
        <p class="text-[15px] font-medium text-pgp-text mb-1">
          {isPGPFile ? 'Decrypted' : 'Encrypted'} successfully
        </p>
        <p class="text-[12px] text-pgp-text-3 mb-3">Output saved to:</p>
        <div class="flex items-center gap-2 px-3 py-[6px] rounded-field
                    bg-pgp-field border border-pgp-field-border">
          <span class="text-[12px] font-mono text-pgp-text-2 select-all break-all">
            {outputPath}
          </span>
          <button
            type="button"
            on:click={copyOutputPath}
            class="flex-shrink-0 text-[12px] text-pgp-text-3 hover:text-pgp-text-2 transition-colors"
          >Copy</button>
        </div>
      </div>

      <div class="flex items-center gap-2 mt-2">
        <button
          type="button"
          on:click={() => OpenFolder(outputPath)}
          class="h-[30px] px-4 rounded-btn text-[13px] font-medium
                 bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                 hover:bg-pgp-fill transition-colors duration-75"
        >Open Folder</button>
        <button
          type="button"
          on:click={reset}
          class="h-[30px] px-4 rounded-btn text-[13px] text-pgp-text-3
                 hover:text-pgp-text-2 transition-colors"
        >New file</button>
      </div>

    </div>
  {/if}

</div>

<RecipientModal
  bind:open={showRecipientModal}
  bind:selectedFingerprints
  on:confirm={onRecipientsConfirm}
/>
<PassphraseModal
  bind:open={showPassphraseModal}
  keyLabel={passphraseKeyLabel}
  confirmLabel="Decrypt"
  allowEmpty={true}
  on:confirm={onPassphraseConfirm}
  on:close={onPassphraseClose}
/>
