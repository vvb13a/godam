import { reactive, watch } from 'vue'
import type {
  AttributeFacetValue,
  AttributeSortMode,
  AttributeSortSetting,
} from '../types'

const STORAGE_KEY = 'godam:attributes:sort'

const USER_MODES: AttributeSortMode[] = [
  'count:desc',
  'count:asc',
  'abc:asc',
  'abc:desc',
]

export const USER_SORT_OPTIONS: { value: AttributeSortMode; label: string }[] = [
  { value: 'count:desc', label: 'Count high–low' },
  { value: 'count:asc', label: 'Count low–high' },
  { value: 'abc:asc', label: 'Name A–Z' },
  { value: 'abc:desc', label: 'Name Z–A' },
]

export const SORT_OPTIONS: {
  value: AttributeSortSetting
  label: string
}[] = [
  { value: 'open', label: 'Open (users choose)' },
  { value: 'custom', label: 'Custom order' },
  ...USER_SORT_OPTIONS,
]

function isUserMode(value: unknown): value is AttributeSortMode {
  return USER_MODES.includes(value as AttributeSortMode)
}

function load(): Record<string, AttributeSortMode> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const result: Record<string, AttributeSortMode> = {}
    for (const [key, mode] of Object.entries(parsed)) {
      if (isUserMode(mode)) result[key] = mode
    }
    return result
  } catch {
    return {}
  }
}

const userSortModes = reactive<Record<string, AttributeSortMode>>(load())

watch(
  userSortModes,
  (value) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
    } catch {
      // Storage unavailable: the choice simply won't persist.
    }
  },
  { deep: true },
)

export function userAttributeSort(key: string): AttributeSortMode {
  return userSortModes[key] ?? 'count:desc'
}

export function setUserAttributeSort(
  key: string,
  mode: AttributeSortMode,
): void {
  userSortModes[key] = mode
}

// effectiveAttributeSort resolves the metadata setting. "open" means each user
// may choose, so the local preference wins. A fixed setting always wins.
export function effectiveAttributeSort(
  key: string,
  setting: AttributeSortSetting | undefined,
): AttributeSortMode {
  if (setting === 'open') return userAttributeSort(key)
  if (setting === 'custom') return 'custom'
  if (isUserMode(setting)) return setting
  return userAttributeSort(key)
}

export function sortAttributeValues(
  values: AttributeFacetValue[],
  mode: AttributeSortMode,
): AttributeFacetValue[] {
  const byName = (a: AttributeFacetValue, b: AttributeFacetValue) =>
    (a.label || a.value).localeCompare(b.label || b.value)
  const list = [...values]
  switch (mode) {
    case 'count:asc':
      list.sort((a, b) => a.count - b.count || byName(a, b))
      break
    case 'abc:asc':
      list.sort((a, b) => byName(a, b))
      break
    case 'abc:desc':
      list.sort((a, b) => byName(b, a))
      break
    case 'custom':
      list.sort(
        (a, b) =>
          a.position - b.position ||
          byName(a, b),
      )
      break
    default:
      list.sort((a, b) => b.count - a.count || byName(a, b))
  }
  return list
}
