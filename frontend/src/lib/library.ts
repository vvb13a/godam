import { reactive } from 'vue'
import * as api from '../api/client'
import type { Collection, Stats, Tag, TagGroup } from '../types'

// library holds the collections, tags, tag groups and aggregate stats shared
// by the sidebar and the views, plus a version counter that is bumped on any
// library-wide change (e.g. a drag-and-drop upload) so mounted views know to
// reload.
export const library = reactive({
  collections: [] as Collection[],
  tags: [] as Tag[],
  tagGroups: [] as TagGroup[],
  stats: null as Stats | null,
  version: 0,
})

export async function loadCollections(): Promise<void> {
  library.collections = await api.listCollections()
}

export async function loadTags(): Promise<void> {
  library.tags = await api.listTags()
}

export async function loadTagGroups(): Promise<void> {
  library.tagGroups = await api.listTagGroups()
}

export async function loadStats(): Promise<void> {
  library.stats = await api.getStats()
}

export async function refreshLibrary(): Promise<void> {
  await Promise.all([
    loadCollections(),
    loadTags(),
    loadTagGroups(),
    loadStats(),
  ])
}

export function notifyLibraryChanged(): void {
  library.version++
}
