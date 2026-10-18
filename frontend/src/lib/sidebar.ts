import { reactive, watch } from 'vue'

// SidebarPrefs controls which optional panels the layout shows. It is a
// per-device preference (like the asset grid display settings) persisted to
// localStorage.
export interface SidebarPrefs {
  quicklinks: boolean
  collections: boolean
  smartCollections: boolean
  tags: boolean
}

const SIDEBAR_STORAGE_KEY = 'godam:sidebar'

const DEFAULT_PREFS: SidebarPrefs = {
  quicklinks: true,
  collections: true,
  smartCollections: true,
  tags: true,
}

function loadPrefs(): SidebarPrefs {
  try {
    const raw = localStorage.getItem(SIDEBAR_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_PREFS }
    const parsed = JSON.parse(raw) as Partial<SidebarPrefs>
    return { ...DEFAULT_PREFS, ...parsed }
  } catch {
    return { ...DEFAULT_PREFS }
  }
}

export const sidebarPrefs = reactive<SidebarPrefs>(loadPrefs())

watch(
  sidebarPrefs,
  (value) => {
    try {
      localStorage.setItem(SIDEBAR_STORAGE_KEY, JSON.stringify(value))
    } catch {
      // Storage can be unavailable (private mode); preferences are non-critical.
    }
  },
  { deep: true },
)

export type SidebarToggleKey =
  | 'quicklinks'
  | 'collections'
  | 'smartCollections'
  | 'tags'

export interface SidebarToggle {
  key: SidebarToggleKey
  label: string
}

export const SIDEBAR_TOGGLES: SidebarToggle[] = [
  { key: 'quicklinks', label: 'Quicklinks' },
  { key: 'collections', label: 'Collections' },
  { key: 'smartCollections', label: 'Smart Collections' },
  { key: 'tags', label: 'Tags' },
]
