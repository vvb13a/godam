import {
  createRouter,
  createWebHistory,
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

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/library' },
  { path: '/all', redirect: '/library' },
  {
    path: '/library',
    name: 'library',
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
      name: 'library',
      query: { ...to.query, embed: '1' },
    }),
  },
  { path: '/:pathMatch(.*)*', redirect: '/library' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Keep the embedding parameters on every in-app navigation so the shell,
// picker bar and postMessage bridge survive route changes.
router.beforeEach((to) => {
  const marker = typeof to.query.embed === 'string' && to.query.embed !== ''
  if (marker) configureEmbed(to.query)
  if (!embed.active || (to.name !== 'library' && to.name !== 'trash')) {
    return true
  }

  if (marker) return true
  return {
    name: to.name,
    query: { ...to.query, ...embedQuery() },
    hash: to.hash,
    replace: true,
  }
})

// Resolve the current actor once, then apply an embed token's scope.
router.beforeEach(async () => {
  if (!auth.ready) await loadMe()
  if (embed.active && auth.user?.scope) applyEmbedScope(auth.user.scope)
  return true
})

// Protect unsaved edits in the detail editor on every navigation (sidebar
// collections/tags, other routes, …), matching the editor's Back control.
router.beforeEach(async () => {
  return await leaveEditor()
})
