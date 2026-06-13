<script>
  import { createEventDispatcher } from 'svelte'
  import { GenerateKey } from '../../wailsjs/go/main/App'

  export let open = false

  const dispatch = createEventDispatcher()

  let name = ''
  let email = ''
  let passphrase = ''
  let passphraseConfirm = ''
  let keyAlgo = 'rsa'
  let rsaBits = '3072'
  let expiryYears = '0'
  let generating = false
  let error = ''
  let success = false

  $: keyType = keyAlgo === 'ed25519' ? 'ed25519' : `rsa${rsaBits}`
  $: mismatch = passphrase && passphraseConfirm && passphrase !== passphraseConfirm
  $: canGenerate = name.trim() && email.trim() && !mismatch && !generating

  $: if (open) {
    error = ''; success = false
    name = ''; email = ''; passphrase = ''; passphraseConfirm = ''
    keyAlgo = 'rsa'; rsaBits = '3072'; expiryYears = '0'
  }

  async function handleGenerate() {
    if (!canGenerate) return
    generating = true
    error = ''
    try {
      await GenerateKey(name.trim(), email.trim(), passphrase, keyType, expiryYears)
      success = true
      await new Promise(r => setTimeout(r, 1200))
      dispatch('generated')
      close()
    } catch (e) {
      error = String(e)
    } finally {
      generating = false
    }
  }

  function close() {
    open = false
    dispatch('close')
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') { e.preventDefault(); close() }
    if (e.key === 'Enter' && !e.isComposing && canGenerate) handleGenerate()
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
</script>

{#if open}
<dialog
  use:dialogAction
  on:keydown={handleKeydown}
  class="rounded-[12px] overflow-hidden
         bg-pgp-elevated border border-pgp-border
         shadow-[0_8px_32px_rgba(0,0,0,0.5)]
         w-[420px]"
>
  <div class="px-6 pt-6 pb-2">
    <h2 class="text-[15px] font-semibold text-pgp-text tracking-[-0.01em] mb-[3px]">
      Generate new key pair
    </h2>
    <p class="text-[12px] text-pgp-text-3 mb-5 leading-relaxed">
      The private key is stored locally and optionally protected with a passphrase.
    </p>

    <div class="mb-3">
      <label for="gen-name"
        class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[5px]">
        Name
      </label>
      <input
        id="gen-name"
        bind:value={name}
        type="text"
        placeholder="Alice"
        autocomplete="name"
        autofocus
        class="w-full h-9 px-3 rounded-[7px] text-[13px]
               bg-pgp-field border border-pgp-field-border
               text-pgp-text placeholder:text-pgp-text-4
               focus:outline-none focus:border-pgp-accent/60"
      />
    </div>

    <div class="mb-3">
      <label for="gen-email"
        class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[5px]">
        Email
      </label>
      <input
        id="gen-email"
        bind:value={email}
        type="email"
        placeholder="alice@example.com"
        autocomplete="email"
        class="w-full h-9 px-3 rounded-[7px] text-[13px]
               bg-pgp-field border border-pgp-field-border
               text-pgp-text placeholder:text-pgp-text-4
               focus:outline-none focus:border-pgp-accent/60"
      />
    </div>

    <div class="mb-3">
      <label for="gen-pass"
        class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[5px]">
        Passphrase <span class="normal-case font-normal">(optional)</span>
      </label>
      <input
        id="gen-pass"
        bind:value={passphrase}
        type="password"
        placeholder="Protects the private key…"
        autocomplete="new-password"
        class="w-full h-9 px-3 rounded-[7px] text-[13px]
               bg-pgp-field border border-pgp-field-border
               text-pgp-text placeholder:text-pgp-text-4
               focus:outline-none focus:border-pgp-accent/60"
      />
    </div>

    {#if passphrase}
      <div class="mb-3">
        <label for="gen-pass2"
          class="block text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[5px]">
          Confirm passphrase
        </label>
        <input
          id="gen-pass2"
          bind:value={passphraseConfirm}
          type="password"
          placeholder="Repeat passphrase…"
          autocomplete="new-password"
          class="w-full h-9 px-3 rounded-[7px] text-[13px]
                 bg-pgp-field border
                 {mismatch ? 'border-red-500/50' : 'border-pgp-field-border'}
                 text-pgp-text placeholder:text-pgp-text-4
                 focus:outline-none focus:border-pgp-accent/60"
        />
        {#if mismatch}
          <p class="mt-1 text-[12px] text-red-500">Passphrases do not match.</p>
        {/if}
      </div>
    {/if}

    <div class="flex items-center gap-3 mt-4 mb-3">
      <div class="h-px flex-1 bg-pgp-border"></div>
      <span class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4">
        Key options
      </span>
      <div class="h-px flex-1 bg-pgp-border"></div>
    </div>

    <div class="mb-3">
      <p id="gen-algo-label"
        class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]">
        Algorithm
      </p>
      <div role="radiogroup" aria-labelledby="gen-algo-label"
           class="flex rounded-[7px] overflow-hidden border border-pgp-border-strong text-[12px]">
        {#each [['rsa', 'RSA'], ['ed25519', 'Ed25519 / Curve25519']] as [val, label]}
          <button
            type="button"
            role="radio"
            aria-checked={keyAlgo === val}
            on:click={() => (keyAlgo = val)}
            class="flex-1 h-8 px-3 transition-colors duration-75 text-center
                   {keyAlgo === val
                     ? 'bg-pgp-accent text-white'
                     : 'bg-pgp-fill text-pgp-text-3 hover:bg-pgp-fill-2 hover:text-pgp-text-2'}"
          >{label}</button>
        {/each}
      </div>
    </div>

    {#if keyAlgo === 'rsa'}
      <div class="mb-3">
        <p id="gen-size-label"
          class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]">
          Key size
        </p>
        <div role="radiogroup" aria-labelledby="gen-size-label"
             class="flex rounded-[7px] overflow-hidden border border-pgp-border-strong text-[12px]">
          {#each [['2048', '2048 bit'], ['3072', '3072 bit'], ['4096', '4096 bit']] as [val, label]}
            <button
              type="button"
              role="radio"
              aria-checked={rsaBits === val}
              on:click={() => (rsaBits = val)}
              class="flex-1 h-8 px-3 transition-colors duration-75 text-center
                     {rsaBits === val
                       ? 'bg-pgp-accent text-white'
                       : 'bg-pgp-fill text-pgp-text-3 hover:bg-pgp-fill-2 hover:text-pgp-text-2'}"
            >{label}</button>
          {/each}
        </div>
      </div>
    {/if}

    <div class="mb-1">
      <p id="gen-expiry-label"
        class="text-[10px] font-bold uppercase tracking-[0.07em] text-pgp-text-4 mb-[6px]">
        Expires
      </p>
      <div role="radiogroup" aria-labelledby="gen-expiry-label"
           class="flex rounded-[7px] overflow-hidden border border-pgp-border-strong text-[12px]">
        {#each [['0','Never'],['1','1 yr'],['2','2 yrs'],['3','3 yrs'],['5','5 yrs']] as [val, label]}
          <button
            type="button"
            role="radio"
            aria-checked={expiryYears === val}
            on:click={() => (expiryYears = val)}
            class="flex-1 h-8 px-1 transition-colors duration-75 text-center
                   {expiryYears === val
                     ? 'bg-pgp-accent text-white'
                     : 'bg-pgp-fill text-pgp-text-3 hover:bg-pgp-fill-2 hover:text-pgp-text-2'}"
          >{label}</button>
        {/each}
      </div>
    </div>

    {#if error}
      <p class="text-[13px] text-red-500 mt-2 mb-1 leading-[1.4]">{error}</p>
    {/if}
    {#if success}
      <p class="text-[13px] text-green-500 mt-2 mb-1">Key pair generated successfully.</p>
    {/if}
  </div>

  <div class="flex items-center justify-between px-6 py-4">
    <button
      type="button"
      on:click={close}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium
             bg-pgp-fill-2 border border-pgp-border text-pgp-text-2
             hover:bg-pgp-fill transition-colors duration-75"
    >Cancel</button>
    <button
      type="button"
      on:click={handleGenerate}
      disabled={!canGenerate || success}
      class="h-7 px-[14px] rounded-[7px] text-[12.5px] font-medium text-white
             bg-pgp-accent hover:opacity-90
             disabled:opacity-40 disabled:cursor-not-allowed
             transition-opacity duration-75"
    >
      {generating ? 'Generating…' : success ? 'Generated' : 'Generate key pair'}
    </button>
  </div>
</dialog>
{/if}
