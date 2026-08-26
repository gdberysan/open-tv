// Barril: reexporta desde idioma.svelte.ts, el único fichero que declara la
// runa $state (Svelte 5 solo la reconoce en .svelte / .svelte.ts). Así el
// resto del cliente sigue importando `from './i18n'` sin saber que por dentro
// cambió de un store de svelte/store a una runa.
export { idioma, t } from './idioma.svelte'
export type { Idioma } from './idioma.svelte'
