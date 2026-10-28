<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import { useToast } from 'primevue/usetoast'
import {
  AngleDoubleDown,
  AngleDoubleUp,
  Plus,
  Search,
  SortAlt,
  Times,
} from '@primeicons/vue'
import { computed, nextTick, ref, watch } from 'vue'
import * as api from '../api/client'
import { collectionsDialog } from '../lib/collectionsDialog'
import { upload } from '../lib/upload'
import { library, notifyLibraryChanged } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import { splitNames } from '../lib/names'
import { useAnimatedHeight } from '../lib/useAnimatedHeight'
import CollectionTree from './CollectionTree.vue'
import PillBadge from './PillBadge.vue'
import Kbd from './Kbd.vue'

const toast = useToast()
const searchRef = ref<HTMLInputElement | null>(null)
const contentRef = ref<HTMLElement | null>(null)
const { setWrapper: setHeightWrapper, style: heightStyle } = useAnimatedHeight(contentRef)
const treeRef = ref<InstanceType<typeof CollectionTree> | null>(null)
const search = ref('')
const selectedIds = ref<string[]>([])
const treeChanged = ref(false)

let initialIds = new Set<string>()
let selectionLoaded = false
let loadRequest = 0

const selectedCount = computed(() => selectedIds.value.length)
const allExpanded = computed(() => Boolean(treeRef.value?.allExpanded))
const isBulk = computed(
  () =>
    collectionsDialog.assetIds.length > 0 && collectionsDialog.mode !== null,
)
const hasTarget = computed(
  () =>
    collectionsDialog.upload ||
    Boolean(collectionsDialog.assetId) ||
    isBulk.value,
)
const selectedSet = computed(() => new Set(selectedIds.value))
const selectedCollections = computed(() =>
  library.collections.filter((collection) => selectedSet.value.has(collection.id)),
)
const searchTerm = computed(() => search.value.trim())
const searchNames = computed(() => splitNames(searchTerm.value))
const existingNames = computed(() =>
  searchNames.value.filter((name) =>
    library.collections.some(
      (collection) =>
        collection.name.trim().toLowerCase() === name.toLowerCase(),
    ),
  ),
)
const newNames = computed(() =>
  searchNames.value.filter(
    (name) =>
      !library.collections.some(
        (collection) =>
          collection.name.trim().toLowerCase() === name.toLowerCase(),
      ),
  ),
)
const showCreate = computed(() => newNames.value.length > 0)

watch(
  () => collectionsDialog.visible,
  (open) => {
    if (open) void initialize()
    else void finalize()
  },
)

async function finalize(): Promise<void> {
  const selectionChanged = await applySelection()
  if (selectionChanged || treeChanged.value) notifyLibraryChanged()
  treeChanged.value = false
}

async function initialize(): Promise<void> {
  search.value = ''
  const request = ++loadRequest
  if (collectionsDialog.upload) {
    selectedIds.value = [...upload.destinationCollectionIds]
    initialIds = new Set(selectedIds.value)
    selectionLoaded = true
    nextTick(focusSearch)
    return
  }
  if (isBulk.value) {
    selectedIds.value = []
    initialIds = new Set()
    selectionLoaded = true
    nextTick(focusSearch)
    return
  }
  selectedIds.value = []
  initialIds = new Set()
  selectionLoaded = false
  const assetId = collectionsDialog.assetId
  if (!assetId) {
    nextTick(focusSearch)
    return
  }
  try {
    const asset = await api.getAsset(assetId)
    if (request !== loadRequest) return
    const ids = (asset.collections ?? []).map((collection) => collection.id)
    selectedIds.value = ids
    initialIds = new Set(ids)
    selectionLoaded = true
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not load collections',
      detail: toMessage(e),
      life: 4000,
    })
  }
  nextTick(focusSearch)
}

async function applySelection(): Promise<boolean> {
  if (collectionsDialog.upload) {
    upload.destinationCollectionIds = [...selectedIds.value]
    return false
  }
  if (isBulk.value && collectionsDialog.mode) {
    try {
      await api.applyAssetsCollections(
        collectionsDialog.assetIds,
        selectedIds.value,
        collectionsDialog.mode,
      )
      return true
    } catch (e) {
      toast.add({
        severity: 'error',
        summary: 'Could not update collections',
        detail: toMessage(e),
        life: 4000,
      })
      return false
    }
  }
  const assetId = collectionsDialog.assetId
  if (!assetId || !selectionLoaded) return false
  selectionLoaded = false
  const target = new Set(selectedIds.value)
  const added = [...target].filter((id) => !initialIds.has(id))
  const removed = [...initialIds].filter((id) => !target.has(id))
  if (!added.length && !removed.length) return false
  try {
    await Promise.all([
      ...added.map((id) => api.addToCollection(assetId, id)),
      ...removed.map((id) => api.removeFromCollection(assetId, id)),
    ])
    return true
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update collections',
      detail: toMessage(e),
      life: 4000,
    })
    return false
  }
}

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function focusSearch(): void {
  searchRef.value?.focus()
}

function addCollection(): void {
  treeRef.value?.addCollection(null)
}

function toggleExpandAll(): void {
  treeRef.value?.toggleExpandAll()
}

function toggleSort(event: Event): void {
  treeRef.value?.toggleSort(event)
}

async function createFromSearch(): Promise<void> {
  const names = searchNames.value
  if (!names.length) return
  const existingIds = library.collections
    .filter((collection) =>
      names.some(
        (name) => collection.name.trim().toLowerCase() === name.toLowerCase(),
      ),
    )
    .map((collection) => collection.id)
  if (hasTarget.value && existingIds.length) {
    selectedIds.value = [...new Set([...selectedIds.value, ...existingIds])]
  }
  if (existingNames.value.length) {
    toast.add({
      severity: 'info',
      summary: 'Skipped existing collections',
      detail: existingNames.value.join(', '),
      life: 3000,
    })
  }
  for (const name of newNames.value) {
    await treeRef.value?.createCollection(name)
  }
  search.value = ''
  nextTick(focusSearch)
}

function clearSelection(): void {
  selectedIds.value = []
}

function removeSelection(id: string): void {
  selectedIds.value = selectedIds.value.filter((item) => item !== id)
}
</script>

<template>
  <Dialog
    v-model:visible="collectionsDialog.visible"
    modal
    dismissable-mask
    :show-header="false"
    :style="{ width: 'min(34rem, 94vw)' }"
    :content-style="{ maxHeight: 'min(42rem, 86vh)', padding: '0' }"
    @show="focusSearch"
  >
    <div :ref="setHeightWrapper" class="overflow-hidden" :style="heightStyle">
      <div
        ref="contentRef"
        class="flex min-h-0 flex-col"
        :style="{
          maxHeight: 'min(42rem, 86vh)',
          minHeight: 'min(30rem, 80vh)',
        }"
      >
      <div
        class="flex items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-slate-800"
      >
        <Search :size="16" class="shrink-0 text-slate-400" />
        <input
          ref="searchRef"
          v-model="search"
          type="text"
          placeholder="Search collections…"
          class="min-w-0 flex-1 border-0 bg-transparent text-sm outline-none"
          @keydown.enter.prevent="createFromSearch"
        />
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-brand-700 dark:hover:bg-slate-800 dark:hover:text-brand-400"
          title="New collection"
          @click="addCollection"
        >
          <Plus :size="16" />
        </button>
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-brand-700 dark:hover:bg-slate-800 dark:hover:text-brand-400"
          title="Sort collections"
          @click="toggleSort"
        >
          <SortAlt :size="16" />
        </button>
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-brand-700 dark:hover:bg-slate-800 dark:hover:text-brand-400"
          :title="allExpanded ? 'Collapse all' : 'Expand all'"
          @click="toggleExpandAll"
        >
          <AngleDoubleUp v-if="allExpanded" :size="16" />
          <AngleDoubleDown v-else :size="16" />
        </button>
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
          title="Close"
          @click="collectionsDialog.visible = false"
        >
          <Times :size="16" />
        </button>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto pt-2">
        <button
          v-if="showCreate"
          type="button"
          class="flex w-full items-center gap-2 border-b border-slate-100 px-2 py-2 text-left text-sm text-brand-700 hover:bg-brand-50 dark:border-slate-800 dark:text-brand-300 dark:hover:bg-brand-950/40"
          @click="createFromSearch"
        >
          <Plus :size="15" class="shrink-0" />
          <span class="min-w-0 flex-1 truncate">
            <template v-if="newNames.length === 1">
              Create “{{ newNames[0] }}”
            </template>
            <template v-else>
              Create {{ newNames.length }} collections
            </template>
          </span>
          <Kbd>↵</Kbd>
        </button>
        <CollectionTree
          ref="treeRef"
          v-model:selected-ids="selectedIds"
          :selectable="hasTarget"
          :navigable="false"
          :defer-refresh="true"
          :show-header="false"
          :filter="search"
          @change="treeChanged = true"
          @open="collectionsDialog.visible = false"
        />
      </div>

      <div
        v-if="hasTarget && selectedCollections.length"
        class="max-h-28 shrink-0 overflow-y-auto border-t border-slate-200 px-2 py-2 dark:border-slate-800"
      >
        <div class="flex flex-wrap gap-1">
          <PillBadge
            v-for="collection in selectedCollections"
            :key="collection.id"
            :name="collection.name"
            :color="libraryColorHex(collection.color)"
          >
            <button
              class="inline-flex text-slate-400 hover:text-red-500"
              title="Remove"
              @click="removeSelection(collection.id)"
            >
              <Times :size="12" />
            </button>
          </PillBadge>
        </div>
      </div>

      <div
        class="flex items-center justify-between border-t border-slate-200 px-3 py-2 dark:border-slate-800"
      >
        <span class="text-xs text-slate-400">
          <template v-if="hasTarget">{{ selectedCount }} selected</template>
          <template v-else>
            {{ library.collections.length }} collection{{ library.collections.length === 1 ? '' : 's' }}
          </template>
        </span>
        <div class="flex items-center gap-2">
          <Button
            v-if="hasTarget"
            label="Clear"
            text
            size="small"
            :disabled="!selectedCount"
            @click="clearSelection"
          />
          <Button
            label="Done"
            size="small"
            @click="collectionsDialog.visible = false"
          />
        </div>
        </div>
      </div>
    </div>
  </Dialog>
</template>
