<script setup lang="ts">
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import { ChevronDown, Search } from '@primeicons/vue'
import { computed, ref } from 'vue'
import { library } from '../lib/library'
import TagBadge from './TagBadge.vue'
import type { Tag } from '../types'

type GroupMode = 'none' | 'alphabet' | 'groups'
type SortMode = 'name-asc' | 'name-desc' | 'count-asc' | 'count-desc'

const GROUP_STORAGE_KEY = 'godam:tags:group'
const SORT_STORAGE_KEY = 'godam:tags:sort'
const GROUP_MODES: GroupMode[] = ['none', 'alphabet', 'groups']
const SORT_MODES: SortMode[] = [
  'name-asc',
  'name-desc',
  'count-asc',
  'count-desc',
]

const props = withDefaults(
  defineProps<{
    modelValue: string[]
    disabled?: boolean
    lockedTitle?: string
  }>(),
  {
    disabled: false,
    lockedTitle: 'This filter is locked by the host application',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const popover = ref<InstanceType<typeof Popover> | null>(null)
const search = ref('')
const draft = ref<string[]>([])
const groupBy = ref<GroupMode>('none')
const sort = ref<SortMode>('name-asc')

interface TagSection {
  key: string
  label: string
  tags: Tag[]
}

function loadSettings(): void {
  const group = localStorage.getItem(GROUP_STORAGE_KEY)
  groupBy.value = GROUP_MODES.includes(group as GroupMode)
    ? (group as GroupMode)
    : 'none'
  const savedSort = localStorage.getItem(SORT_STORAGE_KEY)
  sort.value = SORT_MODES.includes(savedSort as SortMode)
    ? (savedSort as SortMode)
    : 'name-asc'
}

function tagColor(tag: Tag): string | undefined {
  if (!tag.group_id) return undefined
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : undefined
}

const filteredTags = computed(() => {
  const query = search.value.trim().toLowerCase()
  const list = query
    ? library.tags.filter((tag) => tag.name.toLowerCase().includes(query))
    : [...library.tags]
  const direction = sort.value.endsWith('-asc') ? 1 : -1
  if (sort.value.startsWith('name')) {
    list.sort((a, b) => a.name.localeCompare(b.name) * direction)
  } else {
    list.sort(
      (a, b) =>
        (a.asset_count - b.asset_count || a.name.localeCompare(b.name)) *
        direction,
    )
  }
  return list
})

const sections = computed<TagSection[]>(() => {
  const tags = filteredTags.value
  if (groupBy.value === 'alphabet') {
    const buckets = new Map<string, Tag[]>()
    for (const tag of tags) {
      const first = tag.name[0] ?? ''
      const label = /[a-z]/i.test(first) ? first.toUpperCase() : '#'
      const bucket = buckets.get(label)
      if (bucket) bucket.push(tag)
      else buckets.set(label, [tag])
    }
    return [...buckets.entries()].map(([label, list]) => ({
      key: label,
      label,
      tags: list,
    }))
  }
  if (groupBy.value === 'groups') {
    const result: TagSection[] = []
    const starred = tags.filter((tag) => tag.starred)
    if (starred.length) {
      result.push({ key: 'starred', label: 'Starred', tags: starred })
    }
    for (const group of library.tagGroups) {
      const groupTags = tags.filter((tag) => tag.group_id === group.id)
      if (groupTags.length) {
        result.push({
          key: `group:${group.id}`,
          label: group.name,
          tags: groupTags,
        })
      }
    }
    const uncategorized = tags.filter((tag) => !tag.group_id)
    if (uncategorized.length) {
      result.push({
        key: 'uncategorized',
        label: 'Uncategorized',
        tags: uncategorized,
      })
    }
    return result
  }
  return [{ key: 'all', label: '', tags }]
})

const selectedTags = computed(() => {
  const ids = new Set(props.modelValue)
  return library.tags.filter((tag) => ids.has(tag.id))
})

const selectedUntagged = computed(() => props.modelValue.includes('untagged'))
const visibleTags = computed(() =>
  selectedUntagged.value
    ? selectedTags.value.slice(0, 1)
    : selectedTags.value.slice(0, 2),
)
const overflowCount = computed(
  () =>
    selectedTags.value.length +
    (selectedUntagged.value ? 1 : 0) -
    visibleTags.value.length -
    (selectedUntagged.value ? 1 : 0),
)

function toggle(event: Event): void {
  if (props.disabled) return
  loadSettings()
  draft.value = [...props.modelValue]
  search.value = ''
  popover.value?.toggle(event)
}

function apply(): void {
  if (props.modelValue.join(',') !== draft.value.join(',')) {
    emit('update:modelValue', [...draft.value])
  }
  popover.value?.hide()
}

function clear(): void {
  draft.value = []
}

function toggleSpecial(id: string): void {
  if (draft.value.includes(id)) {
    draft.value = draft.value.filter((item) => item !== id)
  } else {
    draft.value = [...draft.value, id]
  }
}

function toggleTag(tag: Tag): void {
  toggleSpecial(tag.id)
}
</script>

<template>
  <button
    type="button"
    class="flex h-9 w-56 items-center gap-2 rounded-md border border-slate-300 bg-white px-2 text-left text-sm text-slate-700 shadow-sm transition hover:border-brand-300 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
    :disabled="disabled"
    :title="disabled ? lockedTitle : 'Filter by tag'"
    @click="toggle"
  >
    <span class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
      <template v-if="selectedUntagged || selectedTags.length">
        <TagBadge
          v-if="selectedUntagged"
          name="Untagged"
          class="max-w-[7rem]"
        />
        <TagBadge
          v-for="tag in visibleTags"
          :key="tag.id"
          :name="tag.name"
          :color="tagColor(tag)"
          :starred="tag.starred"
          class="max-w-[7rem]"
        />
        <span v-if="overflowCount > 0" class="shrink-0 text-xs text-slate-400">
          +{{ overflowCount }}
        </span>
      </template>
      <span v-else class="truncate text-slate-400">All tags</span>
    </span>
    <ChevronDown :size="14" class="shrink-0 text-slate-400" />
  </button>

  <Popover ref="popover">
    <div class="w-80">
      <div
        class="flex items-center gap-2 border-b border-slate-100 pb-2 dark:border-slate-800"
      >
        <Search :size="15" class="shrink-0 text-slate-400" />
        <input
          v-model="search"
          type="text"
          placeholder="Search tags…"
          class="min-w-0 flex-1 bg-transparent text-sm outline-none"
        />
      </div>

      <div class="mt-2 max-h-72 overflow-y-auto">
        <div class="mt-1 flex flex-wrap items-start gap-1.5">
          <fieldset
            v-for="section in sections"
            :key="section.key"
            class="max-w-full min-w-0 shrink-0 rounded-md pb-1.5"
            :class="
              section.label
                ? 'border border-slate-200 bg-slate-50 px-1.5 dark:border-slate-800 dark:bg-slate-950'
                : 'w-full'
            "
          >
            <legend
              v-if="section.label"
              class="px-1 text-[0.625rem] font-normal text-slate-400"
            >
              {{ section.label }}
            </legend>
            <div class="flex flex-wrap items-center gap-1">
              <TagBadge
                v-for="tag in section.tags"
                :key="tag.id"
                :name="tag.name"
                :color="tagColor(tag)"
                :starred="tag.starred"
                :selected="draft.includes(tag.id)"
                class="cursor-pointer select-none"
                @click="toggleTag(tag)"
              >
                <span v-if="tag.asset_count > 0" class="shrink-0 text-[10px] opacity-60">
                  {{ tag.asset_count }}
                </span>
              </TagBadge>
            </div>
          </fieldset>
        </div>
      </div>

      <div
        class="mt-2 flex items-center justify-between border-t border-slate-100 pt-2 dark:border-slate-800"
      >
        <span class="text-xs text-slate-400">
          {{ draft.length }} selected
        </span>
        <div class="flex items-center gap-2">
          <Button label="Clear" text size="small" @click="clear" />
          <Button label="Done" size="small" @click="apply" />
        </div>
      </div>
    </div>
  </Popover>
</template>
