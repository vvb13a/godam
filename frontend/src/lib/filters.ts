import { reactive } from 'vue'
import { embed } from './embed'
import type { EmbedScope, ViewFilter } from '../types'

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
  clearFacetFilters()
}

export function isAttributeLocked(key: string): boolean {
  const grant = embed.scope?.attributes?.find((item) => item.key === key)
  return grant?.mode === 'locked' || grant?.mode === 'hidden'
}

export function toggleAttribute(key: string, value: string): void {
  if (isAttributeLocked(key)) return
  const current = filterState.attributes[key] ?? []
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  if (next.length) filterState.attributes[key] = next
  else delete filterState.attributes[key]
}

export function toggleType(value: string): void {
  if (typeLocked()) return
  const current = filterState.types
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  filterState.types = next
}

export function toggleColor(value: string): void {
  const current = filterState.colors
  const next = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
  filterState.colors = next
}

export function clearTypeValue(value: string): void {
  if (typeLocked()) return
  filterState.types = filterState.types.filter((item) => item !== value)
}

export function clearAttributeFilter(key: string): void {
  if (isAttributeLocked(key)) return
  delete filterState.attributes[key]
}

export function clearAttributeValue(key: string, value: string): void {
  if (isAttributeLocked(key)) return
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

// snapshotFilterState returns a detached copy of every filter value so a view
// can store the complete state at the moment it was saved.
export function snapshotFilterState(): ViewFilter {
  return {
    types: [...filterState.types],
    colors: [...filterState.colors],
    mimeTypes: [...filterState.mimeTypes],
    search: filterState.search,
    sort: filterState.sort,
    attributes: Object.fromEntries(
      Object.entries(filterState.attributes).map(([key, values]) => [
        key,
        [...values],
      ]),
    ),
    minSize: filterState.minSize,
    maxSize: filterState.maxSize,
    minDuration: filterState.minDuration,
    maxDuration: filterState.maxDuration,
    minPages: filterState.minPages,
    maxPages: filterState.maxPages,
    minWidth: filterState.minWidth,
    maxWidth: filterState.maxWidth,
    minHeight: filterState.minHeight,
    maxHeight: filterState.maxHeight,
    addedAfter: filterState.addedAfter,
    addedBefore: filterState.addedBefore,
    modifiedAfter: filterState.modifiedAfter,
    modifiedBefore: filterState.modifiedBefore,
  }
}

// applyFilterState replaces the entire current filter with a saved snapshot.
// Locked token filters are still enforced over the snapshot.
export function applyFilterState(snapshot: ViewFilter): void {
  filterState.types = typeLocked()
    ? filterValues('type')
    : [...snapshot.types]
  filterState.mimeTypes = mimeLocked()
    ? filterValues('mime')
    : [...snapshot.mimeTypes]
  filterState.colors = [...snapshot.colors]
  filterState.search = searchLocked()
    ? (filterValues('search')[0] ?? '')
    : snapshot.search
  if (!((embed.active && embed.sortLocked) || isFilterLocked('sort'))) {
    filterState.sort = snapshot.sort
  }
  filterState.attributes = Object.fromEntries(
    Object.entries(snapshot.attributes).map(([key, values]) => [
      key,
      [...values],
    ]),
  )
  Object.assign(filterState.attributes, enforcedAttributes())
  filterState.minSize = snapshot.minSize
  filterState.maxSize = snapshot.maxSize
  filterState.minDuration = snapshot.minDuration
  filterState.maxDuration = snapshot.maxDuration
  filterState.minPages = snapshot.minPages
  filterState.maxPages = snapshot.maxPages
  filterState.minWidth = snapshot.minWidth
  filterState.maxWidth = snapshot.maxWidth
  filterState.minHeight = snapshot.minHeight
  filterState.maxHeight = snapshot.maxHeight
  filterState.addedAfter = snapshot.addedAfter
  filterState.addedBefore = snapshot.addedBefore
  filterState.modifiedAfter = snapshot.modifiedAfter
  filterState.modifiedBefore = snapshot.modifiedBefore
}

// filterStateMatches reports whether the current filter exactly matches the
// saved snapshot, ignoring the order of list values.
export function filterStateMatches(snapshot: ViewFilter): boolean {
  const normalize = (filter: ViewFilter): string => {
    const attributes = Object.keys(filter.attributes)
      .sort()
      .reduce<Record<string, string[]>>((acc, key) => {
        acc[key] = [...filter.attributes[key]].sort()
        return acc
      }, {})
    return JSON.stringify({
      ...filter,
      types: [...filter.types].sort(),
      colors: [...filter.colors].sort(),
      mimeTypes: [...filter.mimeTypes].sort(),
      attributes,
    })
  }
  return normalize(snapshotFilterState()) === normalize(snapshot)
}

// hasSavableView reports whether the current state is worth saving as a view:
// any filter is active, or the sort differs from the default.
export function hasSavableView(): boolean {
  return hasActiveFilterState() || filterState.sort !== 'date-desc'
}
