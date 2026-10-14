<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import Popover from 'primevue/popover'
import ToggleSwitch from 'primevue/toggleswitch'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import { ExternalLink, SlidersH, Tag, Trash, Upload } from '@primeicons/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { endAssetDrag, startAssetDrag } from '../lib/assetDrag'
import { CARD_SHAPES, CARD_SIZES } from '../lib/display'
import type { CardShape, CardSize } from '../lib/display'
import { library, notifyLibraryChanged } from '../lib/library'
import { openAssetTags } from '../lib/tagsDialog'
import { openUpload } from '../lib/upload'
import type { Asset } from '../types'
import AssetCard from './AssetCard.vue'
import AssetDetailPanel from './AssetDetailPanel.vue'
import MosaicWall from './MosaicWall.vue'
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

const typeFilter = ref('')
const sort = ref('date-desc')

const size = ref<CardSize>('medium')
const shape = ref<CardShape>('square')
const mosaic = ref(false)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

const contextMenu = ref<{ show: (event: MouseEvent) => void } | null>(null)

// Lasso (marquee) selection state. Coordinates are viewport-relative
// (clientX/clientY) so the rectangle and hit-testing stay correct while the
// grid scrolls. `data-asset-id` marks the hittable cards in AssetCard/MosaicWall.
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

function setQueryFilter(key: 'collection' | 'tag', value: string): void {
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

const types = computed(() => {
  const found = new Set(assets.value.map((a) => a.mime_type))
  return [...found].sort()
})

const visibleAssets = computed(() => {
  let list = assets.value
  if (typeFilter.value) {
    list = list.filter((a) => a.mime_type === typeFilter.value)
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
    items.push({
      label: 'Tags',
      icon: Tag,
      command: openTags,
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
    },
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
  typeFilter.value = ''
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
            <option value="">All types</option>
            <option v-for="type in types" :key="type" :value="type">
              {{ type }}
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
        </header>

        <div
          ref="gridContainer"
          class="min-h-0 flex-1 overflow-y-auto px-6 pb-6"
          @mousedown="onLassoStart"
        >
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

    <ContextMenu ref="contextMenu" :model="menuItems" />

    <div
      v-if="lassoStyle"
      class="pointer-events-none fixed z-40 rounded-sm border border-violet-500 bg-violet-500/10"
      :style="lassoStyle"
    />
  </div>
</template>
