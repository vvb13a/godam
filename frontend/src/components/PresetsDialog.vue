<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Dialog from 'primevue/dialog'
import MultiSelect from 'primevue/multiselect'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { Plus, Search, Trash } from '@primeicons/vue'
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import * as api from '../api/client'
import { formatDateTime, formatRelative } from '../lib/format'
import { library, notifyLibraryChanged } from '../lib/library'
import { presetsDialog } from '../lib/presetsDialog'
import type {
  Asset,
  CatalogOption,
  ImgProxyCatalog,
  Preset,
  PresetOption,
} from '../types'

const toast = useToast()
const confirm = useConfirm()

const loading = ref(false)
const saving = ref(false)
const error = ref<string | null>(null)
const presets = ref<Preset[]>([])
const catalog = ref<ImgProxyCatalog>({ formats: ['webp'], options: [] })
const search = ref('')
const category = ref('All')
const nameTouched = ref(false)

const form = reactive({
  id: null as string | null,
  label: '',
  name: '',
  format: 'webp',
  libraryWide: true,
  collectionIds: [] as string[],
  tagIds: [] as string[],
  options: {} as Record<string, string[]>,
})
const rawArgs = reactive<Record<string, string>>({})
const textArgs = reactive<Record<string, string>>({})

const previewAssets = ref<Asset[]>([])
const previewAssetId = ref('')
const previewUrl = ref('')
const previewLoading = ref(false)
const previewError = ref<string | null>(null)
let previewController: AbortController | null = null
let previewTimer: number | null = null

const categories = computed(() => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const option of catalog.value.options) {
    if (!seen.has(option.category)) {
      seen.add(option.category)
      out.push(option.category)
    }
  }
  return ['All', ...out]
})

const visibleGroups = computed(() => {
  const term = search.value.trim().toLowerCase()
  const groups = new Map<string, CatalogOption[]>()
  for (const option of catalog.value.options) {
    if (category.value !== 'All' && option.category !== category.value) continue
    if (
      term &&
      !`${option.label} ${option.name} ${option.description} ${option.category}`
        .toLowerCase()
        .includes(term)
    ) {
      continue
    }
    const list = groups.get(option.category) ?? []
    list.push(option)
    groups.set(option.category, list)
  }
  return [...groups.entries()].map(([name, options]) => ({ name, options }))
})

const selectedCount = computed(
  () => Object.keys(form.options).length,
)

const selectedPreset = computed(
  () => presets.value.find((preset) => preset.id === form.id) ?? null,
)

const pipelinePreview = computed(() => {
  return buildParameters()
    .options.map((option) => [option.name, ...option.args].join(':'))
    .join('/')
})

function onPreviewImageError(event: Event): void {
  const target = event.target as HTMLImageElement
  if (!previewUrl.value || target.src !== previewUrl.value) return
  previewError.value = 'Preview could not be decoded.'
}

function clearPreview(): void {
  if (previewController) {
    previewController.abort()
    previewController = null
  }
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = ''
  }
  previewLoading.value = false
}

function schedulePreview(): void {
  if (previewTimer !== null) {
    window.clearTimeout(previewTimer)
    previewTimer = null
  }
  if (!presetsDialog.visible || !previewAssetId.value) {
    clearPreview()
    return
  }
  if (!buildParameters().options.length) {
    clearPreview()
    return
  }
  previewTimer = window.setTimeout(() => {
    previewTimer = null
    void refreshPreview()
  }, 250)
}

async function refreshPreview(): Promise<void> {
  if (previewController) previewController.abort()
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = ''
  }
  const controller = new AbortController()
  previewController = controller
  previewLoading.value = true
  previewError.value = null
  try {
    const blob = await api.previewPreset(
      previewAssetId.value,
      buildParameters(),
      controller.signal,
    )
    if (previewController !== controller) return
    previewUrl.value = URL.createObjectURL(blob)
  } catch (e) {
    if (controller.signal.aborted) return
    previewError.value = e instanceof Error ? e.message : String(e)
  } finally {
    if (previewController === controller) previewLoading.value = false
  }
}

watch(
  () =>
    [
      presetsDialog.visible,
      previewAssetId.value,
      form.format,
      pipelinePreview.value,
    ] as const,
  ([visible]) => {
    if (!visible) {
      clearPreview()
      return
    }
    schedulePreview()
  },
)

onBeforeUnmount(() => {
  if (previewTimer !== null) window.clearTimeout(previewTimer)
  clearPreview()
})

function optionEnabled(name: string): boolean {
  return Object.prototype.hasOwnProperty.call(form.options, name)
}

function argValue(option: CatalogOption, index: number): string {
  return form.options[option.name]?.[index] ?? ''
}

function setArg(option: CatalogOption, index: number, value: string): void {
  const args = form.options[option.name]
  if (!args) return
  args[index] = value
}

function onBoolArg(option: CatalogOption, index: number, event: Event): void {
  const target = event.target as HTMLInputElement
  setArg(option, index, target.checked ? '1' : '0')
}

function onSelectArg(option: CatalogOption, index: number, event: Event): void {
  const target = event.target as HTMLSelectElement
  setArg(option, index, target.value)
}

function onTextArg(option: CatalogOption, index: number, event: Event): void {
  const target = event.target as HTMLInputElement
  setArg(option, index, target.value)
}

function onBase64Arg(option: CatalogOption, index: number, event: Event): void {
  const target = event.target as HTMLInputElement
  textArgs[base64Key(option, index)] = target.value
}

function onToggleOption(option: CatalogOption, event: Event): void {
  const target = event.target as HTMLInputElement
  toggleOption(option, target.checked)
}

function base64Key(option: CatalogOption, index: number): string {
  return `${option.name}:${index}`
}

function encodeBase64(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

function decodeBase64(value: string): string {
  if (!value) return ''
  try {
    const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
    const binary = atob(padded)
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    return new TextDecoder().decode(bytes)
  } catch {
    return value
  }
}

function toggleOption(option: CatalogOption, enabled: boolean): void {
  if (enabled) {
    if (option.raw) {
      form.options[option.name] = []
      rawArgs[option.name] = ''
      return
    }
    const args = (option.args ?? []).map((spec) => spec.default ?? '')
    form.options[option.name] = args
    for (let i = 0; i < (option.args ?? []).length; i++) {
      const spec = option.args![i]
      if (spec.kind === 'base64') {
        textArgs[base64Key(option, i)] = decodeBase64(args[i] ?? '')
      }
    }
    return
  }
  delete form.options[option.name]
  delete rawArgs[option.name]
  for (const key of Object.keys(textArgs)) {
    if (key.startsWith(`${option.name}:`)) delete textArgs[key]
  }
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64)
}

function onLabelInput(): void {
  if (!nameTouched.value) {
    form.name = slugify(form.label)
  }
}

function resetForm(): void {
  form.id = null
  form.label = ''
  form.name = ''
  form.format = catalog.value.formats[0] ?? 'webp'
  form.libraryWide = true
  form.collectionIds = []
  form.tagIds = []
  for (const key of Object.keys(form.options)) delete form.options[key]
  for (const key of Object.keys(rawArgs)) delete rawArgs[key]
  for (const key of Object.keys(textArgs)) delete textArgs[key]
  nameTouched.value = false
  error.value = null
}

function newPreset(): void {
  presetsDialog.presetId = null
  resetForm()
}

function selectPreset(preset: Preset): void {
  presetsDialog.presetId = preset.id
  form.id = preset.id
  form.label = preset.label
  form.name = preset.name
  form.format = preset.parameters.format
  form.libraryWide = preset.library_wide
  form.collectionIds = [...(preset.collection_ids ?? [])]
  form.tagIds = [...(preset.tag_ids ?? [])]
  for (const key of Object.keys(form.options)) delete form.options[key]
  for (const key of Object.keys(rawArgs)) delete rawArgs[key]
  for (const key of Object.keys(textArgs)) delete textArgs[key]
  error.value = null
  nameTouched.value = true

  for (const saved of preset.parameters.options) {
    const definition = catalog.value.options.find((item) => item.name === saved.name)
    if (!definition) continue
    if (definition.raw) {
      form.options[definition.name] = []
      rawArgs[definition.name] = saved.args.join(':')
      continue
    }
    const args = (definition.args ?? []).map(
      (spec, index) => saved.args[index] ?? spec.default ?? '',
    )
    form.options[definition.name] = args
    for (let i = 0; i < (definition.args ?? []).length; i++) {
      const spec = definition.args![i]
      if (spec.kind === 'base64') {
        textArgs[base64Key(definition, i)] = decodeBase64(args[i] ?? '')
      }
    }
  }
}

function buildParameters() {
  const options: PresetOption[] = []
  for (const definition of catalog.value.options) {
    if (!optionEnabled(definition.name)) continue
    if (definition.raw) {
      const args = (rawArgs[definition.name] ?? '')
        .split(':')
        .map((value) => value.trim())
      while (args.length && args[args.length - 1] === '') args.pop()
      options.push({ name: definition.name, args })
      continue
    }
    const args = (form.options[definition.name] ?? []).map((value, index) => {
      const spec = definition.args?.[index]
      if (spec?.kind === 'base64') {
        return encodeBase64(textArgs[base64Key(definition, index)] ?? '')
      }
      return value
    })
    while (args.length && args[args.length - 1] === '') args.pop()
    options.push({ name: definition.name, args })
  }
  return { format: form.format, options }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const [list, loadedCatalog, page] = await Promise.all([
      api.listPresets(),
      api.getPresetCatalog(),
      api.listAssets({ types: ['image'], limit: 24 }),
    ])
    presets.value = list
    catalog.value = loadedCatalog
    previewAssets.value = page.items

    const requestedAsset = presetsDialog.assetId
    if (requestedAsset) {
      if (!page.items.some((asset) => asset.id === requestedAsset)) {
        try {
          const asset = await api.getAsset(requestedAsset)
          if (asset.mime_type.startsWith('image/')) {
            previewAssets.value = [asset, ...previewAssets.value]
          }
        } catch {
          // The asset may have been deleted; fall back to the first image.
        }
      }
      previewAssetId.value = previewAssets.value.some(
        (asset) => asset.id === requestedAsset,
      )
        ? requestedAsset
        : page.items[0]?.id ?? ''
    } else if (!previewAssets.value.some((asset) => asset.id === previewAssetId.value)) {
      previewAssetId.value = previewAssets.value[0]?.id ?? ''
    }

    const requested = presetsDialog.presetId
    const preset =
      list.find((item) => item.id === requested) ?? list[0] ?? null
    if (preset) {
      selectPreset(preset)
    } else {
      resetForm()
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (!form.label.trim()) {
    error.value = 'Label is required.'
    return
  }
  if (!form.name.trim()) {
    error.value = 'Name is required.'
    return
  }
  const parameters = buildParameters()
  if (!parameters.options.length) {
    error.value = 'Add at least one manipulation.'
    return
  }

  saving.value = true
  error.value = null
  try {
    const input = {
      label: form.label.trim(),
      name: form.name.trim(),
      parameters,
      library_wide: form.libraryWide,
      collection_ids: form.libraryWide ? [] : form.collectionIds,
      tag_ids: form.libraryWide ? [] : form.tagIds,
    }
    const saved = form.id
      ? await api.updatePreset(form.id, input)
      : await api.createPreset(input)
    const list = await api.listPresets()
    presets.value = list
    selectPreset(saved)
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary: 'Preset saved',
      detail: saved.label,
      life: 2500,
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

function remove(): void {
  const preset = presets.value.find((item) => item.id === form.id)
  if (!preset) return
  confirm.require({
    group: 'preset-delete',
    header: 'Delete preset?',
    message: `Delete “${preset.label}”? Existing cached conversions are kept.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => {
      void (async () => {
        try {
          await api.deletePreset(preset.id)
          const list = await api.listPresets()
          presets.value = list
          notifyLibraryChanged()
          if (list.length) {
            selectPreset(list[0])
          } else {
            resetForm()
          }
          toast.add({
            severity: 'success',
            summary: 'Preset deleted',
            life: 2500,
          })
        } catch (e) {
          error.value = e instanceof Error ? e.message : String(e)
        }
      })()
    },
  })
}

watch(
  () => presetsDialog.visible,
  (visible) => {
    if (visible) void load()
  },
)

watch(
  () => form.label,
  () => onLabelInput(),
)
</script>

<template>
  <Dialog
    v-model:visible="presetsDialog.visible"
    header="Image presets"
    modal
    :draggable="false"
    :style="{ width: '76rem', maxWidth: '96vw' }"
    :pt="{ content: { class: '!p-0' } }"
  >
    <div class="grid min-h-[34rem] grid-cols-[16rem_1fr]">
      <aside class="border-r border-slate-200 dark:border-slate-800">
        <div class="flex items-center justify-between px-3 py-2">
          <span class="text-xs font-semibold uppercase tracking-wide text-slate-400">
            Presets
          </span>
          <button
            class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium text-brand-700 hover:bg-brand-50 dark:text-brand-400 dark:hover:bg-brand-950/40"
            @click="newPreset"
          >
            <Plus :size="12" />
            New
          </button>
        </div>
        <div class="max-h-[30rem] space-y-0.5 overflow-y-auto px-2 pb-2">
          <button
            v-for="preset in presets"
            :key="preset.id"
            class="w-full rounded-md px-2 py-2 text-left transition-colors"
            :class="
              form.id === preset.id
                ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/50 dark:text-brand-300'
                : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
            "
            @click="selectPreset(preset)"
          >
            <div class="truncate text-sm font-medium">{{ preset.label }}</div>
            <div class="truncate font-mono text-[11px] text-slate-400">
              {{ preset.name }} · {{ preset.parameters.format }}
            </div>
            <div class="mt-0.5 text-[11px] text-slate-400">
              {{ preset.parameters.options.length }} manipulations
            </div>
          </button>
          <p v-if="!presets.length && !loading" class="px-2 py-6 text-center text-xs text-slate-400">
            No presets yet.
          </p>
        </div>
      </aside>

      <section class="flex min-h-0 flex-col">
        <div class="flex-1 space-y-4 overflow-y-auto p-4">
          <div
            v-if="loading"
            class="flex h-40 items-center justify-center text-sm text-slate-400"
          >
            Loading catalog…
          </div>
          <template v-else>
            <div class="grid grid-cols-[1fr_1fr_9rem] gap-3">
              <div>
                <label class="mb-1 block text-xs font-medium text-slate-400" for="preset-label">
                  Label
                </label>
                <input
                  id="preset-label"
                  v-model="form.label"
                  class="w-full rounded border border-slate-300 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900"
                  placeholder="Thumbnail"
                />
              </div>
              <div>
                <label class="mb-1 block text-xs font-medium text-slate-400" for="preset-name">
                  Name (URL)
                </label>
                <input
                  id="preset-name"
                  v-model="form.name"
                  class="w-full rounded border border-slate-300 bg-white px-3 py-1.5 font-mono text-sm dark:border-slate-700 dark:bg-slate-900"
                  placeholder="thumbnail"
                  @input="nameTouched = true"
                />
              </div>
              <div>
                <label class="mb-1 block text-xs font-medium text-slate-400" for="preset-format">
                  Format
                </label>
                <select
                  id="preset-format"
                  v-model="form.format"
                  class="w-full rounded border border-slate-300 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900"
                >
                  <option v-for="format in catalog.formats" :key="format" :value="format">
                    {{ format.toUpperCase() }}
                  </option>
                </select>
              </div>
            </div>
            <p class="font-mono text-[11px] text-slate-400">
              /&lt;asset-id&gt;/{{ form.name || 'preset' }}.{{ form.format }}
            </p>
            <p v-if="selectedPreset" class="text-[11px] text-slate-400">
              Added by {{ selectedPreset.added_by_name || 'unknown' }}
              · created {{ formatRelative(selectedPreset.created_at) }}
              · modified
              <span :title="formatDateTime(selectedPreset.modified_at)">
                {{ formatRelative(selectedPreset.modified_at) }}
              </span>
            </p>

            <div class="flex items-start gap-3 rounded-lg border border-slate-200 p-3 dark:border-slate-800">
              <div class="min-w-0 flex-1">
                <label class="mb-1 block text-xs font-medium text-slate-400" for="preset-preview-asset">
                  Preview asset
                </label>
                <select
                  id="preset-preview-asset"
                  v-model="previewAssetId"
                  class="w-full rounded border border-slate-300 bg-white px-3 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900"
                >
                  <option v-for="asset in previewAssets" :key="asset.id" :value="asset.id">
                    {{ asset.filename }}
                  </option>
                </select>
                <p class="mt-1 text-xs text-slate-400">
                  Renders as you edit. Previews are not written to the conversion cache.
                </p>
                <button
                  type="button"
                  class="mt-1 text-xs text-brand-700 hover:underline disabled:opacity-40 dark:text-brand-400"
                  :disabled="previewLoading || !previewAssetId"
                  @click="refreshPreview"
                >
                  {{ previewLoading ? 'Rendering…' : 'Refresh preview' }}
                </button>
                <p v-if="previewError" class="mt-1 text-xs text-red-600 dark:text-red-400">
                  {{ previewError }}
                </p>
              </div>
              <div
                class="flex h-40 w-40 shrink-0 items-center justify-center overflow-hidden rounded border border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900"
              >
                <img
                  v-if="previewUrl"
                  :src="previewUrl"
                  alt="Preset preview"
                  class="max-h-full max-w-full object-contain"
                  @error="onPreviewImageError"
                />
                <span v-else class="px-2 text-center text-[11px] text-slate-400">
                  Add a manipulation to preview
                </span>
              </div>
            </div>

            <div class="space-y-2 rounded-lg border border-slate-200 p-3 dark:border-slate-800">
              <label class="flex cursor-pointer items-center gap-2 text-sm font-medium">
                <input v-model="form.libraryWide" type="checkbox" />
                Library-wide
              </label>
              <p class="text-xs text-slate-400">
                Available to every asset, unless you scope it to collections or tags.
              </p>
              <div v-if="!form.libraryWide" class="grid grid-cols-2 gap-3">
                <div>
                  <label class="mb-1 block text-[11px] text-slate-400">
                    Collections
                  </label>
                  <MultiSelect
                    v-model="form.collectionIds"
                    :options="library.collections"
                    option-label="name"
                    option-value="id"
                    display="chip"
                    filter
                    showClear
                    placeholder="No collections"
                    class="w-full"
                  />
                </div>
                <div>
                  <label class="mb-1 block text-[11px] text-slate-400">
                    Tags
                  </label>
                  <MultiSelect
                    v-model="form.tagIds"
                    :options="library.tags"
                    option-label="name"
                    option-value="id"
                    display="chip"
                    filter
                    showClear
                    placeholder="No tags"
                    class="w-full"
                  />
                </div>
              </div>
              <p
                v-if="!form.libraryWide && !form.collectionIds.length && !form.tagIds.length"
                class="text-xs text-brand-700 dark:text-brand-400"
              >
                No asset can use this preset until it is assigned to a collection or tag.
              </p>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <div class="relative min-w-52 flex-1">
                <Search
                  :size="14"
                  class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400"
                />
                <input
                  v-model="search"
                  class="w-full rounded border border-slate-300 bg-white py-1.5 pl-8 pr-3 text-sm dark:border-slate-700 dark:bg-slate-900"
                  placeholder="Search manipulations…"
                />
              </div>
              <div class="flex max-w-full flex-wrap gap-1">
                <button
                  v-for="item in categories"
                  :key="item"
                  type="button"
                  class="rounded-full px-2.5 py-1 text-xs font-medium transition-colors"
                  :class="
                    category === item
                      ? 'bg-indigo-600 text-white dark:bg-orange-500 dark:text-orange-950'
                      : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'
                  "
                  @click="category = item"
                >
                  {{ item }}
                </button>
              </div>
            </div>

            <div class="max-h-[26rem] space-y-4 overflow-y-auto pr-1">
              <div v-for="group in visibleGroups" :key="group.name">
                <h4 class="mb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">
                  {{ group.name }}
                </h4>
                <div class="divide-y divide-slate-100 rounded-lg border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
                  <div v-for="option in group.options" :key="option.name" class="p-2.5">
                    <label class="flex cursor-pointer items-start gap-2">
                      <input
                        type="checkbox"
                        class="mt-1"
                        :checked="optionEnabled(option.name)"
                        @change="onToggleOption(option, $event)"
                      />
                      <span class="min-w-0 flex-1">
                        <span class="flex flex-wrap items-center gap-1.5">
                          <span class="text-sm font-medium">{{ option.label }}</span>
                          <span class="font-mono text-[11px] text-slate-400">
                            {{ option.name }}
                          </span>
                          <span
                            v-if="option.pro"
                            class="rounded bg-brand-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-brand-700 dark:bg-brand-950/50 dark:text-brand-300"
                          >
                            Pro
                          </span>
                        </span>
                        <span class="mt-0.5 block text-xs text-slate-400">
                          {{ option.description }}
                        </span>
                      </span>
                    </label>

                    <div
                      v-if="optionEnabled(option.name)"
                      class="mt-2 pl-6"
                    >
                      <div
                        v-if="option.raw"
                        class="flex items-center gap-2"
                      >
                        <input
                          v-model="rawArgs[option.name]"
                          class="w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 font-mono text-xs dark:border-slate-700 dark:bg-slate-900"
                          placeholder="colon-separated arguments"
                        />
                        <span class="shrink-0 text-[11px] text-slate-400">
                          args:arg:arg
                        </span>
                      </div>
                      <div v-else class="grid grid-cols-2 gap-2 lg:grid-cols-3">
                        <label
                          v-for="(arg, index) in option.args ?? []"
                          :key="arg.name"
                          class="block"
                        >
                          <span class="mb-0.5 block text-[11px] text-slate-400">
                            {{ arg.label }}<span v-if="arg.required" class="text-red-500"> *</span>
                          </span>
                          <input
                            v-if="
                              arg.kind === 'int' ||
                              arg.kind === 'float' ||
                              arg.kind === 'string' ||
                              arg.kind === 'color'
                            "
                            :value="argValue(option, index)"
                            :type="arg.kind === 'int' || arg.kind === 'float' ? 'number' : 'text'"
                            :min="arg.min"
                            :max="arg.max"
                            :placeholder="arg.placeholder"
                            class="w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900"
                            @input="onTextArg(option, index, $event)"
                          />
                          <input
                            v-else-if="arg.kind === 'base64'"
                            :value="textArgs[base64Key(option, index)] ?? ''"
                            type="text"
                            class="w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900"
                            placeholder="plain text (encoded automatically)"
                            @input="onBase64Arg(option, index, $event)"
                          />
                          <select
                            v-else-if="arg.kind === 'enum'"
                            :value="argValue(option, index)"
                            class="w-full rounded border border-slate-300 bg-white px-2.5 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-900"
                            @change="onSelectArg(option, index, $event)"
                          >
                            <option
                              v-for="choice in arg.options ?? []"
                              :key="choice.value"
                              :value="choice.value"
                            >
                              {{ choice.label }}
                            </option>
                          </select>
                          <span
                            v-else-if="arg.kind === 'bool'"
                            class="flex items-center gap-2 py-1.5 text-xs"
                          >
                            <input
                              type="checkbox"
                              :checked="argValue(option, index) === '1'"
                              @change="onBoolArg(option, index, $event)"
                            />
                            enabled
                          </span>
                        </label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <p
                v-if="!visibleGroups.length"
                class="py-8 text-center text-sm text-slate-400"
              >
                No manipulations match this search.
              </p>
            </div>

            <div class="rounded-lg border border-slate-200 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-900/60">
              <div class="flex items-center justify-between text-[11px] uppercase tracking-wide text-slate-400">
                <span>Pipeline</span>
                <span>{{ selectedCount }} manipulations</span>
              </div>
              <code class="mt-1 block break-all text-xs text-slate-600 dark:text-slate-300">
                {{ pipelinePreview || '—' }}
              </code>
            </div>

            <p v-if="error" class="text-sm text-red-600 dark:text-red-400">
              {{ error }}
            </p>
          </template>
        </div>

        <div class="flex items-center justify-between gap-2 border-t border-slate-200 px-4 py-3 dark:border-slate-800">
          <button
            v-if="form.id"
            class="inline-flex items-center gap-1.5 rounded px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30"
            @click="remove"
          >
            <Trash :size="14" />
            Delete
          </button>
          <span v-else />
          <div class="flex items-center gap-2">
            <button
              class="rounded px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
              @click="presetsDialog.visible = false"
            >
              Close
            </button>
            <button
              class="rounded bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50 dark:bg-orange-500 dark:text-orange-950 dark:hover:bg-orange-400"
              :disabled="saving || loading"
              @click="save"
            >
              {{ saving ? 'Saving…' : 'Save preset' }}
            </button>
          </div>
        </div>
      </section>
    </div>
    <ConfirmDialog group="preset-delete" />
  </Dialog>
</template>
