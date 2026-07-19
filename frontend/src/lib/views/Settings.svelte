<script>
  import { onMount } from 'svelte'
  import {
    GetConfig, SaveConfig, OpenDirectoryDialog,
    ListKeyservers, AddCustomKeyserver, RemoveCustomKeyserver,
    GetPlatform, AutostartSupported, GetAutostart, SetAutostart,
    AppVersion, OpenExternal, InstallSupported, IsInstalled, InstallApp,
    ContextMenuSupported, ContextMenuInstalled, InstallContextMenu, UninstallContextMenu
  } from '../../../wailsjs/go/main/App'
  import { themeOverride } from '../../stores.js'
  import Toggle from '../Toggle.svelte'

  const GITHUB_URL = 'https://github.com/developaaah/pgp-manager'
  const SUPPORT_URL = 'https://github.com/developaaah/pgp-manager-test#support-the-project'

  let theme = 'dark'
  let cacheTTL = 15
  let keysDir = ''
  let platform = 'darwin'
  let version = ''

  let startInTray = true
  let clipMessages = true
  let clipPublicKeys = 'unknown'
  let clipPrivateKeys = 'unknown'
  let clipSignatures = true
  let autoCopy = true

  let autostartSupported = false
  let autostartOn = false
  let autostartError = ''

  let installSupported = false
  let installed = false
  let installing = false
  let installMsg = ''
  let installError = ''

  let ctxSupported = false
  let ctxInstalled = false
  let ctxBusy = false
  let ctxMsg = ''
  let ctxError = ''

  const keyModeOptions = [
    { value: 'off',     label: 'Off' },
    { value: 'unknown', label: 'New only' },
    { value: 'all',     label: 'All' },
  ]

  const themeOptions = [
    { value: 'light',  label: 'Light' },
    { value: 'dark',   label: 'Dark' },
    { value: 'system', label: 'System' },
  ]

  const ttlOptions = [
    { value: 0,   label: 'Never' },
    { value: 15,  label: '15 min' },
    { value: 60,  label: '1 hour' },
    { value: -1,  label: 'Until quit' },
  ]

  let saving = false
  let saveError = ''

  let pickingDir = false
  let dirError = ''

  let keyservers = []
  let newServerURL = ''
  let addingServer = false
  let addServerError = ''

  onMount(async () => {
    try {
      const [cfg, p] = await Promise.all([GetConfig(), GetPlatform()])
      theme = cfg.Theme || 'dark'
      cacheTTL = cfg.PassphraseCacheTTLMinutes ?? 15
      keysDir = cfg.KeysDir || ''
      platform = p
      startInTray = cfg.StartInTray ?? true
      clipMessages = cfg.ClipDetectMessages ?? true
      clipPublicKeys = cfg.ClipDetectPublicKeys || 'unknown'
      clipPrivateKeys = cfg.ClipDetectPrivateKeys || 'unknown'
      clipSignatures = cfg.ClipDetectSignatures ?? true
      autoCopy = cfg.AutoCopyResults ?? true
    } catch {}
    try {
      [autostartSupported, autostartOn] = await Promise.all([AutostartSupported(), GetAutostart()])
    } catch {}
    try {
      [version, installSupported, installed] =
        await Promise.all([AppVersion(), InstallSupported(), IsInstalled()])
    } catch {}
    try {
      [ctxSupported, ctxInstalled] =
        await Promise.all([ContextMenuSupported(), ContextMenuInstalled()])
    } catch {}
    await loadKeyservers()
  })

  async function loadKeyservers() {
    try { keyservers = await ListKeyservers() } catch {}
  }

  async function save() {
    saving = true
    saveError = ''
    try {
      await SaveConfig({
        Theme: theme,
        PassphraseCacheTTLMinutes: cacheTTL,
        KeysDir: keysDir,
        CustomKeyservers: keyservers.filter(k => !k.BuiltIn).map(k => k.URL),
        StartInTray: startInTray,
        ClipDetectMessages: clipMessages,
        ClipDetectPublicKeys: clipPublicKeys,
        ClipDetectPrivateKeys: clipPrivateKeys,
        ClipDetectSignatures: clipSignatures,
        AutoCopyResults: autoCopy,
      })
      themeOverride.set(theme)
    } catch (e) {
      saveError = String(e)
    } finally {
      saving = false
    }
  }

  async function pickDir() {
    pickingDir = true
    dirError = ''
    try {
      const chosen = await OpenDirectoryDialog()
      if (chosen) {
        keysDir = chosen
        await save()
      }
    } catch (e) {
      dirError = String(e)
    } finally {
      pickingDir = false
    }
  }

  async function onChange() {
    await save()
  }

  async function handleAddServer() {
    addServerError = ''
    const u = newServerURL.trim().replace(/\/$/, '')
    if (!u) return
    addingServer = true
    try {
      await AddCustomKeyserver(u)
      newServerURL = ''
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
  }

  async function onAutostartChange(e) {
    autostartError = ''
    const enable = e.detail
    try {
      await SetAutostart(enable)
    } catch (err) {
      autostartError = String(err)
      autostartOn = !enable
    }
  }

  async function handleContextMenuToggle() {
    ctxBusy = true
    ctxError = ''
    ctxMsg = ''
    try {
      if (ctxInstalled) {
        await UninstallContextMenu()
        ctxMsg = 'Context-menu entries removed.'
      } else {
        await InstallContextMenu()
        ctxMsg = platform === 'windows'
          ? 'Installed — entries appear in the Explorer right-click menu (Windows 11: under "Show more options").'
          : 'Installed — entries appear in the file manager\'s right-click menu (Nautilus may need a restart).'
      }
      ctxInstalled = await ContextMenuInstalled()
    } catch (e) {
      ctxError = String(e)
    } finally {
      ctxBusy = false
    }
  }

  async function handleInstall() {
    installing = true
    installError = ''
    installMsg = ''
    try {
      await InstallApp()
      installed = await IsInstalled()
      installMsg = platform === 'darwin'
        ? 'Copied to /Applications — quit and relaunch from there.'
        : 'Desktop entry created — the app now appears in your launcher.'
    } catch (e) {
      installError = String(e)
    } finally {
      installing = false
    }
  }
</script>

<div class="px-7 pt-6 pb-5 flex-shrink-0 border-b border-pgp-border">
  <h1 class="text-[20px] font-semibold tracking-[-0.025em] text-pgp-text mb-[4px]">Settings</h1>
  <p class="text-[13px] text-pgp-text-3 leading-[1.4]">
    Preferences are saved automatically
  </p>
</div>

<div class="flex-1 overflow-y-auto px-7 py-6 flex flex-col gap-7 w-full">

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      General
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 divide-y divide-pgp-border">

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Theme</p>
        </div>
        <div class="flex rounded-[7px] border border-pgp-border-strong overflow-hidden flex-shrink-0">
          {#each themeOptions as opt}
            <button
              type="button"
              on:click={() => { theme = opt.value; onChange() }}
              class="h-7 px-3 text-[12px] font-medium transition-colors duration-75
                     {theme === opt.value
                       ? 'bg-pgp-accent text-white'
                       : 'bg-pgp-fill-2 text-pgp-text-2 hover:bg-pgp-fill'}"
            >{opt.label}</button>
          {/each}
        </div>
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Auto-Copy Results</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Copy the result of Encrypt and Sign to the clipboard automatically
          </p>
        </div>
        <Toggle bind:checked={autoCopy} on:change={onChange} />
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Passphrase Cache</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            How long unlocked passphrases stay in memory
          </p>
        </div>
        <div class="flex rounded-[7px] border border-pgp-border-strong overflow-hidden flex-shrink-0">
          {#each ttlOptions as opt}
            <button
              type="button"
              on:click={() => { cacheTTL = opt.value; onChange() }}
              class="h-7 px-3 text-[12px] font-medium transition-colors duration-75
                     {cacheTTL === opt.value
                       ? 'bg-pgp-accent text-white'
                       : 'bg-pgp-fill-2 text-pgp-text-2 hover:bg-pgp-fill'}"
            >{opt.label}</button>
          {/each}
        </div>
      </div>

    </div>
  </section>

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      Startup
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 divide-y divide-pgp-border">

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Run in System Tray</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Start hidden in the tray instead of opening the window
          </p>
        </div>
        <Toggle bind:checked={startInTray} on:change={onChange} />
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Launch at Login</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            {#if autostartSupported}
              Add PGP Manager to your login items
            {:else if platform === 'darwin'}
              Requires macOS 13+ and an installed app bundle
            {:else}
              Not available on this platform
            {/if}
          </p>
        </div>
        <Toggle bind:checked={autostartOn} disabled={!autostartSupported} on:change={onAutostartChange} />
      </div>

    </div>
    {#if autostartError}
      <p class="mt-1.5 px-1 text-[12px] text-red-500">{autostartError}</p>
    {/if}
  </section>

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      Clipboard Auto-Detect
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 divide-y divide-pgp-border">

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Messages</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Open the window and decrypt when a matching private key exists
          </p>
        </div>
        <Toggle bind:checked={clipMessages} on:change={onChange} />
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Public Keys</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Open the import dialog for detected public keys
          </p>
        </div>
        <div class="flex rounded-[7px] border border-pgp-border-strong overflow-hidden flex-shrink-0">
          {#each keyModeOptions as opt}
            <button
              type="button"
              on:click={() => { clipPublicKeys = opt.value; onChange() }}
              class="h-7 px-3 text-[12px] font-medium transition-colors duration-75
                     {clipPublicKeys === opt.value
                       ? 'bg-pgp-accent text-white'
                       : 'bg-pgp-fill-2 text-pgp-text-2 hover:bg-pgp-fill'}"
            >{opt.label}</button>
          {/each}
        </div>
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Private Keys</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Open the import dialog for detected private keys
          </p>
        </div>
        <div class="flex rounded-[7px] border border-pgp-border-strong overflow-hidden flex-shrink-0">
          {#each keyModeOptions as opt}
            <button
              type="button"
              on:click={() => { clipPrivateKeys = opt.value; onChange() }}
              class="h-7 px-3 text-[12px] font-medium transition-colors duration-75
                     {clipPrivateKeys === opt.value
                       ? 'bg-pgp-accent text-white'
                       : 'bg-pgp-fill-2 text-pgp-text-2 hover:bg-pgp-fill'}"
            >{opt.label}</button>
          {/each}
        </div>
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Signatures</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            Verify signed messages silently — the result arrives as a notification
          </p>
        </div>
        <Toggle bind:checked={clipSignatures} on:change={onChange} />
      </div>

    </div>
  </section>

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      Key Storage
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 py-3">
      <p class="text-[12px] text-pgp-text-4 leading-[1.45] mb-2">
        Where your PGP key files are stored. By default keys live next to the
        config file (~/.pgp), so the whole directory is portable.
      </p>
      <div class="flex items-center gap-3">
        <input
          type="text"
          bind:value={keysDir}
          on:change={onChange}
          placeholder="Default (~/.pgp)"
          spellcheck="false"
          class="flex-1 min-w-0 h-9 px-3 rounded-field text-[12px] font-mono
                 bg-pgp-field border border-pgp-field-border text-pgp-text-2
                 placeholder:text-pgp-text-4
                 focus:outline-none focus:border-pgp-accent/50 transition-colors"
        />
        <button
          type="button"
          on:click={pickDir}
          disabled={pickingDir}
          class="flex-shrink-0 h-9 px-4 rounded-btn text-[13px] font-medium
                 bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                 hover:bg-pgp-fill disabled:opacity-50
                 transition-colors duration-75"
          title="Browse…"
        >{pickingDir ? '…' : 'Browse…'}</button>
      </div>
      {#if dirError}
        <p class="mt-2 text-[12px] text-red-500">{dirError}</p>
      {/if}
    </div>
  </section>

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      Keyservers
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 py-3">
      <p class="text-[12px] text-pgp-text-4 leading-[1.45] mb-3">
        Built-in servers are always available. You can add private or custom servers below.
      </p>

      <div class="flex flex-col gap-[3px] mb-3">
        {#each keyservers.filter(k => k.BuiltIn) as ks (ks.URL)}
          <div class="flex items-center gap-3 h-8 px-3 rounded-[7px]
                      bg-pgp-fill border border-pgp-border">
            <span class="text-[12px] text-pgp-text-2 flex-1 truncate">{ks.Label}</span>
            <span class="text-[10px] px-[6px] py-[1px] rounded-full
                         bg-pgp-fill-2 border border-pgp-border text-pgp-text-4">built-in</span>
          </div>
        {/each}
      </div>

      {#each keyservers.filter(k => !k.BuiltIn) as ks (ks.URL)}
        <div class="flex items-center gap-2 mb-[3px]">
          <div class="flex items-center gap-3 h-8 px-3 rounded-[7px] flex-1
                      bg-pgp-fill border border-pgp-border">
            <span class="text-[12px] font-mono text-pgp-text-2 flex-1 truncate">{ks.URL}</span>
            <span class="text-[10px] px-[6px] py-[1px] rounded-full
                         bg-pgp-accent/10 border border-pgp-accent/20 text-pgp-accent">custom</span>
          </div>
          <button
            type="button"
            on:click={() => handleRemoveServer(ks.URL)}
            class="flex-shrink-0 w-7 h-7 rounded-[7px] flex items-center justify-center
                   bg-pgp-fill border border-pgp-border text-pgp-text-3
                   hover:text-red-500 hover:border-red-500/30 transition-colors duration-75"
            title="Remove"
          >
            <svg aria-hidden="true" class="w-3 h-3" viewBox="0 0 12 12" fill="none">
              <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
            </svg>
          </button>
        </div>
      {/each}

      <div class="flex items-center gap-2 mt-2">
        <input
          bind:value={newServerURL}
          on:keydown={onAddKeydown}
          type="url"
          placeholder="https://your-private-keyserver.example.com"
          spellcheck="false"
          class="flex-1 h-8 px-3 rounded-[7px] text-[12px] font-mono
                 bg-pgp-field border border-pgp-field-border text-pgp-text-2
                 placeholder:text-pgp-text-4
                 focus:outline-none focus:border-pgp-accent/50 transition-colors"
        />
        <button
          type="button"
          on:click={handleAddServer}
          disabled={addingServer || !newServerURL.trim()}
          class="flex-shrink-0 h-8 px-4 rounded-[7px] text-[12px] font-medium
                 bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                 hover:bg-pgp-fill disabled:opacity-40 transition-colors duration-75"
        >{addingServer ? 'Adding…' : 'Add'}</button>
      </div>
      {#if addServerError}
        <p class="mt-1.5 text-[12px] text-red-500">{addServerError}</p>
      {/if}
    </div>
  </section>

  {#if installSupported || ctxSupported || platform === 'darwin'}
    <section>
      <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
        System
      </p>
      <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 divide-y divide-pgp-border">

        {#if installSupported}
          <div class="flex items-center justify-between gap-4 py-3">
            <div class="min-w-0">
              <p class="text-[13px] text-pgp-text-2">Install System-Wide</p>
              <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
                {#if platform === 'darwin'}
                  Copy PGP Manager to /Applications
                {:else}
                  Create a desktop entry and icon so the app appears in your launcher
                {/if}
              </p>
            </div>
            <button
              type="button"
              on:click={handleInstall}
              disabled={installing || installed}
              class="flex-shrink-0 h-8 px-4 rounded-btn text-[12px] font-medium
                     bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                     hover:bg-pgp-fill disabled:opacity-50 transition-colors duration-75"
            >{installed ? 'Installed' : installing ? 'Installing…' : 'Install'}</button>
          </div>
          {#if installMsg}
            <p class="py-2 text-[12px] text-pgp-text-3">{installMsg}</p>
          {/if}
          {#if installError}
            <p class="py-2 text-[12px] text-red-500">{installError}</p>
          {/if}
        {/if}

        {#if ctxSupported}
          <div class="flex items-center justify-between gap-4 py-3">
            <div class="min-w-0">
              <p class="text-[13px] text-pgp-text-2">File Manager Context Menu</p>
              <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
                {#if platform === 'windows'}
                  Add "PGP Manager" actions to the Explorer right-click menu
                {:else}
                  Add PGP actions to Nautilus, Dolphin, and Nemo right-click menus
                {/if}
              </p>
            </div>
            <button
              type="button"
              on:click={handleContextMenuToggle}
              disabled={ctxBusy}
              class="flex-shrink-0 h-8 px-4 rounded-btn text-[12px] font-medium
                     bg-pgp-fill-2 border border-pgp-border-strong text-pgp-text-2
                     hover:bg-pgp-fill disabled:opacity-50 transition-colors duration-75"
            >{ctxBusy ? 'Working…' : ctxInstalled ? 'Remove' : 'Install'}</button>
          </div>
          {#if ctxMsg}
            <p class="py-2 text-[12px] text-pgp-text-3">{ctxMsg}</p>
          {/if}
          {#if ctxError}
            <p class="py-2 text-[12px] text-red-500">{ctxError}</p>
          {/if}
        {/if}

        {#if platform === 'darwin'}
          <div class="py-3">
            <p class="text-[13px] text-pgp-text-2">macOS Services</p>
            <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
              Select text or files, right-click, and choose a "PGP:" entry from
              the Services submenu — the actions behave exactly like the tray
              menu. PGP Manager also lives in the menu bar with actions for the
              current clipboard content.
            </p>
          </div>
        {/if}

      </div>
    </section>
  {/if}

  <section>
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      Support
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4">
      <div class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-[13px] text-pgp-text-2">Buy me a coffee</p>
          <p class="text-[12px] text-pgp-text-4 leading-[1.45] mt-[1px]">
            If PGP Manager is useful to you, you can support its development
          </p>
        </div>
        <button
          type="button"
          on:click={() => OpenExternal(SUPPORT_URL)}
          class="flex-shrink-0 h-8 px-4 rounded-btn text-[12px] font-medium
                 bg-pgp-accent text-white hover:opacity-90 transition-opacity duration-75"
        >Buy me a coffee</button>
      </div>
    </div>
  </section>

  <section class="pb-2">
    <p class="text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-2 px-1">
      About
    </p>
    <div class="rounded-[10px] border border-pgp-border bg-pgp-fill/40 px-4 divide-y divide-pgp-border">
      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-[13px] text-pgp-text-3">Application</span>
        <span class="text-[13px] text-pgp-text-2">PGP Manager</span>
      </div>
      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-[13px] text-pgp-text-3">Version</span>
        <span class="text-[13px] text-pgp-text-2 font-mono">{version || 'dev'}</span>
      </div>
      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-[13px] text-pgp-text-3">GitHub</span>
        <button
          type="button"
          on:click={() => OpenExternal(GITHUB_URL)}
          class="text-[13px] text-pgp-accent hover:underline"
        >developaaah/pgp-manager</button>
      </div>
    </div>
  </section>

  {#if saveError}
    <p class="text-[13px] text-red-500 -mt-4 px-1">{saveError}</p>
  {/if}

</div>
