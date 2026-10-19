import { reactive } from 'vue'
import * as api from '../api/client'
import type {
  Collection,
  Instance,
  Preset,
  SmartCollection,
  Stats,
  Tag,
  TagGroup,
} from '../types'

// library is the single instance this godam process serves. The name is kept
// for the UI modules; there is no library list or active-library selector.
export const library = reactive({
  id: null as string | null,
  name: '',
  color: '',
  cover_asset_id: '',
  watermark_asset_id: '',
  allowed_mime_types: [] as string[],
  max_upload_bytes: 0,
  loaded: false,
  collections: [] as Collection[],
  smartCollections: [] as SmartCollection[],
  tags: [] as Tag[],
  tagGroups: [] as TagGroup[],
  presets: [] as Preset[],
  stats: null as Stats | null,
  version: 0,
})

export function currentLibrary(): Instance | null {
  return library.loaded ? (library as unknown as Instance) : null
}

export function hasLibraries(): boolean {
  return library.loaded
}

export async function loadLibraries(): Promise<void> {
  await loadInstance()
}

export async function loadInstance(markLoaded = true): Promise<void> {
  const instance = await api.getInstance()
  Object.assign(library, instance)
  if (markLoaded) library.loaded = true
}

export async function loadCollections(): Promise<void> {
  library.collections = await api.listCollections()
}

export async function loadSmartCollections(): Promise<void> {
  library.smartCollections = await api.listSmartCollections()
}

export async function loadTags(): Promise<void> {
  library.tags = await api.listTags()
}

export async function loadTagGroups(): Promise<void> {
  library.tagGroups = await api.listTagGroups()
}

export async function loadPresets(): Promise<void> {
  library.presets = await api.listPresets()
}

export async function loadStats(): Promise<void> {
  library.stats = await api.getStats()
}

export function clearScopedData(): void {
  library.collections = []
  library.smartCollections = []
  library.tags = []
  library.tagGroups = []
  library.presets = []
  library.stats = null
}

export function resetLibrary(): void {
  library.id = null
  library.name = ''
  library.color = ''
  library.cover_asset_id = ''
  library.watermark_asset_id = ''
  library.allowed_mime_types = []
  library.max_upload_bytes = 0
  library.loaded = false
  clearScopedData()
}

async function loadScopedData(): Promise<void> {
  await Promise.all([
    loadCollections(),
    loadSmartCollections(),
    loadTags(),
    loadTagGroups(),
    loadPresets(),
    loadStats(),
  ])
}

export async function refreshLibrary(): Promise<void> {
  if (!library.id) {
    clearScopedData()
    return
  }
  await Promise.all([loadInstance(), loadScopedData()])
}

export async function initLibrary(): Promise<void> {
  await loadInstance(false)
  clearScopedData()
  library.version++
  library.loaded = true
  await loadScopedData()
}

// notifyLibraryChanged bumps the version so mounted views reload scoped data.
export function notifyLibraryChanged(): void {
  library.version++
}
