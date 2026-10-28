<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import ContextMenu from 'primevue/contextmenu'
import Message from 'primevue/message'
import InputNumber from 'primevue/inputnumber'
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
  Bolt,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  CloudUpload,
  Cog,
  Download,
  Eye,
  EyeSlash,
  ExternalLink,
  Filter,
  GripVertical,
  Images,
  ListCheck,
  Lock,
  Pencil,
  Refresh,
  Save,
  Search,
  SlidersH,
  SortAlt,
  Times,
  Upload,
} from '@primeicons/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as api from '../api/client'
import { auth } from '../lib/auth'
import { hasAnyCapability, hasCapability } from '../lib/capabilities'
import { assetDrag, endAssetDrag, startAssetDrag } from '../lib/assetDrag'
import { selectedIds } from '../lib/assetSelection'
import { attributeCatalog, loadAttributeCatalog } from '../lib/attributeCatalog'
import { effectiveAttributeSort, sortAttributeValues } from '../lib/attributeSort'
import { notifyAttributesChanged, openAttributesDialog } from '../lib/attributesDialog'
import { openBackupSettings } from '../lib/backupSettings'
import { notifyConversionCacheChanged } from '../lib/conversionCache'
import {
  clearAllFilters,
  clearAttributeFilter,
  clearAttributeValue,
  filterState,
  initFilterState,
  isFilterDenied,
  resetSort,
  toggleAttribute,
} from '../lib/filters'
import { confirmDeleteMenuItem } from '../lib/menu'
import {
  CARD_SHAPES,
  CARD_SIZE_DEFAULT,
  CARD_SIZE_MAX,
  CARD_SIZE_MIN,
} from '../lib/display'
import type { CardShape } from '../lib/display'
import { displaySettingsAnchor, displaySettingsRequest } from '../lib/displaySettings'
import {
  cancelEmbedSelection,
  confirmEmbedSelection,
  embed,
  markEmbedReady,
} from '../lib/embed'
import { editorDirty, leaveEditor, setEditorCloser } from '../lib/editorState'
import { searchAssetWithGoogleLens } from '../lib/googleLens'
import { library, loadLibraries, notifyLibraryChanged } from '../lib/library'
import { panels, toggleRightPanel } from '../lib/panels'
import {
  SIDEBAR_TOGGLES,
  sidebarPrefs,
  type SidebarMode,
  type SidebarToggleKey,
} from '../lib/sidebar'
import { openPresetsDialog } from '../lib/presetsDialog'
import { clearSmartSelection, smartSelection } from '../lib/smartSelection'
import { openUpload } from '../lib/upload'
import { openUploadSettings } from '../lib/uploadSettings'
import { warmPresets } from '../lib/warmPresets'
import {
  THEME_OPTIONS,
  setThemeMode,
  theme,
  type ThemeMode,
} from '../lib/theme'
import { ATTRIBUTE_UNASSIGNED } from '../types'
import type { Asset, AttributeFacet, SmartCollection } from '../types'
import AssetCard from './AssetCard.vue'
import AssetEditor from './AssetEditor.vue'
import FilterInspector from './FilterInspector.vue'
import Inspector from './Inspector.vue'
import LibraryInspector from './LibraryInspector.vue'
import MultiSelectionPanel from './MultiSelectionPanel.vue'

const toast = useToast()

initFilterState(auth.user?.scope)
void loadAttributeCatalog()

const searchQuery = computed({
  get: () => filterState.search,
  set: (value: string) => {
    filterState.search = value
  },
})
const searchInput = ref(filterState.search)
const searchInputRef = ref<HTMLInputElement | null>(null)
const hasSearch = computed(() => searchQuery.value.trim() !== '')
const assets = ref<Asset[]>([])
const nextCursor = ref<string | null>(null)
const total = ref(0)
const totalBytes = ref(0)
const totalCapped = ref(false)
const loadingMore = ref(false)
const selectedDetail = ref<Asset | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const loadedOnce = ref(false)
const gridRevision = ref(0)
const error = ref<string | null>(null)
let loadMoreTimer: number | null = null
let searchTimer: number | null = null
let loadSeq = 0

const anchorId = ref<string | null>(null)

const embedPicker = computed(() => embed.active && embed.mode !== 'library')
const embedSingle = computed(() => embed.active && embed.mode === 'single')
const embedTypeLocked = computed(() => embed.active && embed.typeLocked)
const embedMimeLocked = computed(() => embed.active && embed.mimeLocked)
const embedSearchLocked = computed(() => embed.active && embed.searchLocked)
const embedSortLocked = computed(() => embed.active && embed.sortLocked)

const DISPLAY_STORAGE_KEY = 'godam:display'

type SpacingLevel = 'dense' | 'normal' | 'relaxed' | 'spacious'

const SPACING_LEVELS: SpacingLevel[] = ['dense', 'normal', 'relaxed', 'spacious']
const SPACING_OPTIONS = [
  { label: 'Dense', value: 'dense' },
  { label: 'Normal', value: 'normal' },
  { label: 'Relaxed', value: 'relaxed' },
  { label: 'Spacious', value: 'spacious' },
]
const GRID_GAP_PX: Record<SpacingLevel, number> = {
  dense: 8,
  normal: 12,
  relaxed: 16,
  spacious: 24,
}

function normalizeSpacing(value: unknown): SpacingLevel {
  return SPACING_LEVELS.includes(value as SpacingLevel)
    ? (value as SpacingLevel)
    : 'normal'
}

type LayoutSpacing = 'dense' | 'normal' | 'relaxed' | 'spacious'

const LAYOUT_SPACING_LEVELS: LayoutSpacing[] = [
  'dense',
  'normal',
  'relaxed',
  'spacious',
]
const LAYOUT_SPACING_OPTIONS = [
  { label: 'Dense', value: 'dense' },
  { label: 'Normal', value: 'normal' },
  { label: 'Relaxed', value: 'relaxed' },
  { label: 'Spacious', value: 'spacious' },
]
// Total gap from the panel edge, including the scroll container's 4px padding.
const LAYOUT_GAP_PX: Record<LayoutSpacing, number> = {
  dense: 16,
  normal: 32,
  relaxed: 48,
  spacious: 64,
}
const SCROLL_PADDING_PX = 4

function normalizeLayoutSpacing(value: unknown): LayoutSpacing {
  return LAYOUT_SPACING_LEVELS.includes(value as LayoutSpacing)
    ? (value as LayoutSpacing)
    : 'relaxed'
}

interface DisplayPrefs {
  size: number
  shape: CardShape
  gridGap: SpacingLevel
  layoutSpacing: LayoutSpacing
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
const gridGapPx = computed(() => GRID_GAP_PX[gridGap.value])
const layoutSpacing = ref<LayoutSpacing>(
  normalizeLayoutSpacing(savedPrefs.layoutSpacing),
)
const layoutSpacingStyle = computed(() => {
  const side = LAYOUT_GAP_PX[layoutSpacing.value] - SCROLL_PADDING_PX
  return {
    marginRight: `${side}px`,
    marginLeft: `${side}px`,
  }
})
const layoutInlineGap = computed(
  () => `${LAYOUT_GAP_PX[layoutSpacing.value]}px`,
)
const layoutSpacingPx = computed(() => LAYOUT_GAP_PX[layoutSpacing.value])
const showExtension = ref(savedPrefs.showExtension ?? false)
const showName = ref(savedPrefs.showName ?? false)
const showSize = ref(savedPrefs.showSize ?? false)
const showDimensions = ref(savedPrefs.showDimensions ?? false)
const showDuration = ref(savedPrefs.showDuration ?? false)
const playAudioOnHover = ref(savedPrefs.playAudioOnHover ?? true)
const playVideoOnHover = ref(savedPrefs.playVideoOnHover ?? true)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

watch(displaySettingsRequest, () => {
  if (displaySettingsAnchor.value) {
    displayPopover.value?.toggle({
      currentTarget: displaySettingsAnchor.value,
    } as unknown as Event)
  }
})
const sortPopover = ref<InstanceType<typeof Popover> | null>(null)
const managePopover = ref<InstanceType<typeof Popover> | null>(null)
const themeMode = computed<ThemeMode>({
  get: () => theme.mode,
  set: (mode) => setThemeMode(mode),
})
const showSidebar = computed({
  get: () => !panels.leftCollapsed,
  set: (value: boolean) => {
    panels.leftCollapsed = !value
  },
})
const showInspector = computed({
  get: () => !panels.rightCollapsed,
  set: (value: boolean) => {
    panels.rightCollapsed = !value
  },
})

watch(
  [
    size,
    shape,
    gridGap,
    layoutSpacing,
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
      layoutSpacing: layoutSpacing.value,
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

const allPanelsOn = computed(
  () => showSidebar.value && showInspector.value,
)

function toggleAllPanels(): void {
  const next = !allPanelsOn.value
  showSidebar.value = next
  showInspector.value = next
}

const allSidebarSectionsOn = computed(() =>
  SIDEBAR_TOGGLES.every((toggle) => sidebarPrefs[toggle.key]),
)

function toggleAllSidebarSections(): void {
  const next = !allSidebarSectionsOn.value
  for (const toggle of SIDEBAR_TOGGLES) {
    sidebarPrefs[toggle.key] = next
  }
}

const orderedSidebarToggles = computed(() =>
  sidebarPrefs.order.flatMap((key) => {
    const toggle = SIDEBAR_TOGGLES.find((item) => item.key === key)
    return toggle ? [toggle] : []
  }),
)

const draggedSidebarSection = ref<SidebarToggleKey | null>(null)

function onSidebarSectionDragStart(
  event: DragEvent,
  key: SidebarToggleKey,
): void {
  draggedSidebarSection.value = key
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', key)
  }
}

function onSidebarSectionDragEnter(key: SidebarToggleKey): void {
  const dragged = draggedSidebarSection.value
  if (!dragged || dragged === key) return
  const order = [...sidebarPrefs.order]
  const from = order.indexOf(dragged)
  const to = order.indexOf(key)
  if (from < 0 || to < 0) return
  ;[order[from], order[to]] = [order[to], order[from]]
  sidebarPrefs.order = order
}

function onSidebarSectionDragEnd(): void {
  draggedSidebarSection.value = null
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
const gridContent = ref<HTMLElement | null>(null)
const showBackToTop = ref(false)
const transitionScrollTop = ref(0)
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

// Filters live in a shared in-memory store so the sidebar, toolbar, chips,
// and grid all read and mutate the same state.
function formatDateKey(value: Date | null): string {
  if (!value) return ''
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

type NumberFilterKey =
  | 'minSize'
  | 'maxSize'
  | 'minDuration'
  | 'maxDuration'
  | 'minPages'
  | 'maxPages'
  | 'minWidth'
  | 'maxWidth'
  | 'minHeight'
  | 'maxHeight'

type DateFilterKey =
  | 'addedAfter'
  | 'addedBefore'
  | 'modifiedAfter'
  | 'modifiedBefore'

function numberFilter(key: NumberFilterKey) {
  return computed<number | null>({
    get: () => filterState[key],
    set: (value) => {
      clearSmartSelection()
      filterState[key] = value
    },
  })
}

function dateFilter(key: DateFilterKey) {
  return computed<Date | null>({
    get: () => {
      const raw = filterState[key]
      if (!raw) return null
      const value = new Date(raw.includes('T') ? raw : `${raw}T00:00:00`)
      return Number.isNaN(value.getTime()) ? null : value
    },
    set: (value) => {
      clearSmartSelection()
      filterState[key] = value ? formatDateKey(value) : null
    },
  })
}

type OptionalFilterKey =
  | 'color'
  | 'mime'
  | 'size'
  | 'duration'
  | 'pages'
  | 'dimensions'
  | 'added'
  | 'modified'

const OPTIONAL_FILTERS: { key: OptionalFilterKey; label: string }[] = [
  { key: 'color', label: 'Color' },
  { key: 'mime', label: 'MIME type' },
  { key: 'size', label: 'Size' },
  { key: 'duration', label: 'Duration' },
  { key: 'pages', label: 'Pages' },
  { key: 'dimensions', label: 'Dimensions' },
  { key: 'added', label: 'Date added' },
  { key: 'modified', label: 'Date modified' },
]

function hasFilterValue(key: OptionalFilterKey): boolean {
  switch (key) {
    case 'color':
      return filterState.colors.length > 0
    case 'mime':
      return filterState.mimeTypes.length > 0
    case 'size':
      return filterState.minSize !== null || filterState.maxSize !== null
    case 'duration':
      return (
        filterState.minDuration !== null || filterState.maxDuration !== null
      )
    case 'pages':
      return filterState.minPages !== null || filterState.maxPages !== null
    case 'dimensions':
      return (
        filterState.minWidth !== null ||
        filterState.maxWidth !== null ||
        filterState.minHeight !== null ||
        filterState.maxHeight !== null
      )
    case 'added':
      return filterState.addedAfter !== null || filterState.addedBefore !== null
    case 'modified':
      return (
        filterState.modifiedAfter !== null ||
        filterState.modifiedBefore !== null
      )
  }
}

const visibleFilters = ref<OptionalFilterKey[]>(
  OPTIONAL_FILTERS.filter((filter) => hasFilterValue(filter.key)).map(
    (filter) => filter.key,
  ),
)
const availableOptionalFilters = computed(() =>
  OPTIONAL_FILTERS.filter((filter) => !visibleFilters.value.includes(filter.key)),
)
const filtersPopover = ref<InstanceType<typeof Popover> | null>(null)
const searchOpen = ref(false)
const searchWrapper = ref<HTMLElement | null>(null)

function toggleFilters(event: Event): void {
  filtersPopover.value?.toggle(event)
}

function onFiltersHide(): void {}

function toggleSearch(): void {
  searchOpen.value = !searchOpen.value
  if (searchOpen.value) nextTick(focusSearch)
}

function closeSearch(): void {
  searchOpen.value = false
}

function onSearchPointerDown(event: PointerEvent): void {
  if (!searchOpen.value) return
  const wrapper = searchWrapper.value
  if (wrapper && !wrapper.contains(event.target as Node)) closeSearch()
}

function focusSearch(): void {
  nextTick(() => {
    searchInputRef.value?.focus({ preventScroll: true })
    searchInputRef.value?.select()
  })
}

function commitSearch(value: string = searchInput.value): void {
  if (searchTimer !== null) {
    window.clearTimeout(searchTimer)
    searchTimer = null
  }
  if (value !== searchQuery.value) searchQuery.value = value
}

function clearSearch(): void {
  searchInput.value = ''
  commitSearch('')
}

function addFilter(key: OptionalFilterKey): void {
  if (!visibleFilters.value.includes(key)) {
    visibleFilters.value.push(key)
  }
}

function onAddFilter(value: unknown): void {
  if (typeof value === 'string' && value) {
    addFilter(value as OptionalFilterKey)
  }
}

function removeFilter(key: OptionalFilterKey): void {
  clearSmartSelection()
  switch (key) {
    case 'color':
      filterState.colors = []
      break
    case 'mime':
      filterState.mimeTypes = []
      break
    case 'size':
      filterState.minSize = null
      filterState.maxSize = null
      break
    case 'duration':
      filterState.minDuration = null
      filterState.maxDuration = null
      break
    case 'pages':
      filterState.minPages = null
      filterState.maxPages = null
      break
    case 'dimensions':
      filterState.minWidth = null
      filterState.maxWidth = null
      filterState.minHeight = null
      filterState.maxHeight = null
      break
    case 'added':
      filterState.addedAfter = null
      filterState.addedBefore = null
      break
    case 'modified':
      filterState.modifiedAfter = null
      filterState.modifiedBefore = null
      break
  }
  visibleFilters.value = visibleFilters.value.filter((item) => item !== key)
}

const typeFilter = computed<string[]>({
  get: () => filterState.types,
  set: (values) => {
    clearSmartSelection()
    filterState.types = values
  },
})
const colorFilter = computed<string[]>({
  get: () => {
    const values = filterState.colors.map((value) =>
      value === 'teal' ? 'green' : value,
    )
    return [...new Set(values)]
  },
  set: (values) => {
    clearSmartSelection()
    filterState.colors = values
  },
})
const mimeFilter = computed<string[]>({
  get: () => filterState.mimeTypes,
  set: (values) => {
    clearSmartSelection()
    filterState.mimeTypes = values
  },
})
const sort = computed({
  get: () => filterState.sort,
  set: (value: string) => {
    filterState.sort = value
  },
})

const sortField = computed({
  get: () => {
    const field = sort.value.split('-')[0]
    return SORT_FIELDS.some((option) => option.value === field) ? field : 'date'
  },
  set: (value: string) => {
    sort.value = `${value}-${sortDirection.value}`
  },
})

const sortDirection = computed({
  get: () => (sort.value.endsWith('-asc') ? 'asc' : 'desc'),
  set: (value: string) => {
    sort.value = `${sortField.value}-${value}`
  },
})

const minSize = numberFilter('minSize')
const maxSize = numberFilter('maxSize')
const minDuration = numberFilter('minDuration')
const maxDuration = numberFilter('maxDuration')
const minPages = numberFilter('minPages')
const maxPages = numberFilter('maxPages')
const minWidth = numberFilter('minWidth')
const maxWidth = numberFilter('maxWidth')
const minHeight = numberFilter('minHeight')
const maxHeight = numberFilter('maxHeight')
const addedAfter = dateFilter('addedAfter')
const addedBefore = dateFilter('addedBefore')
const modifiedAfter = dateFilter('modifiedAfter')
const modifiedBefore = dateFilter('modifiedBefore')

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

function mediaCategory(mime: string): string {
  if (mime.startsWith('image/')) return 'image'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/') || mime === 'application/ogg') return 'audio'
  if (mime.startsWith('text/') || DOCUMENT_MIMES.has(mime)) return 'document'
  return 'other'
}

const typeCounts = computed(() => {
  const counts = new Map<string, number>()
  for (const item of library.stats?.types ?? []) {
    const category = mediaCategory(item.mime_type)
    counts.set(category, (counts.get(category) ?? 0) + item.count)
  }
  return counts
})

// Broad media categories used by both the type filter and smart collections.
const TYPE_CATEGORIES = computed(() => [
  { value: 'image', label: 'Images', count: typeCounts.value.get('image') ?? 0 },
  { value: 'video', label: 'Videos', count: typeCounts.value.get('video') ?? 0 },
  { value: 'audio', label: 'Audio', count: typeCounts.value.get('audio') ?? 0 },
  { value: 'document', label: 'Documents', count: typeCounts.value.get('document') ?? 0 },
  { value: 'other', label: 'Other', count: typeCounts.value.get('other') ?? 0 },
])

const COLOR_OPTIONS = [
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

const mimeOptions = computed(() =>
  (library.stats?.types ?? [])
    .map((item) => ({
      value: item.mime_type,
      label: item.mime_type,
      count: item.count,
    }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label)),
)

const filterFingerprint = computed(() => JSON.stringify(filterState))

const SORT_FIELDS = [
  { value: 'date', label: 'Date added' },
  { value: 'modified', label: 'Date modified' },
  { value: 'name', label: 'Name' },
  { value: 'size', label: 'Size' },
]

const SORT_DIRECTIONS = [
  { value: 'asc', label: 'Ascending' },
  { value: 'desc', label: 'Descending' },
]

const hasCustomSort = computed(() => sort.value !== 'date-desc')
const sortLabel = computed(() => {
  const field =
    SORT_FIELDS.find((option) => option.value === sortField.value)?.label ??
    'Date added'
  const direction = sortDirection.value === 'asc' ? 'ascending' : 'descending'
  return `${field}, ${direction}`
})

const sidebarMode = computed<SidebarMode>({
  get: () => sidebarPrefs.mode,
  set: (mode) => {
    sidebarPrefs.mode = mode
  },
})

const SIDEBAR_MODE_OPTIONS: { label: string; value: SidebarMode }[] = [
  { label: 'Classic', value: 'classic' },
  { label: 'Attributes', value: 'attributes' },
]

const isAttributesSidebar = computed(
  () => sidebarPrefs.mode === 'attributes',
)

const canManage = computed(() =>
  hasAnyCapability([
    'settings:presets',
    'settings:upload',
    'settings:storage',
    'settings:attributes',
  ]),
)

interface FilterChip {
  key: string
  label: string
  locked?: boolean
  clear: () => void
}

function typeName(value: string): string {
  return (
    TYPE_CATEGORIES.value.find((category) => category.value === value)?.label ??
    value
  )
}

function colorName(value: string): string {
  return COLOR_OPTIONS.find((color) => color.value === value)?.label ?? value
}

function rangeLabel(
  prefix: string,
  min: number | null,
  max: number | null,
  suffix: string,
): string | null {
  if (min === null && max === null) return null
  if (min !== null && max !== null) return `${prefix}: ${min}–${max}${suffix}`
  if (min !== null) return `${prefix}: ≥ ${min}${suffix}`
  return `${prefix}: ≤ ${max}${suffix}`
}

function dateRangeLabel(
  prefix: string,
  from: string | null,
  to: string | null,
): string | null {
  if (!from && !to) return null
  const display = (value: string) => value.replace('T', ' ')
  if (from && to) return `${prefix}: ${display(from)} – ${display(to)}`
  if (from) return `${prefix}: from ${display(from)}`
  return `${prefix}: until ${display(to as string)}`
}

const filterChips = computed<FilterChip[]>(() => {
  const chips: FilterChip[] = []

  for (const [key, values] of Object.entries(filterState.attributes)) {
    const grant = auth.user?.scope?.attributes?.find(
      (item) => item.key === key,
    )
    if (grant?.mode === 'hidden') continue
    const facet = attributeCatalog.items.find((item) => item.key === key)
    for (const value of values) {
      const meta = facet?.values.find((item) => item.value === value)
      chips.push({
        key: `attribute:${key}:${value}`,
        label: `${facet?.label || key}: ${
          meta?.label ||
          (value === ATTRIBUTE_UNASSIGNED ? 'Unassigned' : value)
        }`,
        locked: grant?.mode === 'locked',
        clear: () => clearAttributeValue(key, value),
      })
    }
  }
  for (const value of filterState.types) {
    chips.push({
      key: `type:${value}`,
      label: `Type: ${typeName(value)}`,
      locked: embedTypeLocked.value,
      clear: () => {
        filterState.types = filterState.types.filter((item) => item !== value)
      },
    })
  }
  for (const value of filterState.colors) {
    chips.push({
      key: `color:${value}`,
      label: `Color: ${colorName(value)}`,
      clear: () => {
        filterState.colors = filterState.colors.filter(
          (item) => item !== value,
        )
      },
    })
  }
  for (const value of filterState.mimeTypes) {
    chips.push({
      key: `mime:${value}`,
      label: `MIME: ${value}`,
      locked: embedMimeLocked.value,
      clear: () => {
        filterState.mimeTypes = filterState.mimeTypes.filter(
          (item) => item !== value,
        )
      },
    })
  }
  const size = rangeLabel(
    'Size',
    filterState.minSize,
    filterState.maxSize,
    ' MB',
  )
  if (size) {
    chips.push({
      key: 'size',
      label: size,
      clear: () => {
        filterState.minSize = null
        filterState.maxSize = null
      },
    })
  }
  const duration = rangeLabel(
    'Duration',
    filterState.minDuration,
    filterState.maxDuration,
    's',
  )
  if (duration) {
    chips.push({
      key: 'duration',
      label: duration,
      clear: () => {
        filterState.minDuration = null
        filterState.maxDuration = null
      },
    })
  }
  const pages = rangeLabel(
    'Pages',
    filterState.minPages,
    filterState.maxPages,
    '',
  )
  if (pages) {
    chips.push({
      key: 'pages',
      label: pages,
      clear: () => {
        filterState.minPages = null
        filterState.maxPages = null
      },
    })
  }
  if (
    filterState.minWidth !== null ||
    filterState.maxWidth !== null ||
    filterState.minHeight !== null ||
    filterState.maxHeight !== null
  ) {
    const width = rangeLabel(
      'W',
      filterState.minWidth,
      filterState.maxWidth,
      'px',
    )
    const height = rangeLabel(
      'H',
      filterState.minHeight,
      filterState.maxHeight,
      'px',
    )
    chips.push({
      key: 'dimensions',
      label: `Dimensions: ${[width, height].filter(Boolean).join(' · ')}`,
      clear: () => {
        filterState.minWidth = null
        filterState.maxWidth = null
        filterState.minHeight = null
        filterState.maxHeight = null
      },
    })
  }
  const added = dateRangeLabel(
    'Added',
    filterState.addedAfter,
    filterState.addedBefore,
  )
  if (added) {
    chips.push({
      key: 'added',
      label: added,
      clear: () => {
        filterState.addedAfter = null
        filterState.addedBefore = null
      },
    })
  }
  const modified = dateRangeLabel(
    'Modified',
    filterState.modifiedAfter,
    filterState.modifiedBefore,
  )
  if (modified) {
    chips.push({
      key: 'modified',
      label: modified,
      clear: () => {
        filterState.modifiedAfter = null
        filterState.modifiedBefore = null
      },
    })
  }
  if (filterState.search.trim()) {
    chips.push({
      key: 'search',
      label: `Search: ${filterState.search.trim()}`,
      locked: embedSearchLocked.value,
      clear: () => {
        filterState.search = ''
      },
    })
  }
  if (filterState.sort !== 'date-desc') {
    chips.push({
      key: 'sort',
      label: `Sort: ${sortLabel.value}`,
      locked: embedSortLocked.value,
      clear: () => resetSort(),
    })
  }
  return chips
})

const hasFacetFilters = computed(() =>
  filterChips.value.some(
    (chip) => chip.key !== 'search' && chip.key !== 'sort',
  ),
)

const activeAttributeGrants = computed(() =>
  (auth.user?.scope?.attributes ?? []).filter(
    (grant) => grant.mode === 'active',
  ),
)

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

// A smart collection is only "active" when it was explicitly opened from the
// sidebar (tracked as client-side state); manually setting the same filters
// does not select it. It also drops out once the filters no longer match.
const activeSmartCollection = computed<SmartCollection | null>(() => {
  const id = smartSelection.id
  const filter = smartSelection.filter
  if (!id || !filter) return null
  if (
    filterState.types.length > 1 ||
    filterState.search ||
    OPTIONAL_FILTERS.some((item) => hasFilterValue(item.key))
  ) {
    return null
  }
  const matches =
    (filter.type ?? '') === (filterState.types[0] ?? '') &&
    (filter.sort || 'date-desc') === filterState.sort
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
// Types the editor can save: raster images.
const editingCanSave = computed(() => {
  const mime = editingAsset.value?.mime_type
  return Boolean(mime?.startsWith('image/'))
})
const assetEditorRef = ref<InstanceType<typeof AssetEditor> | null>(null)

const visibleAssets = computed(() => assets.value)

const hasActiveFilters = computed(
  () =>
    typeFilter.value.length > 0 ||
    Object.keys(filterState.attributes).length > 0 ||
    colorFilter.value.length > 0 ||
    mimeFilter.value.length > 0 ||
    minSize.value !== null ||
    maxSize.value !== null ||
    minDuration.value !== null ||
    maxDuration.value !== null ||
    minPages.value !== null ||
    maxPages.value !== null ||
    minWidth.value !== null ||
    maxWidth.value !== null ||
    minHeight.value !== null ||
    maxHeight.value !== null ||
    addedAfter.value !== null ||
    addedBefore.value !== null ||
    modifiedAfter.value !== null ||
    modifiedBefore.value !== null ||
    Boolean(searchQuery.value.trim()),
)

const assetCountLabel = computed(() => {
  if (selectedIds.value.length) return `${selectedIds.value.length} selected`
  const loaded = assets.value.length
  if (!total.value) return '0 item(s)'
  const totalLabel = `${total.value}${totalCapped.value ? '+' : ''}`
  if (!totalCapped.value && loaded >= total.value) return `${totalLabel} item(s)`
  return `${loaded} of ${totalLabel} item(s)`
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
  const el = gridContent.value
  if (!el) return
  containerWidth.value = Math.max(0, el.clientWidth)
}

watch([gridGap, layoutSpacing], () => {
  void nextTick(measureGrid)
})

function clearLoadMoreTimer(): void {
  if (loadMoreTimer !== null) {
    window.clearTimeout(loadMoreTimer)
    loadMoreTimer = null
  }
}

function scheduleLoadMore(): void {
  clearLoadMoreTimer()
  if (
    editingAssetId.value ||
    !nextCursor.value ||
    loadingMore.value ||
    loading.value ||
    refreshing.value ||
    error.value
  ) {
    return
  }
  loadMoreTimer = window.setTimeout(() => {
    loadMoreTimer = null
    const el = gridContainer.value
    if (
      !el ||
      editingAssetId.value ||
      !nextCursor.value ||
      loadingMore.value ||
      loading.value ||
      refreshing.value ||
      error.value
    ) {
      return
    }
    const nearBottom =
      el.scrollTop + el.clientHeight >= el.scrollHeight - 240
    if (nearBottom) void loadMore()
  }, 200)
}

function onGridScroll(): void {
  showBackToTop.value = (gridContainer.value?.scrollTop ?? 0) > 400
  scheduleLoadMore()
}

function scrollGridToTop(): void {
  gridContainer.value?.scrollTo({ top: 0, behavior: 'smooth' })
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

function attributeBulkValueItems(
  facet: AttributeFacet,
  mode: 'add' | 'replace',
): MenuItem[] {
  const values = sortAttributeValues(
    facet.values.filter((value) => value.value !== ATTRIBUTE_UNASSIGNED),
    effectiveAttributeSort(facet.key, facet.sort),
  )
  const items: MenuItem[] = []
  for (const value of values) {
    items.push({
      label: value.label || value.value,
      command: () =>
        void applyBulkAttribute(facet.key, [value.value], mode),
    })
  }
  return items
}

const attributeBulkItems = computed<MenuItem[]>(() =>
  attributeCatalog.items
    .filter((facet) =>
      facet.values.some((value) => value.value !== ATTRIBUTE_UNASSIGNED),
    )
    .map((facet) => ({
      label: facet.label || facet.key,
      items: [
        { label: 'Add', items: attributeBulkValueItems(facet, 'add') },
        { label: 'Replace', items: attributeBulkValueItems(facet, 'replace') },
        {
          label: 'Clear',
          command: () => void applyBulkAttribute(facet.key, [], 'replace'),
        },
      ],
    })),
)

async function applyBulkAttribute(
  key: string,
  values: string[],
  mode: 'add' | 'replace',
): Promise<void> {
  const ids = selectedIds.value.slice()
  if (!ids.length) return
  try {
    await api.bulkSetAssetAttributeValues(ids, key, values, mode)
    notifyLibraryChanged()
    notifyAttributesChanged()
    toast.add({
      severity: 'success',
      summary:
        mode === 'add'
          ? `Added attribute to ${ids.length} asset${ids.length === 1 ? '' : 's'}`
          : `Updated attributes on ${ids.length} asset${ids.length === 1 ? '' : 's'}`,
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update attributes',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
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
    if (hasCapability('assets:update')) {
      items.push({
        label: 'Regenerate preview',
        icon: Refresh,
        command: () => void regeneratePreviews(selectedAssets.value.slice()),
      })
    }
    if (
      hasCapability('conversions:warm') &&
      selectedAssets.value[0]?.mime_type.startsWith('image/')
    ) {
      items.push({
        label: 'Warm presets',
        icon: Bolt,
        command: () => {
          void warmPresets(selectedIds.value.slice()).then(notifyConversionCacheChanged)
        },
      })
    }
    if (
      hasCapability('attributes:select') &&
      attributeBulkItems.value.length
    ) {
      items.push({
        label: 'Attributes',
        icon: ListCheck,
        items: attributeBulkItems.value,
      })
    }
    if (hasCapability('library:update')) {
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
    }
  } else if (count > 1) {
    items.push({
      label: `Download ${count} as zip`,
      icon: Download,
      command: downloadSelectionZip,
    })
    if (hasCapability('assets:update')) {
      items.push({
        label: `Regenerate ${count} previews`,
        icon: Refresh,
        command: () => void regeneratePreviews(selectedAssets.value.slice()),
      })
    }
    if (
      hasCapability('conversions:warm') &&
      selectedAssets.value.some((asset) => asset.mime_type.startsWith('image/'))
    ) {
      items.push({
        label: `Warm presets for ${count} assets`,
        icon: Bolt,
        command: () => {
          void warmPresets(selectedIds.value.slice()).then(notifyConversionCacheChanged)
        },
      })
    }
    if (
      hasCapability('attributes:select') &&
      attributeBulkItems.value.length
    ) {
      items.push({
        label: 'Attributes',
        icon: ListCheck,
        items: attributeBulkItems.value,
      })
    }
  }
  if (hasCapability('assets:delete')) {
    items.push(
      confirmDeleteMenuItem(
        count > 1 ? `Delete ${count} assets` : 'Delete',
        () => void performDelete(selectedIds.value.slice()),
      ),
    )
  }
  return items
})

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function assetParams(): api.AssetListParams {
  const params: api.AssetListParams = {
    search: searchQuery.value.trim() || undefined,
    types: typeFilter.value.length ? typeFilter.value : undefined,
    colors: colorFilter.value.length ? colorFilter.value : undefined,
    mimeTypes: mimeFilter.value.length ? mimeFilter.value : undefined,
    minDuration: minDuration.value ?? undefined,
    maxDuration: maxDuration.value ?? undefined,
    minPages: minPages.value ?? undefined,
    maxPages: maxPages.value ?? undefined,
    minWidth: minWidth.value ?? undefined,
    maxWidth: maxWidth.value ?? undefined,
    minHeight: minHeight.value ?? undefined,
    maxHeight: maxHeight.value ?? undefined,
    minSize:
      minSize.value === null
        ? undefined
        : Math.round(minSize.value * 1024 * 1024),
    maxSize:
      maxSize.value === null
        ? undefined
        : Math.round(maxSize.value * 1024 * 1024),
    addedAfter: filterState.addedAfter || undefined,
    addedBefore: filterState.addedBefore || undefined,
    modifiedAfter: filterState.modifiedAfter || undefined,
    modifiedBefore: filterState.modifiedBefore || undefined,
    sort: isFilterDenied('sort') ? undefined : sort.value || undefined,
    limit: 100,
  }
  if (Object.keys(filterState.attributes).length) {
    params.attributes = { ...filterState.attributes }
  }
  return params
}

async function load(): Promise<void> {
  transitionScrollTop.value = gridContainer.value?.scrollTop ?? 0
  clearLoadMoreTimer()
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
    void nextTick(scheduleLoadMore)
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
    if (seq === loadSeq) void nextTick(scheduleLoadMore)
  }
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

function toggleSort(event: Event): void {
  sortPopover.value?.toggle(event)
}

function toggleManage(event: Event): void {
  managePopover.value?.toggle(event)
}

function openManage(action: () => void): void {
  managePopover.value?.hide()
  action()
}

// onChanged replaces the affected asset client-side (title/note/attributes
// changed), avoiding a full library/asset reload.
function onChanged(asset: Asset): void {
  const index = assets.value.findIndex((item) => item.id === asset.id)
  if (index >= 0) assets.value[index] = asset
  if (selectedDetail.value?.id === asset.id) selectedDetail.value = asset
}

// Refreshes the sidebar/library after a smart collection is edited in the
// Inspector, so this panel picks up the new values.
function onSubjectChanged(): void {
  notifyLibraryChanged()
}

function openEditor(asset: Asset): void {
  clearLoadMoreTimer()
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
  void nextTick(scheduleLoadMore)
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

watch(filterFingerprint, () => {
  // Changing filters while the editor is open leaves it stranded over a grid
  // it no longer belongs to. Close it unless there are unsaved edits.
  if (editingAssetId.value && !editorDirty.value) closeEditor()
  for (const filter of OPTIONAL_FILTERS) {
    if (
      hasFilterValue(filter.key) &&
      !visibleFilters.value.includes(filter.key)
    ) {
      visibleFilters.value.push(filter.key)
    }
  }
  if (!embedPicker.value) clearSelection()
  void load()
})

watch(searchQuery, (value) => {
  if (searchTimer !== null) {
    window.clearTimeout(searchTimer)
    searchTimer = null
  }
  if (value !== searchInput.value) searchInput.value = value
})

watch(searchInput, (value) => {
  if (value === searchQuery.value) return
  if (searchTimer !== null) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    searchTimer = null
    if (value !== searchQuery.value) searchQuery.value = value
  }, 300)
})

const detailRequestId = ref<string | null>(null)
async function loadSelectedDetail(id: string | null): Promise<void> {
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
}

watch(singleAssetId, (id) => void loadSelectedDetail(id), {
  immediate: true,
})

watch(
  () => library.version,
  () => {
    void load()
    void loadSelectedDetail(singleAssetId.value)
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
  document.addEventListener('pointerdown', onSearchPointerDown)
  if (gridContainer.value) {
    gridResizeObserver = new ResizeObserver(measureGrid)
    gridResizeObserver.observe(gridContainer.value)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onLassoMove)
  window.removeEventListener('mouseup', onLassoEnd)
  window.removeEventListener('keydown', onAssetsSelectAll)
  document.removeEventListener('pointerdown', onSearchPointerDown)
  gridResizeObserver?.disconnect()
  clearLoadMoreTimer()
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
          class="pointer-events-none absolute inset-0 z-20"
        >
          <div
            v-if="!isAttributesSidebar"
            class="pointer-events-auto absolute left-10 z-20 flex max-w-[calc(100%-18rem)] flex-wrap items-center gap-1.5"
            :class="embedPicker ? 'bottom-24' : 'bottom-10'"
          >
            <span
              class="inline-flex shrink-0 items-center gap-2 rounded-full border border-slate-200/80 bg-white/80 px-3 py-1.5 text-sm text-slate-500 shadow-lg backdrop-blur dark:border-slate-700/80 dark:bg-slate-900/80 dark:text-slate-400"
            >
              {{ assetCountLabel }}
              <span
                v-if="refreshing"
                class="h-4 w-4 shrink-0 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600 dark:border-slate-700 dark:border-t-brand-400"
                title="Refreshing assets"
              ></span>
              <span
                v-if="
                  embedTypeLocked ||
                  embedMimeLocked ||
                  embedSearchLocked ||
                  embedSortLocked
                "
                class="text-xs font-medium text-brand-700 dark:text-brand-400"
                title="The host application restricted this picker"
              >
                Restricted
              </span>
            </span>
            <button
              v-for="chip in filterChips"
              :key="chip.key"
              type="button"
              class="inline-flex max-w-64 items-center gap-1 rounded-full bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700 shadow-sm dark:bg-brand-950/60 dark:text-brand-300"
              :class="
                chip.locked
                  ? 'cursor-default'
                  : 'transition hover:bg-brand-100 dark:hover:bg-brand-950'
              "
              :disabled="chip.locked"
              :title="chip.locked ? 'Locked by the host application' : chip.label"
              @click="chip.clear()"
            >
              <Lock v-if="chip.locked" :size="11" class="shrink-0" />
              <span class="truncate">{{ chip.label }}</span>
              <Times v-if="!chip.locked" :size="11" class="shrink-0" />
            </button>
            <button
              v-if="filterChips.length > 1"
              type="button"
              class="rounded-full px-2 py-0.5 text-xs font-medium text-slate-500 transition hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              @click="clearAllFilters"
            >
              Clear all
            </button>
          </div>

          <div
            v-if="!isAttributesSidebar"
            class="pointer-events-auto absolute right-10 flex items-center gap-1 rounded-full border border-slate-200/80 bg-white/80 p-1.5 shadow-lg backdrop-blur dark:border-slate-700/80 dark:bg-slate-900/80"
            :class="embedPicker ? 'bottom-24' : 'bottom-10'"
          >
            <button
              v-if="hasCapability('assets:upload')"
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
              title="Upload"
              aria-label="Upload"
              @click="openUpload()"
            >
              <Upload :size="16" />
            </button>
            <div ref="searchWrapper" class="relative">
              <button
                type="button"
                class="relative inline-flex h-9 w-9 items-center justify-center rounded-full transition"
                :class="
                  hasSearch
                    ? 'bg-brand-50 text-brand-700 hover:bg-brand-100 dark:bg-brand-950/60 dark:text-brand-300 dark:hover:bg-brand-950'
                    : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white'
                "
                :aria-pressed="hasSearch"
                title="Search"
                aria-label="Search"
                @click="toggleSearch"
              >
                <Search :size="16" />
                <span
                  v-if="hasSearch"
                  class="absolute top-1.5 right-1.5 h-1.5 w-1.5 rounded-full bg-brand-500 ring-2 ring-white dark:ring-slate-900"
                />
              </button>
              <div
                v-if="searchOpen"
                class="absolute right-0 bottom-full z-30 mb-2 flex w-80 max-w-[calc(100vw-2rem)] items-center gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-slate-900 shadow-xl focus-within:border-brand-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                role="dialog"
                aria-label="Search assets"
                @keydown.esc.prevent="closeSearch"
              >
                <Search :size="16" class="shrink-0 text-slate-400" />
                <input
                  ref="searchInputRef"
                  v-model="searchInput"
                  type="text"
                  placeholder="Search assets…"
                  title="Search title, note, and filename"
                  :disabled="embedSearchLocked"
                  class="min-w-0 flex-1 border-0 bg-transparent text-sm text-slate-900 outline-none placeholder:text-slate-400 disabled:cursor-not-allowed disabled:opacity-50 dark:text-slate-100"
                  @keydown.enter.prevent="commitSearch()"
                />
                <button
                  v-if="searchInput"
                  type="button"
                  class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
                  title="Clear search"
                  aria-label="Clear search"
                  @mousedown.prevent
                  @click="clearSearch"
                >
                  <Times :size="12" />
                </button>
              </div>
            </div>
            <button
              type="button"
              class="relative inline-flex h-9 w-9 items-center justify-center rounded-full transition"
              :class="
                hasFacetFilters
                  ? 'bg-brand-50 text-brand-700 hover:bg-brand-100 dark:bg-brand-950/60 dark:text-brand-300 dark:hover:bg-brand-950'
                  : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white'
              "
              :aria-pressed="hasFacetFilters"
              title="Filters"
              aria-label="Filters"
              @click="toggleFilters"
            >
              <Filter :size="16" />
              <span
                v-if="hasFacetFilters"
                class="absolute top-1.5 right-1.5 h-1.5 w-1.5 rounded-full bg-brand-500 ring-2 ring-white dark:ring-slate-900"
              />
            </button>
            <button
              type="button"
              class="relative inline-flex h-9 w-9 items-center justify-center rounded-full transition disabled:cursor-not-allowed disabled:opacity-40"
              :class="
                hasCustomSort
                  ? 'bg-brand-50 text-brand-700 hover:bg-brand-100 dark:bg-brand-950/60 dark:text-brand-300 dark:hover:bg-brand-950'
                  : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white'
              "
              :aria-pressed="hasCustomSort"
              :title="`Sort: ${sortLabel}`"
              aria-label="Sort"
              :disabled="embedSortLocked"
              @click="toggleSort"
            >
              <SortAlt :size="16" />
              <span
                v-if="hasCustomSort"
                class="absolute top-1.5 right-1.5 h-1.5 w-1.5 rounded-full bg-brand-500 ring-2 ring-white dark:ring-slate-900"
              />
            </button>
            <button
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
              title="Display settings"
              aria-label="Display settings"
              @click="toggleDisplay"
            >
              <SlidersH :size="16" />
            </button>
            <button
              v-if="canManage"
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
              title="Manage library"
              aria-label="Manage library"
              @click="toggleManage"
            >
              <Cog :size="16" />
            </button>
          </div>

          <button
            v-if="showBackToTop"
            type="button"
            class="pointer-events-auto absolute left-10 z-20 inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 bg-white/90 text-slate-500 shadow-lg backdrop-blur transition-colors hover:bg-white hover:text-brand-700 dark:border-slate-800 dark:bg-slate-900/90 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-brand-400"
            :class="embedPicker ? 'bottom-40' : 'bottom-24'"
            title="Back to top"
            aria-label="Back to top"
            @click="scrollGridToTop"
          >
            <ChevronUp :size="16" />
          </button>

          <div
            v-if="embedPicker"
            class="pointer-events-none absolute inset-x-10 bottom-10 z-30 flex items-end justify-between gap-3"
          >
            <span
              class="pointer-events-auto inline-flex shrink-0 items-center rounded-full border border-slate-200/80 bg-white/80 px-3 py-1.5 text-sm text-slate-600 shadow-lg backdrop-blur dark:border-slate-700/80 dark:bg-slate-900/80 dark:text-slate-300"
            >
              {{ selectedIds.length }} selected
            </span>
            <div
              class="pointer-events-auto flex items-center gap-1 rounded-full border border-slate-200/80 bg-white/80 p-1.5 shadow-lg backdrop-blur dark:border-slate-700/80 dark:bg-slate-900/80"
            >
              <button
                type="button"
                class="inline-flex h-9 items-center rounded-full px-3 text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
                @click="cancelEmbedSelection"
              >
                Cancel
              </button>
              <button
                type="button"
                class="inline-flex h-9 items-center rounded-full bg-brand-600 px-3 text-sm font-medium text-white transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-40"
                :disabled="!selectedIds.length"
                @click="confirmEmbedSelection"
              >
                Select
              </button>
            </div>
          </div>

          <Popover
            v-if="!isAttributesSidebar"
            ref="filtersPopover"
            :pt="{ content: { class: '!p-0' } }"
            dismissable
            @hide="onFiltersHide"
          >
            <div class="w-[34rem] max-w-[calc(100vw-2rem)]">
              <div
                class="flex items-center justify-between border-b border-slate-100 px-3 py-2 dark:border-slate-800"
              >
                <span
                  class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Filters
                </span>
                <span class="text-xs text-slate-400">
                  {{ assetCountLabel }}
                </span>
              </div>
              <div class="max-h-[70vh] space-y-2 overflow-y-auto p-3">
                <div class="grid grid-cols-2 gap-2">
                  <MultiSelect
                    v-model="typeFilter"
                    :options="TYPE_CATEGORIES"
                    option-label="label"
                    option-value="value"
                    display="chip"
                    placeholder="Types"
                    filter
                    showClear
                    :max-selected-labels="1"
                    :disabled="embedTypeLocked"
                    class="w-full"
                  >
                    <template #option="slotProps">
                      <span class="flex w-full items-center gap-2">
                        <span class="min-w-0 flex-1 truncate">
                          {{ slotProps.option.label }}
                        </span>
                        <span class="shrink-0 text-xs text-slate-400">
                          {{ slotProps.option.count }}
                        </span>
                      </span>
                    </template>
                  </MultiSelect>
                </div>

                <div
                  v-for="grant in activeAttributeGrants"
                  :key="grant.key"
                  class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                >
                  <div class="mb-1 flex items-center justify-between">
                    <span
                      class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase"
                    >
                      {{ grant.key }}
                    </span>
                    <button
                      v-if="filterState.attributes[grant.key]?.length"
                      type="button"
                      class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                      title="Clear attribute filter"
                      @click="clearAttributeFilter(grant.key)"
                    >
                      <Times :size="12" />
                    </button>
                  </div>
                  <div class="flex flex-wrap gap-1">
                    <button
                      v-for="value in grant.values"
                      :key="value"
                      type="button"
                      class="rounded-full border px-2 py-0.5 text-xs transition"
                      :class="
                        filterState.attributes[grant.key]?.includes(value)
                          ? 'border-brand-300 bg-brand-50 text-brand-700 dark:border-brand-800 dark:bg-brand-950/50 dark:text-brand-300'
                          : 'border-slate-200 text-slate-600 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800'
                      "
                      @click="toggleAttribute(grant.key, value)"
                    >
                      {{ value }}
                    </button>
                  </div>
                </div>

                <div
                  v-if="visibleFilters.length"
                  class="grid grid-cols-2 gap-2"
                >
                  <div
                    v-if="visibleFilters.includes('color')"
                    class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Color</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove color filter"
                        @click="removeFilter('color')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <MultiSelect
                      v-model="colorFilter"
                      :options="COLOR_OPTIONS"
                      option-label="label"
                      option-value="value"
                      display="chip"
                      placeholder="Colors"
                      filter
                      showClear
                      :max-selected-labels="1"
                      class="w-full"
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
                    </MultiSelect>
                  </div>

                  <div
                    v-if="visibleFilters.includes('mime')"
                    class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">MIME type</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-slate-800"
                        :disabled="embedMimeLocked"
                        title="Remove MIME type filter"
                        @click="removeFilter('mime')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <MultiSelect
                      v-model="mimeFilter"
                      :options="mimeOptions"
                      option-label="label"
                      option-value="value"
                      display="chip"
                      placeholder="MIME types"
                      filter
                      showClear
                      :max-selected-labels="1"
                      :disabled="embedMimeLocked"
                      class="w-full"
                    >
                      <template #option="slotProps">
                        <span class="flex w-full items-center gap-2">
                          <span class="min-w-0 flex-1 truncate">
                            {{ slotProps.option.label }}
                          </span>
                          <span class="shrink-0 text-xs text-slate-400">
                            {{ slotProps.option.count }}
                          </span>
                        </span>
                      </template>
                    </MultiSelect>
                  </div>

                  <div
                    v-if="visibleFilters.includes('size')"
                    class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Size (MB)</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove size filter"
                        @click="removeFilter('size')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-2 gap-1">
                      <InputNumber v-model="minSize" :min="0" :max-fraction-digits="1" :use-grouping="false" showClear placeholder="Min" class="w-full" />
                      <InputNumber v-model="maxSize" :min="0" :max-fraction-digits="1" :use-grouping="false" showClear placeholder="Max" class="w-full" />
                    </div>
                  </div>

                  <div
                    v-if="visibleFilters.includes('duration')"
                    class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Duration (s)</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove duration filter"
                        @click="removeFilter('duration')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-2 gap-1">
                      <InputNumber v-model="minDuration" :min="0" :max-fraction-digits="1" :use-grouping="false" showClear placeholder="Min" class="w-full" />
                      <InputNumber v-model="maxDuration" :min="0" :max-fraction-digits="1" :use-grouping="false" showClear placeholder="Max" class="w-full" />
                    </div>
                  </div>

                  <div
                    v-if="visibleFilters.includes('pages')"
                    class="rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Pages</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove pages filter"
                        @click="removeFilter('pages')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-2 gap-1">
                      <InputNumber v-model="minPages" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="Min" class="w-full" />
                      <InputNumber v-model="maxPages" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="Max" class="w-full" />
                    </div>
                  </div>

                  <div
                    v-if="visibleFilters.includes('dimensions')"
                    class="col-span-2 rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Dimensions</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove dimensions filter"
                        @click="removeFilter('dimensions')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-4 gap-1">
                      <InputNumber v-model="minWidth" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="W min" class="w-full" />
                      <InputNumber v-model="maxWidth" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="W max" class="w-full" />
                      <InputNumber v-model="minHeight" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="H min" class="w-full" />
                      <InputNumber v-model="maxHeight" :min="0" :max-fraction-digits="0" :use-grouping="false" showClear placeholder="H max" class="w-full" />
                    </div>
                  </div>

                  <div
                    v-if="visibleFilters.includes('added')"
                    class="col-span-2 rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Date added</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove added date filter"
                        @click="removeFilter('added')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-2 gap-1">
                      <DatePicker v-model="addedAfter" showClear showIcon date-format="yy-mm-dd" placeholder="From" class="w-full" />
                      <DatePicker v-model="addedBefore" showClear showIcon date-format="yy-mm-dd" placeholder="To" class="w-full" />
                    </div>
                  </div>

                  <div
                    v-if="visibleFilters.includes('modified')"
                    class="col-span-2 rounded-md border border-slate-200 p-2 dark:border-slate-800"
                  >
                    <div class="mb-1 flex items-center justify-between">
                      <span class="text-[11px] font-semibold tracking-wide text-slate-400 uppercase">Date modified</span>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-red-500 dark:hover:bg-slate-800"
                        title="Remove modified date filter"
                        @click="removeFilter('modified')"
                      >
                        <Times :size="12" />
                      </button>
                    </div>
                    <div class="grid grid-cols-2 gap-1">
                      <DatePicker v-model="modifiedAfter" showClear showIcon date-format="yy-mm-dd" placeholder="From" class="w-full" />
                      <DatePicker v-model="modifiedBefore" showClear showIcon date-format="yy-mm-dd" placeholder="To" class="w-full" />
                    </div>
                  </div>
                </div>

                <Select
                  v-if="availableOptionalFilters.length"
                  :model-value="''"
                  :options="availableOptionalFilters"
                  option-label="label"
                  option-value="key"
                  placeholder="Add filter"
                  class="w-full"
                  @update:model-value="onAddFilter"
                />
              </div>
            </div>
          </Popover>

          <Popover
            v-if="!isAttributesSidebar"
            ref="sortPopover">
            <div class="w-56 space-y-3">
              <div>
                <p
                  class="mb-1.5 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Sort by
                </p>
                <Select
                  v-model="sortField"
                  :options="SORT_FIELDS"
                  option-label="label"
                  option-value="value"
                  class="w-full"
                />
              </div>
              <div>
                <p
                  class="mb-1.5 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Direction
                </p>
                <SelectButton
                  v-model="sortDirection"
                  :options="SORT_DIRECTIONS"
                  option-label="label"
                  option-value="value"
                  :allow-empty="false"
                  size="small"
                  class="w-full"
                />
              </div>
            </div>
          </Popover>

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
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Layout Spacing
                  </p>
                  <span class="text-xs text-slate-400">
                    {{ layoutSpacingPx }}px
                  </span>
                </div>
                <SelectButton
                  v-model="layoutSpacing"
                  :options="LAYOUT_SPACING_OPTIONS"
                  option-label="label"
                  option-value="value"
                  :allow-empty="false"
                  size="small"
                  class="w-full"
                />
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Grid Spacing
                  </p>
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

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Preview
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-brand-700 dark:hover:text-brand-400"
                    :title="allPreviewOn ? 'Hide all' : 'Show all'"
                    @click="toggleAllPreview"
                  >
                    <component :is="allPreviewOn ? Eye : EyeSlash" :size="14" />
                  </button>
                </div>
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Extension</span>
                    <ToggleSwitch v-model="showExtension" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Name</span>
                    <ToggleSwitch v-model="showName" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Size</span>
                    <ToggleSwitch v-model="showSize" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Dimensions</span>
                    <ToggleSwitch v-model="showDimensions" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Duration</span>
                    <ToggleSwitch v-model="showDuration" />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Sidebar style
                </p>
                <SelectButton
                  v-model="sidebarMode"
                  :options="SIDEBAR_MODE_OPTIONS"
                  option-label="label"
                  option-value="value"
                  :allow-empty="false"
                  size="small"
                  fluid
                  class="w-full"
                />
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Panels
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-brand-700 dark:hover:text-brand-400"
                    :title="allPanelsOn ? 'Hide all' : 'Show all'"
                    @click="toggleAllPanels"
                  >
                    <component :is="allPanelsOn ? Eye : EyeSlash" :size="14" />
                  </button>
                </div>
                <div class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Sidebar</span>
                    <ToggleSwitch v-model="showSidebar" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Inspector</span>
                    <ToggleSwitch v-model="showInspector" />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Sidebar sections
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-brand-700 dark:hover:text-brand-400"
                    :title="allSidebarSectionsOn ? 'Hide all' : 'Show all'"
                    @click="toggleAllSidebarSections"
                  >
                    <component
                      :is="allSidebarSectionsOn ? Eye : EyeSlash"
                      :size="14"
                    />
                  </button>
                </div>
                <div class="space-y-2">
                  <div
                    v-for="toggle in orderedSidebarToggles"
                    :key="toggle.key"
                    draggable="true"
                    class="flex cursor-grab items-center justify-between gap-2 rounded px-1 py-1 select-none transition-colors hover:bg-slate-100 active:cursor-grabbing dark:hover:bg-slate-800"
                    :class="
                      draggedSidebarSection === toggle.key ? 'opacity-50' : ''
                    "
                    @dragstart="onSidebarSectionDragStart($event, toggle.key)"
                    @dragenter.prevent="onSidebarSectionDragEnter(toggle.key)"
                    @dragover.prevent
                    @drop.prevent="onSidebarSectionDragEnd"
                    @dragend="onSidebarSectionDragEnd"
                  >
                    <div class="flex min-w-0 items-center gap-1.5">
                      <GripVertical
                        :size="14"
                        class="shrink-0 text-slate-400"
                      />
                      <span class="truncate text-xs font-medium">
                        {{ toggle.label }}
                      </span>
                    </div>
                    <ToggleSwitch v-model="sidebarPrefs[toggle.key]" />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Filter clicks
                </p>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <div class="mb-2 flex items-center justify-between">
                  <p
                    class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
                  >
                    Autoplay on Hover
                  </p>
                  <button
                    type="button"
                    class="inline-flex items-center text-slate-400 transition-colors hover:text-brand-700 dark:hover:text-brand-400"
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
                    <span class="text-xs font-medium">Audio</span>
                    <ToggleSwitch v-model="playAudioOnHover" />
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-medium">Video</span>
                    <ToggleSwitch v-model="playVideoOnHover" />
                  </div>
                </div>
              </div>

              <div class="border-t border-slate-100 dark:border-slate-800 pt-3">
                <p
                  class="mb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
                >
                  Theme
                </p>
                <SelectButton
                  v-model="themeMode"
                  :options="THEME_OPTIONS"
                  option-label="label"
                  option-value="value"
                  :allow-empty="false"
                  size="small"
                  fluid
                  class="w-full"
                />
              </div>
            </div>
          </Popover>

          <Popover
            v-if="!isAttributesSidebar"
            ref="managePopover">
            <div class="w-60">
              <p
                class="px-2 py-1 text-xs font-semibold tracking-wide text-slate-400 uppercase"
              >
                Manage
              </p>
              <button
                v-if="hasCapability('settings:presets')"
                type="button"
                class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                @click="openManage(openPresetsDialog)"
              >
                <Images :size="14" />
                <span>Presets</span>
              </button>
              <button
                v-if="hasCapability('settings:attributes')"
                type="button"
                class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                @click="openManage(openAttributesDialog)"
              >
                <Cog :size="14" />
                <span>Attributes</span>
              </button>
              <div class="my-1 border-t border-slate-100 dark:border-slate-800" />
              <button
                v-if="hasCapability('settings:upload')"
                type="button"
                class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                @click="openManage(openUploadSettings)"
              >
                <Cog :size="14" />
                <span>Upload settings</span>
              </button>
              <button
                v-if="hasCapability('settings:storage')"
                type="button"
                class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                @click="openManage(openBackupSettings)"
              >
                <CloudUpload :size="14" />
                <span>Storage &amp; backup</span>
              </button>
            </div>
          </Popover>
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
            class="godam-scroll relative h-full overflow-y-auto px-1 [overflow-anchor:none] [&_*]:[overflow-anchor:none]"
            @mousedown="onLassoStart"
            @scroll.passive="onGridScroll"
          >
          <div ref="gridContent" :style="layoutSpacingStyle">
          <Transition name="grid-fade">
            <div
              :key="gridRevision"
              :style="{
                '--grid-inline-gap': layoutInlineGap,
                '--grid-scroll-top': `${transitionScrollTop}px`,
              }"
            >
              <div
                v-if="loading && !loadedOnce"
                class="flex items-center justify-center py-16"
              >
                <span
                  class="h-6 w-6 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600 dark:border-slate-700 dark:border-t-brand-400"
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

          <div v-if="loadingMore" class="flex justify-center py-4">
            <span
              class="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600 dark:border-slate-700 dark:border-t-brand-400"
            ></span>
          </div>
            </div>
          </Transition>
          </div>
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
        class="relative h-full shrink-0 transition-[width] duration-200 ease-in-out"
        :class="panels.rightCollapsed ? '' : 'px-4'"
        :style="{ width: panels.rightCollapsed ? '0px' : '22rem' }"
      >
        <aside
          class="h-full w-full overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-lg"
        >
          <div
            class="flex h-full w-full flex-col"
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
              v-else-if="activeSmartCollection"
              :smart-collection="activeSmartCollection ?? undefined"
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
          class="absolute top-1/2 right-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200 dark:border-slate-800/80 bg-white dark:bg-slate-900/80 text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white dark:hover:bg-slate-800 hover:text-brand-700 dark:hover:text-brand-400"
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
      class="pointer-events-none fixed z-40 rounded-sm border border-brand-500 bg-brand-500/10"
      :style="lassoStyle"
    />
  </div>
</template>

<style scoped>
.grid-fade-leave-active {
  position: absolute;
  inset-inline: var(--grid-inline-gap, 0);
  top: var(--grid-scroll-top, 0px);
  pointer-events: none;
  transition: opacity 120ms ease;
}

.grid-fade-enter-active {
  transition: opacity 120ms ease;
}

.grid-fade-enter-from,
.grid-fade-leave-to {
  opacity: 0;
}

.godam-scroll {
  scrollbar-width: thin;
  scrollbar-color: var(--brand-500) transparent;
}

.godam-scroll::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.godam-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.godam-scroll::-webkit-scrollbar-thumb {
  border: none;
  border-radius: 999px;
  background: var(--brand-500);
}

.godam-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--brand-600);
}

.godam-scroll::-webkit-scrollbar-button {
  display: none;
}

.godam-scroll::-webkit-scrollbar-corner {
  background: transparent;
}
</style>
