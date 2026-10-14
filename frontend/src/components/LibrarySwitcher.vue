<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  Check,
  ChevronDown,
  ChevronUp,
  Database,
  GripVertical,
  Palette,
  Pencil,
  Plus,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import {
  currentLibrary,
  library,
  loadLibraries,
  selectLibrary,
  syncAfterLibraryChange,
} from '../lib/library'
import {
  DEFAULT_LIBRARY_COLOR,
  LIBRARY_COLORS,
  libraryColorHex,
} from '../lib/libraryColors'
import { confirmDeleteMenuItem } from '../lib/menu'
import type { Library } from '../types'

interface LibraryMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

const route = useRoute()
const router = useRouter()
const toast = useToast()

const root = ref<HTMLElement | null>(null)
const open = ref(false)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const contextLib = ref<Library | null>(null)

const editingId = ref<string | null>(null)
const editingName = ref('')

const creating = ref(false)
const newName = ref('')

const dragId = ref<string | null>(null)
const dragOverId = ref<string | null>(null)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function toggleOpen(): void {
  if (open.value) close()
  else open.value = true
}

function close(): void {
  open.value = false
  editingId.value = null
  creating.value = false
  newName.value = ''
}

function choose(lib: Library): void {
  if (editingId.value === lib.id) return
  if (lib.id !== library.currentId) {
    selectLibrary(lib.id)
    if (route.query.collection || route.query.tag) {
      void router.replace({ query: {} })
    }
  }
  close()
}

function replaceLocal(updated: Library): void {
  const index = library.libraries.findIndex((item) => item.id === updated.id)
  if (index >= 0) library.libraries[index] = updated
}

// ---- Rename (inline) ----

function startRename(lib: Library): void {
  menu.value?.hide()
  creating.value = false
  editingId.value = lib.id
  editingName.value = lib.name
  nextTick(() => {
    const el = root.value?.querySelector<HTMLInputElement>(
      `[data-library-rename="${lib.id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

function cancelRename(): void {
  editingId.value = null
}

async function commitRename(): Promise<void> {
  const id = editingId.value
  if (!id) return
  editingId.value = null
  const lib = library.libraries.find((item) => item.id === id)
  const name = editingName.value.trim()
  if (!lib || !name || name === lib.name) return
  try {
    const updated = await api.updateLibrary(id, name, lib.color, lib.sort)
    replaceLocal(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

// ---- Create (inline, appended last; cancelled when unnamed) ----

function nextSort(): number {
  return library.libraries.reduce((max, lib) => Math.max(max, lib.sort), -1) + 1
}

function startCreate(): void {
  editingId.value = null
  creating.value = true
  newName.value = ''
  nextTick(() => {
    root.value?.querySelector<HTMLInputElement>('[data-library-create]')?.focus()
  })
}

function cancelCreate(): void {
  creating.value = false
  newName.value = ''
}

async function commitCreate(): Promise<void> {
  if (!creating.value) return
  const name = newName.value.trim()
  creating.value = false
  newName.value = ''
  if (!name) return
  try {
    await api.createLibrary(name, DEFAULT_LIBRARY_COLOR, nextSort())
    await loadLibraries()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create library',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

// ---- Context menu ----

function openMenu(lib: Library, event: MouseEvent): void {
  contextLib.value = lib
  menu.value?.show(event)
}

const menuItems = computed<LibraryMenuItem[]>(() => {
  const lib = contextLib.value
  if (!lib) return []
  return [
    {
      label: 'Rename',
      menuIcon: Pencil,
      command: () => startRename(lib),
    },
    {
      label: 'Color',
      menuIcon: Palette,
      items: LIBRARY_COLORS.map((option) => ({
        label: option.name,
        color: libraryColorHex(option.value),
        colorOption: true,
        command: () => void setColor(option.value),
      })),
    },
    { separator: true },
    confirmDeleteMenuItem('Delete', () => void removeLib(lib)),
  ]
})

async function setColor(color: string): Promise<void> {
  const lib = contextLib.value
  menu.value?.hide()
  if (!lib) return
  try {
    const updated = await api.updateLibrary(lib.id, lib.name, color, lib.sort)
    replaceLocal(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function removeLib(lib: Library): Promise<void> {
  menu.value?.hide()
  try {
    await api.deleteLibrary(lib.id)
    await syncAfterLibraryChange()
    if (route.query.collection || route.query.tag) {
      void router.replace({ query: {} })
    }
    toast.add({ severity: 'success', summary: 'Library deleted', life: 2500 })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Delete failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

// ---- Sorting (drag to reorder) ----

function onDragStart(lib: Library): void {
  dragId.value = lib.id
}

function onDragEnd(): void {
  dragId.value = null
  dragOverId.value = null
}

function onDragOver(id: string): void {
  if (dragId.value && dragId.value !== id) dragOverId.value = id
}

function onDragLeave(id: string): void {
  if (dragOverId.value === id) dragOverId.value = null
}

async function onDrop(targetId: string): Promise<void> {
  const from = dragId.value
  dragId.value = null
  dragOverId.value = null
  if (!from || from === targetId) return
  const list = [...library.libraries]
  const fromIndex = list.findIndex((item) => item.id === from)
  const toIndex = list.findIndex((item) => item.id === targetId)
  if (fromIndex < 0 || toIndex < 0) return
  const [moved] = list.splice(fromIndex, 1)
  list.splice(toIndex, 0, moved)
  library.libraries = list.map((lib, index) => ({ ...lib, sort: index }))
  try {
    await api.reorderLibraries(
      library.libraries.map((lib) => ({ id: lib.id, sort: lib.sort })),
    )
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Reorder failed',
      detail: toMessage(e),
      life: 4000,
    })
    await loadLibraries()
  }
}

// ---- Dismissal ----

function onPointerDown(event: MouseEvent): void {
  if (!open.value) return
  const target = event.target as HTMLElement | null
  if (!target) return
  if (root.value?.contains(target)) return
  // Keep open while the (teleported) context menu overlay is in use.
  if (target.closest('.p-contextmenu')) return
  close()
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && open.value) close()
}

onMounted(() => {
  window.addEventListener('mousedown', onPointerDown)
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousedown', onPointerDown)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      class="group flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-2 text-left hover:bg-slate-100"
      title="Switch library"
      @click="toggleOpen"
    >
      <Database
        :size="20"
        class="shrink-0"
        :style="{ color: libraryColorHex(currentLibrary()?.color ?? '') }"
      />
      <span class="min-w-0 flex-1 truncate text-sm font-semibold text-slate-800">
        {{ currentLibrary()?.name ?? 'No library' }}
      </span>
      <span
        class="flex shrink-0 flex-col text-slate-400 group-hover:text-slate-500"
        aria-hidden="true"
      >
        <ChevronUp :size="12" class="-mb-1" />
        <ChevronDown :size="12" />
      </span>
    </button>

    <div
      v-if="open"
      class="absolute left-0 right-0 z-50 mt-1 rounded-lg border border-slate-200 bg-white p-2 shadow-lg"
    >
      <div class="mb-1 flex items-center justify-between px-1">
        <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
          Libraries
        </span>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600"
          title="New library"
          @click="startCreate"
        >
          <Plus :size="14" />
        </button>
      </div>

      <div class="space-y-0.5">
        <div
          v-for="lib in library.libraries"
          :key="lib.id"
          class="group flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm select-none"
          :class="[
            dragOverId === lib.id ? 'ring-2 ring-violet-400 ring-inset' : '',
            lib.id === library.currentId
              ? 'bg-violet-50 text-violet-700'
              : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
          ]"
          :draggable="editingId !== lib.id"
          :data-library-id="lib.id"
          @click="choose(lib)"
          @contextmenu.prevent="openMenu(lib, $event)"
          @dragstart="onDragStart(lib)"
          @dragend="onDragEnd"
          @dragover.prevent="onDragOver(lib.id)"
          @dragleave="onDragLeave(lib.id)"
          @drop.prevent="onDrop(lib.id)"
        >
          <GripVertical
            v-if="editingId !== lib.id"
            :size="14"
            class="shrink-0 cursor-grab text-slate-300 group-hover:text-slate-400"
          />
          <span
            class="h-2.5 w-2.5 shrink-0 rounded-full"
            :style="{ backgroundColor: libraryColorHex(lib.color) }"
          />
          <input
            v-if="editingId === lib.id"
            v-model="editingName"
            :data-library-rename="lib.id"
            type="text"
            class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 outline-none"
            @click.stop
            @dblclick.stop
            @keydown.stop
            @keydown.enter="commitRename"
            @keydown.esc="cancelRename"
            @blur="commitRename"
          />
          <span v-else class="min-w-0 flex-1 truncate text-left">{{ lib.name }}</span>
          <Check
            v-if="lib.id === library.currentId"
            :size="14"
            class="shrink-0 text-violet-600"
          />
        </div>

        <div v-if="creating" class="flex items-center gap-2 rounded px-2 py-1.5 text-sm">
          <span
            class="h-2.5 w-2.5 shrink-0 rounded-full"
            :style="{ backgroundColor: libraryColorHex(DEFAULT_LIBRARY_COLOR) }"
          />
          <input
            v-model="newName"
            data-library-create
            type="text"
            placeholder="Library name"
            class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 outline-none placeholder:text-slate-300"
            @keydown.stop
            @keydown.enter="commitCreate"
            @keydown.esc="cancelCreate"
            @blur="commitCreate"
          />
        </div>
      </div>
    </div>

    <ContextMenu ref="menu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700'"
          @click.prevent
        >
          <span
            v-if="slotProps.item.colorOption"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <component
            :is="slotProps.item.menuIcon || slotProps.icon"
            v-else-if="slotProps.item.menuIcon || slotProps.icon"
            :size="14"
            class="shrink-0"
            :style="
              slotProps.item.color ? { color: slotProps.item.color } : undefined
            "
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextLib?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-violet-600"
          />
          <AngleRight
            v-else-if="slotProps.hasSubmenu"
            :size="12"
            class="shrink-0"
          />
        </a>
      </template>
    </ContextMenu>
  </div>
</template>
