<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import Popover from 'primevue/popover'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import ToggleSwitch from 'primevue/toggleswitch'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import { ExternalLink, Sidebar, SlidersH, Trash, Upload } from '@primeicons/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import * as api from '../api/client'
import { endAssetDrag, startAssetDrag } from '../lib/assetDrag'
import { CARD_SHAPES, CARD_SIZES } from '../lib/display'
import type { CardShape, CardSize } from '../lib/display'
import { library, loadCollections } from '../lib/library'
import { openUpload } from '../lib/upload'
import type { Asset } from '../types'
import AssetCard from './AssetCard.vue'
import AssetDetailPanel from './AssetDetailPanel.vue'
import CollectionsPanel from './CollectionsPanel.vue'
import MosaicWall from './MosaicWall.vue'
import MultiSelectionPanel from './MultiSelectionPanel.vue'

type CollectionsMode = 'full' | 'rail' | 'hidden'

const COLLECTIONS_MIN_PX = 200
const RAIL_PX = 56
const LEFT_WIDTH_KEY = 'godam.collectionsWidth'

const route = useRoute()
const confirm = useConfirm()
const toast = useToast()

const assets = ref<Asset[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const selectedIds = ref<string[]>([])
const anchorId = ref<string | null>(null)

const typeFilter = ref('')
const tagFilter = ref('')
const sort = ref('date-desc')

const size = ref<CardSize>('medium')
const shape = ref<CardShape>('square')
const mosaic = ref(false)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

const contextMenu = ref<{ show: (event: MouseEvent) => void } | null>(null)

// Collections panel sizing: the full panel is resizable (width in px kept in
// localStorage, min 200px); the icon rail is a fixed, non-resizable width.
const wrapperEl = ref<HTMLElement | null>(null)
const containerWidth = ref(0)
const collectionsMode = ref<CollectionsMode>('full')
const sizes = ref([22, 78, 0])
const leftPx = ref(loadLeftWidth())
let resizeObserver: ResizeObserver | undefined

function loadLeftWidth(): number {
  const raw = Number(localStorage.getItem(LEFT_WIDTH_KEY))
  return Number.isFinite(raw) && raw >= COLLECTIONS_MIN_PX ? raw : 260
}

function toPercent(px: number, fallback: number): number {
  return containerWidth.value > 0 ? (px / containerWidth.value) * 100 : fallback
}

const collectionsRail = computed(() => collectionsMode.value === 'rail')
const collectionsMinPercent = computed(() => toPercent(COLLECTIONS_MIN_PX, 16))
const railPercent = computed(() => toPercent(RAIL_PX, 6))
const fullLeftPercent = computed(() => toPercent(leftPx.value, 22))
const leftMinSize = computed(() =>
  collectionsRail.value ? railPercent.value : collectionsMinPercent.value,
)
const leftMaxSize = computed(() =>
  collectionsRail.value ? railPercent.value : 70,
)
const collectionsToggleTitle = computed(() => {
  switch (collectionsMode.value) {
    case 'full':
      return 'Collapse collections to icons'
    case 'rail':
      return 'Hide collections'
    default:
      return 'Show collections'
  }
})

const collectionId = computed(
  () => (route.params.collectionId as string | undefined) ?? null,
)
const selectedSet = computed(() => new Set(selectedIds.value))
const selectedAssets = computed(() =>
  assets.value.filter((asset) => selectedSet.value.has(asset.id)),
)
const singleAsset = computed(() =>
  selectedIds.value.length === 1 ? (selectedAssets.value[0] ?? null) : null,
)
const hasSelection = computed(() => selectedIds.value.length > 0)

const types = computed(() => {
  const found = new Set(assets.value.map((a) => a.mime_type))
  return [...found].sort()
})

const tags = computed(() => {
  const found = new Set<string>()
  for (const asset of assets.value) {
    for (const tag of asset.tags ?? []) found.add(tag.name)
  }
  return [...found].sort()
})

const visibleAssets = computed(() => {
  let list = assets.value
  if (typeFilter.value) {
    list = list.filter((a) => a.mime_type === typeFilter.value)
  }
  if (tagFilter.value) {
    list = list.filter((a) =>
      (a.tags ?? []).some((t) => t.name === tagFilter.value),
    )
  }

  const sorted = [...list]
  switch (sort.value) {
    case 'date-asc':
      sorted.sort((a, b) => a.created_at.localeCompare(b.created_at))
      break
    case 'name-asc':
      sorted.sort((a, b) => a.filename.localeCompare(b.filename))
      break
    case 'name-desc':
      sorted.sort((a, b) => b.filename.localeCompare(a.filename))
      break
    case 'size-asc':
      sorted.sort((a, b) => a.byte_size - b.byte_size)
      break
    case 'size-desc':
      sorted.sort((a, b) => b.byte_size - a.byte_size)
      break
    default:
      sorted.sort((a, b) => b.created_at.localeCompare(a.created_at))
  }
  return sorted
})

const gridStyle = computed(() => {
  const min =
    size.value === 'small' ? '96px' : size.value === 'large' ? '180px' : '132px'
  return { gridTemplateColumns: `repeat(auto-fill, minmax(${min}, 1fr))` }
})

const mosaicRowHeight = computed(() => {
  switch (size.value) {
    case 'small':
      return '150px'
    case 'large':
      return '280px'
    default:
      return '200px'
  }
})

const menuItems = computed<MenuItem[]>(() => {
  const count = selectedIds.value.length
  const items: MenuItem[] = []
  if (count === 1) {
    items.push({
      label: 'Open original',
      icon: ExternalLink,
      command: openOriginal,
    })
  }
  items.push({
    label: count > 1 ? `Delete ${count} assets` : 'Delete',
    icon: Trash,
    class: 'text-red-600',
    command: () => confirmDelete(),
  })
  return items
})

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    assets.value = await api.listAssets(collectionId.value ?? undefined)
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    loading.value = false
  }
}

function onSelect(asset: Asset, event: MouseEvent): void {
  if (event.shiftKey && anchorId.value) {
    const list = visibleAssets.value
    const from = list.findIndex((item) => item.id === anchorId.value)
    const to = list.findIndex((item) => item.id === asset.id)
    if (from !== -1 && to !== -1) {
      const [lo, hi] = from < to ? [from, to] : [to, from]
      selectedIds.value = list.slice(lo, hi + 1).map((item) => item.id)
      return
    }
  }
  if (event.metaKey || event.ctrlKey) {
    selectedIds.value = selectedSet.value.has(asset.id)
      ? selectedIds.value.filter((id) => id !== asset.id)
      : [...selectedIds.value, asset.id]
    anchorId.value = asset.id
    return
  }
  selectedIds.value = [asset.id]
  anchorId.value = asset.id
}

function clearSelection(): void {
  selectedIds.value = []
  anchorId.value = null
}

function onContextMenu(event: MouseEvent, asset: Asset): void {
  if (!selectedSet.value.has(asset.id)) {
    selectedIds.value = [asset.id]
    anchorId.value = asset.id
  }
  contextMenu.value?.show(event)
}

function onDragStart(asset: Asset, event: DragEvent): void {
  if (!selectedSet.value.has(asset.id)) {
    selectedIds.value = [asset.id]
    anchorId.value = asset.id
  }
  const ids = selectedIds.value.slice()
  startAssetDrag(ids)
  if (event.dataTransfer) {
    event.dataTransfer.setData('text/plain', JSON.stringify(ids))
    event.dataTransfer.effectAllowed = 'copy'
  }
}

function onDragEnd(): void {
  endAssetDrag()
}

function openOriginal(): void {
  const asset = selectedAssets.value[0]
  if (asset) {
    window.open(api.assetContentUrl(asset.id), '_blank', 'noopener')
  }
}

function confirmDelete(): void {
  const ids = selectedIds.value.slice()
  if (!ids.length) return
  const single = ids.length === 1 ? selectedAssets.value[0] : null
  confirm.require({
    header: ids.length > 1 ? 'Delete assets' : 'Delete asset',
    message: single
      ? `Delete '${single.filename}'? This cannot be undone.`
      : `Delete ${ids.length} assets? This cannot be undone.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: async () => {
      try {
        await Promise.all(ids.map((id) => api.deleteAsset(id)))
        clearSelection()
        await load()
        await loadCollections()
        toast.add({
          severity: 'success',
          summary: ids.length > 1 ? `${ids.length} assets deleted` : 'Asset deleted',
          life: 2500,
        })
      } catch (e) {
        toast.add({
          severity: 'error',
          summary: 'Delete failed',
          detail: toMessage(e),
          life: 4000,
        })
      }
    },
  })
}

function toggleDisplay(event: Event): void {
  displayPopover.value?.toggle(event)
}

function applyCollectionsMode(mode: CollectionsMode): void {
  collectionsMode.value = mode
  const right = sizes.value[2] ?? 0
  let left: number
  if (mode === 'full') {
    left = fullLeftPercent.value
  } else if (mode === 'rail') {
    left = railPercent.value
  } else {
    left = 0
  }
  sizes.value = [left, Math.max(0, 100 - left - right), right]
}

function cycleCollections(): void {
  if (collectionsMode.value === 'full') {
    applyCollectionsMode('rail')
  } else if (collectionsMode.value === 'rail') {
    applyCollectionsMode('hidden')
  } else {
    applyCollectionsMode('full')
  }
}

function expandDetailPanel(): void {
  const left = sizes.value[0] ?? 0
  const right = (sizes.value[2] ?? 0) > 0 ? (sizes.value[2] ?? 26) : 26
  sizes.value = [left, Math.max(0, 100 - left - right), right]
}

function collapseDetailPanel(): void {
  const left = sizes.value[0] ?? 0
  sizes.value = [left, Math.max(0, 100 - left), 0]
}

function onCollapse(event: { index: number; collapsed: boolean }): void {
  if (event.index === 0) {
    collectionsMode.value = event.collapsed ? 'hidden' : 'full'
  }
  if (event.index === 2 && event.collapsed && hasSelection.value) {
    clearSelection()
  }
}

function onChanged(): void {
  void load()
}

watch(collectionId, () => {
  clearSelection()
  typeFilter.value = ''
  tagFilter.value = ''
  void load()
})

watch(hasSelection, (open) => {
  if (open) {
    expandDetailPanel()
  } else {
    collapseDetailPanel()
  }
})

watch(
  () => library.version,
  () => {
    void load()
  },
)

// Persist the full collections panel width (px) while it is the active mode.
watch(sizes, (value) => {
  if (collectionsMode.value !== 'full' || containerWidth.value <= 0) return
  const px = ((value[0] ?? 0) / 100) * containerWidth.value
  if (px >= COLLECTIONS_MIN_PX - 1) {
    leftPx.value = Math.round(px)
    localStorage.setItem(LEFT_WIDTH_KEY, String(leftPx.value))
  }
})

// Recompute percentages from the stored pixel width when the pane resizes.
watch(containerWidth, () => {
  applyCollectionsMode(collectionsMode.value)
})

onMounted(() => {
  if (wrapperEl.value) {
    resizeObserver = new ResizeObserver((entries) => {
      containerWidth.value = entries[0].contentRect.width
    })
    resizeObserver.observe(wrapperEl.value)
  }
  void loadCollections()
  void load()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
})
</script>

<template>
  <div ref="wrapperEl" class="flex h-full flex-col">
    <Message v-if="error" severity="error" class="mb-4">{{ error }}</Message>

    <Splitter
      v-model:sizes="sizes"
      class="min-h-0 flex-1 overflow-hidden rounded border border-slate-200 bg-white"
      @collapse="onCollapse"
    >
      <SplitterPanel
        :min-size="leftMinSize"
        :max-size="leftMaxSize"
        :collapsible="true"
        :collapsed-size="0"
        class="overflow-hidden"
      >
        <CollectionsPanel :rail="collectionsRail" />
      </SplitterPanel>

      <SplitterPanel class="overflow-hidden">
        <div class="flex h-full flex-col">
          <div
            class="flex flex-wrap items-center gap-3 border-b border-slate-200 p-3"
          >
            <Button
              text
              severity="secondary"
              :title="collectionsToggleTitle"
              aria-label="Toggle collections"
              @click="cycleCollections"
            >
              <template #icon><Sidebar :size="16" /></template>
            </Button>

            <select
              v-model="typeFilter"
              class="rounded border border-slate-300 bg-white px-3 py-1.5 text-sm"
            >
              <option value="">All types</option>
              <option v-for="type in types" :key="type" :value="type">
                {{ type }}
              </option>
            </select>

            <select
              v-model="tagFilter"
              class="rounded border border-slate-300 bg-white px-3 py-1.5 text-sm"
            >
              <option value="">All tags</option>
              <option v-for="tag in tags" :key="tag" :value="tag">
                #{{ tag }}
              </option>
            </select>

            <select
              v-model="sort"
              class="rounded border border-slate-300 bg-white px-3 py-1.5 text-sm"
            >
              <option value="date-desc">Newest first</option>
              <option value="date-asc">Oldest first</option>
              <option value="name-asc">Name A–Z</option>
              <option value="name-desc">Name Z–A</option>
              <option value="size-desc">Largest first</option>
              <option value="size-asc">Smallest first</option>
            </select>

            <Button label="Upload" @click="openUpload([], collectionId)">
              <template #icon><Upload :size="16" /></template>
            </Button>

            <span class="ml-auto text-sm text-slate-400">
              {{
                selectedIds.length
                  ? `${selectedIds.length} selected`
                  : `${visibleAssets.length} item(s)`
              }}
            </span>

            <Button
              severity="secondary"
              title="Display settings"
              aria-label="Display settings"
              @click="toggleDisplay"
            >
              <template #icon><SlidersH :size="16" /></template>
            </Button>

            <Popover ref="displayPopover">
              <div class="w-64 space-y-4">
                <div>
                  <p
                    class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Size
                  </p>
                  <div class="flex gap-1">
                    <button
                      v-for="option in CARD_SIZES"
                      :key="option.value"
                      type="button"
                      class="flex-1 rounded px-2 py-1.5 text-sm"
                      :class="
                        size === option.value
                          ? 'bg-violet-600 text-white'
                          : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                      "
                      @click="size = option.value"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                </div>

                <div>
                  <p
                    class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Shape
                  </p>
                  <div class="flex flex-wrap gap-1">
                    <button
                      v-for="option in CARD_SHAPES"
                      :key="option.value"
                      type="button"
                      class="rounded px-2 py-1.5 text-sm"
                      :class="
                        shape === option.value
                          ? 'bg-violet-600 text-white'
                          : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                      "
                      @click="shape = option.value"
                    >
                      {{ option.label }}
                    </button>
                  </div>
                </div>

                <div
                  class="flex items-center justify-between border-t border-slate-100 pt-3"
                >
                  <div>
                    <p class="text-sm font-medium">Mosaic</p>
                    <p class="text-xs text-slate-400">
                      Full-bleed wall, no captions
                    </p>
                  </div>
                  <ToggleSwitch v-model="mosaic" />
                </div>
              </div>
            </Popover>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto p-3">
            <p v-if="loading" class="text-sm text-slate-400">Loading…</p>
            <p v-else-if="!assets.length" class="text-sm text-slate-400">
              No assets yet.
            </p>
            <p v-else-if="!visibleAssets.length" class="text-sm text-slate-400">
              No assets match the current filters.
            </p>
            <MosaicWall
              v-else-if="mosaic"
              :assets="visibleAssets"
              :row-height="mosaicRowHeight"
              :selected-ids="selectedIds"
              @open="onSelect"
              @menu="onContextMenu"
              @dragstart="onDragStart"
              @dragend="onDragEnd"
            />
            <div v-else class="grid gap-3" :style="gridStyle">
              <AssetCard
                v-for="asset in visibleAssets"
                :key="asset.id"
                :asset="asset"
                :shape="shape"
                :selected="selectedSet.has(asset.id)"
                @open="onSelect"
                @menu="onContextMenu"
                @dragstart="onDragStart"
                @dragend="onDragEnd"
              />
            </div>
          </div>
        </div>
      </SplitterPanel>

      <SplitterPanel
        :min-size="18"
        :collapsible="true"
        :collapsed-size="0"
        class="overflow-hidden"
      >
        <AssetDetailPanel
          v-if="singleAsset"
          :asset="singleAsset"
          @close="clearSelection"
          @changed="onChanged"
        />
        <MultiSelectionPanel
          v-else-if="selectedAssets.length > 1"
          :assets="selectedAssets"
          @close="clearSelection"
          @remove="confirmDelete"
        />
      </SplitterPanel>
    </Splitter>

    <ContextMenu ref="contextMenu" :model="menuItems" />
  </div>
</template>
