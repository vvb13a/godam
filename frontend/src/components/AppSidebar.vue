<script setup lang="ts">
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { Images } from '@primeicons/vue'
import { computed, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { library, refreshLibrary } from '../lib/library'
import { sidebarPrefs } from '../lib/sidebar'
import CollectionTree from './CollectionTree.vue'
import LibrarySwitcher from './LibrarySwitcher.vue'
import SmartCollections from './SmartCollections.vue'
import TagsList from './TagsList.vue'
import UserMenu from './UserMenu.vue'

const route = useRoute()

const activeCollection = computed(
  () => (route.query.collection as string | undefined) ?? null,
)
const activeTag = computed(() => (route.query.tag as string | undefined) ?? null)
const allActive = computed(
  () => route.name === 'all' && !activeCollection.value && !activeTag.value,
)

watch(
  () => library.version,
  () => {
    void refreshLibrary()
  },
)
</script>

<template>
  <aside class="flex w-64 shrink-0 flex-col border-r border-slate-200 bg-white">
    <div class="p-3 pb-0">
      <LibrarySwitcher />
    </div>

    <nav class="space-y-1 p-3">
      <RouterLink
        :to="{ name: 'all' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          allActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="18" />
        <span class="flex-1">All</span>
        <span
          v-if="library.stats?.assets"
          class="shrink-0 text-xs text-slate-400"
        >
          {{ library.stats.assets }}
        </span>
      </RouterLink>
    </nav>

    <Splitter
      layout="vertical"
      state-key="godam:sidebar:sections"
      state-storage="local"
      :gutter-size="14"
      class="min-h-0 flex-1"
    >
      <SplitterPanel
        v-if="sidebarPrefs.collections"
        :size="45"
        :min-size="8"
      >
        <div class="h-full overflow-y-auto pb-2">
          <CollectionTree />
        </div>
      </SplitterPanel>
      <SplitterPanel
        v-if="sidebarPrefs.smartCollections"
        :size="20"
        :min-size="8"
      >
        <div class="h-full overflow-y-auto pb-2">
          <SmartCollections />
        </div>
      </SplitterPanel>
      <SplitterPanel v-if="sidebarPrefs.tags" :size="35" :min-size="8">
        <div class="h-full overflow-y-auto pb-2">
          <TagsList />
        </div>
      </SplitterPanel>
    </Splitter>

    <UserMenu />
  </aside>
</template>

<style scoped>
:deep(.p-splitter) {
  background: transparent;
  border: 0;
}

:deep(.p-splitter-gutter) {
  position: relative;
  background: transparent;
}

/* The subtle rule the drag grip sits on. */
:deep(.p-splitter-gutter)::before {
  content: '';
  position: absolute;
  top: 50%;
  right: 0.75rem;
  left: 0.75rem;
  height: 1px;
  transform: translateY(-50%);
  background-color: #e2e8f0;
  transition: background-color 0.15s ease;
}

:deep(.p-splitter-gutter:hover)::before {
  background-color: #94a3b8;
}

:deep(.p-splitter-gutter-handle) {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 1.75rem !important;
  height: 0.875rem !important;
  transform: translate(-50%, -50%);
  border: 1px solid #e2e8f0;
  border-radius: 9999px;
  background-color: #ffffff;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8'%3E%3Cg fill='%2394a3b8'%3E%3Ccircle cx='2' cy='2' r='1'/%3E%3Ccircle cx='6' cy='2' r='1'/%3E%3Ccircle cx='10' cy='2' r='1'/%3E%3Ccircle cx='2' cy='6' r='1'/%3E%3Ccircle cx='6' cy='6' r='1'/%3E%3Ccircle cx='10' cy='6' r='1'/%3E%3C/g%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: center;
}
</style>
