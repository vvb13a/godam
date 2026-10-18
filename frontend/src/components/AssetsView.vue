<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import MultiSelect from 'primevue/multiselect'
import Select from 'primevue/select'
import Popover from 'primevue/popover'
import SelectButton from 'primevue/selectbutton'
import Slider from 'primevue/slider'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Download,
  Eye,
  EyeSlash,
  ExternalLink,
  Images,
  Pencil,
  Refresh,
  Save,
  Search,
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
import {
  cancelEmbedSelection,
  confirmEmbedSelection,
  embed,
  markEmbedReady,
} from '../lib/embed'
import { editorDirty, leaveEditor, setEditorCloser } from '../lib/editorState'
import { searchAssetWithGoogleLens } from '../lib/googleLens'
import { library, loadLibraries, notifyLibraryChanged } from '../lib/library'
import { isTextMime } from '../lib/mimeTypes'
import {
  panels,
  toggleBottomPanel,
  toggleRightPanel,
  toggleTopPanel,
} from '../lib/panels'
import { sidebarPrefs } from '../lib/sidebar'
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
import TagsList from './TagsList.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const PANEL_TRANSITION_MS = 200
const panelAnimating = ref(false)
let panelAnimationTimer: number | null = null

function beginPanelAnimation(): void {
  panelAnimating.value = true
  if (panelAnimationTimer !== null) window.clearTimeout(panelAnimationTimer)
  panelAnimationTimer = window.setTimeout(() => {
    panelAnimating.value = false
    panelAnimationTimer = null
    void nextTick(measureGrid)
  }, PANEL_TRANSITION_MS + 160)
}

function animatePanelEnter(el: Element, done: () => void): void {
  beginPanelAnimation()
  const node = el as HTMLElement
  const height = node.scrollHeight
  node.style.overflow = 'hidden'
  node.style.height = '0px'
  node.style.opacity = '0'
  node.style.willChange = 'height, opacity'
  void node.offsetHeight
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      node.style.transition = `height ${PANEL_TRANSITION_MS}ms ease, opacity ${PANEL_TRANSITION_MS}ms ease`
      node.style.height = `${height}px`
      node.style.opacity = '1'
    })
  })
  window.setTimeout(() => {
    node.style.removeProperty('transition')
    node.style.removeProperty('height')
    node.style.removeProperty('overflow')
    node.style.removeProperty('opacity')
    node.style.removeProperty('will-change')
    done()
  }, PANEL_TRANSITION_MS + 120)
}

function animatePanelLeave(el: Element, done: () => void): void {
  beginPanelAnimation()
  const node = el as HTMLElement
  const height = node.scrollHeight
  node.style.overflow = 'hidden'
  node.style.height = `${height}px`
  node.style.opacity = '1'
  node.style.willChange = 'height, opacity'
  void node.offsetHeight
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      node.style.transition = `height ${PANEL_TRANSITION_MS}ms ease, opacity ${PANEL_TRANSITION_MS}ms ease`
      node.style.height = '0px'
      node.style.opacity = '0'
    })
  })
  window.setTimeout(() => {
    node.style.removeProperty('will-change')
    done()
  }, PANEL_TRANSITION_MS + 120)
}

const searchQuery = computed({
  get: () => {
    const value = route.query.search
    return typeof value === 'string' ? value : ''
  },
  set: (value: string) => {
    const query = { ...route.query }
    if (value.trim()) query.search = value
    else delete query.search
    void router.replace({ name: 'all', query })
  },
})
const searchInput = ref(searchQuery.value)
const assets = ref<Asset[]>([])
const nextCursor = ref<string | null>(null)
const total = ref(0)
const totalBytes = ref(0)
const totalCapped = ref(false)
const loadingMore = ref(false)
const loadMoreSentinel = ref<HTMLElement | null>(null)
const selectedDetail = ref<Asset | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const loadedOnce = ref(false)
const gridRevision = ref(0)
const error = ref<string | null>(null)
let loadMoreObserver: IntersectionObserver | null = null
let searchTimer: number | null = null
let loadSeq = 0

const localSelectedIds = ref<string[]>([])
const selectedIds = computed<string[]>({
  get: () => (embed.active ? embed.selectedIds : localSelectedIds.value),
  set: (value) => {
    if (embed.active) embed.selectedIds = value
    else localSelectedIds.value = value
  },
})
const anchorId = ref<string | null>(null)

const embedPicker = computed(() => embed.active && embed.mode !== 'library')
const embedSingle = computed(() => embed.active && embed.mode === 'single')
const embedCollectionLocked = computed(
  () => embedPicker.value && embed.collectionLocked,
)
const embedTagLocked = computed(() => embedPicker.value && embed.tagLocked)

const DISPLAY_STORAGE_KEY = 'godam:display'

type SpacingLevel = 'dense' | 'normal' | 'relaxed'

const SPACING_LEVELS: SpacingLevel[] = ['dense', 'normal', 'relaxed']
const SPACING_OPTIONS = [
  { label: 'Dense', value: 'dense' },
  { label: 'Normal', value: 'normal' },
  { label: 'Relaxed', value: 'relaxed' },
]
const GRID_GAP_PX: Record<SpacingLevel, number> = {
  dense: 8,
  normal: 12,
  relaxed: 16,
}
const AROUND_GAP_PX: Record<SpacingLevel, number> = {
  dense: 8,
  normal: 16,
  relaxed: 24,
}

function normalizeSpacing(value: unknown): SpacingLevel {
  return SPACING_LEVELS.includes(value as SpacingLevel)
    ? (value as SpacingLevel)
    : 'normal'
}

interface DisplayPrefs {
  size: number
  shape: CardShape
  gridGap: SpacingLevel
  aroundGap: SpacingLevel
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
const gridGap = ref<SpacingLevel>(normalizeSpacing(savedPrefs.gridGap))
const aroundGap = ref<SpacingLevel>(normalizeSpacing(savedPrefs.aroundGap))
const gridGapPx = computed(() => GRID_GAP_PX[gridGap.value])
const aroundGapPx = computed(() => AROUND_GAP_PX[aroundGap.value])
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
    gridGap,
    aroundGap,
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
      gridGap: gridGap.value,
      aroundGap: aroundGap.value,
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
const showBackToTop = ref(false)
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
const colorFilter = computed({
  get: () => {
    const value = route.query.color
    if (value === 'teal') return 'green'
    return typeof value === 'string' ? value : ''
  },
  set: (value: string) => setQueryFilter('color', value),
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

const COLOR_OPTIONS = [
  { value: '', label: 'All colors' },
  { value: 'black', label: 'Black', hex: '#111827' },
  { value: 'blue', label: 'Blue', hex: '#3b82f6' },
  { value: 'brown', label: 'Brown', hex: '#764a1d' },
  { value: 'gray', label: 'Gray', hex: '#9ca3af' },
  { value: 'green', label: 'Green', hex: '#22c55e' },
  { value: 'orange', label: 'Orange', hex: '#f97316' },
  { value: 'pink', label: 'Pink', hex: '#ec4899' },
  { value: 'purple', label: 'Purple', hex: '#8b5cf6' },
  { value: 'red', label: 'Red', hex: '#ef4444' },
  { value: 'white', label: 'White', hex: '#f8fafc' },
  { value: 'yellow', label: 'Yellow', hex: '#eab308' },
]

const SORT_OPTIONS = [
  { value: 'date-desc', label: 'Newest first' },
  { value: 'date-asc', label: 'Oldest first' },
  { value: 'modified-desc', label: 'Recently modified' },
  { value: 'modified-asc', label: 'Least recently modified' },
  { value: 'name-asc', label: 'Name A–Z' },
  { value: 'name-desc', label: 'Name Z–A' },
  { value: 'size-desc', label: 'Largest first' },
  { value: 'size-asc', label: 'Smallest first' },
]

const collectionOptions = computed(() => [
  {
    value: '',
    label: `All collections (${library.stats?.assets ?? 0})`,
  },
  {
    value: 'uncategorized',
    label: `Uncategorized (${library.stats?.uncategorized ?? 0})`,
  },
  ...library.collections.map((collection) => ({
    value: collection.id,
    label: `${collection.name} (${collection.asset_count})`,
  })),
])

function setQueryFilter(
  key: 'collection' | 'tag' | 'type' | 'color' | 'sort',
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
  {
    label: 'Untagged',
    value: 'untagged',
    color: undefined as string | undefined,
    count: library.stats?.untagged ?? 0,
  },
  ...library.tags.map((tag) => {
    const group = tag.group_id
      ? library.tagGroups.find((item) => item.id === tag.group_id)
      : undefined
    return {
      label: tag.name,
      value: tag.id,
      color: group?.color ? `#${group.color}` : undefined,
      count: tag.asset_count,
    }
  }),
])

const selectedSet = computed(() => new Set(selectedIds.value))
const selectedAssets = computed(() =>
  assets.value.filter((asset) => selectedSet.value.has(asset.id)),
)
const singleAssetId = computed(() =>
  selectedIds.value.length === 1 ? selectedIds.value[0] : null,
)
const singleAsset = computed(() => {
  const id = singleAssetId.value
  if (!id) return null
  if (selectedDetail.value?.id === id) return selectedDetail.value
  return assets.value.find((asset) => asset.id === id) ?? null
})

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
    (q.color ?? '') === '' &&
    (filter.sort ?? '') === (typeof q.sort === 'string' ? q.sort : '')
  if (!matches) return null
  return library.smartCollections.find((item) => item.id === id) ?? null
})

// Count and size describe every asset matching the filter, not only the
// currently loaded page.
const filteredSize = computed(() => totalBytes.value)

// Detail/edit mode: the main area shows a single asset (in an editor/viewer)
// instead of the grid. Tracked as local state; the Inspector is unchanged.
const editingAssetId = ref<string | null>(null)
const editingAsset = computed(() => {
  const id = editingAssetId.value
  if (!id) return null
  if (selectedDetail.value?.id === id) return selectedDetail.value
  return assets.value.find((item) => item.id === id) ?? null
})
// Types the editor can save: raster images and editable text/code.
const editingCanSave = computed(() => {
  const mime = editingAsset.value?.mime_type
  if (!mime) return false
  return mime.startsWith('image/') || isTextMime(mime)
})
const assetEditorRef = ref<InstanceType<typeof AssetEditor> | null>(null)

const visibleAssets = computed(() => assets.value)

const hasActiveFilters = computed(
  () =>
    Boolean(collectionFilter.value) ||
    tagFilter.value.length > 0 ||
    Boolean(typeFilter.value) ||
    Boolean(colorFilter.value) ||
    Boolean(searchQuery.value.trim()),
)

const assetCountLabel = computed(() => {
  if (selectedIds.value.length) return `${selectedIds.value.length} selected`
  return `${total.value}${totalCapped.value ? '+' : ''} item(s)`
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

// Content width of the grid container, kept in sync by a ResizeObserver. The
// tracks are spread across it so a row reaches the right edge instead of
// leaving the leftover width as a gap (which auto-fill with fixed tracks does).
const containerWidth = ref(0)

function measureGrid(): void {
  if (panelAnimating.value) return
  const el = gridContainer.value
  if (!el) return
  const style = getComputedStyle(el)
  const padding =
    parseFloat(style.paddingLeft) + parseFloat(style.paddingRight)
  containerWidth.value = Math.max(0, el.clientWidth - padding)
}

watch([gridGap, aroundGap], () => {
  void nextTick(measureGrid)
})

// Fit as many tracks of at least `target` px as possible, then widen them
// evenly so the row fills the container exactly. Returns null before the
// container has been measured so callers can fall back to plain auto-fill.
function onGridScroll(): void {
  showBackToTop.value = (gridContainer.value?.scrollTop ?? 0) > 400
}

function scrollGridToTop(): void {
  gridContainer.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

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
  const gap = gridGapPx.value
  const fit = fitColumns(cardWidth.value, gap)
  if (!fit) {
    return {
      gridTemplateColumns: `repeat(auto-fill, ${cardWidth.value}px)`,
      gap: `${gap}px`,
    }
  }
  return {
    gridTemplateColumns: `repeat(${fit.count}, ${fit.width}px)`,
    gap: `${gap}px`,
  }
})

// Native uses a spanned grid so each preview keeps its real proportions. The
// slider is the base cell: a square is 1x1, a landscape spans columns and a
// portrait spans rows. Columns and rows share the fitted unit so the cells stay
// square, otherwise the spanned previews would be distorted.
const nativeUnit = computed(() => Math.max(1, Math.round(size.value)))
const nativeGridStyle = computed(() => {
  const gap = gridGapPx.value
  const fit = fitColumns(nativeUnit.value, gap)
  const unit = fit ? fit.width : nativeUnit.value
  const columns = fit
    ? `repeat(${fit.count}, ${unit}px)`
    : `repeat(auto-fill, ${unit}px)`
  return {
    gridTemplateColumns: columns,
    gridAutoRows: `${unit}px`,
    gridAutoFlow: 'dense',
    gap: `${gap}px`,
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
    if (selectedAssets.value[0]?.mime_type.startsWith('image/')) {
      items.push({
        label: 'Search with Google Lens',
        icon: Search,
        command: () => {
          const asset = selectedAssets.value[0]
          if (asset) void searchWithGoogleLens(asset)
        },
      })
    }
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

function assetParams(): api.AssetListParams {
  const params: api.AssetListParams = {
    search: searchQuery.value.trim() || undefined,
    type: typeFilter.value || undefined,
    color: colorFilter.value || undefined,
    sort: sort.value || undefined,
    limit: 100,
  }
  const collection = collectionFilter.value
  if (collection === 'uncategorized') params.uncategorized = true
  else if (collection) params.collectionId = collection

  const tags = tagFilter.value
  if (tags.includes('untagged')) params.untagged = true
  const tagIds = tags.filter((value) => value !== 'untagged')
  if (tagIds.length) params.tagIds = tagIds
  return params
}

async function load(): Promise<void> {
  const seq = ++loadSeq
  const initial = !loadedOnce.value
  if (initial) loading.value = true
  else refreshing.value = true
  error.value = null
  nextCursor.value = null
  loadingMore.value = false
  try {
    const page = await api.listAssets(assetParams())
    if (seq !== loadSeq) return
    assets.value = page.items
    nextCursor.value = page.next_cursor ?? null
    total.value = page.total
    totalBytes.value = page.total_bytes
    totalCapped.value = page.total_capped
  } catch (e) {
    if (seq === loadSeq) error.value = toMessage(e)
  } finally {
    if (seq === loadSeq) {
      loading.value = false
      refreshing.value = false
    }
  }
  if (seq !== loadSeq) return

  if (!error.value) {
    loadedOnce.value = true
    gridRevision.value++
  }

  if (embed.active && !error.value) {
    if (!embed.selectionInitialized) {
      if (!embed.selectedIds.length && embed.preselectedIds.length) {
        selectedIds.value = [...embed.preselectedIds]
      }
      embed.selectionInitialized = true
    }
    markEmbedReady(total.value)
  }
}

async function loadMore(): Promise<void> {
  const cursor = nextCursor.value
  if (!cursor || loadingMore.value || loading.value || refreshing.value) return
  const seq = loadSeq
  loadingMore.value = true
  error.value = null
  try {
    const page = await api.listAssets({ ...assetParams(), cursor })
    if (seq !== loadSeq) return
    const existing = new Set(assets.value.map((asset) => asset.id))
    assets.value.push(
      ...page.items.filter((asset) => !existing.has(asset.id)),
    )
    nextCursor.value = page.next_cursor ?? null
    total.value = page.total
    totalBytes.value = page.total_bytes
    totalCapped.value = page.total_capped
    void nextTick(measureGrid)
  } catch (e) {
    if (seq === loadSeq) error.value = toMessage(e)
  } finally {
    loadingMore.value = false
  }
}

function updateLoadMoreObserver(): void {
  loadMoreObserver?.disconnect()
  loadMoreObserver = null
  if (!nextCursor.value || !loadMoreSentinel.value || !gridContainer.value) {
    return
  }
  loadMoreObserver = new IntersectionObserver(
    (entries) => {
      if (!error.value && entries.some((entry) => entry.isIntersecting)) {
        void loadMore()
      }
    },
    { root: gridContainer.value, rootMargin: '300px' },
  )
  loadMoreObserver.observe(loadMoreSentinel.value)
}

function onSelect(asset: Asset, event: MouseEvent): void {
  if (embedSingle.value) {
    selectedIds.value = [asset.id]
    anchorId.value = asset.id
    return
  }
  if (embedPicker.value) {
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
    selectedIds.value = selectedSet.value.has(asset.id)
      ? selectedIds.value.filter((id) => id !== asset.id)
      : [...selectedIds.value, asset.id]
    anchorId.value = asset.id
    return
  }
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
  if (event.button !== 0 || embedSingle.value) return
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
  if (embedSingle.value) {
    selectedIds.value = [asset.id]
    anchorId.value = asset.id
  } else if (!selectedSet.value.has(asset.id)) {
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

async function searchWithGoogleLens(asset: Asset): Promise<void> {
  try {
    await searchAssetWithGoogleLens(asset)
    toast.add({
      severity: 'success',
      summary: 'Searching with Google Lens',
      detail: 'The image is being sent to Google Lens.',
      life: 3000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Google Lens search failed',
      detail: toMessage(e),
      life: 4000,
    })
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
  if (!asset) return
  try {
    await api.setInstanceCover(asset.id)
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
  if (!asset) return
  try {
    await api.setInstanceWatermark(asset.id)
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
  if (selectedDetail.value?.id === asset.id) selectedDetail.value = asset
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
  const asset = singleAsset.value ?? selectedAssets.value[0]
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
    (editingIndex.value < visibleAssets.value.length - 1 ||
      Boolean(nextCursor.value)),
)

async function stepEditor(delta: number): Promise<void> {
  const index = editingIndex.value
  if (index < 0) return
  const targetIndex = index + delta
  if (delta > 0 && targetIndex >= visibleAssets.value.length && nextCursor.value) {
    await loadMore()
  }
  const target = visibleAssets.value[targetIndex]
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

watch(
  [collectionFilter, () => tagFilter.value.join(','), typeFilter, colorFilter, sort],
  () => {
    if (!embedPicker.value) clearSelection()
    void load()
  },
)

watch(searchQuery, (value) => {
  if (searchTimer !== null) {
    window.clearTimeout(searchTimer)
    searchTimer = null
  }
  if (value !== searchInput.value) searchInput.value = value
})

watch(searchQuery, () => {
  if (!embedPicker.value) clearSelection()
  void load()
})

watch(searchInput, (value) => {
  if (value === searchQuery.value) return
  if (searchTimer !== null) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    searchTimer = null
    if (value !== searchQuery.value) searchQuery.value = value
  }, 300)
})

watch([nextCursor, loadingMore], () => {
  void nextTick(updateLoadMoreObserver)
})

const detailRequestId = ref<string | null>(null)
watch(
  singleAssetId,
  async (id) => {
    if (!id) {
      selectedDetail.value = null
      detailRequestId.value = null
      return
    }
    const requestId = id
    detailRequestId.value = requestId
    try {
      const detail = await api.getAsset(id)
      if (detailRequestId.value === requestId && singleAssetId.value === id) {
        selectedDetail.value = detail
      }
    } catch {
      if (detailRequestId.value === requestId) selectedDetail.value = null
    }
  },
  { immediate: true },
)

watch(
  () => library.version,
  () => {
    void load()
  },
)

watch(
  () => library.id,
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
  if (embedSingle.value) return
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
  void nextTick(updateLoadMoreObserver)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onLassoMove)
  window.removeEventListener('mouseup', onLassoEnd)
  window.removeEventListener('keydown', onAssetsSelectAll)
  gridResizeObserver?.disconnect()
  loadMoreObserver?.disconnect()
  if (searchTimer !== null) window.clearTimeout(searchTimer)
  setEditorCloser(null)
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <Message v-if="error" severity="error" class="mx-6 mt-4">{{ error }}</Message>

    <div class="flex min-h-0 flex-1">
      <div class="relative flex min-w-0 flex-1 flex-col">
        <div
          v-if="!editingAsset"
          class="relative pt-2"
          :style="{
            paddingLeft: `${aroundGapPx}px`,
            paddingRight: `${aroundGapPx}px`,
            paddingBottom: `${aroundGapPx}px`,
          }"
        >
          <Transition
            @enter="animatePanelEnter"
            @leave="animatePanelLeave"
          >
            <header
              v-if="!panels.topCollapsed"
              class="flex flex-wrap items-center gap-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-4 py-3 shadow-lg"
            >
          <Select
            v-model="collectionFilter"
            :options="collectionOptions"
            option-label="label"
            option-value="value"
            size="small"
            class="w-44"
            :disabled="embedCollectionLocked"
            :title="
              embedCollectionLocked
                ? 'This collection is locked by the host application'
                : undefined
            "
          />

          <MultiSelect
            v-model="tagFilter"
            :options="tagOptions"
            option-label="label"
            option-value="value"
            display="chip"
            placeholder="All tags"
            :max-selected-labels="3"
            :disabled="embedTagLocked"
            :title="
              embedTagLocked
                ? 'Tags are locked by the host application'
                : undefined
            "
            size="small"
            class="w-48 text-sm"
          >
            <template #option="slotProps">
              <span class="flex w-full items-center gap-2">
                <span
                  v-if="slotProps.option.color"
                  class="h-2.5 w-2.5 shrink-0 rounded-full"
                  :style="{ backgroundColor: slotProps.option.color }"
                />
                <span class="min-w-0 flex-1 truncate">
                  {{ slotProps.option.label }}
                </span>
                <span class="shrink-0 text-xs text-slate-400">
                  {{ slotProps.option.count }}
                </span>
              </span>
            </template>
          </MultiSelect>

          <Select
            v-model="typeFilter"
            :options="TYPE_CATEGORIES"
            option-label="label"
            option-value="value"
            size="small"
            class="w-36"
          />

          <Select
            v-model="colorFilter"
            :options="COLOR_OPTIONS"
            option-label="label"
            option-value="value"
            size="small"
            class="w-36"
            title="Filter images where a color covers at least 10% of the image"
          >
            <template #option="slotProps">
              <span class="flex items-center gap-2">
                <span
                  v-if="slotProps.option.hex"
                  class="h-3 w-3 shrink-0 rounded-full border border-slate-200 dark:border-slate-700"
                  :style="{ backgroundColor: slotProps.option.hex }"
                />
                <span>{{ slotProps.option.label }}</span>
              </span>
            </template>
          </Select>

          <Select
            v-model="sort"
            :options="SORT_OPTIONS"
            option-label="label"
            option-value="value"
            size="small"
            class="w-48"
          />

          <IconField class="w-60">
            <InputIcon>
              <Search :size="14" />
            </InputIcon>
            <InputText
              v-model="searchInput"
              size="small"
              placeholder="Search assets…"
              title="Search title, note, and filename"
              class="w-full"
            />
          </IconField>

          <span
            v-if="embedCollectionLocked || embedTagLocked"
            class="text-xs font-medium text-amber-600 dark:text-amber-400"
            title="The host application restricted this picker"
          >
            Restricted by host
          </span>

          <Button
            label="Upload"
            @click="openUpload([], uploadCollectionId, uploadTagId)"
          >
            <template #icon><Upload :size="16" /></template>
          </Button>

          <span class="ml-auto text-sm text-slate-400">
            {{ assetCountLabel }}
          </span>
          <span
            v-if="refreshing"
            class="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-slate-300 border-t-violet-600 dark:border-slate-700 dark:border-t-violet-400"
            title="Refreshing assets"
          ></span>

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

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Spacing
                </p>
                <div class="space-y-3">
                  <div>
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-sm font-medium">Grid gap</span>
                      <span class="text-xs text-slate-400">
                        {{ gridGapPx }}px
                      </span>
                    </div>
                    <SelectButton
                      v-model="gridGap"
                      :options="SPACING_OPTIONS"
                      option-label="label"
                      option-value="value"
                      :allow-empty="false"
                      size="small"
                      class="w-full"
                    />
                  </div>
                  <div>
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-sm font-medium">Around gap</span>
                      <span class="text-xs text-slate-400">
                        {{ aroundGapPx }}px
                      </span>
                    </div>
                    <SelectButton
                      v-model="aroundGap"
                      :options="SPACING_OPTIONS"
                      option-label="label"
                      option-value="value"
                      :allow-empty="false"
                      size="small"
                      class="w-full"
                    />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Preview
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-violet-600 dark:hover:text-violet-400"
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

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Autoplay
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-violet-600 dark:hover:text-violet-400"
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
          </Transition>
          <button
            type="button"
            class="absolute bottom-1.5 left-1/2 z-20 flex h-5 w-9 -translate-x-1/2 items-center justify-center rounded-full border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 text-slate-400 shadow-sm transition-colors hover:text-violet-600 dark:hover:text-violet-400"
            :title="panels.topCollapsed ? 'Expand filters' : 'Collapse filters'"
            :aria-label="
              panels.topCollapsed ? 'Expand filters' : 'Collapse filters'
            "
            @click="toggleTopPanel"
          >
            <ChevronDown v-if="panels.topCollapsed" :size="12" />
            <ChevronUp v-else :size="12" />
          </button>
        </div>

        <AssetEditor
          v-if="editingAsset"
          ref="assetEditorRef"
          :asset="editingAsset"
          @close="closeEditor"
          @saved="onAssetSaved"
        />

        <div v-show="!editingAsset" class="relative min-h-0 flex-1">
          <div
            ref="gridContainer"
            class="h-full overflow-y-auto py-1.5"
            :style="{
              paddingLeft: `${aroundGapPx}px`,
              paddingRight: `${aroundGapPx}px`,
            }"
            @mousedown="onLassoStart"
            @scroll.passive="onGridScroll"
          >
          <Transition
            name="grid-fade"
            mode="out-in"
            @after-enter="updateLoadMoreObserver"
          >
            <div :key="gridRevision">
              <div
                v-if="loading && !loadedOnce"
                class="flex items-center justify-center py-16"
              >
                <span
                  class="h-6 w-6 animate-spin rounded-full border-2 border-slate-300 border-t-violet-600 dark:border-slate-700 dark:border-t-violet-400"
                ></span>
              </div>
              <p v-else-if="!assets.length" class="text-sm text-slate-400">
                {{
                  hasActiveFilters
                    ? 'No assets match the current filters.'
                    : 'No assets yet.'
                }}
              </p>
          <div
            v-else-if="shape === 'native'"
            class="grid"
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
          <div v-else class="grid" :style="gridStyle">
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

          <div
            v-if="nextCursor"
            ref="loadMoreSentinel"
            class="flex justify-center py-4"
          >
            <Button
              label="Load more"
              severity="secondary"
              size="small"
              :loading="loadingMore"
              @click="loadMore"
            />
          </div>
            </div>
          </Transition>
          </div>

          <button
            v-if="showBackToTop"
            type="button"
            class="absolute bottom-4 right-4 z-20 inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 bg-white/90 text-slate-500 shadow-lg backdrop-blur transition-colors hover:bg-white hover:text-violet-600 dark:border-slate-800 dark:bg-slate-900/90 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-violet-400"
            title="Back to top"
            aria-label="Back to top"
            @click="scrollGridToTop"
          >
            <ChevronUp :size="16" />
          </button>
        </div>

        <div
          v-if="!editingAsset && sidebarPrefs.tags && !embedTagLocked"
          class="relative pb-2"
          :style="{
            paddingLeft: `${aroundGapPx}px`,
            paddingRight: `${aroundGapPx}px`,
            paddingTop: `${aroundGapPx}px`,
          }"
        >
          <Transition
            @enter="animatePanelEnter"
            @leave="animatePanelLeave"
          >
            <div v-if="!panels.bottomCollapsed">
              <div
                class="max-h-44 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg"
              >
                <TagsList />
              </div>
            </div>
          </Transition>
          <button
            type="button"
            class="absolute top-1.5 left-1/2 z-20 flex h-5 w-9 -translate-x-1/2 items-center justify-center rounded-full border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 text-slate-400 shadow-sm transition-colors hover:text-violet-600 dark:hover:text-violet-400"
            :title="panels.bottomCollapsed ? 'Expand tags' : 'Collapse tags'"
            :aria-label="
              panels.bottomCollapsed ? 'Expand tags' : 'Collapse tags'
            "
            @click="toggleBottomPanel"
          >
            <ChevronUp v-if="panels.bottomCollapsed" :size="12" />
            <ChevronDown v-else :size="12" />
          </button>
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
            class="flex items-center gap-1 rounded-full border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-2 py-1 shadow-lg"
          >
            <button
              class="inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
              title="Previous"
              :disabled="!canStepBack"
              @click="stepEditor(-1)"
            >
              <ChevronLeft :size="16" />
            </button>
            <span class="min-w-14 px-1 text-center text-xs tabular-nums text-slate-500 dark:text-slate-400">
              {{ editingIndex + 1 }} / {{ visibleAssets.length }}
            </span>
            <button
              class="inline-flex h-7 w-7 items-center justify-center rounded-full text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
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
          class="h-full w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg"
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
              :total="total"
              :total-capped="totalCapped"
              @changed="onSubjectChanged"
            />
            <LibraryInspector v-else />
          </div>
        </aside>

        <button
          type="button"
          class="absolute top-1/2 right-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200 dark:border-slate-800/80 bg-white dark:bg-slate-900/80 text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white dark:hover:bg-slate-800 hover:text-violet-600 dark:hover:text-violet-400"
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

    <div
      v-if="embedPicker"
      class="flex shrink-0 items-center gap-3 border-t border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-6 py-3"
    >
      <span class="text-sm text-slate-500 dark:text-slate-400">
        {{
          embedSingle ? 'Choose one asset' : 'Choose one or more assets'
        }}
      </span>
      <span class="ml-auto text-sm text-slate-500 dark:text-slate-400">
        {{ selectedIds.length }} selected
      </span>
      <Button
        label="Cancel"
        severity="secondary"
        @click="cancelEmbedSelection"
      />
      <Button
        label="Use selected"
        :disabled="!selectedIds.length"
        @click="confirmEmbedSelection"
      />
    </div>

    <ContextMenu ref="contextMenu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700 dark:text-slate-200'"
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

<style scoped>
.grid-fade-enter-active,
.grid-fade-leave-active {
  transition: opacity 120ms ease;
}

.grid-fade-enter-from,
.grid-fade-leave-to {
  opacity: 0;
}
</style>
