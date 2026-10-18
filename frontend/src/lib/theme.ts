import { reactive } from 'vue'

export type ThemeMode = 'system' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

export const THEME_OPTIONS: { label: string; value: ThemeMode }[] = [
  { label: 'System', value: 'system' },
  { label: 'Light', value: 'light' },
  { label: 'Dark', value: 'dark' },
]

const THEME_STORAGE_KEY = 'godam:theme'
const media = window.matchMedia('(prefers-color-scheme: dark)')

function loadMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(THEME_STORAGE_KEY)
    return raw === 'light' || raw === 'dark' || raw === 'system'
      ? raw
      : 'system'
  } catch {
    return 'system'
  }
}

export const theme = reactive({
  mode: loadMode() as ThemeMode,
  resolved: 'light' as ResolvedTheme,
})

function resolveMode(mode: ThemeMode): ResolvedTheme {
  if (mode === 'system') return media.matches ? 'dark' : 'light'
  return mode
}

function apply(): void {
  const resolved = resolveMode(theme.mode)
  theme.resolved = resolved
  const root = document.documentElement
  root.classList.toggle('dark', resolved === 'dark')
  root.classList.toggle('p-dark', resolved === 'dark')
  root.style.colorScheme = resolved
}

export function setThemeMode(mode: ThemeMode): void {
  theme.mode = mode
  try {
    localStorage.setItem(THEME_STORAGE_KEY, mode)
  } catch {
    // Storage unavailable: the choice simply won't persist.
  }
  apply()
}

export function initTheme(): void {
  apply()
  const onChange = (): void => {
    if (theme.mode === 'system') apply()
  }
  if (media.addEventListener) media.addEventListener('change', onChange)
  else media.addListener(onChange)
}
