import { reactive } from 'vue'
import { embed } from './embed'
import { clearSmartSelection } from './smartSelection'
import type { EmbedScope } from '../types'

export const filterState = reactive({
  types: [] as string[],
  colors: [] as string[],
  mimeTypes: [] as string[],
  search: '',
  sort: 'date-desc',
  attributes: {} as Record<string, string[]>,
  minSize: null as number | null,
  maxSize: null as number | null,
  minDuration: null as number | null,
  maxDuration: null as number | null,
  minPages: null as number | null,
  maxPages: null as number | null,
  minWidth: null as number | null,
  maxWidth: null as number | null,
  minHeight: null as number | null,
  maxHeight: null as number | null,
  addedAfter: null as string | null,
  addedBefore: null as string | null,
  modifiedAfter: null as string | null,
  modifiedBefore: null as string | null,
})

const typeLocked = (): boolean =>
  (embed.active && embed.typeLocked) || filterLocked('type')
const mimeLocked = (): boolean =>
  (embed.active && embed.mimeLocked) || filterLocked('mime')
const searchLocked = (): boolean =>
  (embed.active && embed.searchLocked) || filterLocked('search')

function enforcedAttributes(): Record<string, string[]> {
  const attributes: Record<string, string[]> = {}
  for (const grant of embed.scope?.attributes ?? []) {
    if (grant.mode === 'locked' || grant.mode === 'hidden') {
      attributes[grant.key] = [...grant.values]
    }
  }
  return attributes
}

function scopeFilter(name: string) {
  return embed.scope?.filters?.[name]
}

function filterValues(name: string): string[] {
  return scopeFilter(name)?.values ?? []
}

function filterLocked(name: string): boolean {
  const mode = scopeFilter(name)?.mode
  return mode === 'locked' || mode === 'hidden'
}

// isFilterLocked reports whether the embed token fixes a filter. It is used by
// the sidebar to mark filter chips as locked and to guard the clear actions.
export function isFilterLocked(name: string): boolean {
  return filterLocked(name)
}

// isFilterDenied reports whether the token forbids a filter. Filters without a
// grant are denied unless the token has the wildcard grant.
export function isFilterDenied(name: string): boolean {
  if (!embed.scope) return false
  const grant = scopeFilter(name)
  if (grant) return grant.mode === 'denied'
  return !embed.scope.all_filters
}

// isFilterVisible reports whether the sidebar should show an editable control
// for a filter. Locked, hidden, and denied filters are not editable.
export function isFilterVisible(name: string): boolean {
  if (!embed.scope) return true
  const grant = scopeFilter(name)
  if (grant) return grant.mode === 'allowed' || grant.mode === 'active'
  return embed.scope.all_filters === true
}

function clearFacetFilters(): void {
  filterState.attributes = enforcedAttributes()
  if (!typeLocked()) filterState.types = []
  if (!mimeLocked()) filterState.mimeTypes = []
  if (!searchLocked()) filterState.search = ''
  filterState.colors = []
  filterState.minSize = null
  filterState.maxSize = null
  filterState.minDuration = null
  filterState.maxDuration = null
  filterState.minPages = null
  filterState.maxPages = null
  filterState.minWidth = null
  filterState.maxWidth = null
  filterState.minHeight = null
  filterState.maxHeight = null
  filterState.addedAfter = null
  filterState.addedBefore = null
  filterState.modifiedAfter = null
  filterState.modifiedBefore = null
}

export function clearAllFilters(): void {
  clearSmartSelection()
  clearFacetFilters()
}

export function isAttributeLocked(key: string): boolean {
  const grant = embed.scope?.attributes?.find((item) => item.key === key)
  return grant?.mode === 'locked' || grant?.mode === 'hidden'
}

export function toggleAttribute(key: string, value: string): void {
  if (isAttributeLocked(key)) return
  clearSmartSelection()
  const current = filterState.attributes[key] ?? []
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  if (next.length) filterState.attributes[key] = next
  else delete filterState.attributes[key]
}

export function toggleType(value: string): void {
  if (typeLocked()) return
  clearSmartSelection()
  const current = filterState.types
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  filterState.types = next
}

export function toggleColor(value: string): void {
  clearSmartSelection()
  const current = filterState.colors
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  filterState.colors = next
}

export function clearTypeValue(value: string): void {
  if (typeLocked()) return
  clearSmartSelection()
  filterState.types = filterState.types.filter((item) => item !== value)
}

export function clearAttributeFilter(key: string): void {
  if (isAttributeLocked(key)) return
  clearSmartSelection()
  delete filterState.attributes[key]
}

export function clearAttributeValue(key: string, value: string): void {
  if (isAttributeLocked(key)) return
  clearSmartSelection()
  const next = (filterState.attributes[key] ?? []).filter(
    (item) => item !== value,
  )
  if (next.length) filterState.attributes[key] = next
  else delete filterState.attributes[key]
}

export function resetSort(): void {
  if ((embed.active && embed.sortLocked) || isFilterLocked('sort')) return
  filterState.sort = 'date-desc'
}

let initialized = false

export function initFilterState(scope?: EmbedScope): void {
  if (initialized) return
  initialized = true
  if (!embed.active) return

  // Token attributes are applied as the initial filter state. Active attributes
  // remain editable; locked and hidden attributes are immutable, with hidden
  // ones only omitted from the UI.
  for (const grant of scope?.attributes ?? []) {
    if (grant.values.length) {
      filterState.attributes[grant.key] = [...grant.values]
    }
  }

  const type = filterValues('type')
  if (type.length) {
    if (filterLocked('type') || filterState.types.length === 0) {
      filterState.types = [...type]
    }
  }
  const mimeTypes = filterValues('mime')
  if (mimeTypes.length) {
    if (filterLocked('mime') || filterState.mimeTypes.length === 0) {
      filterState.mimeTypes = [...mimeTypes]
    }
  }
  const search = filterValues('search')[0]
  if (search) {
    if (filterLocked('search') || !filterState.search) {
      filterState.search = search
    }
  }
  const sort = filterValues('sort')[0]
  if (sort) {
    if (filterLocked('sort') || filterState.sort === 'date-desc') {
      filterState.sort = sort
    }
  }
}

export function hasActiveFilterState(): boolean {
  return (
    filterState.types.length > 0 ||
    Object.keys(filterState.attributes).length > 0 ||
    filterState.colors.length > 0 ||
    filterState.mimeTypes.length > 0 ||
    Boolean(filterState.search.trim()) ||
    filterState.minSize !== null ||
    filterState.maxSize !== null ||
    filterState.minDuration !== null ||
    filterState.maxDuration !== null ||
    filterState.minPages !== null ||
    filterState.maxPages !== null ||
    filterState.minWidth !== null ||
    filterState.maxWidth !== null ||
    filterState.minHeight !== null ||
    filterState.maxHeight !== null ||
    filterState.addedAfter !== null ||
    filterState.addedBefore !== null ||
    filterState.modifiedAfter !== null ||
    filterState.modifiedBefore !== null
  )
}
