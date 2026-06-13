<script>
  import SidebarItem from './SidebarItem.svelte'
  import Password from 'carbon-icons-svelte/lib/Password.svelte'
  import Document from 'carbon-icons-svelte/lib/Document.svelte'
  import Encryption from 'carbon-icons-svelte/lib/Encryption.svelte'
  import Earth from 'carbon-icons-svelte/lib/Earth.svelte'
  import Settings from 'carbon-icons-svelte/lib/Settings.svelte'
  import ArrowUp from 'carbon-icons-svelte/lib/ArrowUp.svelte'
  import { OpenExternal } from '../../wailsjs/go/main/App'
  import { availableUpdate } from '../stores.js'

  export let activeView = 'text'

  const RELEASES_URL = 'https://github.com/developaaah/pgp-manager/releases/latest'

  const operations = [
    { id: 'text',  label: 'Text',  icon: Password  },
    { id: 'files', label: 'Files', icon: Document  },
  ]

  const keysGroup = [
    { id: 'keys',      label: 'My Keys',   icon: Encryption },
    { id: 'keyserver', label: 'Keyserver', icon: Earth      },
  ]
</script>

<aside
  class="w-[220px] flex-shrink-0 flex flex-col pt-[18px] pb-3
         bg-pgp-sidebar border-r border-pgp-border"
>
  <nav class="flex flex-col" aria-label="Main navigation">

    <p class="px-4 pb-[6px] text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-nav-label">
      Operations
    </p>
    {#each operations as item}
      <SidebarItem
        label={item.label}
        icon={item.icon}
        active={activeView === item.id}
        onClick={() => (activeView = item.id)}
      />
    {/each}

    <p class="px-4 pb-[6px] mt-[14px] text-[11px] font-bold uppercase tracking-[0.07em] text-pgp-nav-label">
      Keys
    </p>
    {#each keysGroup as item}
      <SidebarItem
        label={item.label}
        icon={item.icon}
        active={activeView === item.id}
        onClick={() => (activeView = item.id)}
      />
    {/each}

  </nav>

  <div class="flex-1"></div>

  {#if $availableUpdate}
    <div class="mx-2 mb-1">
      <button
        type="button"
        on:click={() => OpenExternal(RELEASES_URL)}
        class="w-full flex items-center gap-[10px] py-[8px] pl-[13px] pr-[10px]
               text-[13px] rounded-btn text-left
               bg-amber-500/15 text-amber-400
               hover:bg-amber-500/25 transition-colors duration-100"
        title="Open GitHub releases"
      >
        <span class="w-4 h-4 shrink-0 flex items-center justify-center">
          <ArrowUp size={16} />
        </span>
        <span class="leading-tight">
          <span class="block font-medium">Update available</span>
          <span class="block text-[11px] opacity-70">{$availableUpdate}</span>
        </span>
      </button>
    </div>
  {/if}

  <nav aria-label="Settings navigation">
    <SidebarItem
      label="Settings"
      icon={Settings}
      active={activeView === 'settings'}
      onClick={() => (activeView = 'settings')}
    />
  </nav>
</aside>
