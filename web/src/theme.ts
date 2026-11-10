// Theme control: auto (follow system) / light / dark.
//
// The resolved theme is written to <html data-theme="light|dark">, and all
// colors in style.css are driven by CSS variables under that attribute.
import { computed, ref, watch } from 'vue'

export type ThemeMode = 'auto' | 'light' | 'dark'

const STORAGE_KEY = 'gofs.theme'

function detectMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'auto' || saved === 'light' || saved === 'dark') return saved
  } catch { /* ignore */ }
  return 'auto'
}

export const themeMode = ref<ThemeMode>(detectMode())

const systemDark = ref(false)

let media: MediaQueryList | null = null

function readSystem() {
  if (typeof window !== 'undefined' && window.matchMedia) {
    media = window.matchMedia('(prefers-color-scheme: dark)')
    systemDark.value = media.matches
    media.addEventListener('change', (e) => {
      systemDark.value = e.matches
    })
  }
}

/** The theme actually applied to the document. */
export const resolvedTheme = computed<'light' | 'dark'>(() => {
  if (themeMode.value === 'auto') return systemDark.value ? 'dark' : 'light'
  return themeMode.value
})

function apply(theme: 'light' | 'dark') {
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

watch(resolvedTheme, apply, { immediate: true })

export function setThemeMode(mode: ThemeMode): void {
  themeMode.value = mode
  try {
    localStorage.setItem(STORAGE_KEY, mode)
  } catch { /* ignore */ }
}

/** Cycle auto -> light -> dark -> auto. */
export function cycleTheme(): void {
  const next: Record<ThemeMode, ThemeMode> = {
    auto: 'light',
    light: 'dark',
    dark: 'auto',
  }
  setThemeMode(next[themeMode.value])
}

/** Call once at startup to wire up the system preference listener. */
export function initTheme(): void {
  readSystem()
  apply(resolvedTheme.value)
}
