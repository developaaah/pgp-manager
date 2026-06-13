<script>
  export let label = ''
  export let icon = null
  export let active = false
  export let onClick = undefined
</script>

<div class="mx-2">
  <button
    type="button"
    on:click={onClick}
    class="
      w-full flex items-center gap-[10px]
      py-[8px] pl-[13px] pr-[10px]
      text-[14px] rounded-btn
      transition-colors duration-100
      {active
        ? 'bg-pgp-nav-active-bg text-pgp-nav-active-text'
        : 'text-pgp-nav-text hover:bg-pgp-nav-hover-bg hover:text-pgp-nav-hover-text'
      }
    "
    aria-current={active ? 'page' : undefined}
    aria-label={label}
  >
    {#if icon}
      <span class="w-4 h-4 shrink-0 flex items-center justify-center">
        <svelte:component this={icon} size={16} />
      </span>
    {/if}
    <span>{label}</span>
  </button>
</div>
