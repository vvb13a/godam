<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  Copy,
  Folder,
  Images,
  Pencil,
  Plus,
  Search,
  Tag,
  Tags,
  Times,
  Trash,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { assetDrag } from '../lib/assetDrag'
import { COLLECTION_ICONS, collectionIcon } from '../lib/collectionIcons'
import { openCommandMenu } from '../lib/command'
import {
  initLibrary,
  library,
  notifyLibraryChanged,
  refreshLibrary,
} from '../lib/library'
import type { Collection } from '../types'
import { openTagsDialog } from '../lib/tagsDialog'
import Kbd from './Kbd.vue'
import LibrarySwitcher from './LibrarySwitcher.vue'
import UserMenu from './UserMenu.vue'

const route = useRoute()
const router = useRouter()
const confirm = useConfirm()
const toast = useToast()

const activeCollection = computed(
  () => (route.query.collection as string | undefined) ?? null,
)
const activeTag = computed(() => (route.query.tag as string | undefined) ?? null)
const allActive = computed(
  () => route.name === 'all' && !activeCollection.value && !activeTag.value,
)
const uncategorizedActive = computed(
  () => route.name === 'all' && activeCollection.value === 'uncategorized',
)
const untaggedActive = computed(
  () => route.name === 'all' && activeTag.value === 'untagged',
)

const dialogVisible = ref(false)
const editingId = ref<string | null>(null)
const name = ref('')
const description = ref('')
const icon = ref('folder')
const busy = ref(false)

const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const contextCollection = ref<Collection | null>(null)
const dragOverId = ref<string | null>(null)

watch(
  () => assetDrag.active,
  (active) => {
    if (!active) dragOverId.value = null
  },
)

const menuItems = computed<MenuItem[]>(() => [
  {
    label: 'Edit',
    icon: Pencil,
    command: () => {
      const collection = contextCollection.value
      if (collection) openEdit(collection)
    },
  },
  {
    label: 'Duplicate',
    icon: Copy,
    command: () => {
      const collection = contextCollection.value
      if (collection) duplicate(collection)
    },
  },
  { separator: true },
  {
    label: 'Delete',
    icon: Trash,
    class: 'text-red-600',
    command: () => {
      const collection = contextCollection.value
      if (collection) remove(collection.id, collection.name)
    },
  },
])

function iconFor(collection: Collection): Component {
  return collectionIcon(collection.icon) ?? Folder
}

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function openCreate(): void {
  editingId.value = null
  name.value = ''
  description.value = ''
  icon.value = 'folder'
  dialogVisible.value = true
}

function openEdit(collection: Collection): void {
  editingId.value = collection.id
  name.value = collection.name
  description.value = collection.description ?? ''
  icon.value = collection.icon ?? 'folder'
  dialogVisible.value = true
}

function onContextMenu(event: MouseEvent, collection: Collection): void {
  contextCollection.value = collection
  menu.value?.show(event)
}

function onDragOver(collectionId: string): void {
  if (!assetDrag.active) return
  dragOverId.value = collectionId
}

function onDragLeave(collectionId: string): void {
  if (dragOverId.value === collectionId) dragOverId.value = null
}

async function onDrop(event: DragEvent, collection: Collection): Promise<void> {
  if (!assetDrag.active) return
  dragOverId.value = null

  let ids = assetDrag.assetIds
  if (!ids.length) {
    const raw = event.dataTransfer?.getData('text/plain') ?? ''
    try {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        ids = parsed.filter((value) => typeof value === 'string')
      }
    } catch {
      if (raw) ids = [raw]
    }
  }
  if (!ids.length) return

  try {
    await Promise.all(ids.map((id) => api.addToCollection(id, collection.id)))
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary:
        ids.length > 1 ? `Added ${ids.length} assets` : 'Added to collection',
      detail: collection.name,
      life: 2000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not add to collection',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function save(): Promise<void> {
  if (!name.value.trim()) return
  busy.value = true
  try {
    if (editingId.value) {
      await api.updateCollection(
        editingId.value,
        name.value.trim(),
        description.value.trim(),
        icon.value,
      )
      notifyLibraryChanged()
      toast.add({ severity: 'success', summary: 'Collection updated', life: 2500 })
    } else {
      const libraryId = library.currentId
      if (!libraryId) return
      const collection = await api.createCollection(
        libraryId,
        name.value.trim(),
        description.value.trim(),
        icon.value,
      )
      notifyLibraryChanged()
      void router.push({
        name: 'all',
        query: { ...route.query, collection: collection.id },
      })
      toast.add({ severity: 'success', summary: 'Collection created', life: 2500 })
    }
    dialogVisible.value = false
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Save failed',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    busy.value = false
  }
}

async function duplicate(collection: Collection): Promise<void> {
  try {
    const clone = await api.duplicateCollection(collection.id)
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary: 'Collection duplicated',
      detail: clone.name,
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Duplicate failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function remove(id: string, collectionName: string): void {
  confirm.require({
    header: 'Delete collection',
    message: `Delete '${collectionName}'? Assets are kept in the library.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: async () => {
      try {
        await api.deleteCollection(id)
        notifyLibraryChanged()
        if (activeCollection.value === id) {
          const query = { ...route.query }
          delete query.collection
          void router.push({ name: 'all', query })
        }
        toast.add({ severity: 'success', summary: 'Collection deleted', life: 2500 })
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

watch(
  () => library.version,
  () => {
    void refreshLibrary()
  },
)

onMounted(() => {
  void initLibrary()
})
</script>

<template>
  <aside class="flex w-64 shrink-0 flex-col border-r border-slate-200 bg-white">
    <div class="space-y-2 p-3 pb-0">
      <LibrarySwitcher />
      <button
        class="flex w-full items-center gap-2 rounded border border-slate-200 px-3 py-2 text-sm text-slate-500 hover:border-slate-300 hover:text-slate-700"
        @click="openCommandMenu"
      >
        <Search :size="14" />
        <span class="flex-1 text-left">Search</span>
        <Kbd>⌘K</Kbd>
      </button>
    </div>

    <nav class="space-y-1 p-3">
      <RouterLink
        :to="{ name: 'all' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          allActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="16" />
        <span class="flex-1">All</span>
        <span
          v-if="library.stats?.assets"
          class="shrink-0 rounded bg-slate-100 px-1.5 text-xs text-slate-500"
        >
          {{ library.stats.assets }}
        </span>
      </RouterLink>
      <RouterLink
        :to="{ name: 'all', query: { collection: 'uncategorized' } }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          uncategorizedActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Folder :size="16" />
        <span class="flex-1">Uncategorized</span>
        <span
          v-if="library.stats?.uncategorized"
          class="shrink-0 rounded bg-slate-100 px-1.5 text-xs text-slate-500"
        >
          {{ library.stats.uncategorized }}
        </span>
      </RouterLink>
      <RouterLink
        :to="{ name: 'all', query: { tag: 'untagged' } }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          untaggedActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Tag :size="16" />
        <span class="flex-1">Untagged</span>
        <span
          v-if="library.stats?.untagged"
          class="shrink-0 rounded bg-slate-100 px-1.5 text-xs text-slate-500"
        >
          {{ library.stats.untagged }}
        </span>
      </RouterLink>
      <button
        class="flex w-full items-center gap-2 rounded px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900"
        @click="openTagsDialog"
      >
        <Tags :size="16" />
        <span class="flex-1 text-left">Tags</span>
      </button>
      <RouterLink
        :to="{ name: 'trash' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          route.name === 'trash'
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Trash :size="16" />
        <span class="flex-1">Trash</span>
      </RouterLink>
    </nav>

    <div class="min-h-0 flex-1 overflow-y-auto pb-2">
      <div class="flex items-center justify-between px-4 pt-2 pb-1">
        <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
          Collections
        </span>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600"
          title="New collection"
          @click="openCreate"
        >
          <Plus :size="14" />
        </button>
      </div>

      <nav class="space-y-0.5 px-2">
        <p
          v-if="!library.collections.length"
          class="px-2 py-1 text-xs text-slate-400"
        >
          No collections
        </p>

        <RouterLink
          v-for="collection in library.collections"
          :key="collection.id"
          :to="{ name: 'all', query: { ...route.query, collection: collection.id } }"
          class="flex items-center gap-2 rounded px-2 py-1.5 text-sm font-medium"
          :class="[
            activeCollection === collection.id
              ? 'bg-violet-50 text-violet-700'
              : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900',
            dragOverId === collection.id
              ? 'ring-2 ring-violet-400 ring-inset'
              : '',
          ]"
          @contextmenu.prevent="onContextMenu($event, collection)"
          @dragover.prevent="onDragOver(collection.id)"
          @dragleave="onDragLeave(collection.id)"
          @drop.prevent="onDrop($event, collection)"
        >
          <component :is="iconFor(collection)" :size="14" class="shrink-0" />
          <span class="truncate">{{ collection.name }}</span>
          <span
            class="ml-auto shrink-0 rounded bg-slate-100 px-1.5 text-xs text-slate-500"
          >
            {{ collection.asset_count }}
          </span>
        </RouterLink>
      </nav>
    </div>

    <UserMenu />

    <ContextMenu ref="menu" :model="menuItems" />

    <Dialog
      v-model:visible="dialogVisible"
      modal
      :header="editingId ? 'Edit collection' : 'New collection'"
      :style="{ width: '26rem' }"
    >
      <div class="space-y-4">
        <InputText
          v-model="name"
          placeholder="Collection name"
          class="w-full"
          @keyup.enter="save"
        />
        <InputText
          v-model="description"
          placeholder="Description (optional)"
          class="w-full"
        />
        <div>
          <label class="mb-2 block text-sm font-medium">Icon</label>
          <div class="grid grid-cols-8 gap-1">
            <button
              v-for="option in COLLECTION_ICONS"
              :key="option.name"
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded"
              :class="
                icon === option.name
                  ? 'bg-violet-100 text-violet-700 ring-1 ring-violet-400'
                  : 'text-slate-500 hover:bg-slate-100'
              "
              :title="option.label"
              @click="icon = option.name"
            >
              <component :is="option.component" :size="16" />
            </button>
            <button
              type="button"
              class="flex h-8 w-8 items-center justify-center rounded text-slate-400 hover:bg-slate-100"
              title="No icon"
              @click="icon = ''"
            >
              <Times :size="14" />
            </button>
          </div>
        </div>
      </div>
      <template #footer>
        <Button
          label="Cancel"
          text
          :disabled="busy"
          @click="dialogVisible = false"
        />
        <Button
          :label="editingId ? 'Save' : 'Create'"
          :loading="busy"
          :disabled="!name.trim()"
          @click="save"
        />
      </template>
    </Dialog>
  </aside>
</template>
