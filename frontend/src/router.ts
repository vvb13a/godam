import {
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
} from 'vue-router'

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
  { path: '/:pathMatch(.*)*', redirect: '/all' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
