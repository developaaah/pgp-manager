<script>
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'
  import {
    ListKeyservers, StartKeyserverSearch, ImportFromKeyserver,
    PublishToKeyserver, AddCustomKeyserver, RemoveCustomKeyserver, ListKeys
  } from '../../../wailsjs/go/main/App'

  let keyservers = []
  let selectedURL = ''

  let query = ''
  let searching = false
  let searchError = ''
  let results = []
  let searched = false

  let importingFp = ''
  let importedFps = {}
  let importErrors = {}

  let privateKeys = []
  let publishFp = ''
  let publishServerURL = ''
  let publishing = false
  let publishError = ''
  let publishSuccess = false

  let newServerURL = ''
  let addingServer = false
  let addServerError = ''
  let showAddServer = false

  onMount(async () => {
    await loadKeyservers()
    try {
      const keys = await ListKeys()
      privateKeys = keys.filter(k => k.IsPrivate)
      if (privateKeys.length) publishFp = privateKeys[0].Fingerprint
    } catch {}

    EventsOn('keyserver:results', (data) => {
      results = Array.isArray(data) ? data : []
      searching = false
      searched = true
    })
    EventsOn('keyserver:error', (msg) => {
      searchError = typeof msg === 'string' ? msg : 'Search failed'
      searching = false
      searched = true
    })
  })

  onDestroy(() => {
    EventsOff('keyserver:results')
    EventsOff('keyserver:error')
  })

  async function loadKeyservers() {
    try {
      keyservers = await ListKeyservers()
      if (keyservers.length && !publishServerURL) {
        publishServerURL = keyservers[0].URL
      }
    } catch {}
  }

  function handleSearch() {
    if (!query.trim()) return
    searching = true
    searchError = ''
    results = []
    searched = false
    StartKeyserverSearch(query.trim(), selectedURL)
  }

  function onSearchKeydown(e) {
    if (e.key === 'Enter') handleSearch()
  }

  async function handleImport(fp) {
    importingFp = fp
    importErrors = { ...importErrors, [fp]: '' }
    try {
      await ImportFromKeyserver(fp, selectedURL)
      importedFps = { ...importedFps, [fp]: true }
    } catch (e) {
      importErrors = { ...importErrors, [fp]: String(e) }
    } finally {
      importingFp = ''
    }
  }

  async function handlePublish() {
    if (!publishFp || !publishServerURL) return
    publishing = true
    publishError = ''
    publishSuccess = false
    try {
      await PublishToKeyserver(publishFp, publishServerURL)
      publishSuccess = true
    } catch (e) {
      publishError = String(e)
    } finally {
      publishing = false
    }
  }

  async function handleAddServer() {
    addServerError = ''
    const u = newServerURL.trim().replace(/\/$/, '')
    if (!u) return
    addingServer = true
    try {
      await AddCustomKeyserver(u)
      newServerURL = ''
      showAddServer = false
      await loadKeyservers()
    } catch (e) {
      addServerError = String(e)
    } finally {
      addingServer = false
    }
  }

  async function handleRemoveServer(url) {
    try {
      await RemoveCustomKeyserver(url)
      await loadKeyservers()
    } catch {}
  }

  function onAddKeydown(e) {
    if (e.key === 'Enter') handleAddServer()
    if (e.key === 'Escape') { showAddServer = false; addServerError = '' }
  }

  function shortFp(fp) {
    return fp ? '…' + fp.slice(-8).toUpperCase() : ''
  }

  $: selectedLabel = selectedURL === ''
    ? 'Auto (all servers)'
    : (keyservers.find(k => k.URL === selectedURL)?.Label || selectedURL)
</script>

<div class="px-7 pt-6 pb-5 flex-shrink-0 border-b border-pgp-border">
  <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">Keyserver</h1>
  <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
    Search, import and publish PGP keys
  </p>
</div>

<div class="flex-1 flex flex-col overflow-hidden min-h-0">

  <div class="px-7 py-5 flex-shrink-0 border-b border-pgp-border">

    <div class="flex items-center gap-2 mb-3">
      <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 flex-shrink-0">Server</p>
      <div class="flex flex-wrap gap-[5px]">
        <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
        <button
          type="button"
          on:click={() => { selectedURL = ''; searchError = '' }}
          class="h-6 px-[10px] rounded-full text-[11px] font-medium transition-colors duration-75
                 {selectedURL === ''
                   ? 'bg-pgp-accent text-white'
                   : 'bg-pgp-fill-2 border border-pgp-border text-pgp-text-3 hover:bg-pgp-fill'}"
        >Auto</button>
        {#each keyservers as ks (ks.URL)}
          <button
            type="button"
            on:click={() => { selectedURL = ks.URL; searchError = '' }}
            class="h-6 px-[10px] rounded-full text-[11px] font-medium transition-colors duration-75 flex items-center gap-1
                   {selectedURL === ks.URL
                     ? 'bg-pgp-accent text-white'
                     : 'bg-pgp-fill-2 border border-pgp-border text-pgp-text-3 hover:bg-pgp-fill'}"
          >
            {ks.Label}
            {#if !ks.BuiltIn}
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <span
                on:click|stopPropagation={() => handleRemoveServer(ks.URL)}
                class="ml-[2px] w-3 h-3 rounded-full flex items-center justify-center
                       hover:bg-white/20 transition-colors"
                title="Remove"
              >
                <svg class="w-2 h-2" viewBox="0 0 8 8" fill="none">
                  <path d="M1.5 1.5l5 5M6.5 1.5l-5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
                </svg>
              </span>
            {/if}
          </button>
        {/each}
        {#if showAddServer}
          <div class="flex items-center gap-1">
            <!-- svelte-ignore a11y-autofocus -->
            <input
              bind:value={newServerURL}
              on:keydown={onAddKeydown}
              type="url"
              placeholder="https://…"
              autofocus
              class="h-6 px-2 rounded-full text-[11px] font-mono
                     bg-pgp-field border border-pgp-accent/40 text-pgp-text-2
                     placeholder:text-pgp-text-4 focus:outline-none w-[200px]"
            />
            <button
              type="button"
              on:click={handleAddServer}
              disabled={addingServer || !newServerURL.trim()}
              class="h-6 px-[10px] rounded-full text-[11px] font-medium text-white
                     bg-pgp-accent disabled:opacity-40 transition-opacity duration-75"
            >{addingServer ? '…' : 'Add'}</button>
            <button
              type="button"
              on:click={() => { showAddServer = false; addServerError = '' }}
              class="h-6 px-[8px] rounded-full text-[11px]
                     bg-pgp-fill-2 border border-pgp-border text-pgp-text-3 hover:bg-pgp-fill"
            >Cancel</button>
          </div>
        {:else}
          <button
            type="button"
            on:click={() => { showAddServer = true; addServerError = '' }}
            class="h-6 px-[10px] rounded-full text-[11px] font-medium
                   bg-pgp-fill-2 border border-pgp-border border-dashed text-pgp-text-4
                   hover:border-pgp-accent/50 hover:text-pgp-text-3 transition-colors duration-75"
          >+ Add server</button>
        {/if}
      </div>
    </div>
    {#if addServerError}
      <p class="text-[12px] text-red-500 mb-2">{addServerError}</p>
    {/if}

    <div class="flex gap-2">
      <input
        bind:value={query}
        on:keydown={onSearchKeydown}
        type="search"
        aria-label="Search by email or fingerprint"
        placeholder="Email address or fingerprint…"
        class="flex-1 h-[34px] px-3 rounded-field text-[14px]
               bg-pgp-field border border-pgp-field-border text-pgp-text-2
               placeholder:text-pgp-text-4
               focus:outline-none focus:border-pgp-accent/50 transition-colors"
      />
      <button
        type="button"
        on:click={handleSearch}
        disabled={searching || !query.trim()}
        class="h-[34px] px-4 rounded-btn text-[13px] font-medium text-white
               bg-pgp-accent hover:opacity-90
               disabled:opacity-40 disabled:cursor-not-allowed transition-opacity duration-75"
      >{searching ? 'Searching…' : 'Search'}</button>
    </div>
    {#if !searching && selectedURL === '' && searched}
      <p class="mt-1.5 text-[11px] text-pgp-text-4">Searched on all {keyservers.length} server{keyservers.length !== 1 ? 's' : ''}</p>
    {/if}
    {#if searchError}
      <p class="mt-2 text-[13px] text-red-500 leading-[1.5]">{searchError}</p>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto px-5 py-3">
    {#if searching}
      <p class="px-2 py-4 text-[14px] text-pgp-text-3">Searching…</p>

    {:else if searched && results.length === 0 && !searchError}
      <p class="px-2 py-4 text-[14px] text-pgp-text-3">No keys found for this query.</p>

    {:else if results.length > 0}
      <div class="flex flex-col gap-[2px] mb-6">
        {#each results as result (result.Fingerprint)}
          <div class="flex items-center gap-3 px-3 py-[10px] rounded-field
                      hover:bg-pgp-border/30 transition-colors duration-75">
            <div class="flex-1 min-w-0">
              <p class="text-[14px] text-pgp-text truncate">{result.UID || result.Email}</p>
              <p class="text-[11px] text-pgp-text-3 font-mono mt-[2px]">{shortFp(result.Fingerprint)}</p>
            </div>
            {#if importErrors[result.Fingerprint]}
              <span class="text-[12px] text-red-500">{importErrors[result.Fingerprint]}</span>
            {:else if importedFps[result.Fingerprint]}
              <span class="text-[12px] text-green-600 dark:text-green-400">Imported</span>
            {:else}
              <button
                type="button"
                on:click={() => handleImport(result.Fingerprint)}
                disabled={importingFp === result.Fingerprint}
                class="h-7 px-3 rounded-btn text-[12px] font-medium
                       bg-pgp-accent/10 border border-pgp-accent/30 text-pgp-accent
                       hover:bg-pgp-accent/15 disabled:opacity-40
                       transition-colors duration-75"
              >
                {importingFp === result.Fingerprint ? 'Importing…' : 'Import'}
              </button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  {#if privateKeys.length > 0}
    <div class="flex-shrink-0 px-7 py-4 border-t border-pgp-border">
      <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-3">
        Publish my key
      </p>
      <div class="flex items-center gap-2 flex-wrap">
        {#if privateKeys.length > 1}
          <select
            bind:value={publishFp}
            class="h-[34px] px-2 pr-6 rounded-btn text-[13px]
                   bg-pgp-field border border-pgp-field-border text-pgp-text-2
                   focus:outline-none focus:border-pgp-accent/50"
          >
            {#each privateKeys as key (key.Fingerprint)}
              <option value={key.Fingerprint}>{key.Email || key.PrimaryUID}</option>
            {/each}
          </select>
        {:else}
          <span class="text-[13px] text-pgp-text-2">{privateKeys[0].Email || privateKeys[0].PrimaryUID}</span>
        {/if}

        <select
          bind:value={publishServerURL}
          class="h-[34px] px-2 pr-6 rounded-btn text-[13px]
                 bg-pgp-field border border-pgp-field-border text-pgp-text-2
                 focus:outline-none focus:border-pgp-accent/50"
        >
          {#each keyservers as ks (ks.URL)}
            <option value={ks.URL}>{ks.Label}</option>
          {/each}
        </select>

        <button
          type="button"
          on:click={handlePublish}
          disabled={publishing || !publishFp || !publishServerURL}
          class="h-[34px] px-4 rounded-btn text-[13px] font-medium
                 bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                 hover:bg-pgp-fill disabled:opacity-40 disabled:cursor-not-allowed
                 transition-colors duration-75"
        >{publishing ? 'Publishing…' : 'Publish'}</button>
      </div>
      {#if publishError}
        <p class="mt-2 text-[13px] text-red-500">{publishError}</p>
      {:else if publishSuccess}
        <p class="mt-2 text-[13px] text-green-600 dark:text-green-400">Published successfully.</p>
      {/if}
    </div>
  {/if}

</div>
