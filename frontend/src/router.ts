import {
  createRouter,
  createWebHistory,
  type RouteLocationNormalized,
  type RouteLocationRaw,
  type RouteRecordRaw,
} from 'vue-router'
import { auth, loadMe } from './lib/auth'
import { leaveEditor } from './lib/editorState'
import {
  applyEmbedScope,
  configureEmbed,
  embed,
  embedQuery,
} from './lib/embed'
import type { EmbedScope } from './types'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/all' },
  {
    path: '/all',
    name: 'all',
    component: () => import('./components/AssetsView.vue'),
  },
  {
    path: '/trash',
    name: 'trash',
    component: () => import('./components/TrashView.vue'),
  },
  {
    path: '/embed',
    redirect: (to) => ({
      name: 'all',
      query: { ...to.query, embed: '1' },
    }),
  },
  { path: '/:pathMatch(.*)*', redirect: '/all' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Keep the embedding parameters on every in-app navigation so the shell,
// picker bar and postMessage bridge survive folder/tag/filter changes.
router.beforeEach((to) => {
  const marker = typeof to.query.embed === 'string' && to.query.embed !== ''
  if (marker) configureEmbed(to.query)
  if (!embed.active || (to.name !== 'all' && to.name !== 'trash')) {
    return true
  }

  const query = { ...to.query }
  let changed = false
  if (!marker) {
    Object.assign(query, embedQuery())
    changed = true
  }
  if (to.name === 'all' && embed.mode !== 'library') {
    if (
      embed.collectionLocked &&
      embed.collectionId &&
      query.collection !== embed.collectionId
    ) {
      query.collection = embed.collectionId
      changed = true
    }
    if (embed.tagLocked && embed.tag && query.tag !== embed.tag) {
      query.tag = embed.tag
      changed = true
    }
  }
  if (!changed) return true
  return {
    name: to.name,
    query,
    hash: to.hash,
    replace: true,
  }
})

function scopeList(values?: string[], special?: string): string {
  const list = [...(values ?? [])]
  if (special) list.push(special)
  return list.join(',')
}

// scopedFilterQuery applies the token's default filters when the URL does not
// already carry them, and forces locked filters even when it does. The server
// repeats this logic, so a manipulated URL cannot escape the scope.
function scopedFilterQuery(
  to: RouteLocationNormalized,
  scope: EmbedScope,
): RouteLocationRaw | true {
  const query = { ...to.query }
  const locked = new Set(scope.locked_filters ?? [])
  const defaults = scope.default_filters ?? {}
  let changed = false

  const apply = (key: string, value: string) => {
    const current = typeof query[key] === 'string' ? (query[key] as string) : ''
    if (locked.has(key) || !current) {
      if (value) {
        if (current !== value) {
          query[key] = value
          changed = true
        }
      } else if (locked.has(key) && current) {
        delete query[key]
        changed = true
      }
    }
  }

  apply(
    'collection',
    scopeList(defaults.collection, defaults.uncategorized ? 'uncategorized' : ''),
  )
  apply('tag', scopeList(defaults.tag, defaults.untagged ? 'untagged' : ''))
  apply('type', scopeList(defaults.type))
  apply('mime', scopeList(defaults.mime))
  apply('search', defaults.search ?? '')
  apply('sort', defaults.sort ?? '')

  return changed
    ? { name: to.name, query, hash: to.hash, replace: true }
    : true
}

// Resolve the current actor once, then apply an embed token's scope.
router.beforeEach(async (to) => {
  if (!auth.ready) await loadMe()
  if (embed.active && auth.user?.scope) applyEmbedScope(auth.user.scope)
  if (embed.active && auth.user?.scope && to.name === 'all') {
    return scopedFilterQuery(to, auth.user.scope)
  }
  return true
})

// Protect unsaved edits in the detail editor on every navigation (sidebar
// collections/tags, other routes, …), matching the editor's Back control.
router.beforeEach(async () => {
  return await leaveEditor()
})
