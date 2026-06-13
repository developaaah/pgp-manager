<script>
  import { createEventDispatcher } from 'svelte'

  export let open = false
  export let action = ''
  export let output = ''
  export let error = ''

  const dispatch = createEventDispatcher()

  let copied = false
  let copyTimer = null

  const titleMap = {
    'decrypt-text':  'Decrypted Text',
    'encrypt-text':  'Encrypted Text',
    'sign-text':     'Signed Text',
    'verify-text':   'Signature Verified',
    'import-key':    'Key Import',
    'encrypt-file':  'File Encryption',
    'decrypt-file':  'File Decryption',
    'sign-file':     'File Signing',
  }

  $: title = titleMap[action] || 'Result'
  $: hasOutput = output && output.length > 0

  function close() {
    open = false
    copied = false
    clearTimeout(copyTimer)
    dispatch('close')
  }

  async function copyOutput() {
    try {
      await navigator.clipboard.writeText(output)
      copied = true
      clearTimeout(copyTimer)
      copyTimer = setTimeout(() => { copied = false }, 2000)
    } catch {}
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
  }

  function dialogAction(node) {
    node.showModal()
    const onNativeClose = () => close()
    node.addEventListener('close', onNativeClose)
    return {
      destroy() {
        node.removeEventListener('close', onNativeClose)
        if (node.open) node.close()
      }
    }
  }

  $: if (open) { copied = false }
</script>

{#if open}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[520px] max-w-[90vw]"
>
  <div class="px-6 py-5">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-4">
      {title}
    </h2>

    {#if error}
      <div class="rounded-[8px] bg-red-500/10 border border-red-500/30 px-4 py-3 mb-4">
        <p class="text-[12.5px] text-red-400 whitespace-pre-wrap break-words">{error}</p>
      </div>
    {/if}

    {#if hasOutput}
      <pre
        class="rounded-[8px] bg-pgp-field border border-pgp-field-border
               px-4 py-3 text-[12px] font-mono text-pgp-text leading-relaxed
               overflow-auto max-h-[320px] whitespace-pre-wrap break-all"
      >{output}</pre>
    {/if}
  </div>

  <div class="flex justify-end gap-2 px-6 pb-5">
    {#if hasOutput}
      <button
        type="button"
        on:click={copyOutput}
        aria-label="Copy output to clipboard"
        class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
               bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
               hover:bg-pgp-fill transition-colors duration-75"
      >{copied ? 'Copied!' : 'Copy'}</button>
    {/if}
    <button
      type="button"
      on:click={close}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             transition-opacity duration-75"
    >Close</button>
  </div>
</dialog>
{/if}
