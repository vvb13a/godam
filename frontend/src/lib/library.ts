import { reactive } from 'vue'
import * as api from '../api/client'
import type {
  Collection,
  Library,
  SmartCollection,
  Stats,
  Tag,
  TagGroup,
} from '../types'

// library holds the top-level Library scopes together with the scoped data
// (collections, tags, tag groups, stats) of the active library. The version
// counter is bumped whenever the active library or its data changes so mounted
// views know to reload.
export const library = reactive({
  libraries: [] as Library[],
  currentId: null as string | null,
  loaded: false,
  collections: [] as Collection[],
  smartCollections: [] as SmartCollection[],
  tags: [] as Tag[],
  tagGroups: [] as TagGroup[],
  stats: null as Stats | null,
  version: 0,
})

export function currentLibrary(): Library | null {
  return library.libraries.find((item) => item.id === library.currentId) ?? null
}

export function hasLibraries(): boolean {
  return library.libraries.length > 0
}

export async function loadLibraries(): Promise<void> {
  library.libraries = await api.listLibraries()
  library.loaded = true
}

export async function loadCollections(): Promise<void> {
  const id = library.currentId
  library.collections = id ? await api.listCollections(id) : []
}

export async function loadSmartCollections(): Promise<void> {
  const id = library.currentId
  library.smartCollections = id ? await api.listSmartCollections(id) : []
}

export async function loadTags(): Promise<void> {
  const id = library.currentId
  library.tags = id ? await api.listTags(id) : []
}

export async function loadTagGroups(): Promise<void> {
  const id = library.currentId
  library.tagGroups = id ? await api.listTagGroups(id) : []
}

export async function loadStats(): Promise<void> {
  const id = library.currentId
  library.stats = id ? await api.getStats(id) : null
}

export function clearScopedData(): void {
  library.collections = []
  library.smartCollections = []
  library.tags = []
  library.tagGroups = []
  library.stats = null
}

// resetLibrary clears all library state, used on logout so the next account
// does not briefly see the previous one's data.
export function resetLibrary(): void {
  library.libraries = []
  library.currentId = null
  library.loaded = false
  clearScopedData()
}

export async function refreshLibrary(): Promise<void> {
  if (!library.currentId) {
    clearScopedData()
    return
  }
  await Promise.all([
    loadCollections(),
    loadSmartCollections(),
    loadTags(),
    loadTagGroups(),
    loadStats(),
  ])
}

// selectLibrary switches the active library, drops the previous library's data
// and triggers a scoped reload through the version watchers.
export function selectLibrary(id: string): void {
  if (library.currentId === id) return
  library.currentId = id
  clearScopedData()
  library.version++
}

// initLibrary loads the available libraries and picks the default: the library
// with the lowest sort order (the API returns them pre-sorted), or an explicit
// preferred id when an embedding host scopes the app to one project.
export async function initLibrary(preferredId?: string): Promise<void> {
  await loadLibraries()
  const preferred = preferredId
    ? library.libraries.find((item) => item.id === preferredId)
    : undefined
  const first = library.libraries[0]
  if (!first) {
    library.currentId = null
    return
  }
  const stillValid = library.libraries.some((item) => item.id === library.currentId)
  library.currentId = preferred
    ? preferred.id
    : stillValid
      ? library.currentId
      : first.id
  clearScopedData()
  library.version++
}

// syncAfterLibraryChange reloads the library list and keeps the current
// selection valid, falling back to the first library when the current one was
// removed. Used after create/update/delete in the manager.
export async function syncAfterLibraryChange(): Promise<void> {
  await loadLibraries()
  const exists = library.libraries.some((item) => item.id === library.currentId)
  if (!exists) {
    library.currentId = library.libraries[0]?.id ?? null
    clearScopedData()
  }
  library.version++
}

export function notifyLibraryChanged(): void {
  library.version++
}
