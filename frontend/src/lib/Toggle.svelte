<script>
  import { createEventDispatcher } from 'svelte'

  export let checked = false
  export let disabled = false

  const dispatch = createEventDispatcher()

  function toggle() {
    if (disabled) return
    checked = !checked
    dispatch('change', checked)
  }
</script>

<button
  type="button"
  role="switch"
  aria-checked={checked}
  {disabled}
  on:click={toggle}
  class="relative w-[38px] h-[23px] rounded-full flex-shrink-0
         transition-colors duration-150 ease-out
         {checked ? 'bg-pgp-accent' : 'bg-pgp-fill-2 ring-1 ring-inset ring-pgp-border-strong'}
         disabled:opacity-40 disabled:cursor-not-allowed"
>
  <span
    class="absolute top-[2px] w-[19px] h-[19px] rounded-full bg-white
           shadow-[0_1px_3px_rgba(0,0,0,0.3)]
           transition-[left] duration-150 ease-out
           {checked ? 'left-[17px]' : 'left-[2px]'}"
  ></span>
</button>
