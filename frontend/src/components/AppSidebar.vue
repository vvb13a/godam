<script setup lang="ts">
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { ChevronLeft, ChevronRight, Images } from '@primeicons/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import * as api from '../api/client'
import { embed as embedState } from '../lib/embed'
import { currentLibrary, library, refreshLibrary } from '../lib/library'
import { panels, toggleLeftPanel } from '../lib/panels'
import { hasCapability } from '../lib/capabilities'
import { leaveEditor } from '../lib/editorState'
import { clearAllFilters, hasActiveFilterState } from '../lib/filters'
import { sidebarPrefs, type SidebarToggleKey } from '../lib/sidebar'
import CollectionTree from './CollectionTree.vue'
import Quicklinks from './Quicklinks.vue'
import SmartCollections from './SmartCollections.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const activeLibrary = computed(() => currentLibrary())

const editingName = ref(false)
const nameDraft = ref('')
const nameInput = ref<HTMLInputElement | null>(null)
const renaming = ref(false)
let libraryClickTimer: number | null = null

function onLibraryClick(): void {
  if (libraryClickTimer !== null) {
    window.clearTimeout(libraryClickTimer)
    libraryClickTimer = null
    return
  }
  libraryClickTimer = window.setTimeout(() => {
    libraryClickTimer = null
    openLibrary()
  }, 220)
}

function startRename(): void {
  if (!hasCapability('library:update')) return
  if (libraryClickTimer !== null) {
    window.clearTimeout(libraryClickTimer)
    libraryClickTimer = null
  }
  if (!activeLibrary.value) return
  nameDraft.value = activeLibrary.value.name
  editingName.value = true
  nextTick(() => {
    nameInput.value?.focus()
    nameInput.value?.select()
  })
}

function cancelRename(): void {
  editingName.value = false
}

async function commitRename(): Promise<void> {
  if (!editingName.value || renaming.value) return
  const name = nameDraft.value.trim()
  if (!name || name === activeLibrary.value?.name) {
    editingName.value = false
    return
  }
  renaming.value = true
  try {
    const updated = await api.updateInstance(name, activeLibrary.value?.color ?? '')
    Object.assign(library, updated)
    editingName.value = false
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  } finally {
    renaming.value = false
  }
}

const pickerRestricted = computed(
  () => embedState.active && embedState.mode !== 'library',
)
const collectionsLocked = computed(
  () => pickerRestricted.value && embedState.collectionLocked,
)
const SECTION_COMPONENTS = {
  quicklinks: Quicklinks,
  collections: CollectionTree,
  smartCollections: SmartCollections,
}

const SECTION_SPACE: Record<SidebarToggleKey, number> = {
  quicklinks: 18,
  collections: 34,
  smartCollections: 22,
}

const sectionVisible = (key: SidebarToggleKey): boolean => {
  switch (key) {
    case 'quicklinks':
      return sidebarPrefs.quicklinks
    case 'collections':
      return sidebarPrefs.collections && !collectionsLocked.value
    case 'smartCollections':
      return sidebarPrefs.smartCollections && !collectionsLocked.value
  }
}

const visibleSections = computed<SidebarToggleKey[]>(() =>
  sidebarPrefs.order.filter(sectionVisible),
)

const sectionSizes = computed<Record<SidebarToggleKey, number>>(() => {
  const keys = visibleSections.value
  const total = keys.reduce((sum, key) => sum + SECTION_SPACE[key], 0)
  const sizes = {} as Record<SidebarToggleKey, number>
  for (const key of keys) {
    sizes[key] = total > 0 ? (SECTION_SPACE[key] / total) * 100 : 100
  }
  return sizes
})

const splitterStateKey = computed(
  () => `godam:sidebar:sections:${visibleSections.value.join('|')}`,
)

const libraryActive = computed(
  () => route.name === 'library' && !hasActiveFilterState(),
)

async function openLibrary(): Promise<void> {
  if (!(await leaveEditor())) return
  clearAllFilters()
  void router.push({ name: 'library' })
}

watch(
  () => library.version,
  () => {
    void refreshLibrary()
  },
)
</script>

<template>
  <div
    class="relative h-full shrink-0 transition-[width] duration-200 ease-in-out"
    :class="panels.leftCollapsed ? '' : 'px-4'"
    :style="{ width: panels.leftCollapsed ? '0px' : '22rem' }"
  >
    <aside
      class="h-full w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg"
    >
      <div
        class="flex h-full w-full flex-col"
        :inert="panels.leftCollapsed || undefined"
      >
        <div
          class="cursor-pointer border-b border-slate-100 px-4 py-3 transition dark:border-slate-800"
          :class="
            libraryActive
              ? 'bg-brand-50/80 dark:bg-brand-950/40'
              : 'hover:bg-slate-50 dark:hover:bg-slate-800/50'
          "
          role="button"
          tabindex="0"
          title="Show all assets"
          @click="onLibraryClick"
          @keydown.enter.prevent="openLibrary"
          @keydown.space.prevent="openLibrary"
        >
          <div class="flex items-center justify-between gap-2">
            <p
              class="text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Library
            </p>
            <span
              class="inline-flex items-center gap-1 text-xs font-medium"
              :class="
                libraryActive
                  ? 'text-brand-700 dark:text-brand-300'
                  : 'text-slate-400'
              "
            >
              <Images :size="13" />
              <span v-if="library.stats?.assets">
                {{ library.stats.assets }}
              </span>
            </span>
          </div>
          <input
            v-if="editingName"
            ref="nameInput"
            v-model="nameDraft"
            type="text"
            class="w-full border-b border-brand-400 bg-transparent p-0 text-sm leading-5 font-semibold text-slate-700 outline-none dark:text-slate-200"
            :disabled="renaming"
            @click.stop
            @keydown.stop
            @keydown.enter.prevent="commitRename"
            @keydown.esc.prevent="cancelRename"
            @blur="commitRename"
          />
          <p
            v-else
            class="truncate text-sm font-semibold text-slate-700 dark:text-slate-200"
            title="Double-click to rename"
            @dblclick.stop="startRename"
          >
            {{ activeLibrary?.name ?? 'Library' }}
          </p>
        </div>

        <Splitter
          v-if="visibleSections.length"
          :key="splitterStateKey"
          layout="vertical"
          :state-key="splitterStateKey"
          state-storage="local"
          :gutter-size="14"
          class="min-h-0 flex-1"
        >
          <SplitterPanel
            v-for="section in visibleSections"
            :key="section"
            :size="sectionSizes[section]"
            :min-size="8"
          >
            <div class="h-full overflow-y-auto pb-2">
              <component :is="SECTION_COMPONENTS[section]" />
            </div>
          </SplitterPanel>
        </Splitter>
      </div>
    </aside>

    <button
      type="button"
      class="absolute top-1/2 left-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200 dark:border-slate-800/80 bg-white dark:bg-slate-900/80 text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white dark:hover:bg-slate-800 hover:text-brand-700 dark:hover:text-brand-400"
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
