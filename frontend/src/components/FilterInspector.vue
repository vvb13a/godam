<script setup lang="ts">
import Message from 'primevue/message'
import { Filter, Folder, Tag as TagIcon } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { formatDateTime, formatRelative, formatSize } from '../lib/format'
import { library } from '../lib/library'
import { LIBRARY_COLORS, libraryColorHex } from '../lib/libraryColors'
import type { Asset, Collection, SmartCollection, Tag } from '../types'
import AssetPreviewGrid from './AssetPreviewGrid.vue'

const props = defineProps<{
  collection?: Collection
  smartCollection?: SmartCollection
  tag?: Tag
  assets: Asset[]
  size: number
  total?: number
  totalCapped?: boolean
}>()

const emit = defineEmits<{ changed: [] }>()

const busy = ref(false)
const error = ref<string | null>(null)
const name = ref('')
const description = ref('')

const count = computed(() => props.total ?? props.assets.length)
const countLabel = computed(
  () => `${count.value}${props.totalCapped ? '+' : ''}`,
)

function tagColor(tag?: Tag): string {
  if (!tag?.group_id) return '#94a3b8'
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : '#94a3b8'
}

// Exactly one subject is passed. Normalising to one shape keeps the template
// free of branching on the subject type.
const subject = computed(() => {
  if (props.smartCollection) {
    const smart = props.smartCollection
    return {
      kind: 'Smart collection',
      id: smart.id,
      name: smart.name,
      description: smart.description ?? '',
      color: libraryColorHex(smart.color),
      createdAt: smart.created_at,
      updatedAt: smart.updated_at,
      icon: Filter,
    }
  }
  if (props.collection) {
    return {
      kind: 'Collection',
      id: props.collection.id,
      name: props.collection.name,
      description: props.collection.description ?? '',
      color: libraryColorHex(props.collection.color),
      createdAt: props.collection.created_at,
      updatedAt: props.collection.updated_at,
      icon: Folder,
    }
  }
  const tag = props.tag
  return {
    kind: 'Tag',
    id: tag?.id ?? '',
    name: tag?.name ?? '',
    description: tag?.description ?? '',
    color: tagColor(tag),
    createdAt: tag?.created_at ?? '',
    updatedAt: tag?.updated_at ?? '',
    icon: TagIcon,
  }
})

// Collections and smart collections carry a user-editable colour; tags derive
// theirs from their group.
const canColor = computed(
  () => Boolean(props.collection) || Boolean(props.smartCollection),
)

const currentColor = computed(() => subject.value.color)

// Reset the form when the subject changes (a different collection/tag is
// selected, or the stored values change after a save + library refresh).
watch(
  () => `${subject.value.id}\u0000${subject.value.name}\u0000${subject.value.description}`,
  () => {
    name.value = subject.value.name
    description.value = subject.value.description
  },
  { immediate: true },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// Persist the current form values with an optional colour override, so the
// swatches and the text fields save through one path.
async function persist(color?: string): Promise<void> {
  const nextName = name.value.trim()
  const nextDescription = description.value.trim()
  if (!nextName) {
    error.value = 'Name is required'
    name.value = subject.value.name
    return
  }
  if (
    color === undefined &&
    nextName === subject.value.name &&
    nextDescription === subject.value.description
  ) {
    return
  }
  busy.value = true
  error.value = null
  try {
    if (props.collection) {
      await api.updateCollection(
        props.collection.id,
        nextName,
        color ?? props.collection.color,
        nextDescription,
      )
    } else if (props.smartCollection) {
      await api.updateSmartCollection(
        props.smartCollection.id,
        nextName,
        color ?? props.smartCollection.color,
        nextDescription,
      )
    } else if (props.tag) {
      await api.updateTag(
        props.tag.id,
        nextName,
        props.tag.group_id ?? null,
        props.tag.starred,
        nextDescription,
      )
    }
    emit('changed')
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}

function save(): void {
  void persist()
}

function setColor(color: string): void {
  if (libraryColorHex(color) === currentColor.value) return
  void persist(color)
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="min-h-0 flex-1 space-y-6 overflow-y-auto p-3">
      <div class="flex items-center gap-2">
        <component
          :is="subject.icon"
          :size="16"
          :style="{ color: subject.color }"
        />
        <h3 class="text-sm font-semibold text-slate-600 dark:text-slate-300">{{ subject.kind }}</h3>
      </div>

      <AssetPreviewGrid :assets="assets" />

      <section class="space-y-3">
        <div>
          <label
            class="mb-1 block text-xs font-medium text-slate-400"
            for="filter-inspector-name"
          >
            Name
          </label>
          <input
            id="filter-inspector-name"
            v-model="name"
            class="w-full rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm"
            @blur="save"
            @keydown.enter.prevent="save"
          />
        </div>
        <div>
          <label
            class="mb-1 block text-xs font-medium text-slate-400"
            for="filter-inspector-description"
          >
            Description
          </label>
          <textarea
            id="filter-inspector-description"
            v-model="description"
            rows="3"
            placeholder="Add a description…"
            class="w-full resize-y rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm"
            @blur="save"
          />
        </div>
        <div v-if="canColor">
          <p class="mb-1.5 text-xs font-medium text-slate-400">Color</p>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="option in LIBRARY_COLORS"
              :key="option.value"
              type="button"
              class="h-5 w-5 rounded-full border-2"
              :title="option.name"
              :style="{
                backgroundColor: libraryColorHex(option.value),
                borderColor:
                  libraryColorHex(option.value) === currentColor
                    ? '#0f172a'
                    : 'transparent',
              }"
              :disabled="busy"
              @click="setColor(option.value)"
            />
          </div>
        </div>
        <p v-if="busy" class="text-xs text-slate-400">Saving…</p>
      </section>

      <section>
        <h3 class="mb-2 text-sm font-semibold">Properties</h3>
        <dl class="space-y-1.5 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Assets</dt>
            <dd class="text-right">{{ countLabel }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Size</dt>
            <dd class="text-right">{{ formatSize(size) }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Created</dt>
            <dd class="text-right" v-tooltip="formatDateTime(subject.createdAt)">
              {{ formatRelative(subject.createdAt) }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Modified</dt>
            <dd class="text-right" v-tooltip="formatDateTime(subject.updatedAt)">
              {{ formatRelative(subject.updatedAt) }}
            </dd>
          </div>
        </dl>
      </section>

      <Message v-if="error" severity="error">{{ error }}</Message>
    </div>
  </div>
</template>
