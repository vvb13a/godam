<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import { Copy, Folder, Images, Pencil, Plus, Times, Trash } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { assetDrag } from '../lib/assetDrag'
import { COLLECTION_ICONS, collectionIcon } from '../lib/collectionIcons'
import { library, loadCollections, notifyLibraryChanged } from '../lib/library'
import type { Collection } from '../types'

withDefaults(defineProps<{ rail?: boolean }>(), { rail: false })

const route = useRoute()
const router = useRouter()
const confirm = useConfirm()
const toast = useToast()

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

const selectedCollectionId = computed(() =>
  route.name === 'collection' ? String(route.params.collectionId) : null,
)
const allSelected = computed(() => route.name === 'library')

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
    await loadCollections()
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
      await loadCollections()
      toast.add({ severity: 'success', summary: 'Collection updated', life: 2500 })
    } else {
      const collection = await api.createCollection(
        name.value.trim(),
        description.value.trim(),
        icon.value,
      )
      await loadCollections()
      void router.push({
        name: 'collection',
        params: { collectionId: collection.id },
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
    await loadCollections()
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
        await loadCollections()
        if (selectedCollectionId.value === id) {
          void router.push({ name: 'library' })
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
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Full panel -->
    <div
      v-if="!rail"
      class="flex items-center justify-between border-b border-slate-200 px-3 py-2"
    >
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

    <nav v-if="!rail" class="flex-1 space-y-0.5 overflow-y-auto p-2">
      <RouterLink
        :to="{ name: 'library' }"
        class="flex items-center gap-2 rounded px-2 py-1.5 text-sm font-medium"
        :class="
          allSelected
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="14" />
        All Assets
      </RouterLink>

      <p
        v-if="!library.collections.length"
        class="px-2 py-1 text-xs text-slate-400"
      >
        No collections
      </p>

      <RouterLink
        v-for="collection in library.collections"
        :key="collection.id"
        :to="{ name: 'collection', params: { collectionId: collection.id } }"
        class="flex items-center gap-2 rounded px-2 py-1.5 text-sm font-medium"
        :class="[
          selectedCollectionId === collection.id
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

    <!-- Fixed-width icon rail -->
    <nav
      v-else
      class="flex flex-1 flex-col items-center gap-1 overflow-y-auto py-2"
    >
      <button
        class="flex h-10 w-10 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-violet-600"
        title="New collection"
        @click="openCreate"
      >
        <Plus :size="16" />
      </button>
      <RouterLink
        :to="{ name: 'library' }"
        title="All Assets"
        class="flex h-10 w-10 items-center justify-center rounded"
        :class="
          allSelected
            ? 'bg-violet-100 text-violet-700'
            : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="18" />
      </RouterLink>
      <RouterLink
        v-for="collection in library.collections"
        :key="collection.id"
        :to="{ name: 'collection', params: { collectionId: collection.id } }"
        :title="`${collection.name} (${collection.asset_count})`"
        class="flex h-10 w-10 items-center justify-center rounded"
        :class="[
          selectedCollectionId === collection.id
            ? 'bg-violet-100 text-violet-700'
            : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900',
          dragOverId === collection.id
            ? 'ring-2 ring-violet-400 ring-inset'
            : '',
        ]"
        @contextmenu.prevent="onContextMenu($event, collection)"
        @dragover.prevent="onDragOver(collection.id)"
        @dragleave="onDragLeave(collection.id)"
        @drop.prevent="onDrop($event, collection)"
      >
        <component :is="iconFor(collection)" :size="18" />
      </RouterLink>
    </nav>

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
  </div>
</template>
