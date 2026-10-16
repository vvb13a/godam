<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import Popover from 'primevue/popover'
import SelectButton from 'primevue/selectbutton'
import Slider from 'primevue/slider'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  ChevronLeft,
  ChevronRight,
  Download,
  Eye,
  EyeSlash,
  ExternalLink,
  Images,
  Pencil,
  Refresh,
  Save,
  SlidersH,
  Tag,
  Times,
  Upload,
} from '@primeicons/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { assetDrag, endAssetDrag, startAssetDrag } from '../lib/assetDrag'
import { confirmDeleteMenuItem } from '../lib/menu'
import {
  CARD_SHAPES,
  CARD_SIZE_DEFAULT,
  CARD_SIZE_MAX,
  CARD_SIZE_MIN,
} from '../lib/display'
import type { CardShape } from '../lib/display'
import { editorDirty, leaveEditor, setEditorCloser } from '../lib/editorState'
import { library, loadLibraries, notifyLibraryChanged } from '../lib/library'
import { isTextMime } from '../lib/mimeTypes'
import { panels, toggleRightPanel } from '../lib/panels'
import { clearSmartSelection, smartSelection } from '../lib/smartSelection'
import { openAssetTags, openBulkAssetTags } from '../lib/tagsDialog'
import { openUpload } from '../lib/upload'
import type {
  Asset,
  Collection,
  SmartCollection,
  Tag as TagModel,
} from '../types'
import AssetCard from './AssetCard.vue'
import AssetEditor from './AssetEditor.vue'
import FilterInspector from './FilterInspector.vue'
import Inspector from './Inspector.vue'
import LibraryInspector from './LibraryInspector.vue'
import MultiSelectionPanel from './MultiSelectionPanel.vue'

const route = useRoute()
const router = useRouter()
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
  showDimensions: boolean
  showDuration: boolean
  playAudioOnHover: boolean
  playVideoOnHover: boolean
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
const showDimensions = ref(savedPrefs.showDimensions ?? false)
const showDuration = ref(savedPrefs.showDuration ?? false)
const playAudioOnHover = ref(savedPrefs.playAudioOnHover ?? true)
const playVideoOnHover = ref(savedPrefs.playVideoOnHover ?? true)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

watch(
  [
    size,
    shape,
    showExtension,
    showName,
    showSize,
    showDimensions,
    showDuration,
    playAudioOnHover,
    playVideoOnHover,
  ],
  () => {
    const prefs: DisplayPrefs = {
      size: clampSize(size.value),
      shape: shape.value,
      showExtension: showExtension.value,
      showName: showName.value,
      showSize: showSize.value,
      showDimensions: showDimensions.value,
      showDuration: showDuration.value,
      playAudioOnHover: playAudioOnHover.value,
      playVideoOnHover: playVideoOnHover.value,
    }
    try {
      localStorage.setItem(DISPLAY_STORAGE_KEY, JSON.stringify(prefs))
    } catch {
      // Storage unavailable: settings simply won't persist.
    }
  },
)

const allPreviewOn = computed(
  () =>
    showExtension.value &&
    showName.value &&
    showSize.value &&
    showDimensions.value &&
    showDuration.value,
)

function toggleAllPreview(): void {
  const next = !allPreviewOn.value
  showExtension.value = next
  showName.value = next
  showSize.value = next
  showDimensions.value = next
  showDuration.value = next
}

const allAutoplayOn = computed(
  () => playAudioOnHover.value && playVideoOnHover.value,
)

function toggleAllAutoplay(): void {
  const next = !allAutoplayOn.value
  playAudioOnHover.value = next
  playVideoOnHover.value = next
}

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
    clearSmartSelection()
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
// Only a single, concrete tag filter is auto-applied to uploads; multi-tag and
// the virtual "untagged" filter are skipped.
const uploadTagId = computed(() => {
  const tags = tagFilter.value
  if (tags.length !== 1 || tags[0] === 'untagged') return null
  return tags[0]
})

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
  if (mime === 'application/ogg') return 'audio'
  if (mime.startsWith('text/') || DOCUMENT_MIMES.has(mime)) return 'document'
  return 'other'
}

function setQueryFilter(
  key: 'collection' | 'tag' | 'type' | 'sort',
  value: string,
): void {
  // Changing a filter means we are no longer viewing a smart collection.
  clearSmartSelection()
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

// The Inspector falls back to describing the active collection/tag when no
// asset is selected. Only concrete ids count; the virtual 'uncategorized' /
// 'untagged' filters have no entity to describe.
const activeCollection = computed<Collection | null>(() => {
  const id = collectionFilter.value
  if (!id || id === 'uncategorized') return null
  return library.collections.find((item) => item.id === id) ?? null
})

const activeTag = computed<TagModel | null>(() => {
  const ids = tagFilter.value.filter((value) => value !== 'untagged')
  if (ids.length !== 1) return null
  return library.tags.find((item) => item.id === ids[0]) ?? null
})

// A smart collection is only "active" when it was explicitly opened from the
// sidebar (tracked as client-side state); manually setting the same filters
// does not select it. It also drops out once the filters no longer match.
const activeSmartCollection = computed<SmartCollection | null>(() => {
  const id = smartSelection.id
  const filter = smartSelection.filter
  if (!id || !filter) return null
  const q = route.query
  const matches =
    (filter.collection ?? '') === (typeof q.collection === 'string' ? q.collection : '') &&
    (filter.tag ?? '') === (typeof q.tag === 'string' ? q.tag : '') &&
    (filter.type ?? '') === (typeof q.type === 'string' ? q.type : '') &&
    (filter.sort ?? '') === (typeof q.sort === 'string' ? q.sort : '')
  if (!matches) return null
  return library.smartCollections.find((item) => item.id === id) ?? null
})

// Count and size describe the assets currently listed for the filter, i.e.
// exactly what the grid shows.
const filteredSize = computed(() =>
  visibleAssets.value.reduce((total, asset) => total + asset.byte_size, 0),
)

// Detail/edit mode: the main area shows a single asset (in an editor/viewer)
// instead of the grid. Tracked as local state; the Inspector is unchanged.
const editingAssetId = ref<string | null>(null)
const editingAsset = computed(() =>
  editingAssetId.value
    ? (assets.value.find((item) => item.id === editingAssetId.value) ?? null)
    : null,
)
// Types the editor can save: raster images and editable text/code.
const editingCanSave = computed(() => {
  const mime = editingAsset.value?.mime_type
  if (!mime) return false
  return mime.startsWith('image/') || isTextMime(mime)
})
const assetEditorRef = ref<InstanceType<typeof AssetEditor> | null>(null)

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
    case 'modified-asc':
      sorted.sort((a, b) => a.updated_at.localeCompare(b.updated_at))
      break
    case 'modified-desc':
      sorted.sort((a, b) => b.updated_at.localeCompare(a.updated_at))
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

// Horizontal gap between cards. Mirrors the Tailwind gap classes on the grid
// containers below (gap-3 = 12px, gap-2 = 8px) so the fit maths matches the CSS.
const GRID_GAP = 12
const NATIVE_GAP = 8

// Content width of the grid container, kept in sync by a ResizeObserver. The
// tracks are spread across it so a row reaches the right edge instead of
// leaving the leftover width as a gap (which auto-fill with fixed tracks does).
const containerWidth = ref(0)

function measureGrid(): void {
  const el = gridContainer.value
  if (!el) return
  const style = getComputedStyle(el)
  const padding =
    parseFloat(style.paddingLeft) + parseFloat(style.paddingRight)
  containerWidth.value = Math.max(0, el.clientWidth - padding)
}

// Fit as many tracks of at least `target` px as possible, then widen them
// evenly so the row fills the container exactly. Returns null before the
// container has been measured so callers can fall back to plain auto-fill.
function fitColumns(
  target: number,
  gap: number,
): { count: number; width: number } | null {
  const available = containerWidth.value
  if (available <= 0) return null
  const count = Math.max(1, Math.floor((available + gap) / (target + gap)))
  const width = (available - (count - 1) * gap) / count
  return { count, width }
}

const gridStyle = computed(() => {
  const fit = fitColumns(cardWidth.value, GRID_GAP)
  if (!fit) {
    return { gridTemplateColumns: `repeat(auto-fill, ${cardWidth.value}px)` }
  }
  return {
    gridTemplateColumns: `repeat(${fit.count}, ${fit.width}px)`,
    columnGap: `${GRID_GAP}px`,
  }
})

// Native uses a spanned grid so each preview keeps its real proportions. The
// slider is the base cell: a square is 1x1, a landscape spans columns and a
// portrait spans rows. Columns and rows share the fitted unit so the cells stay
// square, otherwise the spanned previews would be distorted.
const nativeUnit = computed(() => Math.max(1, Math.round(size.value)))
const nativeGridStyle = computed(() => {
  const fit = fitColumns(nativeUnit.value, NATIVE_GAP)
  const unit = fit ? fit.width : nativeUnit.value
  const columns = fit
    ? `repeat(${fit.count}, ${unit}px)`
    : `repeat(auto-fill, ${unit}px)`
  return {
    gridTemplateColumns: columns,
    gridAutoRows: `${unit}px`,
    gridAutoFlow: 'dense',
    columnGap: `${NATIVE_GAP}px`,
    rowGap: `${NATIVE_GAP}px`,
  }
})

function nativeItemStyle(asset: Asset): Record<string, string> {
  const w = asset.metadata?.width ?? 0
  const h = asset.metadata?.height ?? 0
  // Assets with no intrinsic dimensions default to a shape: PDFs read as pages
  // (poster 2:3), audio reads as a wide waveform (landscape 16:9).
  let ratio: number
  if (w > 0 && h > 0) {
    ratio = w / h
  } else if (asset.mime_type === 'application/pdf') {
    ratio = 2 / 3
  } else if (asset.mime_type.startsWith('audio/')) {
    ratio = 16 / 9
  } else {
    ratio = 1
  }
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
      label: 'Open',
      icon: Pencil,
      command: openEditorForSelection,
    })
    items.push({
      label: 'Open original',
      icon: ExternalLink,
      command: openOriginal,
    })
    items.push({
      label: 'Download',
      icon: Download,
      command: downloadAsset,
    })
    items.push({
      label: 'Regenerate preview',
      icon: Refresh,
      command: () => void regeneratePreviews(selectedAssets.value.slice()),
    })
    items.push({
      label: 'Tags',
      icon: Tag,
      command: openTags,
    })
    items.push({
      label: 'Set as library cover',
      icon: Images,
      command: setAsLibraryCover,
    })
    items.push({
      label: 'Set as library watermark',
      icon: SlidersH,
      command: setAsLibraryWatermark,
    })
  } else if (count > 1) {
    items.push({
      label: `Download ${count} as zip`,
      icon: Download,
      command: downloadSelectionZip,
    })
    items.push({
      label: `Regenerate ${count} previews`,
      icon: Refresh,
      command: () => void regeneratePreviews(selectedAssets.value.slice()),
    })
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

const DRAG_IMAGE_SIZE = 64
let dragGhostEl: HTMLElement | null = null

// renderedPreview returns the already-decoded preview <img> for a card in the
// grid, so the ghost can be painted synchronously (no waiting on image loads).
function renderedPreview(assetId: string): HTMLImageElement | null {
  return document.querySelector<HTMLImageElement>(
    `[data-asset-id="${CSS.escape(assetId)}"] img`,
  )
}

function drawCover(
  ctx: CanvasRenderingContext2D,
  img: HTMLImageElement,
  x: number,
  y: number,
  w: number,
  h: number,
): void {
  const iw = img.naturalWidth
  const ih = img.naturalHeight
  if (!iw || !ih) {
    ctx.fillStyle = '#cbd5e1'
    ctx.fillRect(x, y, w, h)
    return
  }
  const scale = Math.max(w / iw, h / ih)
  const dw = iw * scale
  const dh = ih * scale
  ctx.drawImage(img, x + (w - dw) / 2, y + (h - dh) / 2, dw, dh)
}

// buildDragCanvas paints the drag preview: a single translucent thumbnail, or a
// 2x2 grid with the first three previews and a "+N" badge for the remainder.
// A canvas is used so the 50% transparency renders reliably everywhere.
function buildDragCanvas(assets: Asset[], count: number): HTMLCanvasElement {
  const size = DRAG_IMAGE_SIZE
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')
  if (!ctx) return canvas
  ctx.globalAlpha = 0.5

  if (count <= 1) {
    const img = assets[0] ? renderedPreview(assets[0].id) : null
    if (img) drawCover(ctx, img, 0, 0, size, size)
    return canvas
  }

  const cell = size / 2
  for (let i = 0; i < Math.min(3, assets.length); i++) {
    const img = renderedPreview(assets[i].id)
    if (img) {
      drawCover(ctx, img, (i % 2) * cell, Math.floor(i / 2) * cell, cell, cell)
    }
  }

  if (count > 3) {
    ctx.globalAlpha = 1
    ctx.fillStyle = '#0f172a'
    ctx.fillRect(cell, cell, cell, cell)
    ctx.fillStyle = '#ffffff'
    ctx.font = '600 12px system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(`+${count - 3}`, cell + cell / 2, cell + cell / 2)
  }
  return canvas
}

function onDragStart(asset: Asset, event: DragEvent): void {
  if (!selectedSet.value.has(asset.id)) {
    selectedIds.value = [asset.id]
    anchorId.value = asset.id
  }
  const ids = selectedIds.value.slice()
  startAssetDrag(ids)
  if (!event.dataTransfer) return
  event.dataTransfer.setData('text/plain', JSON.stringify(ids))
  event.dataTransfer.effectAllowed = 'copy'
  const ghost = buildDragCanvas(selectedAssets.value, ids.length)
  ghost.style.position = 'fixed'
  ghost.style.top = '-1000px'
  ghost.style.left = '-1000px'
  ghost.style.pointerEvents = 'none'
  document.body.appendChild(ghost)
  event.dataTransfer.setDragImage(
    ghost,
    DRAG_IMAGE_SIZE / 2,
    DRAG_IMAGE_SIZE / 2,
  )
  dragGhostEl = ghost
  // The browser snapshots the image during setDragImage; drop it right after.
  window.setTimeout(() => {
    ghost.remove()
    if (dragGhostEl === ghost) dragGhostEl = null
  }, 0)
}

function onDragEnd(): void {
  endAssetDrag()
  dragGhostEl?.remove()
  dragGhostEl = null
}

function openOriginal(): void {
  const asset = selectedAssets.value[0]
  if (asset) {
    window.open(api.assetContentUrl(asset.id), '_blank', 'noopener')
  }
}

// Single asset downloads as its original file.
function downloadAsset(): void {
  const asset = selectedAssets.value[0]
  if (!asset) return
  const anchor = document.createElement('a')
  anchor.href = api.assetContentUrl(asset.id)
  anchor.download = asset.filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

// Multiple assets download as a server-streamed zip. A hidden iframe keeps the
// browser from navigating the app if the request errors.
function downloadSelectionZip(): void {
  const ids = selectedIds.value
  if (!ids.length) return
  const frame = document.createElement('iframe')
  frame.style.display = 'none'
  frame.src = api.assetsZipUrl(ids.slice())
  document.body.appendChild(frame)
  window.setTimeout(() => frame.remove(), 60000)
}

function openTags(): void {
  const asset = selectedAssets.value[0]
  if (asset) openAssetTags(asset.id)
}

// Re-renders previews one asset at a time (sequential, to avoid spawning many
// media processes at once) and updates each card as it finishes.
async function regeneratePreviews(assets: Asset[]): Promise<void> {
  if (!assets.length) return
  let ok = 0
  let failed = 0
  for (const asset of assets) {
    try {
      const updated = await api.regeneratePreview(asset.id)
      onChanged(updated)
      ok += 1
    } catch {
      failed += 1
    }
  }
  toast.add({
    severity: failed ? 'warn' : 'success',
    summary: `Regenerated ${ok} preview${ok === 1 ? '' : 's'}${
      failed ? ` · ${failed} failed` : ''
    }`,
    life: 3000,
  })
}

// Pins the selected asset's preview as the library cover, shown in the default
// (library) Inspector. The library list is reloaded so the cover propagates.
async function setAsLibraryCover(): Promise<void> {
  const asset = selectedAssets.value[0]
  const libraryId = library.currentId
  if (!asset || !libraryId) return
  try {
    await api.setLibraryCover(libraryId, asset.id)
    await loadLibraries()
    toast.add({
      severity: 'success',
      summary: 'Library cover updated',
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not set cover',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

// Pins the selected asset as the library's default watermark, offered in the
// image editor's watermark tool.
async function setAsLibraryWatermark(): Promise<void> {
  const asset = selectedAssets.value[0]
  const libraryId = library.currentId
  if (!asset || !libraryId) return
  try {
    await api.setLibraryWatermark(libraryId, asset.id)
    await loadLibraries()
    toast.add({
      severity: 'success',
      summary: 'Library watermark set',
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not set watermark',
      detail: toMessage(e),
      life: 4000,
    })
  }
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

function toggleDisplay(event: Event): void {
  displayPopover.value?.toggle(event)
}

// onChanged replaces the affected asset client-side (title/note/tags/
// collections changed), avoiding a full library/asset reload.
function onChanged(asset: Asset): void {
  const index = assets.value.findIndex((item) => item.id === asset.id)
  if (index >= 0) assets.value[index] = asset
}

// Refreshes the sidebar/library after a collection or tag is edited in the
// Inspector, so the tree, tag list and this panel all pick up the new values.
function onSubjectChanged(): void {
  notifyLibraryChanged()
}

function openEditor(asset: Asset): void {
  selectedIds.value = [asset.id]
  anchorId.value = asset.id
  editingAssetId.value = asset.id
}

function openEditorForSelection(): void {
  const asset = selectedAssets.value[0]
  if (asset) openEditor(asset)
}

function closeEditor(): void {
  editingAssetId.value = null
}

// The Back control and the global router guard both go through leaveEditor,
// which asks for confirmation only when there are unsaved edits.
function onEditorBack(): void {
  void leaveEditor()
}

// Previous/next step through the list the editor was opened from (the current
// filtered/sorted grid), so navigation matches how the user got here.
const editingIndex = computed(() =>
  visibleAssets.value.findIndex((item) => item.id === editingAssetId.value),
)
const canStepBack = computed(() => editingIndex.value > 0)
const canStepForward = computed(
  () =>
    editingIndex.value >= 0 &&
    editingIndex.value < visibleAssets.value.length - 1,
)

function stepEditor(delta: number): void {
  const index = editingIndex.value
  const target = visibleAssets.value[index + delta]
  if (target) openEditor(target)
}

// Closing the Inspector also leaves the detail/edit mode.
function closeInspector(): void {
  closeEditor()
  clearSelection()
}

function onAssetSaved(asset: Asset): void {
  onChanged(asset)
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

// Re-measure once the grid content changes: a scrollbar appearing/disappearing
// (or a different number of cards) changes the usable content width.
watch(visibleAssets, () => {
  void nextTick(measureGrid)
})

let gridResizeObserver: ResizeObserver | null = null

// Ctrl/Cmd+A selects every asset in the current view. Guarded so it never
// hijacks select-all inside text fields, while editing, or when a modal is open.
function isEditableTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  return (
    el.tagName === 'INPUT' ||
    el.tagName === 'TEXTAREA' ||
    el.tagName === 'SELECT' ||
    el.isContentEditable
  )
}

function onAssetsSelectAll(event: KeyboardEvent): void {
  if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'a') {
    return
  }
  if (isEditableTarget(event.target) || editingAssetId.value) return
  if (document.querySelector('.p-dialog-mask')) return
  event.preventDefault()
  selectedIds.value = visibleAssets.value.map((asset) => asset.id)
  anchorId.value = visibleAssets.value[0]?.id ?? null
}

onMounted(() => {
  void load()
  measureGrid()
  setEditorCloser(closeEditor)
  window.addEventListener('keydown', onAssetsSelectAll)
  if (gridContainer.value) {
    gridResizeObserver = new ResizeObserver(measureGrid)
    gridResizeObserver.observe(gridContainer.value)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onLassoMove)
  window.removeEventListener('mouseup', onLassoEnd)
  window.removeEventListener('keydown', onAssetsSelectAll)
  gridResizeObserver?.disconnect()
  setEditorCloser(null)
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <Message v-if="error" severity="error" class="mx-6 mt-4">{{ error }}</Message>

    <div class="flex min-h-0 flex-1">
      <div class="relative flex min-w-0 flex-1 flex-col">
        <header v-if="!editingAsset" class="flex flex-wrap items-center gap-3 px-6 py-4">
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
            <option value="modified-desc">Recently modified</option>
            <option value="modified-asc">Least recently modified</option>
            <option value="name-asc">Name A–Z</option>
            <option value="name-desc">Name Z–A</option>
            <option value="size-desc">Largest first</option>
            <option value="size-asc">Smallest first</option>
          </select>

          <Button
            label="Upload"
            @click="openUpload([], uploadCollectionId, uploadTagId)"
          >
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
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Preview
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-violet-600"
                    :title="allPreviewOn ? 'Hide all' : 'Show all'"
                    @click="toggleAllPreview"
                  >
                    <component :is="allPreviewOn ? Eye : EyeSlash" :size="14" />
                  </button>
                </div>
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
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Dimensions</span>
                    <ToggleSwitch v-model="showDimensions" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Duration</span>
                    <ToggleSwitch v-model="showDuration" />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Autoplay
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-violet-600"
                    :title="allAutoplayOn ? 'Disable all' : 'Enable all'"
                    @click="toggleAllAutoplay"
                  >
                    <component
                      :is="allAutoplayOn ? Eye : EyeSlash"
                      :size="14"
                    />
                  </button>
                </div>
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Audio</span>
                    <ToggleSwitch v-model="playAudioOnHover" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium">Video</span>
                    <ToggleSwitch v-model="playVideoOnHover" />
                  </div>
                </div>
                <p class="mt-2 text-xs text-slate-400">
                  Preview audio/video on hover.
                </p>
              </div>
            </div>
          </Popover>
        </header>

        <AssetEditor
          v-if="editingAsset"
          ref="assetEditorRef"
          :asset="editingAsset"
          @close="closeEditor"
          @saved="onAssetSaved"
        />

        <div
          v-show="!editingAsset"
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
              :dragging="assetDrag.active && selectedSet.has(asset.id)"
              :show-extension="showExtension"
              :show-name="showName"
              :show-size="showSize"
              :show-dimensions="showDimensions"
              :show-duration="showDuration"
              :autoplay-audio="playAudioOnHover"
              :autoplay-video="playVideoOnHover"
              @open="onSelect"
              @dblclick="openEditor(asset)"
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
              :dragging="assetDrag.active && selectedSet.has(asset.id)"
              :show-extension="showExtension"
              :show-name="showName"
              :show-size="showSize"
              :show-dimensions="showDimensions"
              :show-duration="showDuration"
              :autoplay-audio="playAudioOnHover"
              :autoplay-video="playVideoOnHover"
              @open="onSelect"
              @dblclick="openEditor(asset)"
              @menu="onContextMenu"
              @dragstart="onDragStart"
              @dragend="onDragEnd"
            />
          </div>
        </div>

        <!-- Bottom-right control cluster, left to right: Save, Cancel, then
             navigation. The nav pill is last (right-anchored) so it never
             shifts, whether or not a Save button is present. -->
        <div
          v-if="editingAsset"
          class="absolute right-6 bottom-6 z-10 flex items-center gap-1"
        >
          <button
            v-if="editingCanSave"
            class="inline-flex h-7 items-center gap-1 rounded-full bg-emerald-600 px-3 text-xs font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-40"
            title="Save changes"
            :disabled="!editorDirty"
            @click="assetEditorRef?.save($event)"
          >
            <Save :size="14" />
            Save
          </button>
          <button
            class="inline-flex h-7 items-center gap-1 rounded-full bg-slate-600 px-3 text-xs font-medium text-white hover:bg-slate-700"
            title="Cancel"
            @click="onEditorBack"
          >
            <Times :size="14" />
            Cancel
          </button>
          <div
            class="flex items-center gap-1 rounded-full border border-slate-200 bg-white px-2 py-1 shadow-lg"
          >
            <button
              class="inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
              title="Previous"
              :disabled="!canStepBack"
              @click="stepEditor(-1)"
            >
              <ChevronLeft :size="16" />
            </button>
            <span class="min-w-14 px-1 text-center text-xs tabular-nums text-slate-500">
              {{ editingIndex + 1 }} / {{ visibleAssets.length }}
            </span>
            <button
              class="inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
              title="Next"
              :disabled="!canStepForward"
              @click="stepEditor(1)"
            >
              <ChevronRight :size="16" />
            </button>
          </div>
        </div>
      </div>

      <div
        class="relative h-full shrink-0 pt-2 pr-2 pb-2 transition-[width] duration-200 ease-in-out"
        :style="{ width: panels.rightCollapsed ? '0px' : '20.5rem' }"
      >
        <aside
          class="h-full w-full overflow-hidden rounded-xl border border-slate-200 bg-white shadow-lg"
        >
          <div
            class="flex h-full w-80 flex-col"
            :inert="panels.rightCollapsed || undefined"
          >
            <Inspector
              v-if="singleAsset"
              :asset="singleAsset"
              @close="closeInspector"
              @changed="onChanged"
            />
            <MultiSelectionPanel
              v-else-if="selectedAssets.length > 1"
              :assets="selectedAssets"
              @close="clearSelection"
            />
            <FilterInspector
              v-else-if="activeSmartCollection || activeCollection || activeTag"
              :collection="activeCollection ?? undefined"
              :smart-collection="activeSmartCollection ?? undefined"
              :tag="activeTag ?? undefined"
              :assets="visibleAssets"
              :size="filteredSize"
              @changed="onSubjectChanged"
            />
            <LibraryInspector v-else />
          </div>
        </aside>

        <button
          type="button"
          class="absolute top-1/2 right-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200/80 bg-white/80 text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white hover:text-violet-600"
          :class="
            panels.rightCollapsed
              ? 'translate-x-0 rounded-l-full'
              : 'translate-x-1/2 rounded-full'
          "
          :title="panels.rightCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
          :aria-label="
            panels.rightCollapsed ? 'Expand sidebar' : 'Collapse sidebar'
          "
          @click="toggleRightPanel"
        >
          <ChevronLeft v-if="panels.rightCollapsed" :size="12" />
          <ChevronRight v-else :size="12" />
        </button>
      </div>
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
