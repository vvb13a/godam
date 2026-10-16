import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import * as api from './api/client'
import { auth, loadMe } from './lib/auth'
import { leaveEditor } from './lib/editorState'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/all' },
  {
    path: '/login',
    name: 'login',
    component: () => import('./components/LoginView.vue'),
    meta: { public: true },
  },
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
  { path: '/:pathMatch(.*)*', redirect: '/all' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Resolve the session once, then gate every non-public route behind it.
router.beforeEach(async (to) => {
  if (!auth.ready) await loadMe()
  if (to.meta.public) {
    if (to.name === 'login' && auth.user) return { name: 'all' }
    return true
  }
  if (!auth.user) {
    const redirect = to.fullPath && to.fullPath !== '/' ? { redirect: to.fullPath } : {}
    return { name: 'login', query: redirect }
  }
  return true
})

// Protect unsaved edits in the detail editor on every navigation (sidebar
// collections/tags, other routes, …), matching the editor's Back control.
router.beforeEach(async () => {
  return await leaveEditor()
})

// A 401 from any non-auth API call means the session expired; drop the user and
// send them back to login.
api.setUnauthorizedHandler(() => {
  auth.user = null
  if (router.currentRoute.value.name !== 'login') {
    void router.push({ name: 'login' })
  }
})
