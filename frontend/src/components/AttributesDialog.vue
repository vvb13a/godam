<script setup lang="ts">
import Dialog from 'primevue/dialog'
import Popover from 'primevue/popover'
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import { Eye, EyeSlash, GripVertical, Lock, LockOpen, Pencil, Plus, Search, Trash } from '@primeicons/vue'
import { computed, nextTick, ref, watch } from 'vue'
import * as api from '../api/client'
import { attributesDialog, notifyAttributesChanged } from '../lib/attributesDialog'
import { RENDER_OPTIONS } from '../lib/attributeRender'
import { SORT_OPTIONS } from '../lib/attributeSort'
import { hasCapability } from '../lib/capabilities'
import { library } from '../lib/library'
import { LIBRARY_COLORS, libraryColorHex } from '../lib/libraryColors'
import { splitNames } from '../lib/names'
import { ATTRIBUTE_UNASSIGNED } from '../types'
import type {
  AttributeFacet,
  AttributeFacetValue,
  AttributeRenderSetting,
  AttributeSortSetting,
} from '../types'

interface ValueEdit {
  key: string
  value: string
}

const toast = useToast()
const confirm = useConfirm()
const facets = ref<AttributeFacet[]>([])

const MULTI_OPTIONS = [
  { value: 'multi', label: 'Multiple' },
  { value: 'single', label: 'Single' },
]
const loading = ref(false)
const loaded = ref(false)
const error = ref('')
const editingKey = ref<string | null>(null)
const editingValue = ref<ValueEdit | null>(null)
const addingKey = ref<string | null>(null)
const addingAttribute = ref(false)
const attributeDraft = ref('')
const draft = ref('')
const addDraft = ref('')
const adding = ref(false)
const draggingKey = ref<string | null>(null)
const draggingValue = ref<{ key: string; value: string } | null>(null)
const keyOrderDirty = ref(false)
const valueOrderDirty = ref(false)
const descriptionPopover = ref<InstanceType<typeof Popover> | null>(null)
const colorPopover = ref<InstanceType<typeof Popover> | null>(null)
const descriptionTarget = ref<{ key: string; value: string } | null>(null)
const descriptionDraft = ref('')
const colorTarget = ref<{ key: string; value: string } | null>(null)

const canManage = computed(() => hasCapability('settings:attributes'))

const orderedFacets = computed(() =>
  [...facets.value].sort(
    (a, b) =>
      (a.position ?? 0) - (b.position ?? 0) ||
      (a.label || a.key).localeCompare(b.label || b.key),
  ),
)

function orderedValues(facet: AttributeFacet): AttributeFacetValue[] {
  const values = [...facet.values]
  if (facet.sort === 'custom') {
    values.sort(
      (a, b) =>
        (a.position ?? 0) - (b.position ?? 0) ||
        (a.label || a.value).localeCompare(b.label || b.value),
    )
  } else {
    values.sort((a, b) =>
      (a.label || a.value).localeCompare(b.label || b.value),
    )
  }
  return values
}

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function load(resetEdits = false): Promise<void> {
  const initial = !loaded.value
  if (initial) loading.value = true
  error.value = ''
  if (resetEdits) {
    editingKey.value = null
    editingValue.value = null
    addingKey.value = null
    addingAttribute.value = false
    attributeDraft.value = ''
  }
  try {
    facets.value = await api.listAttributeCatalog()
    loaded.value = true
  } catch (e) {
    if (!loaded.value) {
      error.value = toMessage(e)
    } else {
      toast.add({
        severity: 'error',
        summary: 'Could not refresh attributes',
        detail: toMessage(e),
        life: 4000,
      })
    }
  } finally {
    loading.value = false
  }
}

watch(
  () => attributesDialog.visible,
  (visible) => {
    if (visible) void load(true)
  },
)

function focusEdit(): void {
  nextTick(() => {
    const input = document.querySelector<HTMLInputElement>(
      '[data-attribute-edit]',
    )
    input?.focus()
    input?.select()
  })
}

function startEditKey(facet: AttributeFacet): void {
  if (!canManage.value) return
  editingValue.value = null
  editingKey.value = facet.key
  draft.value = facet.label || facet.key
  focusEdit()
}

function startEditValue(facet: AttributeFacet, value: AttributeFacetValue): void {
  if (!canManage.value) return
  editingKey.value = null
  editingValue.value = { key: facet.key, value: value.value }
  draft.value = value.label || value.value
  focusEdit()
}

function cancelEdit(): void {
  editingKey.value = null
  editingValue.value = null
  draft.value = ''
}

function focusAdd(): void {
  nextTick(() => {
    document.querySelector<HTMLInputElement>('[data-attribute-add]')?.focus()
  })
}

function focusNewAttribute(): void {
  nextTick(() => {
    document.querySelector<HTMLInputElement>('[data-attribute-new]')?.focus()
  })
}

function startAddAttribute(): void {
  if (!canManage.value) return
  addingAttribute.value = true
  attributeDraft.value = ''
  focusNewAttribute()
}

function cancelAddAttribute(): void {
  addingAttribute.value = false
  attributeDraft.value = ''
}

async function commitAddAttribute(): Promise<void> {
  if (!addingAttribute.value) return
  const keys = splitNames(attributeDraft.value)
  addingAttribute.value = false
  attributeDraft.value = ''
  if (!keys.length) return
  try {
    for (const key of keys) {
      await api.createAttribute(key)
    }
    await load()
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create attribute',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeValueSearch(
  facet: AttributeFacet,
  enabled: boolean,
): Promise<void> {
  const previous = facet.value_search
  facet.value_search = enabled
  try {
    await api.updateAttributeValueSearch(facet.key, enabled)
    notifyAttributesChanged()
  } catch (e) {
    facet.value_search = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update value search',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeValuesLock(
  facet: AttributeFacet,
  locked: boolean,
): Promise<void> {
  const previous = facet.values_locked
  facet.values_locked = locked
  try {
    await api.updateAttributeValuesLocked(facet.key, locked)
    notifyAttributesChanged()
  } catch (e) {
    facet.values_locked = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update value lock',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeStamping(
  kind: 'keys' | 'values',
  value: boolean | undefined,
): Promise<void> {
  const keys =
    kind === 'keys' ? Boolean(value) : (library.upload_attribute_keys ?? true)
  const values =
    kind === 'values'
      ? Boolean(value)
      : (library.upload_attribute_values ?? true)
  try {
    const updated = await api.updateAttributeStamping(keys, values)
    Object.assign(library, updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update upload stamping',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startAddValues(facet: AttributeFacet): void {
  if (!canManage.value) return
  editingKey.value = null
  editingValue.value = null
  addingKey.value = facet.key
  addDraft.value = ''
  focusAdd()
}

function cancelAddValues(): void {
  addingKey.value = null
  addDraft.value = ''
}

async function commitAddValues(): Promise<void> {
  const key = addingKey.value
  if (!key || adding.value) return
  const facet = facets.value.find((item) => item.key === key)
  const existing = new Set(
    (facet?.values ?? []).map((item) => item.value.toLocaleLowerCase()),
  )
  const values = splitNames(addDraft.value).filter(
    (value) => !existing.has(value.toLocaleLowerCase()),
  )
  addingKey.value = null
  addDraft.value = ''
  if (!values.length) return
  adding.value = true
  try {
    await api.createAttributeValues(key, values)
    await load()
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not add attribute values',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    adding.value = false
  }
}

async function commitEdit(): Promise<void> {
  const valueEdit = editingValue.value
  const keyEdit = editingKey.value
  if (!valueEdit && !keyEdit) return

  const key = valueEdit?.key ?? keyEdit
  const value = valueEdit?.value ?? ''
  if (!key) {
    cancelEdit()
    return
  }
  const facet = facets.value.find((item) => item.key === key)
  const target = valueEdit
    ? facet?.values.find((item) => item.value === value)
    : undefined
  const raw = valueEdit ? (target?.value ?? value) : (facet?.key ?? key)
  const label = draft.value.trim()

  editingKey.value = null
  editingValue.value = null
  draft.value = ''

  const override = label === raw ? '' : label
  try {
    if (value === ATTRIBUTE_UNASSIGNED) {
      await api.updateAttributeUnassignedLabel(key, label)
      if (target) target.label = label || 'Unassigned'
    } else {
      await api.updateAttributeLabel(key, value, override)
      if (facet) {
        if (valueEdit && target) target.label = label || raw
        else facet.label = label || facet.key
      }
    }
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not rename attribute',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startKeyDrag(event: DragEvent, key: string): void {
  draggingKey.value = key
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', key)
  }
}

function onKeyDragOver(event: DragEvent): void {
  const source = draggingKey.value
  if (!source) return
  const container = event.currentTarget as HTMLElement
  const cards = [
    ...container.querySelectorAll<HTMLElement>('[data-attribute-key]'),
  ]
  if (!cards.length) return
  let targetKey: string | null = null
  for (const card of cards) {
    const rect = card.getBoundingClientRect()
    if (event.clientY < rect.top + rect.height / 2) {
      targetKey = card.dataset.attributeKey ?? null
      break
    }
  }
  const ordered = orderedFacets.value
  const sourceIndex = ordered.findIndex((facet) => facet.key === source)
  if (sourceIndex < 0) return
  let targetIndex = targetKey
    ? ordered.findIndex((facet) => facet.key === targetKey)
    : ordered.length
  if (targetIndex < 0) targetIndex = ordered.length
  if (sourceIndex < targetIndex) targetIndex--
  if (targetIndex === sourceIndex) return
  const list = [...ordered]
  const [item] = list.splice(sourceIndex, 1)
  list.splice(targetIndex, 0, item)
  list.forEach((facet, index) => {
    facet.position = index
  })
  facets.value = list
  keyOrderDirty.value = true
}

async function finishKeyDrag(): Promise<void> {
  const dirty = keyOrderDirty.value
  keyOrderDirty.value = false
  draggingKey.value = null
  if (!dirty) return
  try {
    await api.updateAttributeKeyOrder(facets.value.map((facet) => facet.key))
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not reorder attributes',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startValueDrag(
  event: DragEvent,
  key: string,
  value: string,
): void {
  draggingValue.value = { key, value }
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', value)
  }
}

function onValueDragOver(
  event: DragEvent,
  facet: AttributeFacet,
): void {
  const drag = draggingValue.value
  if (!drag || drag.key !== facet.key) return
  const container = event.currentTarget as HTMLElement
  const rows = [
    ...container.querySelectorAll<HTMLElement>('[data-value-row]'),
  ]
  if (!rows.length) return
  let targetValue: string | null = null
  for (const row of rows) {
    const rect = row.getBoundingClientRect()
    if (event.clientY < rect.top + rect.height / 2) {
      targetValue = row.dataset.valueRow ?? null
      break
    }
  }
  const ordered = orderedValues(facet)
  const sourceIndex = ordered.findIndex(
    (value) => value.value === drag.value,
  )
  if (sourceIndex < 0) return
  let targetIndex = targetValue
    ? ordered.findIndex((value) => value.value === targetValue)
    : ordered.length
  if (targetIndex < 0) targetIndex = ordered.length
  if (sourceIndex < targetIndex) targetIndex--
  if (targetIndex === sourceIndex) return
  const values = [...ordered]
  const [item] = values.splice(sourceIndex, 1)
  values.splice(targetIndex, 0, item)
  values.forEach((value, index) => {
    value.position = index
  })
  facet.values = values
  valueOrderDirty.value = true
}

async function finishValueDrag(): Promise<void> {
  const drag = draggingValue.value
  const dirty = valueOrderDirty.value
  valueOrderDirty.value = false
  draggingValue.value = null
  if (!dirty || !drag) return
  const facet = facets.value.find((item) => item.key === drag.key)
  if (!facet) return
  const values = orderedValues(facet)
  try {
    await api.updateAttributeValueOrder(
      facet.key,
      values.map((value) => value.value),
    )
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not reorder values',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function openDescription(
  event: Event,
  key: string,
  value = '',
): void {
  descriptionTarget.value = { key, value }
  const facet = facets.value.find((item) => item.key === key)
  if (value) {
    descriptionDraft.value =
      facet?.values.find((item) => item.value === value)?.description ?? ''
  } else {
    descriptionDraft.value = facet?.description ?? ''
  }
  descriptionPopover.value?.toggle(event)
}

async function saveDescription(): Promise<void> {
  const target = descriptionTarget.value
  if (!target) return
  const description = descriptionDraft.value.trim()
  try {
    await api.updateAttributeDescription(
      target.key,
      target.value,
      description,
    )
    const facet = facets.value.find((item) => item.key === target.key)
    if (facet) {
      if (target.value) {
        const value = facet.values.find(
          (item) => item.value === target.value,
        )
        if (value) value.description = description
      } else {
        facet.description = description
      }
    }
    descriptionPopover.value?.hide()
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update description',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function openColor(event: Event, key: string, value = ''): void {
  colorTarget.value = { key, value }
  colorPopover.value?.toggle(event)
}

async function chooseColor(color: string): Promise<void> {
  const target = colorTarget.value
  if (!target) return
  try {
    await api.updateAttributeColor(target.key, target.value, color)
    const facet = facets.value.find((item) => item.key === target.key)
    if (facet) {
      if (target.value) {
        const value = facet.values.find(
          (item) => item.value === target.value,
        )
        if (value) value.color = color
      } else {
        facet.color = color
      }
    }
    colorPopover.value?.hide()
    notifyAttributesChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update color',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function keyColor(facet: AttributeFacet): string {
  return libraryColorHex(facet.color ?? '')
}

function valueColor(
  facet: AttributeFacet,
  value: AttributeFacetValue,
): string {
  return libraryColorHex(value.color || facet.color || '')
}

const descriptionTargetLabel = computed(() => {
  const target = descriptionTarget.value
  if (!target) return ''
  const facet = facets.value.find((item) => item.key === target.key)
  if (!facet) return ''
  if (!target.value) return facet.label || facet.key
  const value = facet.values.find((item) => item.value === target.value)
  return `${facet.label || facet.key} · ${value?.label || target.value}`
})

async function changeSort(
  facet: AttributeFacet,
  sort: AttributeSortSetting,
): Promise<void> {
  const previous = facet.sort
  facet.sort = sort
  try {
    await api.updateAttributeSort(facet.key, sort)
    notifyAttributesChanged()
  } catch (e) {
    facet.sort = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update sorting',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeRender(
  facet: AttributeFacet,
  render: AttributeRenderSetting,
): Promise<void> {
  const previous = facet.render
  facet.render = render
  try {
    await api.updateAttributeRender(facet.key, render)
    notifyAttributesChanged()
  } catch (e) {
    facet.render = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update rendering',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function parentOptions(facet: AttributeFacet) {
  return [
    { value: '', label: 'None' },
    ...facets.value
      .filter((item) => item.key !== facet.key)
      .map((item) => ({
        value: item.key,
        label: item.label || item.key,
      })),
  ]
}

async function changeParent(
  facet: AttributeFacet,
  parent: string | undefined,
): Promise<void> {
  const previous = facet.parent_key ?? ''
  const next = parent ?? ''
  facet.parent_key = next || undefined
  try {
    await api.updateAttributeParent(facet.key, next)
    notifyAttributesChanged()
  } catch (e) {
    facet.parent_key = previous || undefined
    toast.add({
      severity: 'error',
      summary: 'Could not update attribute parent',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeSearchable(
  facet: AttributeFacet,
  searchable: boolean | undefined,
): Promise<void> {
  const previous = facet.searchable
  facet.searchable = Boolean(searchable)
  try {
    await api.updateAttributeSearchable(facet.key, facet.searchable)
    notifyAttributesChanged()
  } catch (e) {
    facet.searchable = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update search indexing',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeMulti(
  facet: AttributeFacet,
  value: string | undefined,
): Promise<void> {
  const previous = facet.multi
  facet.multi = value !== 'single'
  try {
    await api.updateAttributeMulti(facet.key, facet.multi)
    notifyAttributesChanged()
  } catch (e) {
    facet.multi = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update selection mode',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function changeUnassigned(
  facet: AttributeFacet,
  include: boolean | undefined,
): Promise<void> {
  const previous = facet.include_unassigned
  facet.include_unassigned = Boolean(include)
  try {
    await api.updateAttributeUnassigned(facet.key, facet.include_unassigned)
    notifyAttributesChanged()
  } catch (e) {
    facet.include_unassigned = previous
    toast.add({
      severity: 'error',
      summary: 'Could not update unassigned option',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function confirmDeleteValue(
  facet: AttributeFacet,
  value: AttributeFacetValue,
): void {
  if (value.value === ATTRIBUTE_UNASSIGNED) return
  confirm.require({
    group: 'attributes',
    header: 'Delete value?',
    message: `Delete "${value.label || value.value}" from "${facet.label || facet.key}"? This removes it from every asset.`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => {
      void (async () => {
        try {
          await api.deleteAttributeValue(facet.key, value.value)
          await load()
          notifyAttributesChanged()
        } catch (e) {
          toast.add({
            severity: 'error',
            summary: 'Could not delete value',
            detail: toMessage(e),
            life: 4000,
          })
        }
      })()
    },
  })
}

function confirmDeleteKey(facet: AttributeFacet): void {
  confirm.require({
    group: 'attributes',
    header: 'Delete attribute?',
    message: `Delete "${facet.label || facet.key}" and all of its values from every asset?`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => {
      void (async () => {
        try {
          await api.deleteAttributeKey(facet.key)
          await load()
          notifyAttributesChanged()
        } catch (e) {
          toast.add({
            severity: 'error',
            summary: 'Could not delete attribute',
            detail: toMessage(e),
            life: 4000,
          })
        }
      })()
    },
  })
}
</script>

<template>
  <Dialog
    v-model:visible="attributesDialog.visible"
    header="Attributes"
    modal
    :draggable="false"
    :style="{ width: '48rem', maxWidth: '95vw' }"
    :pt="{ content: { class: '!p-0' } }"
  >
    <div
      class="max-h-[70vh] space-y-3 overflow-y-auto p-4"
      @dragover.prevent="onKeyDragOver"
      @drop.prevent="finishKeyDrag"
    >
      <p class="text-sm text-slate-500 dark:text-slate-400">
        Labels only change how attributes appear in the picker. Keys and values
        sent by the host stay unchanged.
        <template v-if="canManage">Double-click a label to rename it.</template>
      </p>

      <div
        v-if="canManage"
        class="flex flex-wrap items-center gap-4 rounded-lg border border-slate-200 bg-slate-50/60 px-3 py-2 text-xs text-slate-600 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-300"
      >
        <button
          type="button"
          class="inline-flex items-center gap-1 font-medium text-brand-700 hover:text-brand-800 dark:text-brand-400"
          @click="startAddAttribute"
        >
          <Plus :size="13" />
          New attribute
        </button>
        <label class="inline-flex items-center gap-2">
          <ToggleSwitch
            :model-value="library.upload_attribute_keys"
            @update:model-value="changeStamping('keys', $event)"
          />
          Upload stamping: new keys
        </label>
        <label class="inline-flex items-center gap-2">
          <ToggleSwitch
            :model-value="library.upload_attribute_values"
            @update:model-value="changeStamping('values', $event)"
          />
          Upload stamping: new values
        </label>
      </div>
      <input
        v-if="addingAttribute"
        v-model="attributeDraft"
        data-attribute-new
        type="text"
        placeholder="New attribute key (use , or ; for multiple)"
        class="w-full rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-700 outline-none focus:border-brand-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
        @keydown.enter.prevent="commitAddAttribute"
        @keydown.esc.prevent="cancelAddAttribute"
        @blur="commitAddAttribute"
      />

      <p v-if="loading" class="text-sm text-slate-400">Loading attributes…</p>
      <p v-else-if="error" class="text-sm text-red-600 dark:text-red-400">
        {{ error }}
      </p>
      <p
        v-else-if="!orderedFacets.length"
        class="rounded-lg border border-dashed border-slate-200 px-4 py-8 text-center text-sm text-slate-400 dark:border-slate-700"
      >
        No attributes yet.
      </p>

      <template v-else>
        <section
          v-for="facet in orderedFacets"
          :key="facet.key"
          :data-attribute-key="facet.key"
          class="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-800"
          :class="{ 'opacity-60': draggingKey === facet.key }"
        >
        <div
          class="flex items-center gap-3 border-b border-slate-100 bg-slate-50/60 px-3 py-2 dark:border-slate-800 dark:bg-slate-900/40"
        >
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 items-center gap-1.5">
              <span
                v-if="canManage"
                draggable="true"
                class="inline-flex shrink-0 cursor-grab text-slate-400 active:cursor-grabbing"
                title="Drag to reorder attributes"
                @dragstart="startKeyDrag($event, facet.key)"
                @dragend="finishKeyDrag"
              >
                <GripVertical :size="13" />
              </span>
              <button
                v-if="canManage"
                type="button"
                class="h-2.5 w-2.5 shrink-0 rounded-full border border-slate-300 dark:border-slate-600"
                :style="{ backgroundColor: keyColor(facet) }"
                title="Attribute color"
                @click="openColor($event, facet.key)"
              />
              <input
                v-if="editingKey === facet.key"
                v-model="draft"
                data-attribute-edit
                type="text"
                class="min-w-0 flex-1 border-b border-brand-400 bg-transparent p-0 text-sm font-semibold text-slate-800 outline-none dark:text-slate-100"
                @keydown.enter.prevent="commitEdit"
                @keydown.esc.prevent="cancelEdit"
                @blur="commitEdit"
              />
              <span
                v-else
                class="min-w-0 flex-1 truncate text-sm font-semibold text-slate-700 dark:text-slate-200"
                :class="canManage ? 'cursor-text' : ''"
                :title="canManage ? 'Double-click to rename' : undefined"
                @dblclick="startEditKey(facet)"
              >
                {{ facet.label || facet.key }}
              </span>
              <button
                v-if="canManage"
                type="button"
                class="inline-flex shrink-0 text-slate-400 transition hover:text-brand-700 dark:hover:text-brand-400"
                title="Edit key description"
                @click="openDescription($event, facet.key)"
              >
                <Pencil :size="11" />
              </button>
            </div>
            <p class="truncate text-[11px] text-slate-400">
              <span class="font-mono">{{ facet.key }}</span>
              · {{ facet.values.length }} values
            </p>
            <p
              v-if="facet.description"
              class="truncate text-[11px] text-slate-400"
              v-tooltip="{ value: facet.description || '' }"
            >
              {{ facet.description }}
            </p>
          </div>
          <div v-if="canManage" class="flex shrink-0 items-start gap-2">
            <div>
              <p
                class="mb-1 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
              >
                Values
              </p>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  class="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-200 text-slate-500 transition hover:bg-slate-100 hover:text-brand-700 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-brand-400"
                  :title="
                    facet.values_locked
                      ? 'New values locked'
                      : 'New values allowed'
                  "
                  @click="changeValuesLock(facet, !facet.values_locked)"
                >
                  <Lock v-if="facet.values_locked" :size="13" />
                  <LockOpen v-else :size="13" />
                </button>
                <button
                  type="button"
                  class="inline-flex h-8 w-8 items-center justify-center rounded-md border transition"
                  :class="
                    facet.value_search
                      ? 'border-brand-300 bg-brand-50 text-brand-700 dark:border-brand-800 dark:bg-brand-950/50 dark:text-brand-300'
                      : 'border-slate-200 text-slate-500 hover:bg-slate-100 hover:text-brand-700 dark:border-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-brand-400'
                  "
                  :title="
                    facet.value_search
                      ? 'Hide value search bar'
                      : 'Show value search bar'
                  "
                  @click="changeValueSearch(facet, !facet.value_search)"
                >
                  <Search :size="13" />
                </button>
              </div>
            </div>
            <div>
              <p
                class="mb-1 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
              >
                Render
              </p>
              <Select
                :model-value="facet.render"
                :options="RENDER_OPTIONS"
                option-label="label"
                option-value="value"
                size="small"
                class="w-44"
                @update:model-value="changeRender(facet, $event)"
              />
            </div>
            <div>
              <p
                class="mb-1 text-[10px] font-semibold tracking-wide text-slate-400 uppercase"
              >
                Sort
              </p>
              <Select
                :model-value="facet.sort"
                :options="SORT_OPTIONS"
                option-label="label"
                option-value="value"
                size="small"
                class="w-44"
                @update:model-value="changeSort(facet, $event)"
              />
            </div>
          </div>
        </div>

        <div
          v-if="canManage"
          class="flex flex-wrap items-center gap-4 border-b border-slate-100 px-3 py-2 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-300"
        >
          <label class="inline-flex items-center gap-2">
            <ToggleSwitch
              :model-value="facet.searchable"
              @update:model-value="changeSearchable(facet, $event)"
            />
            Search index
          </label>
          <label class="inline-flex items-center gap-2">
            <span>Parent</span>
            <Select
              :model-value="facet.parent_key ?? ''"
              :options="parentOptions(facet)"
              option-label="label"
              option-value="value"
              size="small"
              class="w-36"
              @update:model-value="changeParent(facet, $event)"
            />
          </label>
          <label class="inline-flex items-center gap-2">
            <span>Selection</span>
            <Select
              :model-value="facet.multi ? 'multi' : 'single'"
              :options="MULTI_OPTIONS"
              option-label="label"
              option-value="value"
              size="small"
              class="w-32"
              @update:model-value="changeMulti(facet, $event)"
            />
          </label>
          <button
            type="button"
            class="ml-auto inline-flex items-center gap-1 font-medium text-red-600 transition hover:text-red-700 dark:text-red-400"
            @click="confirmDeleteKey(facet)"
          >
            <Trash :size="13" />
            Delete attribute
          </button>
        </div>

        <div
          class="divide-y divide-slate-100 dark:divide-slate-800"
          @dragover.prevent="onValueDragOver($event, facet)"
          @drop.prevent="finishValueDrag"
        >
          <div
            v-for="value in orderedValues(facet)"
            :key="value.value"
            :data-value-row="value.value"
            class="flex min-w-0 items-center gap-2 px-3 py-1.5"
            :class="{
              'opacity-60':
                draggingValue?.key === facet.key &&
                draggingValue?.value === value.value,
            }"
          >
            <span
              v-if="
                canManage &&
                facet.sort === 'custom'
              "
              draggable="true"
              class="inline-flex shrink-0 cursor-grab text-slate-400 active:cursor-grabbing"
              title="Drag to reorder values"
              @dragstart="startValueDrag($event, facet.key, value.value)"
              @dragend="finishValueDrag"
            >
              <GripVertical :size="12" />
            </span>
            <button
              v-if="canManage"
              type="button"
              class="h-2.5 w-2.5 shrink-0 rounded-full border border-slate-300 dark:border-slate-600"
              :style="{ backgroundColor: valueColor(facet, value) }"
              title="Value color"
              @click="openColor($event, facet.key, value.value)"
            />
            <input
              v-if="
                editingValue?.key === facet.key &&
                editingValue?.value === value.value
              "
              v-model="draft"
              data-attribute-edit
              type="text"
              class="min-w-0 flex-1 border-b border-brand-400 bg-transparent p-0 text-sm text-slate-800 outline-none dark:text-slate-100"
              @keydown.enter.prevent="commitEdit"
              @keydown.esc.prevent="cancelEdit"
              @blur="commitEdit"
            />
            <span
              v-else
              class="min-w-0 flex-1 truncate text-sm text-slate-600 dark:text-slate-300"
              :class="canManage ? 'cursor-text' : ''"
              :title="canManage ? 'Double-click to rename' : undefined"
              @dblclick="startEditValue(facet, value)"
            >
              {{ value.label || value.value }}
            </span>
            <span
              v-if="
                value.label &&
                value.label !== value.value &&
                value.value !== ATTRIBUTE_UNASSIGNED
              "
              class="max-w-[14rem] shrink-0 truncate font-mono text-[11px] text-slate-400"
              :title="value.value"
            >
              {{ value.value }}
            </span>
            <span class="shrink-0 text-xs text-slate-400 tabular-nums">
              {{ value.count }}
            </span>
            <button
              v-if="canManage"
              type="button"
              class="inline-flex shrink-0 text-slate-400 transition hover:text-brand-700 dark:hover:text-brand-400"
              title="Edit value description"
              @click="openDescription($event, facet.key, value.value)"
            >
              <Pencil :size="11" />
            </button>
            <button
              v-if="canManage && value.value === ATTRIBUTE_UNASSIGNED"
              type="button"
              class="inline-flex shrink-0 text-slate-400 transition hover:text-brand-700 dark:hover:text-brand-400"
              :title="
                facet.include_unassigned
                  ? 'Hide unassigned option'
                  : 'Show unassigned option'
              "
              @click="changeUnassigned(facet, !facet.include_unassigned)"
            >
              <Eye v-if="facet.include_unassigned" :size="13" />
              <EyeSlash v-else :size="13" />
            </button>
            <button
              v-else-if="canManage"
              type="button"
              class="inline-flex shrink-0 text-slate-400 transition hover:text-red-600 dark:hover:text-red-400"
              title="Delete value"
              @click="confirmDeleteValue(facet, value)"
            >
              <Trash :size="12" />
            </button>
          </div>

          <div v-if="!facet.values_locked" class="px-3 py-1.5">
            <input
              v-if="addingKey === facet.key"
              v-model="addDraft"
              data-attribute-add
              type="text"
              placeholder="Add values (use , or ; for multiple)"
              class="w-full border-b border-brand-400 bg-transparent p-0 text-sm text-slate-700 outline-none placeholder:text-slate-400 dark:text-slate-200"
              :disabled="adding"
              @keydown.enter.prevent="commitAddValues"
              @keydown.esc.prevent="cancelAddValues"
              @blur="commitAddValues"
            />
            <button
              v-else-if="canManage"
              type="button"
              class="inline-flex items-center gap-1 text-xs font-medium text-slate-400 transition hover:text-brand-700 dark:hover:text-brand-400"
              @click="startAddValues(facet)"
            >
              <Plus :size="12" />
              Add values
            </button>
          </div>
          </div>
        </section>
      </template>
    </div>

    <Popover
      ref="descriptionPopover"
      :pt="{ content: { class: '!p-3' } }"
    >
      <div class="w-72 space-y-2">
        <p class="text-xs font-semibold text-slate-600 dark:text-slate-300">
          {{ descriptionTargetLabel }}
        </p>
        <textarea
          v-model="descriptionDraft"
          rows="3"
          placeholder="Description…"
          class="w-full resize-y rounded border border-slate-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900"
          @keydown.esc.prevent="descriptionPopover?.hide()"
        />
        <div class="flex justify-end gap-2">
          <button
            type="button"
            class="rounded px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
            @click="descriptionPopover?.hide()"
          >
            Cancel
          </button>
          <button
            type="button"
            class="rounded bg-brand-600 px-2 py-1 text-xs font-medium text-white hover:bg-brand-700"
            @click="saveDescription"
          >
            Save
          </button>
        </div>
      </div>
    </Popover>

    <Popover ref="colorPopover" :pt="{ content: { class: '!p-3' } }">
      <div class="w-56 space-y-2">
        <p class="text-xs font-semibold text-slate-600 dark:text-slate-300">
          Color
        </p>
        <div class="grid grid-cols-5 gap-2">
          <button
            v-for="color in LIBRARY_COLORS"
            :key="color.value"
            type="button"
            class="h-7 w-7 rounded-full ring-1 ring-slate-300 transition hover:scale-110 dark:ring-slate-600"
            :style="{ backgroundColor: `#${color.value}` }"
            :title="color.name"
            @click="chooseColor(color.value)"
          />
        </div>
        <button
          type="button"
          class="w-full rounded border border-slate-200 px-2 py-1 text-xs text-slate-500 hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
          @click="chooseColor('')"
        >
          {{ colorTarget?.value ? 'Inherit key color' : 'No color' }}
        </button>
      </div>
    </Popover>
  </Dialog>
</template>
