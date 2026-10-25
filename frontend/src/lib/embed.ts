import { reactive } from 'vue'
import { setAuthToken } from '../api/client'
import type { EmbedScope } from '../types'

export type EmbedMode = 'library' | 'single' | 'multi'

export const embed = reactive({
  active: false,
  mode: 'library' as EmbedMode,
  hostOrigin: '*',
  token: '',
  scope: null as EmbedScope | null,
  collectionId: '',
  tag: '',
  collectionLocked: false,
  tagLocked: false,
  typeLocked: false,
  mimeLocked: false,
  searchLocked: false,
  sortLocked: false,
  preselectedIds: [] as string[],
  selectedIds: [] as string[],
  selectionInitialized: false,
  ready: false,
  assetCount: 0,
})

function param(value: unknown): string {
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0] : ''
  }
  return typeof value === 'string' ? value : ''
}

export function configureEmbed(query: Record<string, unknown>): void {
  // The embed contract is fixed when the iframe first loads. Later in-app
  // navigations only change the visible filters, never the mode or locks.
  if (embed.active) return
  embed.active = true
  const mode = param(query.mode)
  embed.mode = mode === 'single' || mode === 'multi' ? mode : 'library'
  embed.hostOrigin = param(query.hostOrigin) || '*'
  embed.token = param(query.token)
  if (embed.token) setAuthToken(embed.token)
  embed.collectionId = param(query.collection)
  embed.tag = param(query.tag)
  const lock = embed.mode === 'library' ? '' : param(query.lock)
  embed.collectionLocked = lock.split(',').includes('collection')
  embed.tagLocked = lock.split(',').includes('tag')
  embed.preselectedIds = param(query.selected)
    .split(',')
    .map((id) => id.trim())
    .filter(Boolean)
}

// applyEmbedScope merges the token's server-enforced scope into the local UI
// state. The server remains authoritative; this only makes locked controls
// look locked and hides uploads the token cannot perform.
export function applyEmbedScope(scope: EmbedScope): void {
  if (!embed.active) return
  embed.scope = scope
  const filters = scope.filters ?? {}
  const locked = (name: string): boolean => {
    const mode = filters[name]?.mode
    return mode === 'locked' || mode === 'hidden'
  }
  embed.collectionLocked = embed.collectionLocked || locked('collection')
  embed.tagLocked = embed.tagLocked || locked('tag')
  embed.typeLocked = locked('type')
  embed.mimeLocked = locked('mime')
  embed.searchLocked = locked('search')
  embed.sortLocked = locked('sort')
}

export function embedQuery(): Record<string, string> {
  const query: Record<string, string> = { embed: '1' }
  if (embed.mode !== 'library') query.mode = embed.mode
  if (embed.hostOrigin !== '*') query.hostOrigin = embed.hostOrigin
  if (embed.token) query.token = embed.token
  const lock = [
    embed.collectionLocked ? 'collection' : '',
    embed.tagLocked ? 'tag' : '',
  ]
    .filter(Boolean)
    .join(',')
  if (lock) query.lock = lock
  if (embed.collectionLocked && embed.collectionId) {
    query.collection = embed.collectionId
  }
  if (embed.tagLocked && embed.tag) query.tag = embed.tag
  return query
}

function post(type: string, payload: Record<string, unknown> = {}): void {
  if (!embed.active) return
  window.parent.postMessage({ type, ...payload }, embed.hostOrigin)
}

function announceReady(): void {
  if (!embed.ready) return
  post('godam:ready', { assetCount: embed.assetCount })
}

export function markEmbedReady(assetCount: number): void {
  if (!embed.active) return
  embed.assetCount = assetCount
  embed.ready = true
  announceReady()
}

export function confirmEmbedSelection(): void {
  const assetIds = embed.selectedIds.slice()
  console.log('[godam embed] selected asset ids:', assetIds)
  post('godam:selection', { assetIds })
}

export function cancelEmbedSelection(): void {
  post('godam:cancel')
}

let listening = false

function onMessage(event: MessageEvent): void {
  if (!embed.active) return
  if (embed.hostOrigin !== '*' && event.origin !== embed.hostOrigin) return
  const data = event.data as { type?: string; assetIds?: unknown } | null
  if (!data) return
  if (data.type === 'godam:ping') {
    announceReady()
    return
  }
  if (data.type === 'godam:setSelection' && Array.isArray(data.assetIds)) {
    embed.selectedIds = data.assetIds.filter(
      (id): id is string => typeof id === 'string',
    )
  }
}

export function startEmbedBridge(): void {
  if (listening) return
  listening = true
  window.addEventListener('message', onMessage)
}
