import {
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
} from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/dashboard' },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('./components/DashboardView.vue'),
  },
  {
    path: '/library',
    name: 'library',
    component: () => import('./components/AssetsView.vue'),
  },
  {
    path: '/collections/:collectionId',
    name: 'collection',
    component: () => import('./components/AssetsView.vue'),
    props: true,
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
