<script>
  import { onMount } from 'svelte'
  import {
    ListKeys, ImportMultipleKeys, ImportPrivateKey, DeleteKey, GetPublicKey,
    ExportKeyToFile, RevokeSubkey
  } from '../../../wailsjs/go/main/App'
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'
  import GenerateKeyModal from '../../modals/GenerateKeyModal.svelte'
  import PassphraseModal from '../../modals/PassphraseModal.svelte'
  import AddSubkeyModal from '../../modals/AddSubkeyModal.svelte'

  let showGenerateModal = false

  let keys = []
  let loading = true
  let loadError = ''

  let showImport = false
  let importText = ''
  let importing = false
  let importError = ''
  let importResults = []

  let search = ''
  let filterType = 'all'

  let showPassphraseModal = false
  let passphraseKeyLabel = ''
  let passphraseConfirmLabel = 'Confirm'
  let passphraseResolve = null

  let expandedFp = ''

  let copiedFp = ''
  let exportingFp = ''
  let deletingFp = ''
  let deleteErrors = {}

  let showAddSubkeyModal = false
  let addSubkeyTarget = null
  let revokeConfirmFp = ''

  onMount(() => {
    load()
    EventsOn('refresh-keys', load)
    return () => EventsOff('refresh-keys')
  })

  async function load() {
    loading = true; loadError = ''
    try { keys = await ListKeys() }
    catch { loadError = 'Could not load keys.' }
    finally { loading = false }
  }

  $: filtered = keys.filter(k => {
    if (filterType === 'private' && !k.IsPrivate) return false
    if (filterType === 'public' && k.IsPrivate) return false
    if (!search) return true
    const q = search.toLowerCase()
    return (k.PrimaryUID || '').toLowerCase().includes(q)
        || (k.Email || '').toLowerCase().includes(q)
        || (k.Fingerprint || '').toLowerCase().includes(q)
  })

  function isPrivateArmored(text) {
    return text.trimStart().startsWith('-----BEGIN PGP PRIVATE KEY BLOCK')
  }

  async function handleImport() {
    if (!importText.trim()) return
    importing = true; importError = ''; importResults = []
    try {
      if (isPrivateArmored(importText) && keyCount === 1) {
        passphraseConfirmLabel = 'Import'
        const pp = await askPassphrase('Private Key')
        if (pp === null) { importing = false; return }
        try {
          await ImportPrivateKey(importText.trim(), pp)
        } catch (e) {
          if (!String(e).includes('already in keystore')) {
            importError = String(e)
            return
          }
        }
        importText = ''; showImport = false
        await load()
        return
      }

      const result = await ImportMultipleKeys(importText.trim())
      importResults = result.Entries || []
      const successCount = importResults.filter(r => !r.Error).length
      if (successCount > 0) {
        importText = ''; showImport = false
        await load()
      }
    } catch (e) {
      importError = String(e)
    } finally {
      importing = false
    }
  }

  function countKeys(text) {
    const matches = text.match(/-----BEGIN PGP .* KEY BLOCK-----/g)
    return matches ? matches.length : 0
  }

  $: keyCount = countKeys(importText)

  async function handleCopyPublicKey(fp) {
    copiedFp = fp
    try {
      const pub = await GetPublicKey(fp)
      await navigator.clipboard.writeText(pub)
      setTimeout(() => { if (copiedFp === fp) copiedFp = '' }, 2000)
    } catch { copiedFp = '' }
  }

  async function handleExport(fp) {
    exportingFp = fp
    try { await ExportKeyToFile(fp) }
    catch { }
    finally { exportingFp = '' }
  }

  function startDelete(fp) {
    deletingFp = fp
    setTimeout(() => { if (deletingFp === fp) deletingFp = '' }, 4000)
  }

  async function confirmDelete(fp) {
    deletingFp = ''
    try { await DeleteKey(fp); await load() }
    catch (e) {
      deleteErrors = { ...deleteErrors, [fp]: String(e) }
      setTimeout(() => { const c = {...deleteErrors}; delete c[fp]; deleteErrors = c }, 3000)
    }
  }

  function openAddSubkey(key) {
    addSubkeyTarget = key
    showAddSubkeyModal = true
  }

  async function handleRevokeSubkey(primaryFp, subkeyFp) {
    revokeConfirmFp = ''
    const key = keys.find(k => k.Fingerprint === primaryFp)
    passphraseConfirmLabel = 'Revoke'
    const pp = await askPassphrase(key?.PrimaryUID || key?.Email || primaryFp)
    if (pp === null) return
    try {
      await RevokeSubkey(primaryFp, subkeyFp, pp)
      await load()
    } catch (e) {
      deleteErrors = { ...deleteErrors, [primaryFp]: String(e) }
      setTimeout(() => { const c = {...deleteErrors}; delete c[primaryFp]; deleteErrors = c }, 3000)
    }
  }

  function askPassphrase(label) {
    return new Promise(resolve => {
      passphraseKeyLabel = label
      passphraseResolve = resolve
      showPassphraseModal = true
    })
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

  function toggleExpand(fp) {
    expandedFp = expandedFp === fp ? '' : fp
  }

  function fmtFp(fp) {
    if (!fp) return ''
    return fp.toUpperCase().match(/.{1,4}/g)?.join(' ') ?? fp.toUpperCase()
  }

  function shortFp(fp) {
    return fp ? '…' + fp.slice(-8).toUpperCase() : ''
  }

  function fmtDate(d) {
    if (!d) return null
    try {
      return new Date(d).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    } catch { return null }
  }
</script>

<!-- Header -->
<div class="px-7 pt-6 pb-4 flex-shrink-0 border-b border-pgp-border flex items-end justify-between">
  <div>
    <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">My Keys</h1>
    <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
      {keys.length} key{keys.length !== 1 ? 's' : ''} — generate, import, export and delete
    </p>
  </div>
  <div class="flex items-center gap-2">
    <button
      type="button"
      on:click={() => (showGenerateModal = true)}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium text-white
             bg-pgp-accent hover:opacity-90 transition-opacity duration-75"
    >New Key</button>
    <button
      type="button"
      on:click={() => { showImport = !showImport; importError = ''; importText = ''; importResults = [] }}
      class="h-[30px] px-4 rounded-btn text-[13px] font-medium
             {showImport
               ? 'bg-pgp-accent/10 border border-pgp-accent/30 text-pgp-accent'
               : 'bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2 hover:bg-pgp-fill'}
             transition-colors duration-75"
    >{showImport ? 'Cancel' : 'Import'}</button>
  </div>
</div>

<!-- Import panel -->
{#if showImport}
  <div class="px-6 py-4 border-b border-pgp-border flex-shrink-0 flex flex-col gap-3 bg-pgp-fill/30">
    <label for="import-textarea" class="sr-only">Paste PGP key blocks to import</label>
    <textarea
      id="import-textarea"
      bind:value={importText}
      placeholder="Paste one or more armored PGP key blocks here…"
      spellcheck="false"
      rows="5"
      class="resize-none rounded-field p-3 text-[12px] leading-[1.6] font-mono
             outline-none transition-colors duration-75
             bg-pgp-field border border-pgp-field-border text-pgp-text-2
             placeholder:text-pgp-text-4 focus:border-pgp-accent/50"
    ></textarea>
    {#if keyCount > 0}
      <span class="text-[11px] text-pgp-text-4 flex-shrink-0">{keyCount} key{keyCount !== 1 ? 's' : ''} detected</span>
    {/if}
    {#if importError}
      <p class="text-[12px] text-red-500">{importError}</p>
    {:else if importResults.length > 0}
      <div class="flex flex-col gap-1.5">
        {#each importResults as result}
          <div class="flex items-center gap-2 text-[12px]">
            {#if !result.Error}
              <svg aria-hidden="true" class="w-3.5 h-3.5 text-green-500 flex-shrink-0" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/>
                <path d="M5.5 8.5l2 2 3.5-4.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <span class="text-pgp-text-2 truncate">{result.UID || result.Fingerprint?.slice(-8).toUpperCase() || 'Key imported'}</span>
            {:else}
              <svg aria-hidden="true" class="w-3.5 h-3.5 text-red-500 flex-shrink-0" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/>
                <path d="M6 6l4 4M10 6l-4 4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
              </svg>
              <span class="text-red-400 truncate">{result.Error || 'Import failed'}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
    <div class="flex items-center gap-2">
      <button
        type="button"
        on:click={handleImport}
        disabled={importing || !importText.trim()}
        class="h-[30px] px-4 rounded-btn text-[13px] font-medium text-white
               bg-pgp-accent hover:opacity-90
               disabled:opacity-40 disabled:cursor-not-allowed transition-opacity duration-75"
      >{importing ? 'Importing…' : keyCount > 0 ? `Import ${keyCount} key${keyCount !== 1 ? 's' : ''}` : 'Import'}</button>
      <span class="text-[11px] text-pgp-text-4">
        Supports multiple keys. Private keys will prompt for a passphrase.
      </span>
    </div>
  </div>
{/if}

<!-- Search + filter bar -->
{#if !loading && !loadError && keys.length > 0}
  <div class="px-5 py-3 flex items-center gap-2 border-b border-pgp-border flex-shrink-0">
    <div class="relative flex-1">
      <svg aria-hidden="true"
           class="absolute left-[9px] top-1/2 -translate-y-1/2 w-[13px] h-[13px] text-pgp-text-4 pointer-events-none"
           viewBox="0 0 16 16" fill="none">
        <circle cx="6.5" cy="6.5" r="4" stroke="currentColor" stroke-width="1.3"/>
        <path d="M10 10l3.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
      </svg>
      <input
        bind:value={search}
        type="search"
        aria-label="Search keys by name, email or fingerprint"
        placeholder="Search name, email, fingerprint…"
        class="w-full h-[28px] pl-[28px] pr-3 rounded-btn text-[12px]
               bg-pgp-field border border-pgp-field-border text-pgp-text-2
               placeholder:text-pgp-text-4 focus:outline-none focus:border-pgp-accent/50
               transition-colors duration-75"
      />
    </div>
    <!-- Type filter -->
    <div role="group" aria-label="Filter by key type"
         class="flex rounded-btn overflow-hidden border border-pgp-border-strong text-[11px]">
      {#each [['all','All'],['private','Private'],['public','Public']] as [val, label]}
        <button
          type="button"
          aria-pressed={filterType === val}
          on:click={() => (filterType = val)}
          class="px-3 h-[28px] transition-colors duration-75
                 {filterType === val
                   ? 'bg-pgp-accent text-white'
                   : 'bg-pgp-fill-2 text-pgp-text-3 hover:bg-pgp-fill'}"
        >{label}</button>
      {/each}
    </div>
  </div>
{/if}

<!-- Key list -->
<div class="flex-1 overflow-y-auto">

  {#if loading}
    <p class="px-7 py-6 text-[14px] text-pgp-text-3">Loading keys…</p>

  {:else if loadError}
    <p class="px-7 py-6 text-[14px] text-red-500">{loadError}</p>

  {:else if keys.length === 0}
    <div class="flex flex-col items-center justify-center py-20 gap-3">
      <svg aria-hidden="true" class="w-10 h-10 text-pgp-text-4" viewBox="0 0 24 24" fill="none">
        <circle cx="8" cy="9" r="4" stroke="currentColor" stroke-width="1.3"/>
        <path d="M11.5 13.5L19 21M16 18l2 2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
      </svg>
      <p class="text-[14px] text-pgp-text-3">No keys yet</p>
      <p class="text-[12px] text-pgp-text-4 text-center max-w-xs leading-relaxed">
        Click "New Key" to generate a key pair, or "Import" to paste an existing PGP key.
      </p>
    </div>

  {:else if filtered.length === 0}
    <p class="px-7 py-6 text-[13px] text-pgp-text-3">No keys match your filter.</p>

  {:else}
    <div class="divide-y divide-pgp-border">
      {#each filtered as key (key.Fingerprint)}
        {@const isExpanded = expandedFp === key.Fingerprint}
        {@const created = fmtDate(key.CreatedAt)}

        <!-- Row -->
        <div
          role="button"
          tabindex="0"
          aria-expanded={isExpanded}
          class="group px-5 cursor-pointer select-none
                 {isExpanded ? 'bg-pgp-fill/60' : 'hover:bg-pgp-fill/40'}
                 transition-colors duration-75"
          on:click={() => toggleExpand(key.Fingerprint)}
          on:keydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), toggleExpand(key.Fingerprint))}
        >

          <!-- Main row content -->
          <div class="flex items-center gap-3 py-[10px]">

            <!-- Key type icons -->
            <div aria-hidden="true" class="flex items-center gap-1 flex-shrink-0 w-[35px]">
              <svg xmlns="http://www.w3.org/2000/svg" class="block h-4 w-4 text-blue-500" viewBox="0 0 32 32"><path fill="currentColor" d="M16 2a14 14 0 1 0 14 14A14.016 14.016 0 0 0 16 2M4.02 16.394l1.338.446L7 19.303v1.283a1 1 0 0 0 .293.707L10 24v2.377a11.994 11.994 0 0 1-5.98-9.983M16 28a11.968 11.968 0 0 1-2.572-.285L14 26l1.805-4.512a1 1 0 0 0-.097-.926l-1.411-2.117a1 1 0 0 0-.832-.445h-4.93l-1.248-1.873L9.414 14H11v2h2v-2.734l3.868-6.77l-1.736-.992L14.277 7h-2.742L10.45 5.371A11.861 11.861 0 0 1 20 4.7V8a1 1 0 0 0 1 1h1.465a1 1 0 0 0 .832-.445l.877-1.316A12.033 12.033 0 0 1 26.894 11H22.82a1 1 0 0 0-.98.804l-.723 4.47a1 1 0 0 0 .54 1.055L25 19l.685 4.056A11.98 11.98 0 0 1 16 28"/></svg>
              {#if key.IsPrivate}
                <svg xmlns="http://www.w3.org/2000/svg" class="block h-4 w-4 text-purple-500" viewBox="0 0 32 32"><path fill="currentColor" d="M24 14h-2V8a6 6 0 0 0-12 0v6H8a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V16a2 2 0 0 0-2-2M12 8a4 4 0 0 1 8 0v6h-8Zm12 20H8V16h16Z"/></svg>
              {/if}
            </div>

            <!-- Identity -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-[6px]">
                <span class="text-[13px] font-medium text-pgp-text truncate leading-tight">
                  {key.PrimaryUID || key.Email || 'Unknown'}
                </span>
                {#if key.Status !== 'valid'}
                  <span class="text-[10px] px-[5px] py-[1px] rounded-full flex-shrink-0
                               {key.Status === 'expired'
                                 ? 'bg-yellow-500/15 text-yellow-600 dark:text-yellow-400'
                                 : 'bg-red-500/15 text-red-600 dark:text-red-400'}">
                    {key.Status}
                  </span>
                {/if}
              </div>
              {#if key.Email && key.PrimaryUID && key.Email !== key.PrimaryUID}
                <p class="text-[11px] text-pgp-text-3 truncate">{key.Email}</p>
              {/if}
            </div>

            <!-- Fingerprint (short) -->
            <span class="text-[11px] font-mono text-pgp-text-4 flex-shrink-0 hidden sm:block">
              {shortFp(key.Fingerprint)}
            </span>

            <!-- Created -->
            {#if created}
              <span class="text-[11px] text-pgp-text-4 flex-shrink-0 hidden md:block w-[90px] text-right">
                {created}
              </span>
            {/if}

            <!-- Expand chevron -->
            <svg
              aria-hidden="true"
              class="w-[14px] h-[14px] text-pgp-text-4 flex-shrink-0 transition-transform duration-150
                     {isExpanded ? 'rotate-180' : ''}"
              viewBox="0 0 16 16" fill="none">
              <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>

          <!-- Expanded detail -->
          {#if isExpanded}
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <div
              role="presentation"
              class="pb-3 pl-[38px] flex flex-col gap-3"
              on:click|stopPropagation
            >
              <!-- Full fingerprint -->
              <div>
                <p class="text-[10px] font-bold uppercase tracking-[0.06em] text-pgp-text-4 mb-[3px]">
                  Fingerprint
                </p>
                <p class="text-[11px] font-mono text-pgp-text-3 break-all select-all leading-relaxed">
                  {fmtFp(key.Fingerprint)}
                </p>
              </div>

              <!-- Subkeys -->
              {#if key.Subkeys && key.Subkeys.length > 0}
                <div>
                  <p class="text-[10px] font-bold uppercase tracking-[0.06em] text-pgp-text-4 mb-[6px]">
                    Subkeys
                  </p>
                  <div class="flex flex-col gap-[4px]">
                    {#each key.Subkeys as sub}
                      <div class="flex items-center gap-2 py-[4px] px-3 rounded-[6px]
                                   bg-pgp-field/40 border border-pgp-border
                                   {sub.IsRevoked ? 'opacity-50' : ''}">
                        <!-- Usage badges -->
                        <div class="flex gap-[3px] flex-shrink-0">
                          {#if sub.Usage?.includes('sign')}
                            <span aria-label="Signing"
                              class="text-[9px] font-bold uppercase px-[5px] py-[1px]
                                     rounded-full bg-blue-500/15 text-blue-500">S</span>
                          {/if}
                          {#if sub.Usage?.includes('encrypt')}
                            <span aria-label="Encryption"
                              class="text-[9px] font-bold uppercase px-[5px] py-[1px]
                                     rounded-full bg-green-500/15 text-green-600">E</span>
                          {/if}
                          {#if sub.Usage?.includes('certify')}
                            <span aria-label="Certify"
                              class="text-[9px] font-bold uppercase px-[5px] py-[1px]
                                     rounded-full bg-purple-500/15 text-purple-500">C</span>
                          {/if}
                        </div>
                        <!-- Algorithm -->
                        <span class="text-[11px] text-pgp-text-3 flex-shrink-0">{sub.Algorithm}</span>
                        <!-- Short fingerprint -->
                        <span class="text-[10px] font-mono text-pgp-text-4 flex-shrink-0">
                          …{sub.Fingerprint?.slice(-8).toUpperCase()}
                        </span>
                        <!-- Expiry -->
                        {#if sub.ExpiresAt}
                          <span class="text-[10px] text-pgp-text-4 flex-shrink-0">
                            exp. {fmtDate(sub.ExpiresAt)}
                          </span>
                        {/if}
                        <!-- Revoked badge -->
                        {#if sub.IsRevoked}
                          <span class="ml-auto text-[9px] px-[5px] py-[1px] rounded-full
                                 bg-red-500/15 text-red-500 flex-shrink-0">revoked</span>
                        {:else if key.IsPrivate && !sub.Usage?.includes('certify')}
                          {#if revokeConfirmFp === sub.Fingerprint}
                            <div class="ml-auto flex items-center gap-2">
                              <button type="button"
                                on:click|stopPropagation={() => handleRevokeSubkey(key.Fingerprint, sub.Fingerprint)}
                                class="text-[11px] text-red-500 hover:underline">Confirm</button>
                              <button type="button"
                                on:click|stopPropagation={() => (revokeConfirmFp = '')}
                                class="text-[11px] text-pgp-text-4 hover:text-pgp-text-3">Cancel</button>
                            </div>
                          {:else}
                            <button type="button"
                              on:click|stopPropagation={() => (revokeConfirmFp = sub.Fingerprint)}
                              class="ml-auto text-[10px] text-pgp-text-4 hover:text-red-500
                                     transition-colors duration-75">Revoke</button>
                          {/if}
                        {/if}
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}

              <!-- Actions -->
              <div class="flex items-center gap-[6px]">
                <button
                  type="button"
                  on:click={() => handleCopyPublicKey(key.Fingerprint)}
                  class="h-[26px] px-3 rounded text-[12px] font-medium
                         bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                         hover:bg-pgp-fill transition-colors duration-75"
                >
                  {copiedFp === key.Fingerprint ? 'Copied!' : 'Copy Public Key'}
                </button>

                {#if key.IsPrivate}
                  <button
                    type="button"
                    on:click={() => handleExport(key.Fingerprint)}
                    disabled={exportingFp === key.Fingerprint}
                    class="h-[26px] px-3 rounded text-[12px] font-medium
                           bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                           hover:bg-pgp-fill disabled:opacity-40 transition-colors duration-75"
                  >{exportingFp === key.Fingerprint ? 'Saving…' : 'Export'}</button>

                  <button
                    type="button"
                    on:click={() => openAddSubkey(key)}
                    class="h-[26px] px-3 rounded text-[12px] font-medium
                           bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                           hover:bg-pgp-fill transition-colors duration-75"
                  >+ Subkey</button>
                {/if}

                <div class="flex-1"></div>

                {#if deleteErrors[key.Fingerprint]}
                  <span class="text-[11px] text-red-500">{deleteErrors[key.Fingerprint]}</span>
                {:else if deletingFp === key.Fingerprint}
                  <button
                    type="button"
                    on:click={() => confirmDelete(key.Fingerprint)}
                    class="h-[26px] px-3 rounded text-[12px] font-medium text-red-500
                           hover:bg-red-500/10 transition-colors duration-75"
                  >Confirm delete</button>
                  <button
                    type="button"
                    on:click={() => (deletingFp = '')}
                    class="h-[26px] px-2 rounded text-[12px] text-pgp-text-4
                           hover:text-pgp-text-3 transition-colors"
                  >Cancel</button>
                {:else}
                  <button
                    type="button"
                    on:click={() => startDelete(key.Fingerprint)}
                    class="h-[26px] px-3 rounded text-[12px] text-pgp-text-4
                           hover:text-red-500 hover:bg-red-500/10 transition-colors duration-75"
                  >Delete</button>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

</div>

<GenerateKeyModal
  bind:open={showGenerateModal}
  on:generated={load}
  on:close={() => (showGenerateModal = false)}
/>

<PassphraseModal
  bind:open={showPassphraseModal}
  keyLabel={passphraseKeyLabel}
  confirmLabel={passphraseConfirmLabel}
  allowEmpty={true}
  on:confirm={onPassphraseConfirm}
  on:close={onPassphraseClose}
/>

{#if showAddSubkeyModal && addSubkeyTarget}
  <AddSubkeyModal
    bind:open={showAddSubkeyModal}
    keyInfo={addSubkeyTarget}
    on:added={load}
    on:close={() => { showAddSubkeyModal = false; addSubkeyTarget = null }}
  />
{/if}
