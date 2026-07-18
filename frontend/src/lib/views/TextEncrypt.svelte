<script>
  import { onMount } from 'svelte'
  import RecipientModal from '../../modals/RecipientModal.svelte'
  import PassphraseModal from '../../modals/PassphraseModal.svelte'
  import ImportKeyModal from '../../modals/ImportKeyModal.svelte'
  import SignModal from '../../modals/SignModal.svelte'
  import {
    ListKeys, EncryptText, DecryptText, SignText, VerifyText,
    FindDecryptionKey, GetCachedPassphrase, HasCachedPassphrase, CachePassphrase, ReadClipboard,
    GetConfig, SetClipboardText
  } from '../../../wailsjs/go/main/App'
  import { pendingClipboardMessage, pendingEncryptText } from '../../stores.js'

  let text = ''
  let prevText = null
  let loading = false
  let error = ''
  let status = null
  let statusTimer = null

  let privateKeys = []
  let verifyResult = null
  let verifiedText = null

  let copyFeedback = false
  let copyTimer = null

  let selectedFingerprints = []
  let showRecipientModal = false

  let showPassphraseModal = false
  let passphraseKeyLabel = ''
  let passphraseConfirmLabel = 'Decrypt'
  let passphraseResolve = null

  let showSignModal = false

  let showImportModal = false
  let pendingImportArmored = ''

  $: trimmed = text.trimStart()
  $: isPGP = trimmed.startsWith('-----BEGIN PGP')
  $: isKeyBlock = trimmed.startsWith('-----BEGIN PGP PUBLIC KEY BLOCK')
                || trimmed.startsWith('-----BEGIN PGP PRIVATE KEY BLOCK')
  $: isEncrypted = trimmed.startsWith('-----BEGIN PGP MESSAGE')
  $: isSigned = trimmed.startsWith('-----BEGIN PGP SIGNED MESSAGE')
                || trimmed.startsWith('-----BEGIN PGP MESSAGE')
  $: canEncrypt = !!text.trim() && !isPGP
  $: canDecrypt = !!text.trim() && isEncrypted
  $: canSign    = !!text.trim() && !isPGP && privateKeys.length > 0
  $: canVerify  = !!text.trim() && (isEncrypted || isSigned)

  $: if (verifiedText !== null && text !== verifiedText) {
    verifyResult = null
    verifiedText = null
  }

  $: if ($pendingClipboardMessage !== null) {
    const armored = $pendingClipboardMessage
    pendingClipboardMessage.set(null)
    text = armored
    prevText = null
    error = ''
    handleDecrypt()
  }

  $: if ($pendingEncryptText !== null) {
    const plain = $pendingEncryptText
    pendingEncryptText.set(null)
    text = plain
    prevText = null
    error = ''
    handleEncrypt()
  }

  let autoCopy = false

  onMount(async () => {
    try {
      const keys = await ListKeys()
      privateKeys = keys.filter(k => k.IsPrivate)
    } catch {}
    try {
      autoCopy = (await GetConfig()).AutoCopyResults ?? false
    } catch {}
  })

  async function autoCopyResult(armored) {
    if (!autoCopy) return ''
    try {
      await SetClipboardText(armored)
      return ' — copied to clipboard'
    } catch { return '' }
  }

  function setStatus(label) {
    if (statusTimer) clearTimeout(statusTimer)
    status = { label }
    statusTimer = setTimeout(() => (status = null), 4000)
  }

  function handleEncrypt() {
    if (!text.trim()) return
    showRecipientModal = true
  }

  async function doEncrypt() {
    const original = text
    loading = true; error = ''; verifyResult = null; verifiedText = null
    try {
      const result = await EncryptText(text, selectedFingerprints, '', '')
      if (result.Error) { error = result.Error }
      else {
        prevText = original; text = result.Armored
        setStatus('Encrypted' + await autoCopyResult(result.Armored))
      }
    } catch (e) { error = String(e) }
    finally { loading = false }
  }

  async function handleDecrypt() {
    if (!text.trim()) return
    loading = true; error = ''; verifyResult = null
    try {
      const matchFp = await FindDecryptionKey(text)
      if (!matchFp) { error = 'No matching private key found for this message.'; return }

      const allKeys = await ListKeys()
      const key = allKeys.find(k => k.Fingerprint === matchFp)
      if (!key) { error = 'No matching private key found for this message.'; return }

      let pp
      if (await HasCachedPassphrase(matchFp)) {
        pp = await GetCachedPassphrase(matchFp)
      } else {
        passphraseConfirmLabel = 'Decrypt'
        pp = await askPassphrase(key.Email || key.PrimaryUID)
        if (pp === null) return
      }

      const result = await DecryptText(text, matchFp, pp, false)
      if (result.Error) { error = result.Error; return }
      await CachePassphrase(matchFp, pp)
      prevText = text
      text = result.Plaintext
      if (result.SignedBy) text += `\n\n[Signed by: ${result.SignedBy}]`
      error = ''; setStatus('Decrypted')
    } catch (e) { error = String(e) }
    finally { loading = false }
  }

  function handleSign() {
    if (!text.trim()) return
    showSignModal = true
  }

  async function onSignConfirm(e) {
    const { signingFp, keyLabel } = e.detail
    const original = text
    loading = true; error = ''; verifyResult = null; verifiedText = null
    try {
      let pp
      if (await HasCachedPassphrase(signingFp)) {
        pp = await GetCachedPassphrase(signingFp)
      } else {
        passphraseConfirmLabel = 'Sign'
        pp = await askPassphrase(keyLabel || 'Signing key')
        if (pp === null) return
      }
      const result = await SignText(text, signingFp, pp)
      if (result.Error) { error = result.Error }
      else {
        if (pp) await CachePassphrase(signingFp, pp)
        prevText = original; text = result.Armored
        setStatus('Signed' + await autoCopyResult(result.Armored))
      }
    } catch (e) { error = String(e) }
    finally { loading = false }
  }

  async function handleVerify() {
    if (!text.trim()) return
    loading = true; error = ''; verifyResult = null; verifiedText = null
    try {
      const result = await VerifyText(text)
      if (result.Error) { error = result.Error }
      else if (result.Valid) { verifyResult = result; verifiedText = text; setStatus('Verified') }
      else { error = 'Signature invalid or could not be verified.' }
    } catch (e) { error = String(e) }
    finally { loading = false }
  }

  async function copyText() {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      if (copyTimer) clearTimeout(copyTimer)
      copyFeedback = true
      copyTimer = setTimeout(() => (copyFeedback = false), 1500)
    } catch {}
  }

  async function handlePaste() {
    try {
      const t = await ReadClipboard()
      if (t?.trim()) text = t
    } catch {}
  }

  function undo() {
    if (prevText !== null) {
      text = prevText; prevText = null
      error = ''; verifyResult = null; verifiedText = null
      if (statusTimer) clearTimeout(statusTimer); status = null
    }
  }

  function clearAll() {
    text = ''; prevText = null; error = ''; verifyResult = null; verifiedText = null
    selectedFingerprints = []
    if (statusTimer) clearTimeout(statusTimer); status = null
    if (copyTimer) clearTimeout(copyTimer); copyFeedback = false
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
    if (!text.trim() || !selectedFingerprints.length) return
    doEncrypt()
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

  function openImportFromText() {
    pendingImportArmored = text
    showImportModal = true
  }

  function onImportDone() {
    showImportModal = false
    clearAll()
    ListKeys().then(keys => {
      privateKeys = keys.filter(k => k.IsPrivate)
    }).catch(() => {})
  }

  function onImportClose() {
    showImportModal = false
    clearAll()
  }
</script>

<div class="px-7 pt-6 pb-5 flex-shrink-0 border-b border-pgp-border">
  <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">Text</h1>
  <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
    Encrypt, decrypt, sign or verify — result replaces your input
  </p>
</div>

<div class="flex flex-col flex-1 overflow-hidden min-h-0 p-6 gap-3">

  <div class="relative flex-1 min-h-0">
    <textarea
      bind:value={text}
      on:contextmenu|preventDefault
      placeholder="Paste or type PGP content here…"
      spellcheck="false"
      disabled={loading}
      class="w-full h-full resize-none rounded-field p-4 text-sm leading-[1.7]
             font-mono outline-none transition-colors duration-75
             bg-pgp-field border border-pgp-field-border text-pgp-text-2
             placeholder:text-pgp-text-4
             focus:border-pgp-accent/50
             disabled:opacity-60"
    ></textarea>

    {#if text}
      <button
        type="button"
        on:click={copyText}
        class="absolute bottom-3 right-3
               h-7 px-3 rounded-btn text-[12px]
               bg-pgp-titlebar/90 border border-pgp-border text-pgp-text-3
               hover:text-pgp-text-2 hover:border-pgp-accent/30
               transition-colors duration-75 select-none"
        style="backdrop-filter: blur(4px);"
      >{copyFeedback ? 'Copied!' : 'Copy'}</button>
    {/if}
  </div>

  {#if isKeyBlock}
    <div class="flex items-center justify-between gap-3 px-4 py-[10px] rounded-field
                bg-pgp-accent/[0.07] border border-pgp-accent/20 flex-shrink-0">
      <div class="flex items-center gap-2 min-w-0">
        <svg aria-hidden="true" class="w-4 h-4 text-pgp-accent flex-shrink-0" viewBox="0 0 16 16" fill="none">
          <circle cx="6" cy="7" r="3" stroke="currentColor" stroke-width="1.25"/>
          <path d="M8.2 9.2l5.3 5.3" stroke="currentColor" stroke-width="1.25" stroke-linecap="round"/>
        </svg>
        <p class="text-[13px] text-pgp-text-2">
          PGP key detected
        </p>
      </div>
      <button
        type="button"
        on:click={openImportFromText}
        class="h-7 px-3 rounded-[7px] text-[12px] font-medium text-white flex-shrink-0
               bg-pgp-accent hover:opacity-90 transition-opacity duration-75"
      >Import…</button>
    </div>
  {/if}

  {#if verifyResult}
    <div class="flex items-start gap-3 px-4 py-3 rounded-field
                bg-green-500/[0.07] border border-green-500/20 flex-shrink-0">
      <svg class="w-4 h-4 mt-[1px] text-green-600 shrink-0" viewBox="0 0 16 16" fill="none">
        <path d="M3 8l3.5 3.5L13 4.5" stroke="currentColor" stroke-width="1.5"
              stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <div>
        <p class="text-[13px] font-medium text-green-700 dark:text-green-400">Signature valid</p>
        {#if verifyResult.UID || verifyResult.Email}
          <p class="text-[12px] text-pgp-text-3 mt-[2px]">
            Signed by {verifyResult.UID || verifyResult.Email}{#if verifyResult.UID && verifyResult.Email}&nbsp;<span class="opacity-60">&lt;{verifyResult.Email}&gt;</span>{/if}
          </p>
        {/if}
      </div>
    </div>
  {/if}

  {#if error}
    <p class="text-[13px] text-red-500 leading-[1.5] px-1 flex-shrink-0">{error}</p>
  {/if}

  {#if status && !error && !verifyResult}
    <p class="text-[12px] text-pgp-text-3 px-1 flex-shrink-0">
      {status.label} — result is in the field above
    </p>
  {/if}

  <div class="flex items-center gap-[6px] flex-shrink-0 flex-wrap">

    <button
      type="button"
      on:click={handleEncrypt}
      disabled={loading || !canEncrypt}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             disabled:opacity-35 disabled:cursor-not-allowed transition-opacity duration-75"
    >Encrypt</button>

    <button
      type="button"
      on:click={handleDecrypt}
      disabled={loading || !canDecrypt}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium
             bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
             hover:bg-pgp-fill disabled:opacity-35 disabled:cursor-not-allowed
             transition-colors duration-75"
    >Decrypt</button>

    <div class="w-px h-4 bg-pgp-border mx-1 flex-shrink-0"></div>

    <button
      type="button"
      on:click={handleSign}
      disabled={loading || !canSign}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium
             bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
             hover:bg-pgp-fill disabled:opacity-35 disabled:cursor-not-allowed
             transition-colors duration-75"
    >Sign</button>

    <button
      type="button"
      on:click={handleVerify}
      disabled={loading || !canVerify}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium
             bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
             hover:bg-pgp-fill disabled:opacity-35 disabled:cursor-not-allowed
             transition-colors duration-75"
    >Verify</button>

    {#if loading}
      <span class="text-[13px] text-pgp-text-3 ml-1">Working…</span>
    {/if}

    <div class="flex-1"></div>

    {#if !text}
      <button type="button" on:click={handlePaste}
        class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-3
               hover:text-pgp-text-2 transition-colors">
        Paste
      </button>
    {:else}
      {#if prevText !== null}
        <button type="button" on:click={undo}
          class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-3
                 hover:text-pgp-text-2 transition-colors">
          ← Back
        </button>
      {/if}
      <button type="button" on:click={clearAll}
        class="h-[30px] px-3 rounded-btn text-[13px] text-pgp-text-4
               hover:text-pgp-text-3 transition-colors">
        Clear
      </button>
    {/if}

  </div>
</div>

<RecipientModal
  bind:open={showRecipientModal}
  bind:selectedFingerprints
  on:confirm={onRecipientsConfirm}
/>
<PassphraseModal
  bind:open={showPassphraseModal}
  keyLabel={passphraseKeyLabel}
  confirmLabel={passphraseConfirmLabel}
  allowEmpty={true}
  on:confirm={onPassphraseConfirm}
  on:close={onPassphraseClose}
/>
<SignModal
  bind:open={showSignModal}
  on:confirm={onSignConfirm}
  on:close={() => (showSignModal = false)}
/>
<ImportKeyModal
  bind:open={showImportModal}
  armored={pendingImportArmored}
  on:imported={onImportDone}
  on:close={onImportClose}
/>
