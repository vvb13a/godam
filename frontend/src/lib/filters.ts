import { reactive, watch } from 'vue'
import { embed } from './embed'
import { clearSmartSelection } from './smartSelection'
import type { EmbedScope } from '../types'

export type FacetClickMode = 'add' | 'replace'

const FILTER_SETTINGS_KEY = 'godam:filter-settings'

function loadClickMode(): FacetClickMode {
  try {
    const raw = localStorage.getItem(FILTER_SETTINGS_KEY)
    return raw === 'add' || raw === 'replace' ? raw : 'replace'
  } catch {
    return 'replace'
  }
}

export const filterSettings = reactive({
  clickMode: loadClickMode(),
})

watch(
  filterSettings,
  (value) => {
    try {
      localStorage.setItem(FILTER_SETTINGS_KEY, value.clickMode)
    } catch {
      // Storage can be unavailable; the preference is non-critical.
    }
  },
  { deep: true },
)

export const filterState = reactive({
  collections: [] as string[],
  tags: [] as string[],
  types: [] as string[],
  colors: [] as string[],
  mimeTypes: [] as string[],
  uncategorized: false,
  untagged: false,
  preset: '',
  presetExclude: false,
  search: '',
  sort: 'date-desc',
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

const collectionLocked = (): boolean => embed.active && embed.collectionLocked
const tagLocked = (): boolean => embed.active && embed.tagLocked
const typeLocked = (): boolean => embed.active && embed.typeLocked
const mimeLocked = (): boolean => embed.active && embed.mimeLocked
const searchLocked = (): boolean => embed.active && embed.searchLocked

function clearFacetFilters(): void {
  if (!collectionLocked()) filterState.collections = []
  if (!tagLocked()) filterState.tags = []
  if (!typeLocked()) filterState.types = []
  if (!mimeLocked()) filterState.mimeTypes = []
  if (!searchLocked()) filterState.search = ''
  filterState.colors = []
  filterState.uncategorized = false
  filterState.untagged = false
  filterState.preset = ''
  filterState.presetExclude = false
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

export function clearCollectionFilter(id: string): void {
  if (collectionLocked()) return
  clearSmartSelection()
  filterState.collections = filterState.collections.filter(
    (item) => item !== id,
  )
}

export function clearTagFilter(id: string): void {
  if (tagLocked()) return
  clearSmartSelection()
  filterState.tags = filterState.tags.filter((item) => item !== id)
}

export function toggleCollection(id: string): void {
  if (collectionLocked()) return
  clearSmartSelection()
  if (filterState.collections.includes(id)) {
    clearCollectionFilter(id)
    return
  }
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  filterState.collections = [
    ...filterState.collections.filter((item) => item !== id),
    id,
  ]
  filterState.uncategorized = false
}

export function applyCollectionFilter(id: string): void {
  if (collectionLocked()) return
  clearSmartSelection()
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  if (!filterState.collections.includes(id)) {
    filterState.collections = [...filterState.collections, id]
  }
  filterState.uncategorized = false
}

export function toggleTag(id: string): void {
  if (tagLocked()) return
  clearSmartSelection()
  if (filterState.tags.includes(id)) {
    clearTagFilter(id)
    return
  }
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  filterState.tags = [...filterState.tags.filter((item) => item !== id), id]
  filterState.untagged = false
}

export function applyTagFilter(id: string): void {
  if (tagLocked()) return
  clearSmartSelection()
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  if (!filterState.tags.includes(id)) {
    filterState.tags = [...filterState.tags, id]
  }
  filterState.untagged = false
}

export function toggleUncategorized(): void {
  if (collectionLocked()) return
  clearSmartSelection()
  if (filterState.uncategorized) {
    filterState.uncategorized = false
    return
  }
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  filterState.uncategorized = true
  filterState.collections = []
}

export function toggleUntagged(): void {
  if (tagLocked()) return
  clearSmartSelection()
  if (filterState.untagged) {
    filterState.untagged = false
    return
  }
  if (filterSettings.clickMode === 'replace') clearFacetFilters()
  filterState.untagged = true
  filterState.tags = []
}

export function resetSort(): void {
  if (embed.active && embed.sortLocked) return
  filterState.sort = 'date-desc'
}

let initialized = false

export function initFilterState(scope?: EmbedScope): void {
  if (initialized) return
  initialized = true
  if (!embed.active) return

  const defaults = scope?.default_filters ?? {}
  const locked = new Set(scope?.locked_filters ?? [])

  if (embed.collectionId) filterState.collections = [embed.collectionId]
  if (embed.tag) filterState.tags = [embed.tag]

  if (defaults.collection?.length) {
    if (locked.has('collection') || filterState.collections.length === 0) {
      filterState.collections = [...defaults.collection]
    }
  }
  if (defaults.tag?.length) {
    if (locked.has('tag') || filterState.tags.length === 0) {
      filterState.tags = [...defaults.tag]
    }
  }
  if (defaults.type?.length) {
    if (locked.has('type') || filterState.types.length === 0) {
      filterState.types = [...defaults.type]
    }
  }
  if (defaults.mime?.length) {
    if (locked.has('mime') || filterState.mimeTypes.length === 0) {
      filterState.mimeTypes = [...defaults.mime]
    }
  }
  if (defaults.uncategorized) filterState.uncategorized = true
  if (defaults.untagged) filterState.untagged = true
  if (defaults.search) {
    if (locked.has('search') || !filterState.search) {
      filterState.search = defaults.search
    }
  }
  if (defaults.sort) {
    if (locked.has('sort') || filterState.sort === 'date-desc') {
      filterState.sort = defaults.sort
    }
  }
}

export function hasActiveFilterState(): boolean {
  return (
    filterState.collections.length > 0 ||
    filterState.tags.length > 0 ||
    filterState.types.length > 0 ||
    filterState.colors.length > 0 ||
    filterState.mimeTypes.length > 0 ||
    filterState.uncategorized ||
    filterState.untagged ||
    Boolean(filterState.preset) ||
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
