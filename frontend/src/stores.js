import { writable } from 'svelte/store'

export const themeOverride = writable('dark')

export const pendingImportArmored = writable(null)

export const pendingClipboardMessage = writable(null)

export const pendingEncryptText = writable(null)

export const pendingEncryptFiles = writable(null)

export const availableUpdate = writable('')
