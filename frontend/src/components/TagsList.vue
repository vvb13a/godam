<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import Popover from 'primevue/popover'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  Bookmark,
  Check,
  Cog,
  Folder,
  FolderOpen,
  Pencil,
  Plus,
  Search,
  Star,
  StarFill,
  Tags,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { assetDrag } from '../lib/assetDrag'
import { library, notifyLibraryChanged } from '../lib/library'
import { confirmDeleteMenuItem } from '../lib/menu'
import { openTagsDialog } from '../lib/tagsDialog'
import type { Tag } from '../types'
import TagBadge from './TagBadge.vue'

interface TagMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

type GroupBy = 'none' | 'alphabet' | 'groups'
type SortMode = 'name-asc' | 'name-desc' | 'count-asc' | 'count-desc'

const GROUP_STORAGE_KEY = 'godam:tags:group'
const SORT_STORAGE_KEY = 'godam:tags:sort'
const GROUP_MODES: GroupBy[] = ['none', 'alphabet', 'groups']
const SORT_MODES: SortMode[] = ['name-asc', 'name-desc', 'count-asc', 'count-desc']

const GROUP_OPTIONS: { value: GroupBy; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'alphabet', label: 'Alphabet' },
  { value: 'groups', label: 'Groups' },
]

const SORT_OPTIONS: { value: SortMode; label: string }[] = [
  { value: 'name-asc', label: 'Name A–Z' },
  { value: 'name-desc', label: 'Name Z–A' },
  { value: 'count-desc', label: 'Count high–low' },
  { value: 'count-asc', label: 'Count low–high' },
]

function loadGroup(): GroupBy {
  const raw = localStorage.getItem(GROUP_STORAGE_KEY)
  return GROUP_MODES.includes(raw as GroupBy) ? (raw as GroupBy) : 'none'
}

function loadSort(): SortMode {
  const raw = localStorage.getItem(SORT_STORAGE_KEY)
  return SORT_MODES.includes(raw as SortMode) ? (raw as SortMode) : 'name-asc'
}

const route = useRoute()
const router = useRouter()
const toast = useToast()

const root = ref<HTMLElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const optionsPopover = ref<InstanceType<typeof Popover> | null>(null)
const groupBy = ref<GroupBy>(loadGroup())
const sort = ref<SortMode>(loadSort())

const contextTag = ref<Tag | null>(null)
const editingTagId = ref<string | null>(null)
const editingTagName = ref('')
const draftTagId = ref<string | null>(null)
const dragOverTagId = ref<string | null>(null)

watch(groupBy, (value) => {
  try {
    localStorage.setItem(GROUP_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the choice simply won't persist.
  }
})

watch(sort, (value) => {
  try {
    localStorage.setItem(SORT_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the choice simply won't persist.
  }
})

watch(
  () => assetDrag.active,
  (active) => {
    if (!active) dragOverTagId.value = null
  },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function toggleOptions(event: Event): void {
  optionsPopover.value?.toggle(event)
}

function setGroup(value: GroupBy): void {
  groupBy.value = value
}

function setSort(value: SortMode): void {
  sort.value = value
}

// A tag inherits the colour of its group; ungrouped tags fall back to grey.
function tagColor(tag: Tag): string | undefined {
  if (!tag.group_id) return undefined
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : undefined
}

interface TagSection {
  key: string
  label: string
  tags: Tag[]
}

const orderedTags = computed(() => {
  const list = [...library.tags]
  switch (sort.value) {
    case 'name-desc':
      list.sort((a, b) => b.name.localeCompare(a.name))
      break
    case 'count-asc':
      list.sort(
        (a, b) => a.asset_count - b.asset_count || a.name.localeCompare(b.name),
      )
      break
    case 'count-desc':
      list.sort(
        (a, b) => b.asset_count - a.asset_count || a.name.localeCompare(b.name),
      )
      break
    default:
      list.sort((a, b) => a.name.localeCompare(b.name))
  }
  return list
})

// Sections mirror the Manage Tags dialog's right-side list grouping.
const sections = computed<TagSection[]>(() => {
  const tags = orderedTags.value
  if (groupBy.value === 'alphabet') {
    const buckets = new Map<string, Tag[]>()
    for (const tag of tags) {
      const first = tag.name[0] ?? ''
      const letter = /[a-z]/i.test(first) ? first.toUpperCase() : '#'
      const bucket = buckets.get(letter)
      if (bucket) bucket.push(tag)
      else buckets.set(letter, [tag])
    }
    return [...buckets.entries()].map(([letter, list]) => ({
      key: letter,
      label: letter,
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
      const groupTags = tags.filter(
        (tag) => tag.group_id === group.id && !tag.starred,
      )
      if (groupTags.length) {
        result.push({ key: `group:${group.id}`, label: group.name, tags: groupTags })
      }
    }
    const uncategorized = tags.filter((tag) => !tag.group_id && !tag.starred)
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

const activeTagIds = computed(() => {
  const raw = route.query.tag
  if (typeof raw !== 'string' || !raw) return new Set<string>()
  return new Set(raw.split(',').filter(Boolean))
})

function isActive(tag: Tag): boolean {
  return activeTagIds.value.has(tag.id)
}

function filterByTag(tag: Tag): void {
  void router.push({ name: 'all', query: { tag: tag.id } })
}

function recomputeGroupCounts(): void {
  const counts = new Map<string, number>()
  for (const tag of library.tags) {
    if (tag.group_id) {
      counts.set(tag.group_id, (counts.get(tag.group_id) ?? 0) + 1)
    }
  }
  for (const group of library.tagGroups) {
    group.tag_count = counts.get(group.id) ?? 0
  }
}

function upsertTag(tag: Tag): void {
  const index = library.tags.findIndex((item) => item.id === tag.id)
  if (index >= 0) library.tags[index] = tag
  else library.tags.push(tag)
  recomputeGroupCounts()
}

function removeTagLocal(id: string): void {
  library.tags = library.tags.filter((tag) => tag.id !== id)
  recomputeGroupCounts()
}

function uniqueUntitledTagName(): string {
  const names = new Set(library.tags.map((tag) => tag.name))
  if (!names.has('untitled')) return 'untitled'
  let index = 2
  while (names.has(`untitled ${index}`)) index++
  return `untitled ${index}`
}

// createTag adds a draft tag then opens its inline editor. If the name is left
// unchanged the draft is rolled back, so nothing is persisted.
async function createTag(): Promise<void> {
  const libraryId = library.currentId
  if (!libraryId) return
  try {
    const tag = await api.createTag(libraryId, uniqueUntitledTagName())
    upsertTag(tag)
    draftTagId.value = tag.id
    startRename(tag)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create tag',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startRename(tag: Tag): void {
  editingTagId.value = tag.id
  editingTagName.value = tag.name
  nextTick(() => {
    const el = root.value?.querySelector<HTMLInputElement>(
      `[data-tag-rename="${tag.id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

function cancelRename(): void {
  const id = editingTagId.value
  editingTagId.value = null
  if (id && id === draftTagId.value) {
    draftTagId.value = null
    void api.deleteTag(id).then(() => removeTagLocal(id))
  }
}

async function commitRename(): Promise<void> {
  const id = editingTagId.value
  if (!id) return
  editingTagId.value = null
  const tag = library.tags.find((item) => item.id === id)
  const name = editingTagName.value.trim()
  if (!tag) return
  if (id === draftTagId.value) {
    draftTagId.value = null
    if (!name || name === tag.name) {
      try {
        await api.deleteTag(id)
        removeTagLocal(id)
      } catch (e) {
        toast.add({
          severity: 'error',
          summary: 'Delete failed',
          detail: toMessage(e),
          life: 4000,
        })
      }
      return
    }
  }
  if (!name || name === tag.name) return
  try {
    upsertTag(
      await api.updateTag(id, name, tag.group_id ?? null, tag.starred),
    )
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function toggleStar(tag: Tag): Promise<void> {
  try {
    upsertTag(
      await api.updateTag(tag.id, tag.name, tag.group_id ?? null, !tag.starred),
    )
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function assignGroup(tag: Tag, groupId: string | null): Promise<void> {
  try {
    upsertTag(await api.updateTag(tag.id, tag.name, groupId, tag.starred))
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function deleteTag(id: string): Promise<void> {
  try {
    await api.deleteTag(id)
    removeTagLocal(id)
    toast.add({ severity: 'success', summary: 'Tag deleted', life: 2000 })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Delete failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function openMenu(event: MouseEvent, tag: Tag): void {
  contextTag.value = tag
  menu.value?.show(event)
}

function tagFromTarget(target: EventTarget | null): Tag | null {
  const el =
    target instanceof Element
      ? target.closest<HTMLElement>('[data-tag-id]')
      : null
  const id = el?.getAttribute('data-tag-id')
  if (!id) return null
  return library.tags.find((tag) => tag.id === id) ?? null
}

// Dropping assets onto a tag adds that tag to every dragged asset (additive).
function onAssetDragOver(event: DragEvent): void {
  if (!assetDrag.active) return
  const tag = tagFromTarget(event.target)
  if (!tag) return
  event.preventDefault()
  event.stopPropagation()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dragOverTagId.value = tag.id
}

async function onAssetDrop(event: DragEvent): Promise<void> {
  if (!assetDrag.active) return
  const tag = tagFromTarget(event.target)
  if (!tag) return
  event.preventDefault()
  event.stopPropagation()
  dragOverTagId.value = null
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
  const libraryId = library.currentId
  if (!ids.length || !libraryId) return
  try {
    await api.applyAssetsTags(libraryId, ids, [tag.id], 'add')
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary: ids.length > 1 ? `Tagged ${ids.length} assets` : 'Tag added',
      detail: tag.name,
      life: 2000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not add tag',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

const menuItems = computed<TagMenuItem[]>(() => {
  const tag = contextTag.value
  if (!tag) return []
  const items: TagMenuItem[] = [
    {
      label: tag.starred ? 'Unstar' : 'Star',
      menuIcon: tag.starred ? StarFill : Star,
      command: () => void toggleStar(tag),
    },
    {
      label: 'Rename',
      menuIcon: Pencil,
      command: () => startRename(tag),
    },
    {
      label: 'Filter',
      menuIcon: Search,
      command: () => filterByTag(tag),
    },
  ]
  if (library.tagGroups.length) {
    items.push({
      label: 'Move',
      menuIcon: Folder,
      items: library.tagGroups.map((group) => ({
        label: group.name,
        menuIcon: Bookmark,
        color: group.color ? `#${group.color}` : undefined,
        disabled: tag.group_id === group.id,
        command: () => void assignGroup(tag, group.id),
      })),
    })
  }
  if (tag.group_id) {
    items.push({
      label: 'Ungroup',
      menuIcon: FolderOpen,
      command: () => void assignGroup(tag, null),
    })
  }
  items.push(
    { separator: true },
    {
      label: 'Create',
      menuIcon: Plus,
      command: () => void createTag(),
    },
    confirmDeleteMenuItem('Delete', () => void deleteTag(tag.id)),
  )
  return items
})
</script>

<template>
  <div
    ref="root"
    @dragover.capture="onAssetDragOver"
    @drop.capture="onAssetDrop"
  >
    <div
      class="sticky top-0 z-10 flex items-center justify-between bg-white pt-3 pb-0 pr-3 pl-[1.375rem]"
    >
      <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
        Tags
        <template v-if="library.tags.length">
          ({{ library.tags.length }})
        </template>
      </span>
      <div class="flex items-center gap-2">
        <button
          class="inline-flex text-slate-400 hover:text-violet-600"
          title="Manage tags"
          @click="openTagsDialog"
        >
          <Tags :size="14" />
        </button>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600"
          title="Group and sort tags"
          @click="toggleOptions"
        >
          <Cog :size="14" />
        </button>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600"
          title="New tag"
          @click="createTag"
        >
          <Plus :size="14" />
        </button>
      </div>
    </div>

    <Popover ref="optionsPopover">
      <div class="w-56">
        <p
          class="px-2 pt-1 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase"
        >
          Group by
        </p>
        <button
          v-for="option in GROUP_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            groupBy === option.value
              ? 'bg-violet-50 text-violet-700'
              : 'text-slate-600 hover:bg-slate-100'
          "
          @click="setGroup(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="groupBy === option.value" :size="12" class="shrink-0" />
        </button>

        <div class="my-1 border-t border-slate-100" />

        <p
          class="px-2 pt-1 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase"
        >
          Sort by
        </p>
        <button
          v-for="option in SORT_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            sort === option.value
              ? 'bg-violet-50 text-violet-700'
              : 'text-slate-600 hover:bg-slate-100'
          "
          @click="setSort(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="sort === option.value" :size="12" class="shrink-0" />
        </button>
      </div>
    </Popover>

    <div class="flex flex-wrap items-start gap-1 px-[0.875rem] py-1">
      <p
        v-if="!library.tags.length"
        class="w-full px-2 py-2 text-xs text-slate-400"
      >
        No tags
      </p>
      <fieldset
        v-for="section in sections"
        :key="section.key"
        class="max-w-full min-w-0 shrink-0 rounded-md pb-1.5"
        :class="
          section.label
            ? 'border border-slate-200 bg-slate-50 px-1.5'
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
          <template v-for="tag in section.tags" :key="tag.id">
            <input
              v-if="editingTagId === tag.id"
              v-model="editingTagName"
              :data-tag-rename="tag.id"
              type="text"
              class="w-24 rounded-full border border-violet-400 bg-white px-2 py-0.5 text-xs text-slate-800 outline-none"
              @click.stop
              @keydown.stop
              @keydown.enter="commitRename"
              @keydown.esc="cancelRename"
              @blur="commitRename"
            />
          <TagBadge
            v-else
            :name="tag.name"
            :color="tagColor(tag)"
            :starred="tag.starred"
            :data-tag-id="tag.id"
            class="cursor-pointer select-none"
            :class="[
              isActive(tag) ? 'ring-1 ring-violet-400' : '',
              dragOverTagId === tag.id ? 'ring-2 ring-violet-500' : '',
            ]"
            @click="filterByTag(tag)"
            @contextmenu.prevent="openMenu($event, tag)"
          >
              <span v-if="tag.asset_count > 0" class="shrink-0 text-slate-400">
                {{ tag.asset_count }}
              </span>
            </TagBadge>
          </template>
        </div>
      </fieldset>
    </div>

    <ContextMenu ref="menu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700'"
          @click.prevent
        >
          <span
            v-if="slotProps.item.colorOption"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300"
            :style="{ backgroundColor: slotProps.item.color ?? '#cbd5e1' }"
          />
          <component
            :is="slotProps.item.menuIcon || slotProps.item.icon"
            v-else-if="slotProps.item.menuIcon || slotProps.item.icon"
            :size="14"
            class="shrink-0"
            :style="
              slotProps.item.color ? { color: slotProps.item.color } : undefined
            "
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <AngleRight v-if="slotProps.hasSubmenu" :size="12" class="shrink-0" />
        </a>
      </template>
    </ContextMenu>
  </div>
</template>
