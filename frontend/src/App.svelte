<script>
  import { onMount } from 'svelte'
  import TextEncrypt from './lib/views/TextEncrypt.svelte'
  import FileEncrypt from './lib/views/FileEncrypt.svelte'
  import MyKeys from './lib/views/MyKeys.svelte'
  import Keyserver from './lib/views/Keyserver.svelte'
  import Settings from './lib/views/Settings.svelte'
  import Sidebar from './lib/Sidebar.svelte'
  import ImportKeyModal from './modals/ImportKeyModal.svelte'
  import ActionResultModal from './modals/ActionResultModal.svelte'
  import PassphraseModal from './modals/PassphraseModal.svelte'
  import SetupModal from './modals/SetupModal.svelte'
  import { GetPlatform, GetConfig, NeedsSetup, GetAvailableUpdate, FrontendReady, ProvidePassphrase } from '../wailsjs/go/main/App'
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
  import { themeOverride, pendingImportArmored, pendingDecryptText, pendingSignText, pendingEncryptText, pendingEncryptFiles, availableUpdate } from './stores.js'

  let activeView = 'text'
  let platform = 'darwin'
  let focused = true

  let needsSetup = false
  let setupChecked = false

  let modalOpen = false
  let modalArmored = ''

  let actionResultOpen = false
  let actionResultAction = ''
  let actionResultOutput = ''
  let actionResultError = ''

  // Global passphrase prompt for tray/context actions — the backend blocks
  // until ProvidePassphrase answers the request id.
  let ppReqOpen = false
  let ppReqId = 0
  let ppReqLabel = ''
  let ppReqAnswered = false

  function answerPassphrase(passphrase, cancelled) {
    if (ppReqAnswered) return
    ppReqAnswered = true
    ProvidePassphrase(ppReqId, passphrase, cancelled).catch(() => {})
  }

  const mq = typeof window !== 'undefined'
    ? window.matchMedia('(prefers-color-scheme: dark)')
    : null
  let systemDark = mq ? mq.matches : false
  mq?.addEventListener('change', e => { systemDark = e.matches })

  $: isDark = $themeOverride === 'system'
    ? systemDark
    : $themeOverride === 'dark'

  $: if ($pendingImportArmored !== null) {
    modalOpen = true
    modalArmored = $pendingImportArmored
    pendingImportArmored.set(null)
  }

  onMount(async () => {
    try {
      [platform, needsSetup] = await Promise.all([GetPlatform(), NeedsSetup()])
      const cfg = await GetConfig()
      themeOverride.set(cfg.Theme || 'dark')
    } catch {}
    setupChecked = true

    const onFocus = () => (focused = true)
    const onBlur = () => (focused = false)
    window.addEventListener('focus', onFocus)
    window.addEventListener('blur', onBlur)

    GetAvailableUpdate().then(tag => { if (tag) availableUpdate.set(tag) }).catch(() => {})
    EventsOn('update:available', (tag) => availableUpdate.set(tag))

    EventsOn('notification:imported', () => {
      modalOpen = false
      modalArmored = ''
      pendingImportArmored.set(null)
    })

    EventsOn('action:result', (data) => {
      actionResultAction = data.action || ''
      actionResultOutput = data.output || ''
      actionResultError = data.error || ''
      actionResultOpen = true
    })

    EventsOn('clipboard-key-detected', (armored) => {
      pendingImportArmored.set(armored)
    })

    EventsOn('decrypt-text-requested', (armored) => {
      activeView = 'text'
      pendingDecryptText.set(armored)
    })

    EventsOn('sign-text-requested', (text) => {
      activeView = 'text'
      pendingSignText.set(text)
    })

    EventsOn('encrypt-text-requested', (text) => {
      activeView = 'text'
      pendingEncryptText.set(text)
    })

    EventsOn('encrypt-file-requested', (paths) => {
      activeView = 'files'
      const arr = Array.isArray(paths) ? paths : [paths]
      // Merge with a not-yet-consumed request instead of replacing it — a
      // second event must never silently drop the first one's files.
      pendingEncryptFiles.update(prev => (prev ? [...prev, ...arr] : arr))
    })

    EventsOn('passphrase-requested', (req) => {
      // A newer request supersedes an unanswered one — cancel the old
      // waiter so its action goroutine does not hang.
      if (ppReqOpen && !ppReqAnswered) {
        ProvidePassphrase(ppReqId, '', true).catch(() => {})
      }
      ppReqId = req.id
      ppReqLabel = req.keyLabel || ''
      ppReqAnswered = false
      ppReqOpen = true
    })

    // Backend action events are gated until the listeners above exist.
    FrontendReady().catch(() => {})

    return () => {
      window.removeEventListener('focus', onFocus)
      window.removeEventListener('blur', onBlur)
      EventsOff('notification:imported')
      EventsOff('action:result')
      EventsOff('clipboard-key-detected')
      EventsOff('decrypt-text-requested')
      EventsOff('sign-text-requested')
      EventsOff('encrypt-text-requested')
      EventsOff('encrypt-file-requested')
      EventsOff('passphrase-requested')
      EventsOff('update:available')
    }
  })

  $: isMac = platform === 'darwin'

  function handleKeydown(e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'w') e.preventDefault()
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="h-screen w-screen flex flex-col overflow-hidden font-sans {isDark ? 'dark' : ''}">

  {#if isMac}
    <div
      class="drag-region relative flex-shrink-0 bg-pgp-titlebar border-b border-pgp-border select-none"
      style="height: 38px;"
    >
      <div class="no-drag absolute left-0 top-0 h-full w-[78px]"></div>
      <div class="drag-region absolute inset-0 flex items-center justify-center">
        <span
          class="text-[14px] font-semibold tracking-[-0.01em] text-white pointer-events-none
                 transition-opacity duration-300
                 {focused ? 'opacity-100' : 'opacity-30'}"
        >
          PGP Manager
        </span>
      </div>
    </div>
  {/if}

  <div class="flex flex-1 overflow-hidden min-h-0">
    {#if setupChecked && !needsSetup}
      <Sidebar bind:activeView />
      <main class="flex-1 flex flex-col overflow-hidden bg-pgp-window text-pgp-text">
        {#if activeView === 'text'}
          <TextEncrypt />
        {:else if activeView === 'files'}
          <FileEncrypt />
        {:else if activeView === 'keys'}
          <MyKeys />
        {:else if activeView === 'keyserver'}
          <Keyserver />
        {:else if activeView === 'settings'}
          <Settings />
        {/if}
      </main>
    {:else}
      <div class="flex-1 bg-pgp-window"></div>
    {/if}
  </div>

  {#if setupChecked && needsSetup}
    <SetupModal
      on:done={async () => {
        needsSetup = false
        try {
          const cfg = await GetConfig()
          themeOverride.set(cfg.Theme || 'dark')
        } catch {}
      }}
    />
  {/if}

  {#if modalOpen}
    <ImportKeyModal
      bind:open={modalOpen}
      armored={modalArmored}
      on:close={() => {
        modalOpen = false
        modalArmored = ''
        pendingImportArmored.set(null)
      }}
    />
  {/if}

  {#if actionResultOpen}
    <ActionResultModal
      bind:open={actionResultOpen}
      action={actionResultAction}
      output={actionResultOutput}
      error={actionResultError}
      on:close={() => { actionResultOpen = false }}
    />
  {/if}

  <PassphraseModal
    bind:open={ppReqOpen}
    keyLabel={ppReqLabel}
    confirmLabel="Unlock"
    allowEmpty={true}
    on:confirm={(e) => { answerPassphrase(e.detail.passphrase, false); ppReqOpen = false }}
    on:close={() => answerPassphrase('', true)}
  />

</div>
