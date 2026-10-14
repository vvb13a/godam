<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import { Check, Filter, Palette, Pencil, Plus } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { library } from '../lib/library'
import { LIBRARY_COLORS, libraryColorHex } from '../lib/libraryColors'
import { confirmDeleteMenuItem } from '../lib/menu'
import type { SmartCollection, SmartCollectionFilter } from '../types'

interface SmartMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

const DEFAULT_SMART_COLOR = '8b5cf6'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const root = ref<HTMLElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const contextItem = ref<SmartCollection | null>(null)
const editingId = ref<string | null>(null)
const editingName = ref('')
const creating = ref(false)
const draftName = ref('')

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function colorOf(item: SmartCollection): string {
  return libraryColorHex(item.color)
}

// The smart collection is a preset link: it stores the current asset-view
// filters (and sort), and selecting it just navigates with those query params.
function currentFilter(): SmartCollectionFilter {
  const q = route.query
  const filter: SmartCollectionFilter = {}
  if (typeof q.collection === 'string' && q.collection) {
    filter.collection = q.collection
  }
  if (typeof q.tag === 'string' && q.tag) filter.tag = q.tag
  if (typeof q.type === 'string' && q.type) filter.type = q.type
  if (typeof q.sort === 'string' && q.sort) filter.sort = q.sort
  return filter
}

const hasFilters = computed(() => {
  const q = route.query
  return Boolean(
    (typeof q.collection === 'string' && q.collection) ||
      (typeof q.tag === 'string' && q.tag) ||
      (typeof q.type === 'string' && q.type),
  )
})

function isActive(item: SmartCollection): boolean {
  const current = currentFilter()
  const f = item.filter
  return (
    (f.collection ?? '') === (current.collection ?? '') &&
    (f.tag ?? '') === (current.tag ?? '') &&
    (f.type ?? '') === (current.type ?? '') &&
    (f.sort ?? '') === (current.sort ?? '')
  )
}

function apply(item: SmartCollection): void {
  if (editingId.value) return
  const f = item.filter
  const query: Record<string, string> = {}
  if (f.collection) query.collection = f.collection
  if (f.tag) query.tag = f.tag
  if (f.type) query.type = f.type
  if (f.sort) query.sort = f.sort
  void router.push({ name: 'all', query })
}

function focusRename(id: string): void {
  nextTick(() => {
    const el = root.value?.querySelector<HTMLInputElement>(
      `[data-smart-rename="${id}"]`,
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
    root.value
      ?.querySelector<HTMLInputElement>('[data-smart-draft]')
      ?.focus()
  })
}

function cancelCreate(): void {
  creating.value = false
  draftName.value = ''
}

async function commitCreate(): Promise<void> {
  if (!creating.value) return
  const libraryId = library.currentId
  const name = draftName.value.trim()
  const filter = currentFilter()
  creating.value = false
  draftName.value = ''
  if (!name || !libraryId) return
  try {
    const created = await api.createSmartCollection(
      libraryId,
      name,
      DEFAULT_SMART_COLOR,
      filter,
    )
    library.smartCollections.push(created)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create smart collection',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startRename(item: SmartCollection): void {
  menu.value?.hide()
  creating.value = false
  editingId.value = item.id
  editingName.value = item.name
  focusRename(item.id)
}

function cancelRename(): void {
  editingId.value = null
}

function replaceLocal(updated: SmartCollection): void {
  const index = library.smartCollections.findIndex(
    (item) => item.id === updated.id,
  )
  if (index >= 0) library.smartCollections[index] = updated
  else library.smartCollections.push(updated)
}

async function commitRename(): Promise<void> {
  const id = editingId.value
  if (!id) return
  editingId.value = null
  const item = library.smartCollections.find((entry) => entry.id === id)
  const name = editingName.value.trim()
  if (!item || !name || name === item.name) return
  try {
    replaceLocal(await api.updateSmartCollection(id, name, item.color))
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function setColor(color: string): Promise<void> {
  const item = contextItem.value
  menu.value?.hide()
  if (!item) return
  try {
    replaceLocal(await api.updateSmartCollection(item.id, item.name, color))
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function remove(item: SmartCollection): Promise<void> {
  menu.value?.hide()
  try {
    await api.deleteSmartCollection(item.id)
    library.smartCollections = library.smartCollections.filter(
      (entry) => entry.id !== item.id,
    )
    toast.add({
      severity: 'success',
      summary: 'Smart collection deleted',
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

function openMenu(event: MouseEvent, item: SmartCollection): void {
  contextItem.value = item
  menu.value?.show(event)
}

const menuItems = computed<SmartMenuItem[]>(() => {
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
        command: () => void setColor(option.value),
      })),
    },
    { separator: true },
    confirmDeleteMenuItem('Delete', () => void remove(item)),
  ]
})
</script>

<template>
  <div ref="root">
    <div class="flex items-center justify-between pt-3 pb-0 pr-3 pl-[1.375rem]">
      <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
        Smart Collections
        <template v-if="library.smartCollections.length">
          ({{ library.smartCollections.length }})
        </template>
      </span>
      <button
        class="inline-flex"
        :class="
          hasFilters
            ? 'text-slate-400 hover:text-violet-600'
            : 'cursor-not-allowed text-slate-300'
        "
        :disabled="!hasFilters"
        :title="
          hasFilters
            ? 'Save the current filters as a smart collection'
            : 'Apply some filters first'
        "
        @click="startCreate"
      >
        <Plus :size="14" />
      </button>
    </div>

    <div class="space-y-0.5 px-[0.875rem] py-1">
      <div
        v-for="item in library.smartCollections"
        :key="item.id"
        class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 select-none"
        :class="
          isActive(item)
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
        @click="apply(item)"
        @contextmenu.prevent="openMenu($event, item)"
      >
        <Filter :size="18" class="shrink-0" :style="{ color: colorOf(item) }" />
        <input
          v-if="editingId === item.id"
          v-model="editingName"
          :data-smart-rename="item.id"
          type="text"
          class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 outline-none"
          @click.stop
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
          :style="{ color: libraryColorHex(DEFAULT_SMART_COLOR) }"
        />
        <input
          v-model="draftName"
          data-smart-draft
          type="text"
          placeholder="Smart collection name"
          class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 outline-none placeholder:text-slate-300"
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
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextItem?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-violet-600"
          />
        </a>
      </template>
    </ContextMenu>
  </div>
</template>
