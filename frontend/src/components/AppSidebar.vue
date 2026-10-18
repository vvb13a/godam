<script setup lang="ts">
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { ChevronLeft, ChevronRight, Images } from '@primeicons/vue'
import { computed, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { embed as embedState } from '../lib/embed'
import { currentLibrary, library, refreshLibrary } from '../lib/library'
import { panels, toggleLeftPanel } from '../lib/panels'
import { sidebarPrefs } from '../lib/sidebar'
import CollectionTree from './CollectionTree.vue'
import LibrarySwitcher from './LibrarySwitcher.vue'
import SmartCollections from './SmartCollections.vue'
import UserMenu from './UserMenu.vue'

const props = withDefaults(defineProps<{ embed?: boolean }>(), {
  embed: false,
})

const route = useRoute()
const activeLibrary = computed(() => currentLibrary())

const pickerRestricted = computed(
  () => embedState.active && embedState.mode !== 'library',
)
const collectionsLocked = computed(
  () => pickerRestricted.value && embedState.collectionLocked,
)
const tagsLocked = computed(
  () => pickerRestricted.value && embedState.tagLocked,
)
const showCollections = computed(
  () => sidebarPrefs.collections && !collectionsLocked.value,
)
const showSmartCollections = computed(
  () =>
    sidebarPrefs.smartCollections &&
    !collectionsLocked.value &&
    !tagsLocked.value,
)
const showSections = computed(
  () => showCollections.value || showSmartCollections.value,
)

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
  <div
    class="relative h-full shrink-0 pt-2 pb-2 pl-2 transition-[width] duration-200 ease-in-out"
    :style="{ width: panels.leftCollapsed ? '0px' : '16.5rem' }"
  >
    <aside
      class="h-full w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg"
    >
      <div
        class="flex h-full w-64 flex-col"
        :inert="panels.leftCollapsed || undefined"
      >
        <div v-if="props.embed" class="px-4 pt-4 pb-1">
          <p
            class="text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
          >
            Library
          </p>
          <p class="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
            {{ activeLibrary?.name ?? 'Library' }}
          </p>
        </div>

        <div v-else class="p-3 pb-0">
          <LibrarySwitcher />
        </div>

        <nav class="space-y-1 p-3">
          <RouterLink
            :to="{ name: 'all' }"
            class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
            :class="
              allActive
                ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300'
                : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white'
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
          v-if="showSections"
          layout="vertical"
          state-key="godam:sidebar:sections"
          state-storage="local"
          :gutter-size="14"
          class="min-h-0 flex-1"
        >
          <SplitterPanel v-if="showCollections" :size="60" :min-size="8">
            <div class="h-full overflow-y-auto pb-2">
              <CollectionTree />
            </div>
          </SplitterPanel>
          <SplitterPanel v-if="showSmartCollections" :size="40" :min-size="8">
            <div class="h-full overflow-y-auto pb-2">
              <SmartCollections />
            </div>
          </SplitterPanel>
        </Splitter>

        <UserMenu v-if="!props.embed" />
      </div>
    </aside>

    <button
      type="button"
      class="absolute top-1/2 left-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200 dark:border-slate-800/80 bg-white dark:bg-slate-900/80 text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white dark:hover:bg-slate-800 hover:text-violet-600 dark:hover:text-violet-400"
      :class="
        panels.leftCollapsed
          ? 'translate-x-0 rounded-r-full'
          : '-translate-x-1/2 rounded-full'
      "
      :title="panels.leftCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      :aria-label="panels.leftCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      @click="toggleLeftPanel"
    >
      <ChevronRight v-if="panels.leftCollapsed" :size="12" />
      <ChevronLeft v-else :size="12" />
    </button>
  </div>
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

:global(.dark) :deep(.p-splitter-gutter)::before {
  background-color: #1e293b;
}

:global(.dark) :deep(.p-splitter-gutter:hover)::before {
  background-color: #475569;
}

:global(.dark) :deep(.p-splitter-gutter-handle) {
  border-color: #334155;
  background-color: #0f172a;
}
