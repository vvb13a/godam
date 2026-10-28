<script setup lang="ts">
import Message from 'primevue/message'
import MultiSelect from 'primevue/multiselect'
import Popover from 'primevue/popover'
import Select from 'primevue/select'
import { Bolt, ListCheck, Refresh, Times } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { auth } from '../lib/auth'
import { catalogFacet, loadAttributeCatalog } from '../lib/attributeCatalog'
import { notifyAttributesChanged } from '../lib/attributesDialog'
import { effectiveAttributeSort, sortAttributeValues } from '../lib/attributeSort'
import { hasCapability } from '../lib/capabilities'
import { conversionCacheVersion } from '../lib/conversionCache'
import { formatDateTime, formatDuration, formatRelative, formatSize } from '../lib/format'
import { hasMediaPreview } from '../lib/mimeTypes'
import { libraryColorHex } from '../lib/libraryColors'
import { openPresetsDialog } from '../lib/presetsDialog'
import { warmPresets, warmProgress } from '../lib/warmPresets'
import { ATTRIBUTE_UNASSIGNED } from '../types'
import type { Asset, AttributeFacetValue, Conversion } from '../types'
import FileTypePreview from './FileTypePreview.vue'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ close: []; changed: [asset: Asset] }>()

const canUpdate = computed(() => hasCapability('assets:update'))
const canWarm = computed(() => hasCapability('conversions:warm'))
const canManagePresets = computed(() => hasCapability('settings:presets'))

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

const visibleAttributes = computed(() => {
  const grants = new Map(
    (auth.user?.scope?.attributes ?? []).map((grant) => [grant.key, grant]),
  )
  return (props.asset.attributes ?? []).filter(
    (attribute) => grants.get(attribute.key)?.mode !== 'hidden',
  )
})

const canSelectAttributes = computed(() =>
  hasCapability('attributes:select'),
)

const attributePopover = ref<InstanceType<typeof Popover> | null>(null)
const editingAttributeKey = ref<string | null>(null)
const editingAttributeValues = ref<string[]>([])
const attributeSaving = ref(false)

interface AttributeGroup {
  key: string
  label: string
  description?: string
  color?: string
  position: number
  values: string[]
}

function attributeKeyLabel(key: string): string {
  return catalogFacet(key)?.label || key
}

function attributeValueLabel(key: string, value: string): string {
  const facet = catalogFacet(key)
  return facet?.values.find((item) => item.value === value)?.label || value
}

function attributeValueDescription(key: string, value: string): string {
  const facet = catalogFacet(key)
  return facet?.values.find((item) => item.value === value)?.description ?? ''
}

function attributeValueColor(key: string, value: string): string {
  const facet = catalogFacet(key)
  const color =
    facet?.values.find((item) => item.value === value)?.color || facet?.color
  return libraryColorHex(color ?? '')
}

const groupedAttributes = computed<AttributeGroup[]>(() => {
  const groups = new Map<string, AttributeGroup>()
  for (const attribute of visibleAttributes.value) {
    const facet = catalogFacet(attribute.key)
    const group = groups.get(attribute.key) ?? {
      key: attribute.key,
      label: attributeKeyLabel(attribute.key),
      description: facet?.description,
      color: facet?.color,
      position: facet?.position ?? 0,
      values: [],
    }
    if (!group.values.includes(attribute.value)) {
      group.values.push(attribute.value)
    }
    groups.set(attribute.key, group)
  }
  return [...groups.values()].sort(
    (a, b) =>
      a.position - b.position || a.label.localeCompare(b.label),
  )
})

function canEditAttribute(key: string): boolean {
  if (!canSelectAttributes.value) return false
  const scope = auth.user?.scope
  if (!scope) return true
  const grant = scope.attributes?.find((item) => item.key === key)
  if (grant) return grant.mode === 'active'
  return Boolean(scope.all_attributes)
}

function attributeMulti(key: string): boolean {
  return catalogFacet(key)?.multi ?? true
}

function attributeOptions(key: string): AttributeFacetValue[] {
  const facet = catalogFacet(key)
  const options = facet
    ? facet.values.filter((item) => item.value !== ATTRIBUTE_UNASSIGNED)
    : []
  const known = new Set(options.map((item) => item.value))
  const group = groupedAttributes.value.find((item) => item.key === key)
  for (const value of group?.values ?? []) {
    if (!known.has(value)) {
      options.push({ value, label: value, position: 0, count: 0 })
      known.add(value)
    }
  }
  return sortAttributeValues(
    options,
    effectiveAttributeSort(key, facet?.sort),
  )
}

const editingAttributeLabel = computed(() =>
  editingAttributeKey.value
    ? attributeKeyLabel(editingAttributeKey.value)
    : '',
)

function openAttributeEditor(event: Event, key: string): void {
  editingAttributeKey.value = key
  const group = groupedAttributes.value.find((item) => item.key === key)
  editingAttributeValues.value = [...(group?.values ?? [])]
  attributePopover.value?.toggle(event)
}

async function saveAttributeValues(key: string, values: string[]): Promise<void> {
  if (!key) return
  attributeSaving.value = true
  error.value = null
  try {
    const updated = await api.setAssetAttributeValues(
      props.asset.id,
      key,
      values,
    )
    editingAttributeValues.value = [...values]
    emit('changed', updated)
    notifyAttributesChanged()
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    attributeSaving.value = false
  }
}

void loadAttributeCatalog()

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

async function save(): Promise<void> {
  if (!canUpdate.value) return
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
            :disabled="!canUpdate"
            class="w-full rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm disabled:opacity-60"
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
            :disabled="!canUpdate"
            class="w-full resize-y rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm disabled:opacity-60"
            @blur="save"
          />
        </div>
        <p v-if="saving" class="text-xs text-slate-400">Saving…</p>
      </section>

      <section v-if="groupedAttributes.length">
        <h3 class="mb-2 text-sm font-semibold">Attributes</h3>
        <dl class="space-y-1 text-xs">
          <div
            v-for="group in groupedAttributes"
            :key="group.key"
            class="flex items-start gap-2"
          >
            <dt
              class="flex w-24 shrink-0 items-center gap-1.5 text-slate-400"
              v-tooltip="{ value: group.description || '' }"
            >
              <span
                class="h-2 w-2 shrink-0 rounded-full"
                :style="{ backgroundColor: libraryColorHex(group.color ?? '') }"
              />
              <span class="truncate">{{ group.label }}</span>
            </dt>
            <dd class="min-w-0 flex-1 break-words">
              <template v-for="(value, index) in group.values" :key="value">
                <span class="inline-flex items-center gap-1">
                  <span
                    class="h-1.5 w-1.5 shrink-0 rounded-full"
                    :style="{
                      backgroundColor: attributeValueColor(group.key, value),
                    }"
                  />
                  <span
                    v-tooltip="{
                      value: attributeValueDescription(group.key, value),
                    }"
                  >
                    {{ attributeValueLabel(group.key, value) }}
                  </span>
                </span>
                <span v-if="index < group.values.length - 1">, </span>
              </template>
            </dd>
            <button
              v-if="canEditAttribute(group.key)"
              type="button"
              class="inline-flex shrink-0 text-slate-400 transition hover:text-brand-700 dark:hover:text-brand-400"
              title="Select attribute values"
              @click="openAttributeEditor($event, group.key)"
            >
              <ListCheck :size="14" />
            </button>
          </div>
        </dl>

        <Popover ref="attributePopover" :pt="{ content: { class: '!p-3' } }">
          <div class="w-64 space-y-2">
            <p
              class="text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
            >
              {{ editingAttributeLabel }}
            </p>
            <Select
              v-if="!attributeMulti(editingAttributeKey ?? '')"
              :model-value="editingAttributeValues[0] ?? null"
              :options="attributeOptions(editingAttributeKey ?? '')"
              option-label="label"
              option-value="value"
              showClear
              size="small"
              class="w-full"
              :disabled="attributeSaving"
              @update:model-value="
                saveAttributeValues(
                  editingAttributeKey ?? '',
                  $event ? [$event] : [],
                )
              "
            />
            <MultiSelect
              v-else
              :model-value="editingAttributeValues"
              :options="attributeOptions(editingAttributeKey ?? '')"
              option-label="label"
              option-value="value"
              display="chip"
              :max-selected-labels="2"
              showClear
              size="small"
              class="w-full"
              :disabled="attributeSaving"
              @update:model-value="
                saveAttributeValues(editingAttributeKey ?? '', $event)
              "
            />
            <p class="text-[11px] text-slate-400">
              Changes apply to this asset only.
            </p>
          </div>
        </Popover>
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
              v-if="canManagePresets"
              class="text-xs text-brand-700 hover:underline dark:text-brand-400"
              title="Manage presets"
              @click="openPresetsDialog(null, asset.id)"
            >
              Manage
            </button>
            <button
              v-if="canWarm"
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
