import { reactive, watch } from 'vue'

// SidebarPrefs controls which optional panels the layout shows and their order.
// It is a per-device preference (like the asset grid display settings)
// persisted to localStorage.
export type SidebarToggleKey =
  | 'quicklinks'
  | 'collections'
  | 'smartCollections'

export type SidebarMode = 'classic' | 'attributes'

export interface SidebarPrefs {
  quicklinks: boolean
  collections: boolean
  smartCollections: boolean
  order: SidebarToggleKey[]
  mode: SidebarMode
}

const SIDEBAR_STORAGE_KEY = 'godam:sidebar'
const SIDEBAR_KEYS: SidebarToggleKey[] = [
  'quicklinks',
  'collections',
  'smartCollections',
]

const DEFAULT_PREFS: SidebarPrefs = {
  quicklinks: true,
  collections: true,
  smartCollections: true,
  order: [...SIDEBAR_KEYS],
  mode: 'classic',
}

function normalizeOrder(value: unknown): SidebarToggleKey[] {
  const raw = Array.isArray(value) ? value : []
  const order = raw.filter(
    (key): key is SidebarToggleKey =>
      typeof key === 'string' && SIDEBAR_KEYS.includes(key as SidebarToggleKey),
  )
  for (const key of SIDEBAR_KEYS) {
    if (!order.includes(key)) order.push(key)
  }
  return order
}

function loadPrefs(): SidebarPrefs {
  try {
    const raw = localStorage.getItem(SIDEBAR_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_PREFS, order: [...SIDEBAR_KEYS] }
    const parsed = JSON.parse(raw) as Partial<SidebarPrefs>
    return {
      ...DEFAULT_PREFS,
      ...parsed,
      order: normalizeOrder(parsed.order),
      mode: parsed.mode === 'attributes' ? 'attributes' : 'classic',
    }
  } catch {
    return { ...DEFAULT_PREFS, order: [...SIDEBAR_KEYS] }
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

export interface SidebarToggle {
  key: SidebarToggleKey
  label: string
}

export const SIDEBAR_TOGGLES: SidebarToggle[] = [
  { key: 'quicklinks', label: 'Quicklinks' },
  { key: 'collections', label: 'Collections' },
  { key: 'smartCollections', label: 'Smart Collections' },
]
