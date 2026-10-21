<script setup lang="ts">
import Message from 'primevue/message'
import { Bolt, Plus, Refresh, Times } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { conversionCacheVersion } from '../lib/conversionCache'
import { formatDateTime, formatDuration, formatRelative, formatSize } from '../lib/format'
import { library } from '../lib/library'
import { hasMediaPreview } from '../lib/mimeTypes'
import { libraryColorHex } from '../lib/libraryColors'
import { openAssetCollections } from '../lib/collectionsDialog'
import { openPresetsDialog } from '../lib/presetsDialog'
import { openAssetTags } from '../lib/tagsDialog'
import { warmPresets, warmProgress } from '../lib/warmPresets'
import type { Asset, Conversion, Tag } from '../types'
import FileTypePreview from './FileTypePreview.vue'
import TagBadge from './TagBadge.vue'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ close: []; changed: [asset: Asset] }>()

const busy = ref(false)
const error = ref<string | null>(null)
const previewFailed = ref(false)

const conversions = ref<Conversion[]>([])
const conversionsLoading = ref(false)
const conversionsError = ref<string | null>(null)
let conversionsRequest = 0
let conversionsAssetID = ''

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

// Conversions are derived files, so refresh the list when the selected asset
// changes or its content is replaced (updated_at moves).
watch(
  () => [props.asset.id, props.asset.updated_at] as const,
  ([id]) => {
    if (id !== conversionsAssetID) {
      conversionsAssetID = id
      conversions.value = []
    }
    conversionsError.value = null
    if (props.asset.mime_type.startsWith('image/')) {
      void loadConversions(id)
    } else {
      conversionsRequest++
      conversionsLoading.value = false
      conversions.value = []
    }
  },
  { immediate: true },
)

watch(conversionCacheVersion, () => {
  if (props.asset.mime_type.startsWith('image/')) {
    void loadConversions(props.asset.id)
  }
})

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function loadConversions(id: string): Promise<void> {
  const request = ++conversionsRequest
  conversionsLoading.value = true
  conversionsError.value = null
  try {
    const list = await api.listAssetConversions(id)
    if (request === conversionsRequest) {
      conversions.value = list
    }
  } catch (e) {
    if (request === conversionsRequest) {
      conversions.value = []
      conversionsError.value = toMessage(e)
    }
  } finally {
    if (request === conversionsRequest) {
      conversionsLoading.value = false
    }
  }
}

async function warmAsset(): Promise<void> {
  await warmPresets([props.asset.id])
  await loadConversions(props.asset.id)
}

function conversionLabel(conversion: Conversion): string {
  if (conversion.kind === 'preset') {
    return `${conversion.label || conversion.preset || 'Preset'} · ${conversion.format.toUpperCase()}`
  }
  if (conversion.kind === 'signed') {
    return `Signed · ${conversion.format.toUpperCase()}`
  }
  const width = conversion.width || 'auto'
  const height = conversion.height || 'auto'
  return `${width} × ${height} · ${conversion.format.toUpperCase()}`
}

function tagColor(tag: Tag): string | undefined {
  if (!tag.group_id) return undefined
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : undefined
}

function editTags(): void {
  openAssetTags(props.asset.id)
}

function editCollections(): void {
  openAssetCollections(props.asset.id)
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
      <div
        v-if="!hasMediaPreview(asset.mime_type)"
        class="aspect-video overflow-hidden rounded"
      >
        <FileTypePreview
          :mime-type="asset.mime_type"
          :filename="asset.filename"
        />
      </div>
      <img
        v-else-if="!previewFailed"
        :src="api.assetPreviewUrl(asset.id, asset.updated_at)"
        :alt="displayTitle"
        class="max-h-72 w-full object-contain"
        @error="previewFailed = true"
      />
      <div v-else class="aspect-video overflow-hidden rounded">
        <FileTypePreview
          :mime-type="asset.mime_type"
          :filename="asset.filename"
        />
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

      <section
        v-if="asset.mime_type.startsWith('image/') && asset.colors?.length"
      >
        <h3 class="mb-2 text-sm font-semibold">Colors</h3>
        <div
          class="flex h-3 overflow-hidden rounded-full ring-1 ring-slate-200 dark:ring-slate-800"
        >
          <span
            v-for="(color, index) in asset.colors"
            :key="`${color.hex}-${index}`"
            :style="{
              width: `${color.percentage}%`,
              backgroundColor: color.hex,
            }"
            :title="`${color.bucket} · ${color.percentage.toFixed(1)}%`"
          />
        </div>
        <ul class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
          <li
            v-for="(color, index) in asset.colors"
            :key="`${color.hex}-row-${index}`"
            class="flex min-w-0 items-center gap-1.5"
          >
            <span
              class="h-2.5 w-2.5 shrink-0 rounded-sm border border-slate-200 dark:border-slate-700"
              :style="{ backgroundColor: color.hex }"
            />
            <span class="font-mono uppercase text-slate-500">
              {{ color.hex }}
            </span>
            <span class="ml-auto shrink-0 text-slate-400">
              {{ color.percentage.toFixed(1) }}%
            </span>
          </li>
        </ul>
      </section>

      <section v-if="asset.mime_type.startsWith('image/')">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h3 class="text-sm font-semibold">Conversions</h3>
          <div class="flex items-center gap-2">
            <span v-if="conversionsLoading" class="text-xs text-slate-400">
              Loading…
            </span>
            <span
              v-else-if="conversions.length"
              class="text-xs text-slate-400"
            >
              {{ conversions.length }}
            </span>
            <button
              class="text-xs text-brand-700 hover:underline dark:text-brand-400"
              title="Manage presets"
              @click="openPresetsDialog(null, asset.id)"
            >
              Manage
            </button>
            <button
              class="inline-flex text-slate-400 hover:text-brand-700 dark:hover:text-brand-400 disabled:opacity-40"
              title="Warm all presets for this asset"
              :disabled="warmProgress.active"
              @click="warmAsset"
            >
              <Bolt :size="14" />
            </button>
            <button
              class="inline-flex text-slate-400 hover:text-brand-700 dark:hover:text-brand-400"
              title="Refresh conversions"
              :disabled="conversionsLoading"
              @click="loadConversions(asset.id)"
            >
              <Refresh :size="14" />
            </button>
          </div>
        </div>
        <p
          v-if="conversionsError"
          class="text-xs text-red-600 dark:text-red-400"
        >
          {{ conversionsError }}
        </p>
        <p
          v-else-if="!conversionsLoading && !conversions.length"
          class="text-xs text-slate-400"
        >
          No conversions yet
        </p>
        <ul v-if="conversions.length" class="space-y-1.5 text-sm">
          <li
            v-for="conversion in conversions"
            :key="conversion.key"
            class="flex items-center justify-between gap-3"
          >
            <a
              v-if="conversion.url"
              :href="conversion.url"
              target="_blank"
              rel="noopener"
              :title="conversion.url"
              class="min-w-0 truncate text-brand-700 hover:underline dark:text-brand-400"
            >
              {{ conversionLabel(conversion) }}
            </a>
            <span v-else class="min-w-0 truncate">
              {{ conversionLabel(conversion) }}
            </span>
            <span class="shrink-0 text-xs text-slate-400">
              {{ formatSize(conversion.byte_size) }}
            </span>
          </li>
        </ul>
      </section>

      <section>
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-sm font-semibold">Tags</h3>
          <button
            class="inline-flex text-slate-400 hover:text-brand-700 dark:hover:text-brand-400"
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
            class="inline-flex text-slate-400 hover:text-brand-700 dark:hover:text-brand-400"
            title="Edit collections"
            @click="editCollections"
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
