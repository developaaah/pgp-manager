<script>
  import { onMount } from 'svelte'
  import PassphraseModal from '../../modals/PassphraseModal.svelte'
  import { ListKeys, SignText, VerifyText, ReadClipboard, GetConfig, SetClipboardText } from '../../../wailsjs/go/main/App'

  let text = ''
  let prevText = null
  let loading = false
  let error = ''
  let verifyResult = null
  let status = null
  let statusTimer = null

  let privateKeys = []
  let signingKeyFp = ''

  let showPassphraseModal = false
  let passphraseResolve = null

  $: isPGPSigned = text.trimStart().startsWith('-----BEGIN PGP SIGNED MESSAGE')
                || text.trimStart().startsWith('-----BEGIN PGP MESSAGE')
  $: mode = isPGPSigned ? 'verify' : 'sign'

  let autoCopy = false

  onMount(async () => {
    try {
      const keys = await ListKeys()
      privateKeys = keys.filter(k => k.IsPrivate)
      if (privateKeys.length) signingKeyFp = privateKeys[0].Fingerprint
    } catch {}
    try {
      autoCopy = (await GetConfig()).AutoCopyResults ?? false
    } catch {}
  })

  async function handleFocus() {
    if (text) return
    try {
      const t = await ReadClipboard()
      if (t) text = t
    } catch {}
  }

  function setStatus(label) {
    if (statusTimer) clearTimeout(statusTimer)
    status = { label }
    statusTimer = setTimeout(() => (status = null), 5000)
  }

  async function handleSign() {
    if (!text.trim() || !signingKeyFp) return
    const key = privateKeys.find(k => k.Fingerprint === signingKeyFp)
    const passphrase = await askPassphrase(key?.Email || key?.PrimaryUID || signingKeyFp)
    if (passphrase === null) return

    loading = true
    error = ''
    verifyResult = null
    try {
      const result = await SignText(text, signingKeyFp, passphrase)
      if (result.Error) {
        error = result.Error
      } else {
        prevText = text
        text = result.Armored
        let suffix = ''
        if (autoCopy) {
          try { await SetClipboardText(result.Armored); suffix = ' — copied to clipboard' } catch {}
        }
        setStatus('Signed' + suffix)
      }
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  async function handleVerify() {
    if (!text.trim()) return
    loading = true
    error = ''
    verifyResult = null
    try {
      const result = await VerifyText(text)
      if (result.Error) {
        error = result.Error
      } else if (result.Valid) {
        verifyResult = result
        setStatus('Verified')
      } else {
        error = 'Signature is invalid or could not be verified.'
      }
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  function askPassphrase(label) {
    return new Promise(resolve => {
      passphraseResolve = resolve
      showPassphraseModal = true
    })
  }

  async function copyText() {
    if (text) try { await navigator.clipboard.writeText(text) } catch {}
  }

  function undo() {
    if (prevText !== null) { text = prevText; prevText = null; error = ''; verifyResult = null }
  }

  function clearAll() {
    text = ''; prevText = null; error = ''; verifyResult = null
    if (statusTimer) clearTimeout(statusTimer)
    status = null
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

<!-- Header -->
<div class="px-7 pt-6 pb-5 flex-shrink-0 border-b border-pgp-border">
  <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">Sign & Verify</h1>
  <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
    Sign text with your key, or verify a signed message
  </p>
</div>

<!-- Workspace -->
<div class="flex flex-col flex-1 overflow-hidden min-h-0 p-6 gap-3">

  <textarea
    bind:value={text}
    on:focus={handleFocus}
    placeholder="Click to paste, or type here…"
    spellcheck="false"
    disabled={loading}
    class="flex-1 min-h-0 resize-none rounded-field p-4 text-sm leading-[1.7]
           font-mono outline-none transition-colors duration-75
           bg-pgp-field border border-pgp-field-border text-pgp-text-2
           placeholder:text-pgp-text-4
           focus:border-pgp-accent/50
           disabled:opacity-60"
  ></textarea>

  <!-- Verify result banner -->
  {#if verifyResult}
    <div class="flex items-start gap-3 px-4 py-3 rounded-field
                bg-green-500/[0.07] border border-green-500/20">
      <svg aria-hidden="true" class="w-4 h-4 mt-[1px] text-green-600 shrink-0" viewBox="0 0 16 16" fill="none">
        <path d="M3 8l3.5 3.5L13 4.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <div>
        <p class="text-[13px] font-medium text-green-700 dark:text-green-400">Signature valid</p>
        {#if verifyResult.UID}
          <p class="text-[12px] text-pgp-text-3 mt-[2px]">Signed by {verifyResult.UID}</p>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Error -->
  {#if error}
    <p class="text-[13px] text-red-500 leading-[1.5] px-1">{error}</p>
  {/if}

  <!-- Status -->
  {#if status && !error && !verifyResult}
    <p class="text-[12px] text-pgp-text-3 px-1">{status.label} — result is in the field above</p>
  {/if}

  <!-- Actions -->
  <div class="flex items-center gap-2 flex-shrink-0">

    {#if mode === 'sign'}
      {#if privateKeys.length > 1}
        <select
          bind:value={signingKeyFp}
          class="h-[30px] px-2 pr-6 rounded-btn text-[13px]
                 bg-pgp-field border border-pgp-field-border text-pgp-text-2
                 focus:outline-none focus:border-pgp-accent/50"
        >
          {#each privateKeys as key (key.Fingerprint)}
            <option value={key.Fingerprint}>{key.Email || key.PrimaryUID}</option>
          {/each}
        </select>
      {:else if privateKeys.length === 0}
        <span class="text-[13px] text-pgp-text-3">No private key found</span>
      {/if}

      <button
        type="button"
        on:click={handleSign}
        disabled={loading || !text.trim() || !signingKeyFp}
        class="h-[30px] px-4 rounded-btn text-[13px] font-medium text-white
               bg-pgp-accent hover:opacity-90
               disabled:opacity-40 disabled:cursor-not-allowed transition-opacity duration-75"
      >Sign</button>

    {:else}
      <button
        type="button"
        on:click={handleVerify}
        disabled={loading || !text.trim()}
        class="h-[30px] px-4 rounded-btn text-[13px] font-medium text-white
               bg-pgp-accent hover:opacity-90
               disabled:opacity-40 disabled:cursor-not-allowed transition-opacity duration-75"
      >Verify</button>
    {/if}

    <span class="text-[11px] uppercase tracking-[0.06em] font-semibold
                 {mode === 'verify' ? 'text-pgp-accent' : 'text-pgp-text-4'}
                 px-2 py-[3px] rounded-full
                 {mode === 'verify' ? 'bg-pgp-accent-bg' : 'bg-pgp-border/50'}">
      {mode === 'verify' ? 'Verify mode' : 'Sign mode'}
    </span>

    {#if loading}
      <span class="text-[13px] text-pgp-text-3">Working…</span>
    {/if}

    <div class="flex-1"></div>

    {#if prevText !== null}
      <button type="button" on:click={undo}
        class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-3 hover:text-pgp-text-2 transition-colors">
        ← Back
      </button>
    {/if}

    {#if text}
      <button type="button" on:click={copyText}
        class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-3 hover:text-pgp-text-2 transition-colors">
        Copy
      </button>
      <button type="button" on:click={clearAll}
        class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-4 hover:text-pgp-text-3 transition-colors">
        Clear
      </button>
    {/if}

  </div>
</div>

<PassphraseModal
  bind:open={showPassphraseModal}
  confirmLabel="Sign"
  on:confirm={onPassphraseConfirm}
  on:close={onPassphraseClose}
/>
