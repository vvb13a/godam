<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import Popover from 'primevue/popover'
import SelectButton from 'primevue/selectbutton'
import Slider from 'primevue/slider'
import ToggleSwitch from 'primevue/toggleswitch'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  ExternalLink,
  SlidersH,
  Tag,
  Upload,
} from '@primeicons/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { endAssetDrag, startAssetDrag } from '../lib/assetDrag'
import { confirmDeleteMenuItem } from '../lib/menu'
import {
  CARD_SHAPES,
  CARD_SIZE_DEFAULT,
  CARD_SIZE_MAX,
  CARD_SIZE_MIN,
} from '../lib/display'
import type { CardShape } from '../lib/display'
import { library, notifyLibraryChanged } from '../lib/library'
import { openAssetTags, openBulkAssetTags } from '../lib/tagsDialog'
import { openUpload } from '../lib/upload'
import type { Asset } from '../types'
import AssetCard from './AssetCard.vue'
import AssetDetailPanel from './AssetDetailPanel.vue'
import MultiSelectionPanel from './MultiSelectionPanel.vue'

const route = useRoute()
const router = useRouter()
const confirm = useConfirm()
const toast = useToast()

const assets = ref<Asset[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const selectedIds = ref<string[]>([])
const anchorId = ref<string | null>(null)

const DISPLAY_STORAGE_KEY = 'godam:display'

interface DisplayPrefs {
  size: number
  shape: CardShape
  showExtension: boolean
  showName: boolean
  showSize: boolean
}

function loadDisplayPrefs(): Partial<DisplayPrefs> {
  try {
    const raw = localStorage.getItem(DISPLAY_STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : null
    return parsed && typeof parsed === 'object'
      ? (parsed as Partial<DisplayPrefs>)
      : {}
  } catch {
    return {}
  }
}

function clampSize(value: number): number {
  if (!Number.isFinite(value)) return CARD_SIZE_DEFAULT
  return Math.min(CARD_SIZE_MAX, Math.max(CARD_SIZE_MIN, Math.round(value)))
}

const savedPrefs = loadDisplayPrefs()
const size = ref(clampSize(savedPrefs.size ?? CARD_SIZE_DEFAULT))
const validShapes = CARD_SHAPES.map((option) => option.value)
const shape = ref<CardShape>(
  validShapes.includes(savedPrefs.shape as CardShape)
    ? (savedPrefs.shape as CardShape)
    : 'square',
)
const showExtension = ref(savedPrefs.showExtension ?? false)
const showName = ref(savedPrefs.showName ?? false)
const showSize = ref(savedPrefs.showSize ?? false)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

watch(
  [size, shape, showExtension, showName, showSize],
  () => {
    const prefs: DisplayPrefs = {
      size: clampSize(size.value),
      shape: shape.value,
      showExtension: showExtension.value,
      showName: showName.value,
      showSize: showSize.value,
    }
    try {
      localStorage.setItem(DISPLAY_STORAGE_KEY, JSON.stringify(prefs))
    } catch {
      // Storage unavailable: settings simply won't persist.
    }
  },
)

const contextMenu = ref<{ show: (event: MouseEvent) => void } | null>(null)

// Lasso (marquee) selection state. Coordinates are viewport-relative
// (clientX/clientY) so the rectangle and hit-testing stay correct while the
// grid scrolls. `data-asset-id` marks the hittable cards in AssetCard.
interface LassoRect {
  startX: number
  startY: number
  x: number
  y: number
}

interface LassoSession {
  startX: number
  startY: number
  additive: boolean
  base: string[]
}

const gridContainer = ref<HTMLElement | null>(null)
const lasso = ref<LassoRect | null>(null)
let lassoSession: LassoSession | null = null
const LASSO_THRESHOLD = 4

const lassoStyle = computed(() => {
  const rect = lasso.value
  if (!rect) return null
  return {
    left: `${Math.min(rect.startX, rect.x)}px`,
    top: `${Math.min(rect.startY, rect.y)}px`,
    width: `${Math.abs(rect.x - rect.startX)}px`,
    height: `${Math.abs(rect.y - rect.startY)}px`,
  }
})

// Collection and tag filters live in the route query so the sidebar and the
// toolbar stay in sync and the selection is shareable. Special values
// 'uncategorized' and 'untagged' select assets with no collection/tags.
const collectionFilter = computed({
  get: () => (route.query.collection as string | undefined) ?? '',
  set: (value: string) => setQueryFilter('collection', value),
})
const tagFilter = computed({
  get: () => {
    const raw = route.query.tag
    if (typeof raw !== 'string' || !raw) return [] as string[]
    return raw.split(',').filter(Boolean)
  },
  set: (values: string[]) => {
    const query = { ...route.query }
    if (values.length) query.tag = values.join(',')
    else delete query.tag
    void router.push({ name: 'all', query })
  },
})
const uploadCollectionId = computed(() =>
  collectionFilter.value && collectionFilter.value !== 'uncategorized'
    ? collectionFilter.value
    : null,
)

// The media-type and sort filters also live in the route query so a smart
// collection (a saved preset) can restore them.
const typeFilter = computed({
  get: () => (route.query.type as string | undefined) ?? '',
  set: (value: string) => setQueryFilter('type', value),
})
const sort = computed({
  get: () => (route.query.sort as string | undefined) ?? 'date-desc',
  set: (value: string) => setQueryFilter('sort', value),
})

// Broad media categories used by both the type filter and smart collections.
const TYPE_CATEGORIES = [
  { value: '', label: 'All types' },
  { value: 'image', label: 'Images' },
  { value: 'video', label: 'Videos' },
  { value: 'audio', label: 'Audio' },
  { value: 'document', label: 'Documents' },
  { value: 'other', label: 'Other' },
]

const DOCUMENT_MIMES = new Set([
  'application/pdf',
  'application/msword',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'application/vnd.ms-excel',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'application/vnd.ms-powerpoint',
  'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  'application/rtf',
])

function mimeCategory(mime: string): string {
  if (mime.startsWith('image/')) return 'image'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  if (mime.startsWith('text/') || DOCUMENT_MIMES.has(mime)) return 'document'
  return 'other'
}

function setQueryFilter(
  key: 'collection' | 'tag' | 'type' | 'sort',
  value: string,
): void {
  const query = { ...route.query }
  if (value) query[key] = value
  else delete query[key]
  void router.push({ name: 'all', query })
}

const tagOptions = computed(() => [
  { label: 'Untagged', value: 'untagged', color: undefined as string | undefined },
  ...library.tags.map((tag) => {
    const group = tag.group_id
      ? library.tagGroups.find((item) => item.id === tag.group_id)
      : undefined
    return {
      label: tag.name,
      value: tag.id,
      color: group?.color ? `#${group.color}` : undefined,
    }
  }),
])

const selectedSet = computed(() => new Set(selectedIds.value))
const selectedAssets = computed(() =>
  assets.value.filter((asset) => selectedSet.value.has(asset.id)),
)
const singleAsset = computed(() =>
  selectedIds.value.length === 1 ? (selectedAssets.value[0] ?? null) : null,
)

const visibleAssets = computed(() => {
  let list = assets.value
  if (typeFilter.value) {
    list = list.filter((a) => mimeCategory(a.mime_type) === typeFilter.value)
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

// The slider is the preview's shorter axis, so the grid column is widened for
// landscape shapes to keep the height equal to the slider value.
const shapeAspect = computed(() => {
  switch (shape.value) {
    case 'landscape':
      return 16 / 9
    case 'poster':
      return 2 / 3
    default:
      return 1
  }
})

const cardWidth = computed(() =>
  Math.round(size.value * Math.max(1, shapeAspect.value)),
)

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(auto-fill, ${cardWidth.value}px)`,
}))

// Native uses a spanned grid so each preview keeps its real proportions. The
// slider is the base cell: a square is 1x1, a landscape spans columns and a
// portrait spans rows.
const nativeUnit = computed(() => Math.max(1, Math.round(size.value)))
const nativeGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(auto-fill, ${nativeUnit.value}px)`,
  gridAutoRows: `${nativeUnit.value}px`,
  gridAutoFlow: 'dense',
}))

function nativeItemStyle(asset: Asset): Record<string, string> {
  const w = asset.metadata?.width ?? 0
  const h = asset.metadata?.height ?? 0
  const ratio = w > 0 && h > 0 ? w / h : 1
  // Classify with thresholds so ratios near the 2:3 / 3:2 rounding midpoints
  // (e.g. 1 / 0.6667 = 1.5) land consistently instead of flipping between
  // 1x1 and the spanning box.
  let cols = 1
  let rows = 1
  if (ratio >= 1.25) {
    cols = Math.max(2, Math.round(ratio))
  } else if (ratio <= 0.8) {
    rows = Math.max(2, Math.round(1 / ratio))
  }
  return { gridColumn: `span ${cols}`, gridRow: `span ${rows}` }
}

const menuItems = computed<MenuItem[]>(() => {
  const count = selectedIds.value.length
  const items: MenuItem[] = []
  if (count === 1) {
    items.push({
      label: 'Open original',
      icon: ExternalLink,
      command: openOriginal,
    })
    items.push({
      label: 'Tags',
      icon: Tag,
      command: openTags,
    })
  } else if (count > 1) {
    items.push({
      label: 'Tags',
      icon: Tag,
      items: [
        {
          label: 'Replace',
          command: () => openBulkAssetTags(selectedIds.value.slice(), 'replace'),
        },
        {
          label: 'Add',
          command: () => openBulkAssetTags(selectedIds.value.slice(), 'add'),
        },
      ],
    })
  }
  items.push(
    confirmDeleteMenuItem(
      count > 1 ? `Delete ${count} assets` : 'Delete',
      () => void performDelete(selectedIds.value.slice()),
    ),
  )
  return items
})

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function load(): Promise<void> {
  const libraryId = library.currentId
  if (!libraryId) {
    assets.value = []
    return
  }
  loading.value = true
  error.value = null
  try {
    const params: api.AssetListParams = { libraryId }
    const c = collectionFilter.value
    if (c === 'uncategorized') params.uncategorized = true
    else if (c) params.collectionId = c

    const tags = tagFilter.value
    if (tags.includes('untagged')) params.untagged = true
    const tagIds = tags.filter((value) => value !== 'untagged')
    if (tagIds.length) params.tagIds = tagIds

    assets.value = await api.listAssets(params)
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

// onLassoStart begins a marquee drag when the press lands on empty grid
// background. Pressing a card (or any control) is left to the normal click /
// native-drag handling. Ctrl/Cmd/Shift keeps the existing selection.
function onLassoStart(event: MouseEvent): void {
  if (event.button !== 0) return
  const target = event.target as HTMLElement | null
  if (!target || target.closest('[data-asset-id], button, a, input, select, textarea')) {
    return
  }
  lassoSession = {
    startX: event.clientX,
    startY: event.clientY,
    additive: event.metaKey || event.ctrlKey || event.shiftKey,
    base: selectedIds.value.slice(),
  }
  window.addEventListener('mousemove', onLassoMove)
  window.addEventListener('mouseup', onLassoEnd)
  event.preventDefault()
}

function onLassoMove(event: MouseEvent): void {
  const session = lassoSession
  if (!session) return
  if (
    !lasso.value &&
    Math.hypot(event.clientX - session.startX, event.clientY - session.startY) <
      LASSO_THRESHOLD
  ) {
    return
  }
  lasso.value = {
    startX: session.startX,
    startY: session.startY,
    x: event.clientX,
    y: event.clientY,
  }
  applyLasso(event.clientX, event.clientY)
}

// applyLasso selects every card whose bounding box intersects the marquee.
function applyLasso(x: number, y: number): void {
  const session = lassoSession
  const container = gridContainer.value
  if (!session || !container) return
  const left = Math.min(session.startX, x)
  const right = Math.max(session.startX, x)
  const top = Math.min(session.startY, y)
  const bottom = Math.max(session.startY, y)

  const hits: string[] = []
  container.querySelectorAll<HTMLElement>('[data-asset-id]').forEach((el) => {
    const id = el.dataset.assetId
    if (!id) return
    const rect = el.getBoundingClientRect()
    if (rect.left < right && rect.right > left && rect.top < bottom && rect.bottom > top) {
      hits.push(id)
    }
  })

  selectedIds.value = session.additive
    ? [...new Set([...session.base, ...hits])]
    : hits
  anchorId.value = selectedIds.value[selectedIds.value.length - 1] ?? null
}

function onLassoEnd(): void {
  window.removeEventListener('mousemove', onLassoMove)
  window.removeEventListener('mouseup', onLassoEnd)
  const session = lassoSession
  lassoSession = null
  if (lasso.value) {
    lasso.value = null
    return
  }
  // A press with no drag on empty background clears the selection.
  if (session && !session.additive) clearSelection()
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

function openTags(): void {
  const asset = selectedAssets.value[0]
  if (asset) openAssetTags(asset.id)
}

async function performDelete(ids: string[]): Promise<void> {
  if (!ids.length) return
  try {
    await Promise.all(ids.map((id) => api.deleteAsset(id)))
    clearSelection()
    notifyLibraryChanged()
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
    accept: () => performDelete(ids),
  })
}

function toggleDisplay(event: Event): void {
  displayPopover.value?.toggle(event)
}

// onChanged replaces the affected asset client-side (tags/collections
// changed), avoiding a full library/asset reload.
function onChanged(asset: Asset): void {
  const index = assets.value.findIndex((item) => item.id === asset.id)
  if (index >= 0) assets.value[index] = asset
}

function onDeleted(): void {
  notifyLibraryChanged()
}

watch([collectionFilter, tagFilter], () => {
  clearSelection()
  void load()
})

watch(
  () => library.version,
  () => {
    void load()
  },
)

watch(
  () => library.currentId,
  () => {
    clearSelection()
  },
)

onMounted(() => {
  void load()
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onLassoMove)
  window.removeEventListener('mouseup', onLassoEnd)
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <Message v-if="error" severity="error" class="mx-6 mt-4">{{ error }}</Message>

    <div class="flex min-h-0 flex-1">
      <div class="flex min-w-0 flex-1 flex-col">
        <header class="flex flex-wrap items-center gap-3 px-6 py-4">
          <select
            v-model="collectionFilter"
            class="rounded border border-slate-300 bg-white px-3 py-1.5 text-sm"
          >
            <option value="">All collections</option>
            <option value="uncategorized">Uncategorized</option>
            <option
              v-for="collection in library.collections"
              :key="collection.id"
              :value="collection.id"
            >
              {{ collection.name }}
            </option>
          </select>

          <MultiSelect
            v-model="tagFilter"
            :options="tagOptions"
            option-label="label"
            option-value="value"
            display="chip"
            placeholder="All tags"
            :max-selected-labels="3"
            class="rounded border border-slate-300 bg-white text-sm"
          >
            <template #option="slotProps">
              <span class="flex items-center gap-2">
                <span
                  v-if="slotProps.option.color"
                  class="h-2.5 w-2.5 shrink-0 rounded-full"
                  :style="{ backgroundColor: slotProps.option.color }"
                />
                {{ slotProps.option.label }}
              </span>
            </template>
          </MultiSelect>

          <select
            v-model="typeFilter"
            class="rounded border border-slate-300 bg-white px-3 py-1.5 text-sm"
          >
            <option
              v-for="option in TYPE_CATEGORIES"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
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

          <Button label="Upload" @click="openUpload([], uploadCollectionId)">
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
            <div class="w-72 space-y-4">
              <div>
                <div class="mb-2 flex items-center justify-between">
                  <p class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
                    Size
                  </p>
                  <span class="text-xs text-slate-400">{{ size }}px</span>
                </div>
                <Slider
                  v-model="size"
                  :min="CARD_SIZE_MIN"
                  :max="CARD_SIZE_MAX"
                  :step="2"
                  class="w-full"
                />
              </div>

              <div>
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Shape
                </p>
                <SelectButton
                  v-model="shape"
                  :options="CARD_SHAPES"
                  option-label="label"
                  option-value="value"
                  :allow-empty="false"
                  size="small"
                  class="w-full"
                />
              </div>

              <div class="border-t border-slate-100 pt-3">
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  On preview
                </p>
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Extension</span>
                    <ToggleSwitch v-model="showExtension" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Name</span>
                    <ToggleSwitch v-model="showName" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Size</span>
                    <ToggleSwitch v-model="showSize" />
                  </div>
                </div>
              </div>
            </div>
          </Popover>
        </header>

        <div
          ref="gridContainer"
          class="min-h-0 flex-1 overflow-y-auto px-6 pt-2 pb-6"
          @mousedown="onLassoStart"
        >
          <p v-if="loading" class="text-sm text-slate-400">Loading…</p>
          <p v-else-if="!assets.length" class="text-sm text-slate-400">
            No assets yet.
          </p>
          <p v-else-if="!visibleAssets.length" class="text-sm text-slate-400">
            No assets match the current filters.
          </p>
          <div
            v-else-if="shape === 'native'"
            class="grid gap-2"
            :style="nativeGridStyle"
          >
            <AssetCard
              v-for="asset in visibleAssets"
              :key="asset.id"
              fill
              :style="nativeItemStyle(asset)"
              :asset="asset"
              :shape="shape"
              :selected="selectedSet.has(asset.id)"
              :show-extension="showExtension"
              :show-name="showName"
              :show-size="showSize"
              @open="onSelect"
              @menu="onContextMenu"
              @dragstart="onDragStart"
              @dragend="onDragEnd"
            />
          </div>
          <div v-else class="grid gap-3" :style="gridStyle">
            <AssetCard
              v-for="asset in visibleAssets"
              :key="asset.id"
              :asset="asset"
              :shape="shape"
              :selected="selectedSet.has(asset.id)"
              :show-extension="showExtension"
              :show-name="showName"
              :show-size="showSize"
              @open="onSelect"
              @menu="onContextMenu"
              @dragstart="onDragStart"
              @dragend="onDragEnd"
            />
          </div>
        </div>
      </div>

      <aside
        class="flex w-80 shrink-0 flex-col overflow-hidden border-l border-slate-200"
      >
        <AssetDetailPanel
          v-if="singleAsset"
          :asset="singleAsset"
          @close="clearSelection"
          @changed="onChanged"
          @deleted="onDeleted"
        />
        <MultiSelectionPanel
          v-else-if="selectedAssets.length > 1"
          :assets="selectedAssets"
          @close="clearSelection"
          @remove="confirmDelete"
        />
        <div
          v-else
          class="flex h-full items-center justify-center p-6 text-center text-sm text-slate-400"
        >
          Select an asset to view its details.
        </div>
      </aside>
    </div>

    <ContextMenu ref="contextMenu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700'"
          @click.prevent
        >
          <component
            :is="slotProps.icon"
            v-if="slotProps.icon"
            :size="14"
            class="shrink-0"
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <AngleRight v-if="slotProps.hasSubmenu" :size="12" class="shrink-0" />
        </a>
      </template>
    </ContextMenu>

    <div
      v-if="lassoStyle"
      class="pointer-events-none fixed z-40 rounded-sm border border-violet-500 bg-violet-500/10"
      :style="lassoStyle"
    />
  </div>
</template>
