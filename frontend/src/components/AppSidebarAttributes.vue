<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputGroup from 'primevue/inputgroup'
import InputNumber from 'primevue/inputnumber'
import MultiSelect from 'primevue/multiselect'
import InputGroupAddon from 'primevue/inputgroupaddon'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import { useToast } from 'primevue/usetoast'
import {
  ChevronLeft,
  ChevronRight,
  File,
  Image,
  Images,
  Lock,
  Search,
  SlidersH,
  SortAmountDown,
  SortAmountUp,
  Times,
  Upload,
  Video,
  VolumeUp,
} from '@primeicons/vue'
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { auth } from '../lib/auth'
import { hasCapability } from '../lib/capabilities'
import { openDisplaySettings } from '../lib/displaySettings'
import { leaveEditor } from '../lib/editorState'
import { embed } from '../lib/embed'
import {
  clearAllFilters,
  clearAttributeValue,
  clearCollectionFilter,
  clearTagFilter,
  clearTypeValue,
  filterState,
  hasActiveFilterState,
  isFilterLocked,
  isFilterVisible,
  resetSort,
  toggleColor,
} from '../lib/filters'
import { currentLibrary, library, refreshLibrary } from '../lib/library'
import { panels, toggleLeftPanel } from '../lib/panels'
import { clearSmartSelection } from '../lib/smartSelection'
import { openUpload } from '../lib/upload'
import type { AttributeFacet } from '../types'

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
    const updated = await api.updateInstance(
      name,
      activeLibrary.value?.color ?? '',
    )
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

const libraryActive = computed(
  () => route.name === 'library' && !hasActiveFilterState(),
)

async function openLibrary(): Promise<void> {
  if (!(await leaveEditor())) return
  clearAllFilters()
  void router.push({ name: 'library' })
}

const typeLocked = computed(() => embed.active && embed.typeLocked)
const searchLocked = computed(() => embed.active && embed.searchLocked)
const sortLocked = computed(
  () => (embed.active && embed.sortLocked) || isFilterLocked('sort'),
)

const grants = computed(() => auth.user?.scope?.attributes ?? [])
const lockedGrants = computed(() =>
  grants.value.filter((grant) => grant.mode === 'locked'),
)
const activeGrants = computed(() =>
  grants.value.filter((grant) => grant.mode === 'active'),
)

const attributeFacets = ref<AttributeFacet[]>([])

const displayFacets = computed<AttributeFacet[]>(() => {
  if (attributeFacets.value.length) return attributeFacets.value
  return activeGrants.value.map((grant) => ({
    key: grant.key,
    values: grant.values.map((value) => ({ value, count: 0 })),
  }))
})

async function loadAttributeFacets(): Promise<void> {
  try {
    attributeFacets.value = await api.listAttributeFacets()
  } catch {
    attributeFacets.value = []
  }
}

const TYPES = [
  { value: 'image', label: 'Image', icon: Image },
  { value: 'video', label: 'Video', icon: Video },
  { value: 'audio', label: 'Audio', icon: VolumeUp },
  { value: 'document', label: 'Document', icon: File },
]

const selectedTypes = computed({
  get: () => filterState.types,
  set: (values: string[]) => {
    if (typeLocked.value) return
    clearSmartSelection()
    filterState.types = values
  },
})

function setAttributeValues(key: string, values: string[] | null): void {
  clearSmartSelection()
  if (values?.length) filterState.attributes[key] = values
  else delete filterState.attributes[key]
}

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

function colorName(value: string): string {
  return COLOR_OPTIONS.find((color) => color.value === value)?.label ?? value
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

function numberFilter(key: NumberFilterKey) {
  return computed<number | null>({
    get: () => filterState[key],
    set: (value) => {
      clearSmartSelection()
      filterState[key] = value
    },
  })
}

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

type DateTimeFilterKey =
  | 'addedAfter'
  | 'addedBefore'
  | 'modifiedAfter'
  | 'modifiedBefore'

function parseDateTime(raw: string | null): Date | null {
  if (!raw) return null
  const zone = /(?:z|[+-]\d{2}:?\d{2})$/i.test(raw)
  const normalized =
    raw.length === 10
      ? `${raw}T00:00:00`
      : zone
        ? raw
        : raw.length === 16
          ? `${raw}:00Z`
          : `${raw}Z`
  const value = new Date(normalized)
  return Number.isNaN(value.getTime()) ? null : value
}

function dateTimeFilter(key: DateTimeFilterKey) {
  return computed<Date | null>({
    get: () => parseDateTime(filterState[key]),
    set: (value) => {
      clearSmartSelection()
      filterState[key] = value ? value.toISOString().slice(0, 16) : null
    },
  })
}

const addedAfter = dateTimeFilter('addedAfter')
const addedBefore = dateTimeFilter('addedBefore')
const modifiedAfter = dateTimeFilter('modifiedAfter')
const modifiedBefore = dateTimeFilter('modifiedBefore')

const showColorFilter = computed(
  () =>
    filterState.types.length === 0 || filterState.types.includes('image'),
)
const showDurationFilter = computed(
  () =>
    filterState.types.length === 0 ||
    filterState.types.includes('audio') ||
    filterState.types.includes('video'),
)
const showPagesFilter = computed(
  () =>
    filterState.types.length === 0 ||
    filterState.types.includes('document'),
)
const showDimensionsFilter = computed(
  () =>
    filterState.types.length === 0 ||
    filterState.types.includes('image') ||
    filterState.types.includes('video'),
)

watch(
  () => filterState.types,
  () => {
    if (!showColorFilter.value) filterState.colors = []
    if (!showDurationFilter.value) {
      filterState.minDuration = null
      filterState.maxDuration = null
    }
    if (!showPagesFilter.value) {
      filterState.minPages = null
      filterState.maxPages = null
    }
    if (!showDimensionsFilter.value) {
      filterState.minWidth = null
      filterState.maxWidth = null
      filterState.minHeight = null
      filterState.maxHeight = null
    }
  },
)

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

function dateTimeRangeLabel(
  prefix: string,
  from: string | null,
  to: string | null,
): string | null {
  if (!from && !to) return null
  const display = (value: string) => {
    const date = parseDateTime(value)
    if (!date) return value.replace('T', ' ')
    const pad = (part: number) => String(part).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
  }
  if (from && to) return `${prefix}: ${display(from)} – ${display(to)}`
  if (from) return `${prefix}: ≥ ${display(from)}`
  return `${prefix}: ≤ ${display(to as string)}`
}

const SORT_OPTIONS = [
  { value: 'date', label: 'Last uploaded' },
  { value: 'modified', label: 'Last modified' },
  { value: 'name', label: 'Name' },
  { value: 'size', label: 'Size' },
]

const sortField = computed({
  get: () => {
    const field = filterState.sort.split('-')[0]
    return SORT_OPTIONS.some((option) => option.value === field) ? field : 'date'
  },
  set: (value: string) => {
    filterState.sort = `${value}-${sortDirection.value}`
  },
})

const sortDirection = computed({
  get: () => (filterState.sort.endsWith('-asc') ? 'asc' : 'desc'),
  set: (value: string) => {
    filterState.sort = `${sortField.value}-${value}`
  },
})

const sortIsDefault = computed(() => filterState.sort === 'date-desc')

const sortFieldLabel = computed(
  () =>
    SORT_OPTIONS.find((option) => option.value === sortField.value)?.label ??
    'Last uploaded',
)

const uploadCollectionId = computed(() =>
  filterState.collections.length === 1 ? filterState.collections[0] : null,
)
const uploadTagId = computed(() =>
  filterState.tags.length === 1 ? filterState.tags[0] : null,
)

const searchInput = ref(filterState.search)
let searchTimer: number | null = null

watch(
  () => filterState.search,
  (value) => {
    if (searchTimer !== null) {
      window.clearTimeout(searchTimer)
      searchTimer = null
    }
    if (searchInput.value !== value) searchInput.value = value
  },
)

watch(searchInput, (value) => {
  if (value === filterState.search) return
  if (searchTimer !== null) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    searchTimer = null
    filterState.search = value
  }, 300)
})

function clearSearch(): void {
  if (isFilterLocked('search')) return
  searchInput.value = ''
  filterState.search = ''
}

interface Chip {
  key: string
  label: string
  locked?: boolean
  icon?: Component
  clear: () => void
}

const chips = computed<Chip[]>(() => {
  const out: Chip[] = []
  const grantFor = (key: string) =>
    grants.value.find((grant) => grant.key === key)

  for (const [key, values] of Object.entries(filterState.attributes)) {
    const grant = grantFor(key)
    if (grant?.mode === 'hidden') continue
    for (const value of values) {
      out.push({
        key: `attribute:${key}:${value}`,
        label: `${key}: ${value}`,
        locked: grant?.mode === 'locked',
        clear: () => clearAttributeValue(key, value),
      })
    }
  }
  for (const id of filterState.collections) {
    out.push({
      key: `collection:${id}`,
      label:
        library.collections.find((item) => item.id === id)?.name ?? 'Collection',
      locked:
        (embed.active && embed.collectionLocked) ||
        isFilterLocked('collection'),
      clear: () => clearCollectionFilter(id),
    })
  }
  if (filterState.uncategorized) {
    out.push({
      key: 'uncategorized',
      label: 'Uncategorized',
      locked: isFilterLocked('uncategorized'),
      clear: () => {
        if (isFilterLocked('uncategorized')) return
        filterState.uncategorized = false
      },
    })
  }
  for (const id of filterState.tags) {
    out.push({
      key: `tag:${id}`,
      label: library.tags.find((item) => item.id === id)?.name ?? 'Tag',
      locked: (embed.active && embed.tagLocked) || isFilterLocked('tag'),
      clear: () => clearTagFilter(id),
    })
  }
  if (filterState.untagged) {
    out.push({
      key: 'untagged',
      label: 'Untagged',
      locked: isFilterLocked('untagged'),
      clear: () => {
        if (isFilterLocked('untagged')) return
        filterState.untagged = false
      },
    })
  }
  for (const value of filterState.types) {
    out.push({
      key: `type:${value}`,
      label: TYPES.find((item) => item.value === value)?.label ?? value,
      locked: isFilterLocked('type'),
      clear: () => clearTypeValue(value),
    })
  }
  for (const value of filterState.colors) {
    out.push({
      key: `color:${value}`,
      label: `Color: ${colorName(value)}`,
      clear: () => {
        filterState.colors = filterState.colors.filter(
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
    out.push({
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
    out.push({
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
    out.push({
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
      'X',
      filterState.minWidth,
      filterState.maxWidth,
      'px',
    )
    const height = rangeLabel(
      'Y',
      filterState.minHeight,
      filterState.maxHeight,
      'px',
    )
    out.push({
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
  const uploaded = dateTimeRangeLabel(
    'Uploaded',
    filterState.addedAfter,
    filterState.addedBefore,
  )
  if (uploaded) {
    out.push({
      key: 'uploaded',
      label: uploaded,
      clear: () => {
        filterState.addedAfter = null
        filterState.addedBefore = null
      },
    })
  }
  const modified = dateTimeRangeLabel(
    'Modified',
    filterState.modifiedAfter,
    filterState.modifiedBefore,
  )
  if (modified) {
    out.push({
      key: 'modified',
      label: modified,
      clear: () => {
        filterState.modifiedAfter = null
        filterState.modifiedBefore = null
      },
    })
  }
  if (filterState.search.trim()) {
    out.push({
      key: 'search',
      label: `Search: ${filterState.search.trim()}`,
      locked: isFilterLocked('search'),
      clear: clearSearch,
    })
  }
  if (!sortIsDefault.value) {
    out.push({
      key: 'sort',
      label: sortFieldLabel.value,
      locked: sortLocked.value,
      icon: sortDirection.value === 'asc' ? SortAmountUp : SortAmountDown,
      clear: resetSort,
    })
  }
  return out
})

watch(
  () => library.version,
  () => {
    void refreshLibrary()
    void loadAttributeFacets()
  },
)

void loadAttributeFacets()
</script>

<template>
  <div
    class="relative h-full shrink-0 transition-[width] duration-200 ease-in-out"
    :class="panels.leftCollapsed ? '' : 'px-4'"
    :style="{ width: panels.leftCollapsed ? '0px' : '22rem' }"
  >
    <aside
      class="h-full w-full overflow-hidden rounded-xl border border-slate-200 bg-white shadow-lg dark:border-slate-800 dark:bg-slate-900"
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

        <div
          v-if="isFilterVisible('search')"
          class="border-b border-slate-100 p-3 dark:border-slate-800"
        >
          <div class="flex items-center gap-2 rounded-md border border-slate-200 px-2 py-1.5 dark:border-slate-700">
            <Search :size="15" class="shrink-0 text-slate-400" />
            <input
              v-model="searchInput"
              type="text"
              placeholder="Search assets…"
              :disabled="searchLocked"
              class="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-slate-400 disabled:cursor-not-allowed disabled:opacity-50"
            />
            <button
              v-if="searchInput"
              type="button"
              class="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              title="Clear search"
              @click="clearSearch"
            >
              <Times :size="12" />
            </button>
          </div>
        </div>

        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-3">
          <section v-if="lockedGrants.length">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Scope
            </p>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="grant in lockedGrants"
                :key="grant.key"
                class="inline-flex items-center gap-1 rounded-full bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700 dark:bg-brand-950/60 dark:text-brand-300"
                :title="`${grant.key} is locked by the host application`"
              >
                <Lock :size="10" />
                {{ grant.key }}: {{ grant.values.join(', ') }}
              </span>
            </div>
          </section>

          <section v-if="isFilterVisible('sort')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Sort
            </p>
            <InputGroup class="w-full">
              <Select
                v-model="sortField"
                :options="SORT_OPTIONS"
                option-label="label"
                option-value="value"
                :disabled="sortLocked"
                class="min-w-0 flex-1"
              />
              <InputGroupAddon class="!p-0">
                <div class="flex h-full items-center">
                  <button
                    type="button"
                    class="inline-flex h-full w-10 items-center justify-center transition disabled:cursor-not-allowed disabled:opacity-40"
                    :class="
                      sortDirection === 'asc'
                        ? 'bg-indigo-600 text-white dark:bg-orange-500 dark:text-orange-950'
                        : 'text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800'
                    "
                    :disabled="sortLocked"
                    title="Sort ascending"
                    aria-label="Sort ascending"
                    @click="sortDirection = 'asc'"
                  >
                    <SortAmountUp :size="15" />
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-full w-10 items-center justify-center transition disabled:cursor-not-allowed disabled:opacity-40"
                    :class="
                      sortDirection === 'desc'
                        ? 'bg-indigo-600 text-white dark:bg-orange-500 dark:text-orange-950'
                        : 'text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800'
                    "
                    :disabled="sortLocked"
                    title="Sort descending"
                    aria-label="Sort descending"
                    @click="sortDirection = 'desc'"
                  >
                    <SortAmountDown :size="15" />
                  </button>
                </div>
              </InputGroupAddon>
            </InputGroup>
          </section>

          <section v-for="facet in displayFacets" :key="facet.key">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              {{ facet.key }}
            </p>
            <MultiSelect
              :model-value="filterState.attributes[facet.key] ?? []"
              :options="facet.values"
              option-label="value"
              option-value="value"
              :placeholder="`Any ${facet.key}`"
              display="chip"
              :max-selected-labels="2"
              showClear
              size="small"
              class="w-full"
              @update:model-value="setAttributeValues(facet.key, $event)"
            />
          </section>

          <section v-if="isFilterVisible('type')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Media type
            </p>
            <SelectButton
              v-model="selectedTypes"
              :options="TYPES"
              option-label="label"
              option-value="value"
              multiple
              :allow-empty="true"
              :disabled="typeLocked"
              size="small"
              class="w-full"
            >
              <template #option="{ option }">
                <span class="inline-flex items-center gap-1">
                  <component :is="option.icon" :size="11" class="shrink-0" />
                  {{ option.label }}
                </span>
              </template>
            </SelectButton>
          </section>

          <section v-if="showColorFilter && isFilterVisible('color')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Color
            </p>
            <div class="grid grid-cols-11 gap-1">
              <button
                v-for="color in COLOR_OPTIONS"
                :key="color.value"
                type="button"
                class="aspect-square w-full border transition"
                :class="
                  filterState.colors.includes(color.value)
                    ? 'rounded-none border-brand-500 ring-2 ring-brand-200 dark:border-brand-400 dark:ring-brand-900'
                    : 'rounded-full border-slate-300 hover:scale-110 dark:border-slate-600'
                "
                :style="{ backgroundColor: color.hex }"
                :title="`${color.label} (at least 10% of the image)`"
                :aria-label="color.label"
                :aria-pressed="filterState.colors.includes(color.value)"
                @click="toggleColor(color.value)"
              />
            </div>
          </section>

          <section v-if="isFilterVisible('size')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Size (MB)
            </p>
            <InputGroup class="w-full">
              <InputNumber
                v-model="minSize"
                :min="0"
                :max-fraction-digits="1"
                :use-grouping="false"
                showClear
                placeholder="Min"
                size="small"
              />
              <InputNumber
                v-model="maxSize"
                :min="0"
                :max-fraction-digits="1"
                :use-grouping="false"
                showClear
                placeholder="Max"
                size="small"
                :input-style="{ borderInlineStartWidth: '0' }"
              />
            </InputGroup>
          </section>

          <section v-if="showDurationFilter && isFilterVisible('duration')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Duration (s)
            </p>
            <InputGroup class="w-full">
              <InputNumber
                v-model="minDuration"
                :min="0"
                :max-fraction-digits="1"
                :use-grouping="false"
                showClear
                placeholder="Min"
                size="small"
              />
              <InputNumber
                v-model="maxDuration"
                :min="0"
                :max-fraction-digits="1"
                :use-grouping="false"
                showClear
                placeholder="Max"
                size="small"
                :input-style="{ borderInlineStartWidth: '0' }"
              />
            </InputGroup>
          </section>

          <section v-if="showPagesFilter && isFilterVisible('pages')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Pages
            </p>
            <InputGroup class="w-full">
              <InputNumber
                v-model="minPages"
                :min="0"
                :max-fraction-digits="0"
                :use-grouping="false"
                showClear
                placeholder="Min"
                size="small"
              />
              <InputNumber
                v-model="maxPages"
                :min="0"
                :max-fraction-digits="0"
                :use-grouping="false"
                showClear
                placeholder="Max"
                size="small"
                :input-style="{ borderInlineStartWidth: '0' }"
              />
            </InputGroup>
          </section>

          <section v-if="showDimensionsFilter && isFilterVisible('dimensions')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Dimensions (px)
            </p>
            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <span class="w-3 shrink-0 text-[11px] font-medium text-slate-400">
                  X
                </span>
                <InputGroup class="w-full">
                  <InputNumber
                    v-model="minWidth"
                    :min="0"
                    :max-fraction-digits="0"
                    :use-grouping="false"
                    showClear
                    placeholder="Min"
                    size="small"
                  />
                  <InputNumber
                    v-model="maxWidth"
                    :min="0"
                    :max-fraction-digits="0"
                    :use-grouping="false"
                    showClear
                    placeholder="Max"
                    size="small"
                    :input-style="{ borderInlineStartWidth: '0' }"
                  />
                </InputGroup>
              </div>
              <div class="flex items-center gap-2">
                <span class="w-3 shrink-0 text-[11px] font-medium text-slate-400">
                  Y
                </span>
                <InputGroup class="w-full">
                  <InputNumber
                    v-model="minHeight"
                    :min="0"
                    :max-fraction-digits="0"
                    :use-grouping="false"
                    showClear
                    placeholder="Min"
                    size="small"
                  />
                  <InputNumber
                    v-model="maxHeight"
                    :min="0"
                    :max-fraction-digits="0"
                    :use-grouping="false"
                    showClear
                    placeholder="Max"
                    size="small"
                    :input-style="{ borderInlineStartWidth: '0' }"
                  />
                </InputGroup>
              </div>
            </div>
          </section>

          <section v-if="isFilterVisible('added')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Uploaded at
            </p>
            <InputGroup class="w-full">
              <DatePicker
                v-model="addedAfter"
                showTime
                hourFormat="24"
                dateFormat="yy-mm-dd"
                showClear
                showIcon
                placeholder="From"
                size="small"
                class="min-w-0"
              />
              <DatePicker
                v-model="addedBefore"
                showTime
                hourFormat="24"
                dateFormat="yy-mm-dd"
                showClear
                showIcon
                placeholder="To"
                size="small"
                class="min-w-0"
                :input-style="{ borderInlineStartWidth: '0' }"
              />
            </InputGroup>
          </section>

          <section v-if="isFilterVisible('modified')">
            <p
              class="mb-1.5 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              Modified at
            </p>
            <InputGroup class="w-full">
              <DatePicker
                v-model="modifiedAfter"
                showTime
                hourFormat="24"
                dateFormat="yy-mm-dd"
                showClear
                showIcon
                placeholder="From"
                size="small"
                class="min-w-0"
              />
              <DatePicker
                v-model="modifiedBefore"
                showTime
                hourFormat="24"
                dateFormat="yy-mm-dd"
                showClear
                showIcon
                placeholder="To"
                size="small"
                class="min-w-0"
                :input-style="{ borderInlineStartWidth: '0' }"
              />
            </InputGroup>
          </section>

          <section>
            <div class="mb-1.5 flex items-center justify-between">
              <p
                class="text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
              >
                Active State
              </p>
              <button
                v-if="chips.length > 1"
                type="button"
                class="text-[11px] font-medium text-slate-400 hover:text-brand-700 dark:hover:text-brand-400"
                @click="clearAllFilters"
              >
                Clear all
              </button>
            </div>
            <div v-if="chips.length" class="flex flex-wrap gap-1">
              <button
                v-for="chip in chips"
                :key="chip.key"
                type="button"
                class="inline-flex max-w-full items-center gap-1 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                :class="
                  chip.locked
                    ? 'cursor-default'
                    : 'transition hover:bg-slate-200 dark:hover:bg-slate-700'
                "
                :disabled="chip.locked"
                :title="chip.locked ? 'Locked by the host application' : chip.label"
                @click="chip.clear()"
              >
                <component
                  :is="chip.icon"
                  v-if="chip.icon"
                  :size="10"
                  class="shrink-0"
                />
                <Lock v-if="chip.locked" :size="10" class="shrink-0" />
                <span class="truncate">{{ chip.label }}</span>
                <Times v-if="!chip.locked" :size="10" class="shrink-0" />
              </button>
            </div>
            <p v-else class="text-xs text-slate-400">No active state.</p>
          </section>

        </div>

        <div class="border-t border-slate-100 p-3 dark:border-slate-800">
          <div class="flex items-center gap-2">
            <button
              v-if="hasCapability('assets:upload')"
              type="button"
              class="inline-flex flex-1 items-center justify-center gap-2 rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-brand-700"
              @click="openUpload([], uploadCollectionId, uploadTagId)"
            >
              <Upload :size="15" />
              Upload
            </button>
            <button
              type="button"
              class="ml-auto inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-slate-200 text-slate-500 transition hover:bg-slate-50 hover:text-slate-700 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              title="Display settings"
              aria-label="Display settings"
              @click="openDisplaySettings"
            >
              <SlidersH :size="16" />
            </button>
          </div>
        </div>
      </div>
    </aside>

    <button
      type="button"
      class="absolute top-1/2 left-full z-30 flex h-7 w-5 -translate-y-1/2 cursor-pointer items-center justify-center border border-slate-200 bg-white text-slate-400 shadow-sm transition-[translate] duration-200 ease-in-out hover:bg-white hover:text-brand-700 dark:border-slate-800/80 dark:bg-slate-900/80 dark:hover:bg-slate-800 dark:hover:text-brand-400"
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
