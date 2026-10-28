<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import Popover from 'primevue/popover'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import { Check, Filter, Palette, Pencil, Plus, SortAlt } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  applyFilterState,
  filterStateMatches,
  hasSavableView,
  snapshotFilterState,
} from '../lib/filters'
import { LIBRARY_COLORS, libraryColorHex } from '../lib/libraryColors'
import { confirmDeleteMenuItem } from '../lib/menu'
import {
  createView,
  deleteView,
  reorderViews,
  updateView,
  views,
} from '../lib/views'
import type { View } from '../types'

interface ViewMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

type SortMode = 'custom' | 'name-asc' | 'name-desc'

const DEFAULT_VIEW_COLOR = 'f97316'
const SORT_STORAGE_KEY = 'godam:views:sort'
const SORT_MODES: SortMode[] = ['custom', 'name-asc', 'name-desc']

const SORT_OPTIONS: { value: SortMode; label: string }[] = [
  { value: 'custom', label: 'Custom (drag & drop)' },
  { value: 'name-asc', label: 'Name A–Z' },
  { value: 'name-desc', label: 'Name Z–A' },
]

function loadSortMode(): SortMode {
  const raw = localStorage.getItem(SORT_STORAGE_KEY)
  return SORT_MODES.includes(raw as SortMode) ? (raw as SortMode) : 'custom'
}

const router = useRouter()
const toast = useToast()

const root = ref<HTMLElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const sortPopover = ref<InstanceType<typeof Popover> | null>(null)
const contextItem = ref<View | null>(null)
const editingId = ref<string | null>(null)
const editingName = ref('')
const creating = ref(false)
const draftName = ref('')
const sortMode = ref<SortMode>(loadSortMode())
const dragIndex = ref<number | null>(null)
const dragList = ref<View[] | null>(null)
const suppressClick = ref(false)

watch(sortMode, (value) => {
  try {
    localStorage.setItem(SORT_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the choice simply won't persist.
  }
})

function colorOf(item: View): string {
  return libraryColorHex(item.color)
}

// Sibling order depends on the chosen display mode; "custom" keeps the stored
// drag-and-drop order.
function compareViews(a: View, b: View): number {
  switch (sortMode.value) {
    case 'name-asc':
      return a.name.localeCompare(b.name)
    case 'name-desc':
      return b.name.localeCompare(a.name)
    default:
      return a.sort - b.sort || a.name.localeCompare(b.name)
  }
}

const sortedItems = computed(() => [...views.items].sort(compareViews))

// While dragging in custom mode the live order lives here so the rows can
// reflow under the pointer; otherwise the computed sorted order is shown.
const displayItems = computed(() => dragList.value ?? sortedItems.value)

const hasFilters = computed(() => hasSavableView())

function isActive(item: View): boolean {
  return filterStateMatches(item.filter)
}

function apply(item: View): void {
  if (suppressClick.value) {
    suppressClick.value = false
    return
  }
  applyFilterState(item.filter)
  void router.push({ name: 'library' })
}

function focusRename(id: string): void {
  nextTick(() => {
    const el = root.value?.querySelector<HTMLInputElement>(
      `[data-view-rename="${id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

function startCreate(): void {
  if (!hasFilters.value) return
  editingId.value = null
  creating.value = true
  draftName.value = ''
  nextTick(() => {
    root.value?.querySelector<HTMLInputElement>('[data-view-draft]')?.focus()
  })
}

function cancelCreate(): void {
  creating.value = false
  draftName.value = ''
}

function commitCreate(): void {
  if (!creating.value) return
  const name = draftName.value.trim()
  creating.value = false
  draftName.value = ''
  if (!name) return
  createView(name, snapshotFilterState(), DEFAULT_VIEW_COLOR)
}

function startRename(item: View): void {
  menu.value?.hide()
  creating.value = false
  editingId.value = item.id
  editingName.value = item.name
  focusRename(item.id)
}

function cancelRename(): void {
  editingId.value = null
}

function commitRename(): void {
  const id = editingId.value
  if (!id) return
  editingId.value = null
  const item = views.items.find((entry) => entry.id === id)
  const name = editingName.value.trim()
  if (!item || !name || name === item.name) return
  updateView(id, { name })
}

function setColor(color: string): void {
  const item = contextItem.value
  menu.value?.hide()
  if (!item) return
  updateView(item.id, { color })
}

function remove(item: View): void {
  menu.value?.hide()
  deleteView(item.id)
  toast.add({
    severity: 'success',
    summary: 'View deleted',
    life: 2500,
  })
}

function openMenu(event: MouseEvent, item: View): void {
  contextItem.value = item
  menu.value?.show(event)
}

const menuItems = computed<ViewMenuItem[]>(() => {
  const item = contextItem.value
  if (!item) return []
  return [
    {
      label: 'Rename',
      menuIcon: Pencil,
      command: () => startRename(item),
    },
    {
      label: 'Color',
      menuIcon: Palette,
      items: LIBRARY_COLORS.map((option) => ({
        label: option.name,
        color: libraryColorHex(option.value),
        colorOption: true,
        command: () => setColor(option.value),
      })),
    },
    { separator: true },
    confirmDeleteMenuItem('Delete', () => remove(item)),
  ]
})

function toggleSort(event: Event): void {
  sortPopover.value?.toggle(event)
}

function setSort(mode: SortMode): void {
  sortMode.value = mode
  sortPopover.value?.hide()
}

function onDragStart(index: number, event: DragEvent): void {
  if (sortMode.value !== 'custom' || editingId.value) {
    event.preventDefault()
    return
  }
  dragList.value = [...sortedItems.value]
  dragIndex.value = index
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', displayItems.value[index]?.id ?? '')
  }
}

// As the pointer passes over another row the dragged row takes its place, so
// the list reflows live instead of only highlighting a drop target.
function onDragEnter(index: number): void {
  const list = dragList.value
  const from = dragIndex.value
  if (!list || from === null || from === index) return
  const next = [...list]
  const [moved] = next.splice(from, 1)
  if (!moved) return
  next.splice(index, 0, moved)
  dragList.value = next
  dragIndex.value = index
}

function onDrop(): void {
  commitDrag()
}

function onDragEnd(): void {
  // Fires after drop too; only reset here if the drag was cancelled.
  if (dragList.value) {
    dragList.value = null
    dragIndex.value = null
  }
}

function commitDrag(): void {
  const list = dragList.value
  dragList.value = null
  dragIndex.value = null
  if (!list) return
  // The drop is followed by a click on the row; suppress navigation once.
  suppressClick.value = true
  setTimeout(() => {
    suppressClick.value = false
  }, 0)
  list.forEach((item, position) => {
    item.sort = position
  })
  views.items = list
  reorderViews(list.map((item, position) => ({ id: item.id, sort: position })))
}
</script>

<template>
  <div ref="root">
    <div class="mb-1 flex items-center justify-between">
      <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
        Views
        <template v-if="views.items.length">
          ({{ views.items.length }})
        </template>
      </span>
      <div class="flex items-center gap-2">
        <button
          class="inline-flex text-slate-400 hover:text-brand-700 dark:hover:text-brand-400"
          title="Sort views"
          @click="toggleSort"
        >
          <SortAlt :size="14" />
        </button>
        <button
          class="inline-flex"
          :class="
            hasFilters
              ? 'text-slate-400 hover:text-brand-700 dark:hover:text-brand-400'
              : 'cursor-not-allowed text-slate-300'
          "
          :disabled="!hasFilters"
          :title="
            hasFilters
              ? 'Save the current filters as a view'
              : 'Apply some filters first'
          "
          @click="startCreate"
        >
          <Plus :size="14" />
        </button>
      </div>
    </div>

    <Popover ref="sortPopover">
      <div class="w-56">
        <button
          v-for="option in SORT_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            sortMode === option.value
              ? 'bg-brand-50 dark:bg-brand-950/50 text-brand-700 dark:text-brand-300'
              : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
          "
          @click="setSort(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="sortMode === option.value" :size="12" class="shrink-0" />
        </button>
      </div>
    </Popover>

    <div class="space-y-0.5">
      <div
        v-for="(item, index) in displayItems"
        :key="item.id"
        :draggable="sortMode === 'custom' && editingId !== item.id"
        class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 select-none"
        :class="[
          isActive(item)
            ? 'bg-brand-50 dark:bg-brand-950/50 text-brand-700 dark:text-brand-300'
            : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white',
          dragIndex === index ? 'opacity-40' : '',
        ]"
        @click="apply(item)"
        @contextmenu.prevent="openMenu($event, item)"
        @dragstart="onDragStart(index, $event)"
        @dragenter="onDragEnter(index)"
        @dragover.prevent
        @drop.prevent="onDrop"
        @dragend="onDragEnd"
      >
        <Filter :size="18" class="shrink-0" :style="{ color: colorOf(item) }" />
        <input
          v-if="editingId === item.id"
          v-model="editingName"
          :data-view-rename="item.id"
          type="text"
          class="min-w-0 flex-1 border-b border-brand-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none"
          @click.stop
          @dragstart.stop.prevent
          @keydown.stop
          @keydown.enter="commitRename"
          @keydown.esc="cancelRename"
          @blur="commitRename"
        />
        <span v-else class="min-w-0 flex-1 truncate text-sm">{{ item.name }}</span>
      </div>

      <div v-if="creating" class="flex items-center gap-2 rounded-md px-2 py-1">
        <Filter
          :size="18"
          class="shrink-0"
          :style="{ color: libraryColorHex(DEFAULT_VIEW_COLOR) }"
        />
        <input
          v-model="draftName"
          data-view-draft
          type="text"
          placeholder="View name"
          class="min-w-0 flex-1 border-b border-brand-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none placeholder:text-slate-300 dark:placeholder:text-slate-500"
          @keydown.stop
          @keydown.enter="commitCreate"
          @keydown.esc="cancelCreate"
          @blur="commitCreate"
        />
      </div>
    </div>

    <ContextMenu ref="menu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700 dark:text-slate-200'"
          @click.prevent
        >
          <span
            v-if="slotProps.item.colorOption"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <component
            :is="slotProps.item.menuIcon || slotProps.item.icon"
            v-else-if="slotProps.item.menuIcon || slotProps.item.icon"
            :size="14"
            class="shrink-0"
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextItem?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-brand-700 dark:text-brand-400"
          />
        </a>
      </template>
    </ContextMenu>
  </div>
</template>
