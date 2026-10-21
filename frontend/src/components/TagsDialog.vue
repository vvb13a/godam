<script setup lang="ts">
import Button from 'primevue/button'
import ContextMenu from 'primevue/contextmenu'
import Dialog from 'primevue/dialog'
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
  Palette,
  Pencil,
  Plus,
  Search,
  Star,
  StarFill,
  Times,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '../api/client'
import { library, notifyLibraryChanged } from '../lib/library'
import { confirmDeleteMenuItem } from '../lib/menu'
import { splitNames } from '../lib/names'
import { isPinned, toggleQuicklink } from '../lib/quicklinks'
import { useAnimatedHeight } from '../lib/useAnimatedHeight'
import { tagsDialog } from '../lib/tagsDialog'
import { upload } from '../lib/upload'
import type { Tag, TagGroup } from '../types'
import TagBadge from './TagBadge.vue'


interface GroupMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

const DEFAULT_COLOR = '6366f1'
const GROUP_STORAGE_KEY = 'godam:tags:group'
const SORT_STORAGE_KEY = 'godam:tags:sort'

type GroupMode = 'none' | 'alphabet' | 'groups'
type SortMode = 'name-asc' | 'name-desc' | 'count-asc' | 'count-desc'

const GROUP_MODES: GroupMode[] = ['none', 'alphabet', 'groups']
const SORT_MODES: SortMode[] = [
  'name-asc',
  'name-desc',
  'count-asc',
  'count-desc',
]

function loadGroupMode(): GroupMode {
  const raw = localStorage.getItem(GROUP_STORAGE_KEY)
  return GROUP_MODES.includes(raw as GroupMode) ? (raw as GroupMode) : 'none'
}

function loadSortMode(): SortMode {
  const raw = localStorage.getItem(SORT_STORAGE_KEY)
  return SORT_MODES.includes(raw as SortMode)
    ? (raw as SortMode)
    : 'name-asc'
}

const COLOR_OPTIONS = [
  { name: 'Red', value: '#ef4444' },
  { name: 'Orange', value: '#f97316' },
  { name: 'Amber', value: '#f59e0b' },
  { name: 'Green', value: '#22c55e' },
  { name: 'Teal', value: '#14b8a6' },
  { name: 'Blue', value: '#3b82f6' },
  { name: 'Indigo', value: '#6366f1' },
  { name: 'Purple', value: '#a855f7' },
  { name: 'Pink', value: '#ec4899' },
  { name: 'Slate', value: '#64748b' },
]

const toast = useToast()
const router = useRouter()

const rootRef = ref<HTMLElement | null>(null)
const { setWrapper: setHeightWrapper, style: heightStyle } = useAnimatedHeight(rootRef)
const searchRef = ref<HTMLInputElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const groupMenu = ref<InstanceType<typeof ContextMenu> | null>(null)
const displayPopover = ref<InstanceType<typeof Popover> | null>(null)

const search = ref('')
const selectedTagIds = ref<string[]>([])
const sort = ref<SortMode>(loadSortMode())
const groupBy = ref<GroupMode>(loadGroupMode())

const editingTagId = ref<string | null>(null)
const editingTagName = ref('')
const draftTagId = ref<string | null>(null)

const contextTag = ref<Tag | null>(null)
const contextGroup = ref<TagGroup | null>(null)
const pendingAction = ref<'create-tag' | null>(null)
const draggingTagId = ref<string | null>(null)
const dragOverTarget = ref<string | null>(null)

const editingGroupId = ref<string | null>(null)
const editingName = ref('')
const groupDraftRef = ref<HTMLInputElement | null>(null)
const creatingGroup = ref(false)
const groupDraftName = ref('')
const groupDraftTagId = ref<string | null>(null)

watch(sort, (value) => {
  try {
    localStorage.setItem(SORT_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the setting simply won't persist.
  }
})

watch(groupBy, (value) => {
  try {
    localStorage.setItem(GROUP_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the setting simply won't persist.
  }
})

const isAssetContext = computed(() => !!tagsDialog.assetId)
const isBulkContext = computed(
  () => tagsDialog.assetIds.length > 0 && !!tagsDialog.mode,
)
const isAssetEditing = computed(
  () => isAssetContext.value || isBulkContext.value || tagsDialog.upload,
)
const selectionLoaded = ref(false)
let skipSaveOnClose = false

const viewTags = computed(() => {
  let list = library.tags
  const query = search.value.trim().toLowerCase()
  if (query) {
    list = list.filter((tag) => tag.name.toLowerCase().includes(query))
  }

  const sorted = [...list]
  switch (sort.value) {
    case 'name-desc':
      sorted.sort((a, b) => b.name.localeCompare(a.name))
      break
    case 'count-asc':
      sorted.sort((a, b) => a.asset_count - b.asset_count)
      break
    case 'count-desc':
      sorted.sort((a, b) => b.asset_count - a.asset_count)
      break
    default:
      sorted.sort((a, b) => a.name.localeCompare(b.name))
  }
  return sorted
})

interface TagSection {
  key: string
  label: string
  color?: string
  tags: Tag[]
  groupId?: string
  dropId?: string | null
}

const sections = computed<TagSection[]>(() => {
  const tags = viewTags.value
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
      result.push({ key: 'starred', label: 'Starred', color: '#f97316', tags: starred })
    }
    for (const group of library.tagGroups) {
      const groupTags = tags.filter(
        (tag) => tag.group_id === group.id && !tag.starred,
      )
      if (groupTags.length) {
        result.push({
          key: `group:${group.id}`,
          label: group.name,
          color: group.color ? `#${group.color}` : undefined,
          tags: groupTags,
          groupId: group.id,
          dropId: group.id,
        })
      }
    }
    const uncategorized = tags.filter((tag) => !tag.group_id && !tag.starred)
    if (uncategorized.length) {
      result.push({
        key: 'uncategorized',
        label: 'Uncategorized',
        tags: uncategorized,
        dropId: null,
      })
    }
    return result
  }
  return [{ key: 'all', label: '', tags }]
})

const flatTags = computed(() => sections.value.flatMap((section) => section.tags))

const tagMenuItems = computed<MenuItem[]>(() => {
  const tag = contextTag.value
  if (!tag) return []
  const items: MenuItem[] = [
    {
      label: 'Open',
      icon: Search,
      command: () => filterByTag(tag),
    },
    {
      label: isPinned('tag', tag.id)
        ? 'Unpin from Quicklinks'
        : 'Pin to Quicklinks',
      icon: Bookmark,
      command: () =>
        toggleQuicklink({
          type: 'tag',
          id: tag.id,
          name: tag.name,
          color: tagColor(tag),
        }),
    },
    { separator: true },
    {
      label: tag.starred ? 'Unstar' : 'Star',
      icon: tag.starred ? StarFill : Star,
      command: () => toggleStar(tag),
    },
    {
      label: 'Rename',
      icon: Pencil,
      command: () => startTagRename(tag),
    },
  ]
  items.push({
    label: 'Move',
    icon: Folder,
    items: [
      ...library.tagGroups.map((group) => ({
        label: group.name,
        icon: Bookmark,
        color: group.color ? `#${group.color}` : undefined,
        disabled: tag.group_id === group.id,
        command: () => {
          if (tag.group_id !== group.id) assignGroup(tag, group.id)
        },
      })),
      ...(library.tagGroups.length ? [{ separator: true }] : []),
      {
        label: 'New group…',
        icon: Plus,
        command: () => startGroupDraft(tag),
      },
    ],
  })
  if (tag.group_id) {
    items.push({
      label: 'Ungroup',
      icon: FolderOpen,
      command: () => assignGroup(tag, null),
    })
  }
  items.push(
    { separator: true },
    {
      label: 'Create',
      icon: Plus,
      command: () => {
        pendingAction.value = 'create-tag'
      },
    },
    confirmDeleteMenuItem('Delete', () => performDeleteTag(tag.id)),
  )
  return items
})

const groupMenuItems = computed<GroupMenuItem[]>(() => {
  const group = contextGroup.value
  if (!group) return []
  return [
    {
      label: 'Rename',
      menuIcon: Pencil,
      command: () => startGroupRename(group),
    },
    {
      label: 'Color',
      menuIcon: Palette,
      items: COLOR_OPTIONS.map((option) => ({
        label: option.name,
        color: option.value,
        colorOption: true,
        command: () => setGroupColor(option.value),
      })),
    },
    { separator: true },
    confirmDeleteMenuItem('Delete', () => performDeleteGroup(group.id)),
  ]
})

watch(
  () => tagsDialog.visible,
  (open) => {
    if (open) {
      search.value = ''
      selectedTagIds.value = []
      editingTagId.value = null
      editingGroupId.value = null
      draftTagId.value = null
      creatingGroup.value = false
      groupDraftName.value = ''
      groupDraftTagId.value = null
      selectionLoaded.value = false
      dirty.value = false
      if (tagsDialog.upload) {
        selectedTagIds.value = [...upload.destinationTagIds]
        selectionLoaded.value = true
      } else if (tagsDialog.assetId) {
        void loadAssetSelection(tagsDialog.assetId)
      } else if (isBulkContext.value) {
        selectionLoaded.value = true
      }
      void pruneEmptyGroups()
    } else {
      if (!skipSaveOnClose) void saveAssetSelection()
      skipSaveOnClose = false
      if (dirty.value) {
        dirty.value = false
        notifyLibraryChanged()
      }
    }
  },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// Mutations inside the dialog update the shared library state locally so the
// dialog stays fresh; a single full refresh is deferred until the dialog
// closes (see the visible watcher).
const dirty = ref(false)

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
  dirty.value = true
}

function removeTagLocal(id: string): void {
  library.tags = library.tags.filter((tag) => tag.id !== id)
  recomputeGroupCounts()
  dirty.value = true
}

function upsertGroup(group: TagGroup): void {
  const index = library.tagGroups.findIndex((item) => item.id === group.id)
  if (index >= 0) library.tagGroups[index] = group
  else library.tagGroups.push(group)
  dirty.value = true
}

function removeGroupLocal(id: string): void {
  library.tagGroups = library.tagGroups.filter((group) => group.id !== id)
  for (const tag of library.tags) {
    if (tag.group_id === id) tag.group_id = undefined
  }
  dirty.value = true
}

function focusSearch(): void {
  nextTick(() => searchRef.value?.focus())
}

function toggleDisplayOptions(event: Event): void {
  displayPopover.value?.toggle(event)
}

const GROUP_OPTIONS: { label: string; value: GroupMode }[] = [
  { label: 'None', value: 'none' },
  { label: 'Alphabet', value: 'alphabet' },
  { label: 'Groups', value: 'groups' },
]

const SORT_OPTIONS: {
  label: string
  value: SortMode
}[] = [
  { label: 'Name A–Z', value: 'name-asc' },
  { label: 'Name Z–A', value: 'name-desc' },
  { label: 'Count high–low', value: 'count-desc' },
  { label: 'Count low–high', value: 'count-asc' },
]

function setGroup(value: GroupMode): void {
  groupBy.value = value
  displayPopover.value?.hide()
}

function setSort(
  value: SortMode,
): void {
  sort.value = value
  displayPopover.value?.hide()
}

const selectedTags = computed(() => {
  const ids = new Set(selectedTagIds.value)
  return library.tags.filter((tag) => ids.has(tag.id))
})

const searchTerm = computed(() => search.value.trim())
const searchNames = computed(() => splitNames(searchTerm.value))
const existingNames = computed(() =>
  searchNames.value.filter((name) =>
    library.tags.some(
      (tag) => tag.name.trim().toLowerCase() === name.toLowerCase(),
    ),
  ),
)
const newNames = computed(() =>
  searchNames.value.filter(
    (name) =>
      !library.tags.some(
        (tag) => tag.name.trim().toLowerCase() === name.toLowerCase(),
      ),
  ),
)
const showCreate = computed(() => newNames.value.length > 0)

function tagColor(tag: Tag): string | undefined {
  if (!tag.group_id) return undefined
  const group = library.tagGroups.find((item) => item.id === tag.group_id)
  return group?.color ? `#${group.color}` : undefined
}

async function loadAssetSelection(assetId: string): Promise<void> {
  try {
    const asset = await api.getAsset(assetId)
    selectedTagIds.value = (asset.tags ?? []).map((tag) => tag.id)
    selectionLoaded.value = true
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not load asset tags',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function saveAssetSelection(): Promise<void> {
  if (tagsDialog.upload) {
    upload.destinationTagIds = selectedTagIds.value.slice()
    selectionLoaded.value = false
    return
  }
  if (isBulkContext.value) {
    await saveBulkSelection()
    return
  }
  const assetId = tagsDialog.assetId
  if (!assetId || !selectionLoaded.value) return
  selectionLoaded.value = false
  try {
    const target = selectedTagIds.value.slice()
    await api.applyAssetsTags([assetId], target, 'replace')
    notifyLibraryChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not save asset tags',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

// Applies the chosen tags to every selected asset in one request: "replace"
// makes the selection the exact tag set, "add" merges it into each asset's tags.
async function saveBulkSelection(): Promise<void> {
  if (!selectionLoaded.value) return
  selectionLoaded.value = false
  const ids = tagsDialog.assetIds
  const mode = tagsDialog.mode
  const target = selectedTagIds.value.slice()
  if (!ids.length || !mode) return
  try {
    await api.applyAssetsTags(ids, target, mode)
    notifyLibraryChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update tags',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function toggleTagSelection(tag: Tag): void {
  if (!isAssetEditing.value) return
  if (selectedTagIds.value.includes(tag.id)) {
    selectedTagIds.value = selectedTagIds.value.filter((id) => id !== tag.id)
  } else {
    selectedTagIds.value = [...selectedTagIds.value, tag.id]
  }
}

function onTagClick(tag: Tag): void {
  toggleTagSelection(tag)
  focusSearch()
}

function openTagMenu(tag: Tag, event: Event): void {
  contextTag.value = tag
  menu.value?.show(event)
}

function openGroupMenu(group: TagGroup, event: MouseEvent): void {
  contextGroup.value = group
  groupMenu.value?.show(event)
}

function openGroupMenuById(event: MouseEvent, id: string): void {
  const group = library.tagGroups.find((item) => item.id === id)
  if (group) openGroupMenu(group, event)
}

function startGroupRenameById(id: string): void {
  const group = library.tagGroups.find((item) => item.id === id)
  if (group) startGroupRename(group)
}

// onMenuHide runs after a context menu has fully closed. Deferred create
// actions wait for this so the menu is gone before the inline editor opens.
function onMenuHide(): void {
  const action = pendingAction.value
  pendingAction.value = null
  if (action === 'create-tag') {
    void addTag()
    return
  }
  if (editingGroupId.value || editingTagId.value || creatingGroup.value) return
  focusSearch()
}

async function toggleStar(tag: Tag): Promise<void> {
  try {
    const updated = await api.updateTag(
      tag.id,
      tag.name,
      tag.group_id ?? null,
      !tag.starred,
      tag.description,
    )
    upsertTag(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function filterByTag(tag: Tag): void {
  tagsDialog.visible = false
  void router.push({ name: 'all', query: { tag: tag.id } })
}

async function assignGroup(tag: Tag, groupId: string | null): Promise<void> {
  const previousGroup = tag.group_id
  try {
    const updated = await api.updateTag(
      tag.id,
      tag.name,
      groupId,
      tag.starred,
      tag.description,
    )
    upsertTag(updated)
    if (previousGroup && previousGroup !== groupId) {
      await pruneEmptyGroup(previousGroup)
    }
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function pruneEmptyGroups(): Promise<void> {
  const empty = library.tagGroups.filter(
    (group) => !library.tags.some((tag) => tag.group_id === group.id),
  )
  for (const group of empty) {
    await pruneEmptyGroup(group.id)
  }
}

async function pruneEmptyGroup(groupId: string): Promise<void> {
  if (library.tags.some((tag) => tag.group_id === groupId)) return
  try {
    await api.deleteTagGroup(groupId)
    removeGroupLocal(groupId)
  } catch {
    // Keep the group if deletion fails; it is still recoverable.
  }
}

function startGroupDraft(tag: Tag): void {
  creatingGroup.value = true
  groupDraftTagId.value = tag.id
  groupDraftName.value = ''
  menu.value?.hide()
  nextTick(() => groupDraftRef.value?.focus())
}

function cancelGroupDraft(): void {
  creatingGroup.value = false
  groupDraftName.value = ''
  groupDraftTagId.value = null
}

async function commitGroupDraft(): Promise<void> {
  if (!creatingGroup.value) return
  const name = groupDraftName.value.trim()
  const tagId = groupDraftTagId.value
  creatingGroup.value = false
  groupDraftName.value = ''
  groupDraftTagId.value = null
  if (!name || !tagId) return
  const tag = library.tags.find((item) => item.id === tagId)
  if (!tag) return
  let group =
    library.tagGroups.find(
      (item) => item.name.trim().toLowerCase() === name.toLowerCase(),
    ) ?? null
  try {
    if (!group) {
      group = await api.createTagGroup(name, '', DEFAULT_COLOR)
      upsertGroup(group)
    }
    await assignGroup(tag, group.id)
    groupBy.value = 'groups'
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create group',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function onTagDragStart(tag: Tag, event: DragEvent): void {
  draggingTagId.value = tag.id
  if (event.dataTransfer) {
    event.dataTransfer.setData('text/plain', tag.id)
    event.dataTransfer.effectAllowed = 'move'
  }
}

function onTagDragEnd(): void {
  draggingTagId.value = null
  dragOverTarget.value = null
}

function onGroupDragOver(key: string): void {
  if (draggingTagId.value) dragOverTarget.value = key
}

function onGroupDrop(groupId: string | null): void {
  const id = draggingTagId.value
  draggingTagId.value = null
  dragOverTarget.value = null
  if (!id) return
  const tag = library.tags.find((item) => item.id === id)
  if (tag && tag.group_id !== (groupId ?? undefined)) {
    void assignGroup(tag, groupId)
  }
}

async function performDeleteTag(id: string): Promise<void> {
  const groupId = library.tags.find((tag) => tag.id === id)?.group_id
  try {
    await api.deleteTag(id)
    removeTagLocal(id)
    if (groupId) await pruneEmptyGroup(groupId)
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

async function performDeleteGroup(id: string): Promise<void> {
  try {
    await api.deleteTagGroup(id)
    removeGroupLocal(id)
    toast.add({ severity: 'success', summary: 'Group deleted', life: 2000 })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Delete failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function uniqueUntitledTagName(): string {
  const names = new Set(library.tags.map((tag) => tag.name))
  if (!names.has('untitled')) return 'untitled'
  let index = 2
  while (names.has(`untitled ${index}`)) index++
  return `untitled ${index}`
}

async function createFromSearch(): Promise<void> {
  const names = searchNames.value
  if (!names.length) return
  const existing = library.tags.filter((tag) =>
    names.some(
      (name) => tag.name.trim().toLowerCase() === name.toLowerCase(),
    ),
  )
  if (isAssetEditing.value && existing.length) {
    selectedTagIds.value = [
      ...new Set([...selectedTagIds.value, ...existing.map((tag) => tag.id)]),
    ]
  }
  if (existingNames.value.length) {
    toast.add({
      severity: 'info',
      summary: 'Skipped existing tags',
      detail: existingNames.value.join(', '),
      life: 3000,
    })
  }
  for (const name of newNames.value) {
    try {
      const tag = await api.createTag(name)
      upsertTag(tag)
      if (isAssetEditing.value) {
        selectedTagIds.value = [...selectedTagIds.value, tag.id]
      }
    } catch (e) {
      toast.add({
        severity: 'error',
        summary: 'Could not create tag',
        detail: toMessage(e),
        life: 4000,
      })
    }
  }
  search.value = ''
  nextTick(focusSearch)
}

function cancelDialog(): void {
  skipSaveOnClose = true
  tagsDialog.visible = false
}

// addTag creates a draft tag then opens its inline editor. If the name is left
// unchanged the draft is rolled back, so nothing is persisted.
async function addTag(): Promise<void> {
  try {
    const tag = await api.createTag(uniqueUntitledTagName())
    upsertTag(tag)
    draftTagId.value = tag.id
    startTagRename(tag)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create tag',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startTagRename(tag: Tag): void {
  editingTagId.value = tag.id
  editingTagName.value = tag.name
  nextTick(() => {
    const el = rootRef.value?.querySelector<HTMLInputElement>(
      `[data-tag-rename="${tag.id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

function cancelTagRename(): void {
  const id = editingTagId.value
  editingTagId.value = null
  if (id && id === draftTagId.value) {
    draftTagId.value = null
    void api.deleteTag(id).then(() => removeTagLocal(id))
  }
}

async function commitTagRename(): Promise<void> {
  const id = editingTagId.value
  if (!id) return
  editingTagId.value = null
  const tag = library.tags.find((item) => item.id === id)
  const names = splitNames(editingTagName.value)
  if (!tag) return
  if (id === draftTagId.value) {
    draftTagId.value = null
    if (!names.length) {
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
    if (names.length > 1) {
      try {
        await api.deleteTag(id)
        removeTagLocal(id)
        for (const name of names) {
          upsertTag(await api.createTag(name))
        }
      } catch (e) {
        toast.add({
          severity: 'error',
          summary: 'Could not create tags',
          detail: toMessage(e),
          life: 4000,
        })
      }
      return
    }
    if (names[0] === tag.name) {
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
  const name = names[0]
  if (!name || name === tag.name) return
  try {
    const updated = await api.updateTag(
      id,
      name,
      tag.group_id ?? null,
      tag.starred,
      tag.description,
    )
    upsertTag(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function startGroupRename(group: TagGroup): void {
  groupMenu.value?.hide()
  editingGroupId.value = group.id
  editingName.value = group.name
  nextTick(() => {
    const el = rootRef.value?.querySelector<HTMLInputElement>(
      `[data-group-rename="${group.id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

function cancelGroupRename(): void {
  editingGroupId.value = null
}

async function commitGroupRename(): Promise<void> {
  const id = editingGroupId.value
  if (!id) return
  editingGroupId.value = null
  const group = library.tagGroups.find((item) => item.id === id)
  const name = editingName.value.trim()
  if (!group || !name || name === group.name) return
  try {
    const updated = await api.updateTagGroup(
      id,
      name,
      group.description ?? '',
      group.color,
    )
    upsertGroup(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function setGroupColor(color: string): Promise<void> {
  const group = contextGroup.value
  groupMenu.value?.hide()
  if (!group) return
  try {
    const updated = await api.updateTagGroup(
      group.id,
      group.name,
      group.description ?? '',
      color.replace('#', ''),
    )
    upsertGroup(updated)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}
</script>

<template>
  <Dialog
    v-model:visible="tagsDialog.visible"
    modal
    dismissable-mask
    :show-header="false"
    :style="{ width: '32rem', maxWidth: '95vw' }"
    :pt="{ content: { class: '!p-0' } }"
    @show="focusSearch"
  >
    <div
      :ref="setHeightWrapper"
      class="overflow-hidden"
      :style="heightStyle"
    >
      <div
        ref="rootRef"
        class="flex max-h-[80vh] flex-col"
        :style="{ minHeight: 'min(30rem, 80vh)' }"
      >
      <div class="flex items-center gap-3 border-b border-slate-200 dark:border-slate-800 px-4 py-3">
        <Search :size="16" class="shrink-0 text-slate-400" />
        <input
          ref="searchRef"
          v-model="search"
          type="text"
          placeholder="Search tags…"
          class="min-w-0 flex-1 border-0 bg-transparent text-sm outline-none"
          @keydown.enter.prevent.stop="createFromSearch"
        />
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-brand-700 dark:hover:text-brand-400"
          title="New tag"
          @click="addTag"
        >
          <Plus :size="16" />
        </button>
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 hover:text-brand-700 dark:hover:bg-slate-800 dark:hover:text-brand-400"
          title="Group and sort tags"
          @click="toggleDisplayOptions"
        >
          <Cog :size="16" />
        </button>
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200"
          title="Close"
          @click="tagsDialog.visible = false"
        >
          <Times :size="16" />
        </button>
      </div>

      <div class="flex min-h-0 flex-1">
        <div class="flex min-w-0 flex-1 flex-col">
          <div class="min-h-0 flex-1 overflow-y-auto px-3 pt-3 pb-1.5">
            <div
              v-if="creatingGroup"
              class="mb-1 flex items-center gap-2 rounded border border-brand-300 bg-brand-50/40 px-2 py-1.5 dark:border-brand-800 dark:bg-brand-950/20"
            >
              <Folder :size="16" class="shrink-0 text-brand-500" />
              <input
                ref="groupDraftRef"
                v-model="groupDraftName"
                type="text"
                placeholder="New group name…"
                class="min-w-0 flex-1 border-b border-brand-400 bg-transparent p-0 text-sm leading-5 text-slate-800 outline-none dark:text-slate-100"
                @keydown.enter.prevent="commitGroupDraft"
                @keydown.esc.prevent="cancelGroupDraft"
                @blur="commitGroupDraft"
              />
            </div>
            <button
              v-if="showCreate"
              type="button"
              class="mb-1 flex w-full items-center gap-2 rounded px-2 py-2 text-left text-sm text-brand-700 hover:bg-brand-50 dark:text-brand-300 dark:hover:bg-brand-950/40"
              @click="createFromSearch"
            >
              <Plus :size="15" class="shrink-0" />
              <span class="min-w-0 flex-1 truncate">
                <template v-if="newNames.length === 1">
                  Create “{{ newNames[0] }}”
                </template>
                <template v-else>
                  Create {{ newNames.length }} tags
                </template>
              </span>
            </button>
            <p
              v-if="!flatTags.length"
              class="px-2 py-6 text-center text-sm text-slate-400"
            >
              No tags
            </p>
            <div class="flex flex-wrap items-start gap-1.5">
              <fieldset
                v-for="section in sections"
                :key="section.key"
                class="max-w-full min-w-0 shrink-0 rounded-md pb-1.5"
                :class="[
                  section.label
                    ? 'border border-slate-200 bg-slate-50 px-1.5 dark:border-slate-800 dark:bg-slate-950'
                    : 'w-full',
                  section.dropId !== undefined &&
                  dragOverTarget === (section.dropId ?? 'uncategorized')
                    ? 'ring-2 ring-brand-400 ring-inset'
                    : '',
                ]"
                @dragover.prevent="
                  section.dropId !== undefined &&
                  onGroupDragOver(section.dropId ?? 'uncategorized')
                "
                @dragleave="dragOverTarget = null"
                @drop.prevent="
                  section.dropId !== undefined &&
                  onGroupDrop(section.dropId ?? null)
                "
              >
                <legend
                  v-if="section.label"
                  class="px-1 text-[0.625rem] font-normal text-slate-400"
                  @contextmenu.prevent="
                    section.groupId && openGroupMenuById($event, section.groupId)
                  "
                >
                  <input
                    v-if="section.groupId && editingGroupId === section.groupId"
                    v-model="editingName"
                    :data-group-rename="section.groupId"
                    type="text"
                    class="w-24 border-b border-brand-400 bg-transparent p-0 text-[0.625rem] text-slate-800 outline-none dark:text-slate-100"
                    @click.stop
                    @dblclick.stop
                    @keydown.stop
                    @keydown.enter="commitGroupRename"
                    @keydown.esc="cancelGroupRename"
                    @blur="commitGroupRename"
                  />
                  <span
                    v-else
                    :class="section.groupId ? 'cursor-text' : ''"
                    @dblclick.stop="
                      section.groupId && startGroupRenameById(section.groupId)
                    "
                  >
                    {{ section.label }}
                  </span>
                </legend>
                <div class="flex flex-wrap items-center gap-1.5">
                  <TagBadge
                    v-for="tag in section.tags"
                    :key="tag.id"
                    :name="tag.name"
                    :color="tagColor(tag)"
                    :starred="tag.starred"
                    :selected="selectedTagIds.includes(tag.id)"
                    draggable="true"
                    :title="tag.name"
                    @click="onTagClick(tag)"
                    @dblclick="startTagRename(tag)"
                    @contextmenu.prevent="openTagMenu(tag, $event)"
                    @dragstart="onTagDragStart(tag, $event)"
                    @dragend="onTagDragEnd"
                  >
                    <template #label>
                      <input
                        v-if="editingTagId === tag.id"
                        v-model="editingTagName"
                        :data-tag-rename="tag.id"
                        type="text"
                        placeholder="Tag name (use , or ; to batch)"
                        class="w-28 min-w-0 border-b border-brand-400 bg-transparent p-0 text-xs leading-5 text-slate-800 outline-none dark:text-slate-100"
                        @click.stop
                        @dblclick.stop
                        @keydown.stop
                        @keydown.enter="commitTagRename"
                        @keydown.esc="cancelTagRename"
                        @blur="commitTagRename"
                      />
                      <span v-else class="max-w-56 truncate">{{ tag.name }}</span>
                    </template>
                    <span
                      v-if="tag.asset_count > 0"
                      class="shrink-0 text-[10px] opacity-60"
                    >
                      {{ tag.asset_count }}
                    </span>
                  </TagBadge>
                </div>
              </fieldset>
            </div>
          </div>

        </div>
      </div>

      <div
        class="flex items-center justify-between border-t border-slate-200 px-3 py-2 dark:border-slate-800"
      >
        <span class="text-xs text-slate-400">
          <template v-if="isAssetEditing">{{ selectedTags.length }} selected</template>
          <template v-else>
            {{ library.tags.length }} tag{{ library.tags.length === 1 ? '' : 's' }}
          </template>
        </span>
        <div class="flex items-center gap-2">
          <Button label="Cancel" text size="small" @click="cancelDialog" />
          <Button
            label="Done"
            size="small"
            @click="tagsDialog.visible = false"
          />
        </div>
      </div>
      </div>
    </div>

    <ContextMenu ref="menu" :model="tagMenuItems" @hide="onMenuHide">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700 dark:text-slate-200'"
          @click.prevent
        >
          <span
            v-if="slotProps.item.colorOption"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <component
            :is="slotProps.item.menuIcon || slotProps.icon"
            v-else-if="slotProps.item.menuIcon || slotProps.icon"
            :size="14"
            class="shrink-0"
            :style="
              slotProps.item.color ? { color: slotProps.item.color } : undefined
            "
          />
          <span
            v-else-if="slotProps.item.color"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextGroup?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-brand-700 dark:text-brand-400"
          />
          <AngleRight
            v-else-if="slotProps.hasSubmenu"
            :size="12"
            class="shrink-0"
          />
        </a>
      </template>
    </ContextMenu>

    <ContextMenu ref="groupMenu" :model="groupMenuItems" @hide="onMenuHide">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700 dark:text-slate-200'"
          @click.prevent
        >
          <span
            v-if="slotProps.item.colorOption"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <component
            :is="slotProps.item.menuIcon || slotProps.icon"
            v-else-if="slotProps.item.menuIcon || slotProps.icon"
            :size="14"
            class="shrink-0"
            :style="
              slotProps.item.color ? { color: slotProps.item.color } : undefined
            "
          />
          <span
            v-else-if="slotProps.item.color"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
            :style="{ backgroundColor: slotProps.item.color }"
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextGroup?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-brand-700 dark:text-brand-400"
          />
          <AngleRight
            v-else-if="slotProps.hasSubmenu"
            :size="12"
            class="shrink-0"
          />
        </a>
      </template>
    </ContextMenu>

    <Popover ref="displayPopover">
      <div class="w-56">
        <p class="px-2 pt-1 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase">
          Group by
        </p>
        <button
          v-for="option in GROUP_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            groupBy === option.value
              ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/50 dark:text-brand-300'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
          "
          @click="setGroup(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="groupBy === option.value" :size="12" class="shrink-0" />
        </button>

        <div class="my-1 border-t border-slate-100 dark:border-slate-800" />

        <p class="px-2 pt-1 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase">
          Sort by
        </p>
        <button
          v-for="option in SORT_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            sort === option.value
              ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/50 dark:text-brand-300'
              : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'
          "
          @click="setSort(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="sort === option.value" :size="12" class="shrink-0" />
        </button>
      </div>
    </Popover>
  </Dialog>
</template>
