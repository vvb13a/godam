<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { ExternalLink, Plus, Times, Trash } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { formatDate, formatSize } from '../lib/format'
import { library } from '../lib/library'
import type { Asset } from '../types'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ close: []; changed: [] }>()

const confirm = useConfirm()
const toast = useToast()

const tagName = ref('')
const busy = ref(false)
const error = ref<string | null>(null)
const previewFailed = ref(false)

const dimensions = computed(() => {
  const meta = props.asset.metadata
  if (meta?.width && meta?.height) return `${meta.width} × ${meta.height} px`
  return null
})

watch(
  () => props.asset.id,
  () => {
    previewFailed.value = false
    tagName.value = ''
    error.value = null
  },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function memberOf(collectionId: string): boolean {
  return !!props.asset.collections?.some((c) => c.id === collectionId)
}

async function run(action: () => Promise<unknown>): Promise<void> {
  busy.value = true
  error.value = null
  try {
    await action()
    emit('changed')
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}

function addTag(): void {
  const name = tagName.value.trim()
  if (!name) return
  void run(async () => {
    await api.addTag(props.asset.id, name)
    tagName.value = ''
  })
}

function removeTag(tagId: string): void {
  void run(() => api.removeTag(props.asset.id, tagId))
}

function addCollection(collectionId: string): void {
  void run(() => api.addToCollection(props.asset.id, collectionId))
}

function removeCollection(collectionId: string): void {
  void run(() => api.removeFromCollection(props.asset.id, collectionId))
}

function confirmDelete(): void {
  const asset = props.asset
  confirm.require({
    header: 'Delete asset',
    message: `Delete '${asset.filename}'? This cannot be undone.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: async () => {
      try {
        await api.deleteAsset(asset.id)
        toast.add({ severity: 'success', summary: 'Asset deleted', life: 2500 })
        emit('close')
        emit('changed')
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
    <div
      class="flex items-center justify-between gap-2 border-b border-slate-200 px-3 py-2"
    >
      <span class="truncate text-sm font-semibold" :title="asset.filename">
        {{ asset.filename }}
      </span>
      <button
        class="inline-flex shrink-0 text-slate-400 hover:text-slate-700"
        title="Close"
        @click="emit('close')"
      >
        <Times :size="16" />
      </button>
    </div>

    <div class="min-h-0 flex-1 space-y-6 overflow-y-auto p-3">
      <div>
        <img
          v-if="!previewFailed"
          :src="api.assetPreviewUrl(asset.id)"
          :alt="asset.filename"
          class="max-h-80 w-full rounded border border-slate-200 bg-slate-50 object-contain"
          @error="previewFailed = true"
        />
        <div
          v-else
          class="flex h-48 items-center justify-center rounded border border-slate-200 bg-slate-50 text-sm text-slate-400"
        >
          No preview available
        </div>
        <div class="mt-3 flex items-center gap-1">
          <a
            :href="api.assetContentUrl(asset.id)"
            target="_blank"
            rel="noreferrer"
          >
            <Button label="Open original" text size="small">
              <template #icon><ExternalLink :size="16" /></template>
            </Button>
          </a>
          <Button text size="small" severity="danger" @click="confirmDelete">
            <template #icon><Trash :size="16" /></template>
            Delete
          </Button>
        </div>
      </div>

      <dl class="space-y-1.5 text-sm">
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Type</dt>
          <dd class="text-right">{{ asset.mime_type }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Size</dt>
          <dd>{{ formatSize(asset.byte_size) }}</dd>
        </div>
        <div v-if="dimensions" class="flex justify-between gap-4">
          <dt class="text-slate-400">Dimensions</dt>
          <dd>{{ dimensions }}</dd>
        </div>
        <div v-if="asset.metadata.page_count" class="flex justify-between gap-4">
          <dt class="text-slate-400">Pages</dt>
          <dd>{{ asset.metadata.page_count }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Driver</dt>
          <dd>{{ asset.storage_driver }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Added</dt>
          <dd>{{ formatDate(asset.created_at) }}</dd>
        </div>
      </dl>

      <section>
        <h3 class="mb-2 text-sm font-semibold">Tags</h3>
        <div class="mb-2 flex flex-wrap gap-1">
          <span
            v-for="tag in asset.tags"
            :key="tag.id"
            class="flex items-center gap-1 rounded bg-slate-100 px-2 py-0.5 text-xs text-slate-600"
          >
            #{{ tag.name }}
            <button
              class="inline-flex text-slate-400 hover:text-red-500"
              title="Remove tag"
              @click="removeTag(tag.id)"
            >
              <Times :size="14" />
            </button>
          </span>
          <span v-if="!asset.tags?.length" class="text-xs text-slate-400">
            No tags
          </span>
        </div>
        <div class="flex gap-2">
          <InputText
            v-model="tagName"
            placeholder="Add a tag"
            size="small"
            class="flex-1"
            @keyup.enter="addTag"
          />
          <Button
            label="Add"
            size="small"
            :disabled="busy || !tagName.trim()"
            @click="addTag"
          >
            <template #icon><Plus :size="16" /></template>
          </Button>
        </div>
      </section>

      <section>
        <h3 class="mb-2 text-sm font-semibold">Collections</h3>
        <div v-if="library.collections.length" class="space-y-1">
          <div
            v-for="collection in library.collections"
            :key="collection.id"
            class="flex items-center justify-between rounded border border-slate-200 px-2 py-1 text-sm"
          >
            <span class="truncate">{{ collection.name }}</span>
            <Button
              v-if="!memberOf(collection.id)"
              label="Add"
              text
              size="small"
              :disabled="busy"
              @click="addCollection(collection.id)"
            >
              <template #icon><Plus :size="16" /></template>
            </Button>
            <Button
              v-else
              label="Remove"
              text
              size="small"
              severity="danger"
              :disabled="busy"
              @click="removeCollection(collection.id)"
            >
              <template #icon><Times :size="16" /></template>
            </Button>
          </div>
        </div>
        <p v-else class="text-xs text-slate-400">
          No collections yet. Create one from the collections panel.
        </p>
      </section>

      <Message v-if="error" severity="error">{{ error }}</Message>
    </div>
  </div>
</template>
