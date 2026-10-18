<script setup lang="ts">
import Message from 'primevue/message'
import { Plus, Times } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { formatDateTime, formatDuration, formatRelative, formatSize } from '../lib/format'
import { library } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import { openAssetTags } from '../lib/tagsDialog'
import type { Asset, Tag } from '../types'
import TagBadge from './TagBadge.vue'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ close: []; changed: [asset: Asset] }>()

const busy = ref(false)
const error = ref<string | null>(null)
const previewFailed = ref(false)

const title = ref('')
const note = ref('')
const saving = ref(false)

const displayTitle = computed(() => props.asset.title || props.asset.filename)

interface PropertyRow {
  label: string
  value: string
  title?: string
}

const properties = computed<PropertyRow[]>(() => {
  const asset = props.asset
  const meta = asset.metadata ?? {}
  const rows: PropertyRow[] = [
    { label: 'Type', value: asset.mime_type },
    { label: 'Size', value: formatSize(asset.byte_size) },
  ]
  if (meta.width && meta.height) {
    rows.push({ label: 'Dimensions', value: `${meta.width} × ${meta.height} px` })
  }
  if (meta.page_count) {
    rows.push({ label: 'Pages', value: String(meta.page_count) })
  }
  if (meta.duration_sec) {
    rows.push({ label: 'Duration', value: formatDuration(meta.duration_sec) })
  }
  rows.push({ label: 'Driver', value: asset.storage_driver })
  rows.push({
    label: 'Added',
    value: formatRelative(asset.created_at),
    title: formatDateTime(asset.created_at),
  })
  if (asset.updated_at) {
    rows.push({
      label: 'Modified',
      value: formatRelative(asset.updated_at),
      title: formatDateTime(asset.updated_at),
    })
  }
  if (asset.uploaded_by_name) {
    rows.push({ label: 'Uploaded by', value: asset.uploaded_by_name })
  }
  return rows
})

watch(
  () => props.asset,
  (asset) => {
    title.value = asset.title ?? ''
    note.value = asset.note ?? ''
    previewFailed.value = false
    error.value = null
  },
  { immediate: true },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function tagColor(tag: Tag): string | undefined {
  if (!tag.group_id) return undefined
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : undefined
}

function editTags(): void {
  openAssetTags(props.asset.id)
}

async function save(): Promise<void> {
  const nextTitle = title.value.trim()
  const nextNote = note.value.trim()
  if (
    nextTitle === (props.asset.title ?? '') &&
    nextNote === (props.asset.note ?? '')
  ) {
    return
  }
  saving.value = true
  error.value = null
  try {
    const updated = await api.updateAsset(props.asset.id, {
      title: nextTitle,
      note: nextNote,
    })
    emit('changed', updated)
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    saving.value = false
  }
}

async function run(action: () => Promise<Asset>): Promise<void> {
  busy.value = true
  error.value = null
  try {
    const asset = await action()
    emit('changed', asset)
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}

function removeTag(tagId: string): void {
  void run(() => api.removeTag(props.asset.id, tagId))
}

function removeCollection(collectionId: string): void {
  void run(() => api.removeFromCollection(props.asset.id, collectionId))
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-center justify-end px-3 py-2">
      <button
        class="inline-flex shrink-0 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
        title="Close"
        @click="emit('close')"
      >
        <Times :size="16" />
      </button>
    </div>

    <div class="min-h-0 flex-1 space-y-6 overflow-y-auto px-3 pb-4">
      <img
        v-if="!previewFailed"
        :src="api.assetPreviewUrl(asset.id, asset.updated_at)"
        :alt="displayTitle"
        class="max-h-72 w-full object-contain"
        @error="previewFailed = true"
      />
      <div
        v-else
        class="flex h-40 items-center justify-center text-sm text-slate-400"
      >
        No preview available
      </div>

      <section class="space-y-3">
        <div>
          <label
            class="mb-1 block text-xs font-medium text-slate-400"
            for="inspector-title"
          >
            Title
          </label>
          <input
            id="inspector-title"
            v-model="title"
            :placeholder="asset.filename"
            class="w-full rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm"
            @blur="save"
            @keydown.enter.prevent="save"
          />
        </div>
        <div>
          <label
            class="mb-1 block text-xs font-medium text-slate-400"
            for="inspector-note"
          >
            Note
          </label>
          <textarea
            id="inspector-note"
            v-model="note"
            rows="3"
            placeholder="Add a note…"
            class="w-full resize-y rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm"
            @blur="save"
          />
        </div>
        <p v-if="saving" class="text-xs text-slate-400">Saving…</p>
      </section>

      <section>
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-sm font-semibold">Tags</h3>
          <button
            class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
            title="Edit tags"
            @click="editTags"
          >
            <Plus :size="14" />
          </button>
        </div>
        <div class="flex flex-wrap gap-1">
          <TagBadge
            v-for="tag in asset.tags"
            :key="tag.id"
            :name="tag.name"
            :color="tagColor(tag)"
          >
            <button
              class="inline-flex text-slate-400 hover:text-red-500"
              title="Remove tag"
              :disabled="busy"
              @click="removeTag(tag.id)"
            >
              <Times :size="12" />
            </button>
          </TagBadge>
          <span v-if="!asset.tags?.length" class="text-xs text-slate-400">
            No tags
          </span>
        </div>
      </section>

      <section>
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-sm font-semibold">Collections</h3>
          <button
            class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
            title="Add to collection"
          >
            <Plus :size="14" />
          </button>
        </div>
        <div class="flex flex-wrap gap-1">
          <TagBadge
            v-for="collection in asset.collections"
            :key="collection.id"
            :name="collection.name"
            :color="libraryColorHex(collection.color)"
          >
            <button
              class="inline-flex text-slate-400 hover:text-red-500"
              title="Remove from collection"
              :disabled="busy"
              @click="removeCollection(collection.id)"
            >
              <Times :size="12" />
            </button>
          </TagBadge>
          <span v-if="!asset.collections?.length" class="text-xs text-slate-400">
            No collections
          </span>
        </div>
      </section>

      <section>
        <h3 class="mb-2 text-sm font-semibold">Properties</h3>
        <dl class="space-y-1.5 text-sm">
          <div
            v-for="row in properties"
            :key="row.label"
            class="flex justify-between gap-4"
          >
            <dt class="text-slate-400">{{ row.label }}</dt>
            <dd class="break-all text-right" v-tooltip="row.title">
              {{ row.value }}
            </dd>
          </div>
        </dl>
      </section>

      <Message v-if="error" severity="error">{{ error }}</Message>
    </div>
  </div>
</template>
