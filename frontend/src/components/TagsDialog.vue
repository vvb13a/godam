<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import Dialog from 'primevue/dialog'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleRight,
  Bars,
  Bookmark,
  Check,
  CheckCircle,
  Folder,
  FolderOpen,
  Palette,
  Pencil,
  Plus,
  Search,
  SlidersH,
  Star,
  StarFill,
  ThLarge,
  Times,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '../api/client'
import { library, notifyLibraryChanged } from '../lib/library'
import { confirmDeleteMenuItem } from '../lib/menu'
import { splitNames } from '../lib/names'
import { tagsDialog } from '../lib/tagsDialog'
import { upload } from '../lib/upload'
import type { Tag, TagGroup } from '../types'
import Kbd from './Kbd.vue'
import TagBadge from './TagBadge.vue'

interface NavEntry {
  key: string
  label: string
  count: number
  icon?: Component
  color?: string
}

interface GroupMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

const DEFAULT_COLOR = '6366f1'
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
const searchRef = ref<HTMLInputElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const groupMenu = ref<InstanceType<typeof ContextMenu> | null>(null)
const displayMenu = ref<InstanceType<typeof ContextMenu> | null>(null)

const search = ref('')
const activeView = ref('all')
const focusArea = ref<'nav' | 'tags'>('tags')
const navIndex = ref(0)
const tagIndex = ref(0)
const selectedTagIds = ref<string[]>([])
const tagLayout = ref<'list' | 'grid'>('list')
const collapsed = ref(false)
const sort = ref<'name-asc' | 'name-desc' | 'count-asc' | 'count-desc'>(
  'name-asc',
)
const groupBy = ref<'none' | 'alphabet' | 'groups'>('none')

const editingTagId = ref<string | null>(null)
const editingTagName = ref('')
const draftTagId = ref<string | null>(null)

const contextTag = ref<Tag | null>(null)
const contextGroup = ref<TagGroup | null>(null)
const pendingAction = ref<'create-tag' | 'create-group' | null>(null)
const draggingTagId = ref<string | null>(null)
const dragOverTarget = ref<string | null>(null)

const editingGroupId = ref<string | null>(null)
const editingName = ref('')
const draftGroupId = ref<string | null>(null)

const isAssetContext = computed(() => !!tagsDialog.assetId)
const isBulkContext = computed(
  () => tagsDialog.assetIds.length > 0 && !!tagsDialog.mode,
)
const isAssetEditing = computed(
  () => isAssetContext.value || isBulkContext.value || tagsDialog.upload,
)
const selectionLoaded = ref(false)

const syntheticNav = computed<NavEntry[]>(() => {
  const tags = library.tags
  return [
    { key: 'all', label: 'All', count: tags.length, icon: Bookmark },
    {
      key: 'uncategorized',
      label: 'Uncategorized',
      count: tags.filter((tag) => !tag.group_id).length,
      icon: Folder,
    },
    {
      key: 'starred',
      label: 'Starred',
      count: tags.filter((tag) => tag.starred).length,
      icon: StarFill,
    },
  ]
})

const groupNav = computed<NavEntry[]>(() =>
  library.tagGroups.map((group) => ({
    key: `group:${group.id}`,
    label: group.name,
    count: group.tag_count,
    color: group.color,
  })),
)

const navItems = computed<NavEntry[]>(() => [
  ...syntheticNav.value,
  ...groupNav.value,
])

const activeNavLabel = computed(
  () =>
    navItems.value.find((item) => item.key === activeView.value)?.label ?? 'All',
)

const viewTags = computed(() => {
  let list = library.tags
  const view = activeView.value
  if (view === 'uncategorized') {
    list = list.filter((tag) => !tag.group_id)
  } else if (view === 'starred') {
    list = list.filter((tag) => tag.starred)
  } else if (view.startsWith('group:')) {
    const id = view.slice('group:'.length)
    list = list.filter((tag) => tag.group_id === id)
  }

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
      result.push({ key: 'starred', label: 'Starred', color: '#f59e0b', tags: starred })
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
        })
      }
    }
    const uncategorized = tags.filter((tag) => !tag.group_id && !tag.starred)
    if (uncategorized.length) {
      result.push({ key: 'uncategorized', label: 'Uncategorized', tags: uncategorized })
    }
    return result
  }
  return [{ key: 'all', label: '', tags }]
})

const flatTags = computed(() => sections.value.flatMap((section) => section.tags))

const tagIndexMap = computed(() => {
  const map = new Map<string, number>()
  flatTags.value.forEach((tag, index) => map.set(tag.id, index))
  return map
})

const colorFor = (group: TagGroup): string =>
  group.color ? `#${group.color}` : '#cbd5e1'

const displayMenuItems = computed<MenuItem[]>(() => {
  return [
    {
      label: 'Layout',
      items: [
        {
          label: 'List',
          icon: Bars,
          active: tagLayout.value === 'list',
          command: () => {
            tagLayout.value = 'list'
          },
        },
        {
          label: 'Grid',
          icon: ThLarge,
          active: tagLayout.value === 'grid',
          command: () => {
            tagLayout.value = 'grid'
          },
        },
      ],
    },
    {
      label: 'Sidebar',
      items: [
        {
          label: 'Show',
          active: !collapsed.value,
          command: () => {
            collapsed.value = false
          },
        },
        {
          label: 'Hide',
          active: collapsed.value,
          command: () => {
            collapsed.value = true
            focusArea.value = 'tags'
          },
        },
      ],
    },
    {
      label: 'Sort',
      items: [
        {
          label: 'Name A–Z',
          active: sort.value === 'name-asc',
          command: () => {
            sort.value = 'name-asc'
          },
        },
        {
          label: 'Name Z–A',
          active: sort.value === 'name-desc',
          command: () => {
            sort.value = 'name-desc'
          },
        },
        {
          label: 'Count low–high',
          active: sort.value === 'count-asc',
          command: () => {
            sort.value = 'count-asc'
          },
        },
        {
          label: 'Count high–low',
          active: sort.value === 'count-desc',
          command: () => {
            sort.value = 'count-desc'
          },
        },
      ],
    },
    {
      label: 'Group by',
      items: [
        {
          label: 'None',
          active: groupBy.value === 'none',
          command: () => {
            groupBy.value = 'none'
          },
        },
        {
          label: 'Alphabet',
          active: groupBy.value === 'alphabet',
          command: () => {
            groupBy.value = 'alphabet'
          },
        },
        {
          label: 'Groups',
          active: groupBy.value === 'groups',
          command: () => {
            groupBy.value = 'groups'
          },
        },
      ],
    },
  ]
})

const tagMenuItems = computed<MenuItem[]>(() => {
  const tag = contextTag.value
  if (!tag) return []
  const items: MenuItem[] = [
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
    {
      label: 'Filter',
      icon: Search,
      command: () => filterByTag(tag),
    },
  ]
  if (library.tagGroups.length) {
    items.push({
      label: 'Move',
      icon: Folder,
      items: library.tagGroups.map((group) => ({
        label: group.name,
        icon: Bookmark,
        color: group.color ? `#${group.color}` : undefined,
        disabled: tag.group_id === group.id,
        command: () => {
          if (tag.group_id !== group.id) assignGroup(tag, group.id)
        },
      })),
    })
  }
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
    {
      label: 'Filter',
      menuIcon: Search,
      command: () => filterByGroup(group),
    },
    { separator: true },
    {
      label: 'Create',
      menuIcon: Plus,
      command: () => {
        pendingAction.value = 'create-group'
      },
    },
    confirmDeleteMenuItem('Delete', () => performDeleteGroup(group.id)),
  ]
})

watch(
  () => tagsDialog.visible,
  (open) => {
    if (open) {
      window.addEventListener('keydown', onWindowKeydown)
      search.value = ''
      activeView.value = 'all'
      focusArea.value = 'tags'
      navIndex.value = 0
      tagIndex.value = 0
      selectedTagIds.value = []
      editingTagId.value = null
      editingGroupId.value = null
      draftTagId.value = null
      draftGroupId.value = null
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
    } else {
      window.removeEventListener('keydown', onWindowKeydown)
      void saveAssetSelection()
      if (dirty.value) {
        dirty.value = false
        notifyLibraryChanged()
      }
    }
  },
)

watch(flatTags, () => {
  if (tagIndex.value >= flatTags.value.length) {
    tagIndex.value = Math.max(0, flatTags.value.length - 1)
  }
})

watch(navItems, () => {
  if (navIndex.value >= navItems.value.length) {
    navIndex.value = Math.max(0, navItems.value.length - 1)
  }
})

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

const selectedTags = computed(() => {
  const ids = new Set(selectedTagIds.value)
  return library.tags.filter((tag) => ids.has(tag.id))
})

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

function applyNavIndex(): void {
  const item = navItems.value[navIndex.value]
  if (!item) return
  activeView.value = item.key
  tagIndex.value = 0
}

function selectNav(key?: string, index?: number): void {
  if (!key) return
  activeView.value = key
  tagIndex.value = 0
  focusArea.value = 'nav'
  if (typeof index === 'number') navIndex.value = index
  focusSearch()
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
  focusArea.value = 'tags'
  tagIndex.value = tagIndexMap.value.get(tag.id) ?? 0
  toggleTagSelection(tag)
  focusSearch()
}

function scrollFocusedTag(): void {
  const tag = flatTags.value[tagIndex.value]
  if (!tag) return
  void nextTick(() => {
    rootRef.value
      ?.querySelector<HTMLElement>(`[data-tag-id="${tag.id}"]`)
      ?.scrollIntoView({ block: 'nearest' })
  })
}

function onKeydown(event: KeyboardEvent): void {
  if (event.ctrlKey && event.key.toLowerCase() === 'm') {
    event.preventDefault()
    openMenuForFocused()
    return
  }
  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      if (focusArea.value === 'nav') {
        navIndex.value = Math.min(navItems.value.length - 1, navIndex.value + 1)
        applyNavIndex()
      } else {
        tagIndex.value = Math.min(flatTags.value.length - 1, tagIndex.value + 1)
        scrollFocusedTag()
      }
      break
    case 'ArrowUp':
      event.preventDefault()
      if (focusArea.value === 'nav') {
        navIndex.value = Math.max(0, navIndex.value - 1)
        applyNavIndex()
      } else {
        tagIndex.value = Math.max(0, tagIndex.value - 1)
        scrollFocusedTag()
      }
      break
    case 'ArrowRight':
      if (focusArea.value === 'nav') {
        event.preventDefault()
        focusArea.value = 'tags'
        tagIndex.value = 0
      }
      break
    case 'ArrowLeft':
      if (focusArea.value === 'tags' && !collapsed.value) {
        event.preventDefault()
        const index = navItems.value.findIndex(
          (item) => item.key === activeView.value,
        )
        navIndex.value = index >= 0 ? index : 0
        focusArea.value = 'nav'
      }
      break
    case 'Tab':
      if (collapsed.value) break
      event.preventDefault()
      focusArea.value = focusArea.value === 'nav' ? 'tags' : 'nav'
      break
    case 'Enter':
      event.preventDefault()
      if (focusArea.value === 'nav' && !collapsed.value) {
        selectNav(navItems.value[navIndex.value]?.key)
      } else {
        const tag = flatTags.value[tagIndex.value]
        if (tag) toggleTagSelection(tag)
      }
      break
  }
}

// onWindowKeydown drives the dialog globally while it is open so navigation
// keeps working no matter where focus lands. Events originating from overlays
// (context menus, popovers) or from inputs that stop propagation (inline
// editors) are excluded.
function onWindowKeydown(event: KeyboardEvent): void {
  const target = event.target as HTMLElement | null
  if (target && target.closest('button, a, select')) return
  if (target && target.closest('.p-contextmenu, .p-popover, .p-menu')) return
  onKeydown(event)
}

function openTagMenu(tag: Tag, event: Event): void {
  contextTag.value = tag
  menu.value?.show(event)
}

function openGroupMenu(group: TagGroup, event: MouseEvent): void {
  contextGroup.value = group
  groupMenu.value?.show(event)
}

// menuEventFrom fabricates the page coordinates PrimeVue's ContextMenu needs
// when it is opened from a keyboard action rather than a pointer event.
function menuEventFrom(el: HTMLElement): Event {
  const rect = el.getBoundingClientRect()
  return {
    pageX: rect.left + window.scrollX + 24,
    pageY: rect.bottom + window.scrollY + 2,
    currentTarget: el,
    target: el,
    preventDefault() {},
    stopPropagation() {},
  } as unknown as Event
}

// openMenuForFocused opens the context menu for the focused tag/group so the
// whole dialog is usable from the keyboard.
function openMenuForFocused(): void {
  if (focusArea.value === 'tags') {
    const tag = flatTags.value[tagIndex.value]
    if (!tag) return
    const el = rootRef.value?.querySelector<HTMLElement>(
      `[data-tag-id="${tag.id}"]`,
    )
    if (!el) return
    contextTag.value = tag
    menu.value?.show(menuEventFrom(el))
    return
  }

  const group = library.tagGroups[navIndex.value - syntheticNav.value.length]
  if (!group) return
  const el = rootRef.value?.querySelector<HTMLElement>(
    `[data-group-id="${group.id}"]`,
  )
  if (!el) return
  contextGroup.value = group
  groupMenu.value?.show(menuEventFrom(el))
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
  if (action === 'create-group') {
    void addGroup()
    return
  }
  if (editingGroupId.value || editingTagId.value) return
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

function filterByGroup(group: TagGroup): void {
  const ids = library.tags
    .filter((tag) => tag.group_id === group.id)
    .map((tag) => tag.id)
  if (!ids.length) return
  tagsDialog.visible = false
  void router.push({ name: 'all', query: { tag: ids.join(',') } })
}

async function assignGroup(tag: Tag, groupId: string | null): Promise<void> {
  try {
    const updated = await api.updateTag(
      tag.id,
      tag.name,
      groupId,
      tag.starred,
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

async function performDeleteGroup(id: string): Promise<void> {
  try {
    await api.deleteTagGroup(id)
    if (activeView.value === `group:${id}`) activeView.value = 'all'
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

// addTag creates a draft tag then opens its inline editor. If the name is left
// unchanged the draft is rolled back, so nothing is persisted.
async function addTag(): Promise<void> {
  try {
    let tag = await api.createTag(uniqueUntitledTagName())
    const view = activeView.value
    if (view.startsWith('group:')) {
      tag = await api.updateTag(
        tag.id,
        tag.name,
        view.slice('group:'.length),
        false,
        tag.description,
      )
    }
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

function uniqueUntitledName(): string {
  const names = new Set(library.tagGroups.map((group) => group.name))
  if (!names.has('Untitled')) return 'Untitled'
  let index = 2
  while (names.has(`Untitled ${index}`)) index++
  return `Untitled ${index}`
}

async function addGroup(): Promise<void> {
  try {
    const group = await api.createTagGroup(
      uniqueUntitledName(),
      '',
      DEFAULT_COLOR,
    )
    upsertGroup(group)
    draftGroupId.value = group.id
    startGroupRename(group)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create group',
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
  const id = editingGroupId.value
  editingGroupId.value = null
  if (id && id === draftGroupId.value) {
    draftGroupId.value = null
    void api.deleteTagGroup(id).then(() => removeGroupLocal(id))
  }
}

async function commitGroupRename(): Promise<void> {
  const id = editingGroupId.value
  if (!id) return
  editingGroupId.value = null
  const group = library.tagGroups.find((item) => item.id === id)
  const name = editingName.value.trim()
  if (!group) return
  if (id === draftGroupId.value) {
    draftGroupId.value = null
    if (!name || name === group.name) {
      try {
        await api.deleteTagGroup(id)
        removeGroupLocal(id)
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
  if (!name || name === group.name) return
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

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKeydown)
})
</script>

<template>
  <Dialog
    v-model:visible="tagsDialog.visible"
    modal
    dismissable-mask
    :show-header="false"
    :style="{ width: '48rem', maxWidth: '95vw' }"
    :pt="{ content: { class: '!p-0' } }"
    @show="focusSearch"
  >
    <div ref="rootRef" class="flex h-[36rem] max-h-[80vh] flex-col">
      <div class="flex items-center gap-3 border-b border-slate-200 dark:border-slate-800 px-4 py-3">
        <Search :size="16" class="shrink-0 text-slate-400" />
        <input
          ref="searchRef"
          v-model="search"
          type="text"
          placeholder="Search tags…"
          class="min-w-0 flex-1 border-0 bg-transparent text-sm outline-none"
        />
        <button
          class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-violet-600 dark:hover:text-violet-400"
          title="Display options"
          @click="displayMenu?.show($event)"
        >
          <SlidersH :size="16" />
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
        <nav
          v-if="!collapsed"
          class="w-1/3 shrink-0 overflow-y-auto border-r border-slate-200 dark:border-slate-800 p-2"
        >
          <button
            v-for="(item, index) in syntheticNav"
            :key="item.key"
            class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
            :class="[
              item.key === 'uncategorized' && dragOverTarget === 'uncategorized'
                ? 'ring-2 ring-violet-400 ring-inset'
                : focusArea === 'nav' && navIndex === index
                  ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300'
                  : item.key === activeView
                    ? 'bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-slate-100'
                    : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white',
            ]"
            @click="selectNav(item.key, index)"
            @dragover.prevent="item.key === 'uncategorized' && onGroupDragOver('uncategorized')"
            @dragleave="dragOverTarget = null"
            @drop.prevent="item.key === 'uncategorized' && onGroupDrop(null)"
          >
            <component :is="item.icon" :size="14" class="shrink-0" />
            <span class="truncate text-left">{{ item.label }}</span>
            <span
              v-if="item.count > 0"
              class="ml-auto shrink-0 text-xs text-slate-400"
            >
              {{ item.count }}
            </span>
          </button>

          <div class="mt-2 flex items-center justify-between px-2 py-1">
            <span
              class="text-xs font-semibold tracking-wide text-slate-400 uppercase"
            >
              Groups
              <template v-if="library.tagGroups.length">
                ({{ library.tagGroups.length }})
              </template>
            </span>
            <button
              class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
              title="New group"
              @click="addGroup"
            >
              <Plus :size="14" />
            </button>
          </div>

          <p
            v-if="!library.tagGroups.length"
            class="px-2 py-1 text-xs text-slate-400"
          >
            No groups
          </p>

          <div
            v-for="(group, index) in library.tagGroups"
            :key="group.id"
            :data-group-id="group.id"
            class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm select-none"
            :class="[
              dragOverTarget === group.id
                ? 'ring-2 ring-violet-400 ring-inset'
                : focusArea === 'nav' && navIndex === syntheticNav.length + index
                  ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300'
                  : activeView === `group:${group.id}`
                    ? 'bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-slate-100'
                    : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white',
            ]"
            @click="selectNav(`group:${group.id}`, syntheticNav.length + index)"
            @dblclick="startGroupRename(group)"
            @contextmenu.prevent="openGroupMenu(group, $event)"
            @dragover.prevent="onGroupDragOver(group.id)"
            @dragleave="dragOverTarget = null"
            @drop.prevent="onGroupDrop(group.id)"
          >
            <Bookmark
              :size="14"
              class="shrink-0"
              :style="{ color: colorFor(group) }"
            />
            <input
              v-if="editingGroupId === group.id"
              v-model="editingName"
              :data-group-rename="group.id"
              type="text"
              class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none"
              @click.stop
              @dblclick.stop
              @keydown.stop
              @keydown.enter="commitGroupRename"
              @keydown.esc="cancelGroupRename"
              @blur="commitGroupRename"
            />
            <span
              v-else
              class="min-w-0 flex-1 truncate border-b border-transparent text-left leading-5"
            >
              {{ group.name }}
            </span>
            <span
              v-if="group.tag_count > 0"
              class="shrink-0 text-xs text-slate-400"
            >
              {{ group.tag_count }}
            </span>
          </div>
        </nav>

        <div class="flex min-w-0 flex-1 flex-col">
          <div
            class="flex items-center gap-2 border-b border-slate-100 dark:border-slate-800 px-3 py-2"
          >
            <span class="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
              {{ activeNavLabel }}
            </span>
            <span
              v-if="viewTags.length > 0"
              class="shrink-0 text-xs text-slate-400"
            >
              {{ viewTags.length }}
            </span>
            <button
              class="ml-auto inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
              title="New tag"
              @click="addTag"
            >
              <Plus :size="14" />
            </button>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto p-2">
            <p
              v-if="!flatTags.length"
              class="px-2 py-6 text-center text-sm text-slate-400"
            >
              No tags
            </p>
            <template v-for="section in sections" :key="section.key">
              <p
                v-if="section.label"
                class="sticky top-0 z-10 bg-white dark:bg-slate-900 px-2 pt-2 pb-1 text-xs font-semibold tracking-wide text-slate-400 uppercase"
              >
                {{ section.label }}
              </p>
              <div
                :class="
                  tagLayout === 'grid' ? 'grid grid-cols-2 gap-1' : 'space-y-0.5'
                "
              >
                <div
                  v-for="tag in section.tags"
                  :key="tag.id"
                  :data-tag-id="tag.id"
                  draggable="true"
                  class="flex w-full cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm select-none"
                  :class="
                    focusArea === 'tags' &&
                    tagIndexMap.get(tag.id) === tagIndex
                      ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300'
                      : 'text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800'
                  "
                  @click="onTagClick(tag)"
                  @dblclick="startTagRename(tag)"
                  @contextmenu.prevent="openTagMenu(tag, $event)"
                  @dragstart="onTagDragStart(tag, $event)"
                  @dragend="onTagDragEnd"
                >
                  <CheckCircle
                    v-if="selectedTagIds.includes(tag.id)"
                    :size="14"
                    class="shrink-0 text-green-500"
                  />
                  <StarFill
                    v-else-if="tag.starred"
                    :size="14"
                    class="shrink-0"
                    :style="{ color: tagColor(tag) ?? '#f59e0b' }"
                  />
                  <span
                    v-else
                    class="flex h-3.5 w-3.5 shrink-0 items-center justify-center"
                  >
                    <span
                      class="h-2 w-2 rounded-full"
                      :style="{
                        backgroundColor: tagColor(tag) ?? '#cbd5e1',
                      }"
                    />
                  </span>
                  <input
                    v-if="editingTagId === tag.id"
                    v-model="editingTagName"
                    :data-tag-rename="tag.id"
                    type="text"
                    placeholder="Tag name (use , or ; to batch)"
                    class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none"
                    @click.stop
                    @dblclick.stop
                    @keydown.stop
                    @keydown.enter="commitTagRename"
                    @keydown.esc="cancelTagRename"
                    @blur="commitTagRename"
                  />
                  <span
                    v-else
                    class="min-w-0 flex-1 truncate border-b border-transparent text-left leading-5"
                  >
                    {{ tag.name }}
                  </span>
                  <span
                    v-if="tag.asset_count > 0"
                    class="shrink-0 text-xs text-slate-400"
                  >
                    {{ tag.asset_count }}
                  </span>
                </div>
              </div>
            </template>
          </div>

          <div
            v-if="isAssetEditing"
            class="flex flex-wrap items-center gap-1.5 border-t border-slate-100 dark:border-slate-800 px-3 py-2"
          >
            <span v-if="!selectedTags.length" class="text-xs text-slate-400">
              No tags selected
            </span>
            <TagBadge
              v-for="tag in selectedTags"
              :key="tag.id"
              :name="tag.name"
              :color="tagColor(tag)"
            >
              <button
                class="inline-flex text-slate-400 hover:text-red-500"
                title="Remove"
                @click="toggleTagSelection(tag)"
              >
                <Times :size="12" />
              </button>
            </TagBadge>
          </div>
        </div>
      </div>

      <div
        class="flex items-center gap-4 border-t border-slate-200 dark:border-slate-800 px-4 py-2 text-xs text-slate-400"
      >
        <span class="flex items-center gap-1.5">
          <Kbd>↑</Kbd>
          <Kbd>↓</Kbd>
          <Kbd>←</Kbd>
          <Kbd>→</Kbd>
          <span class="ml-1">Move</span>
        </span>
        <span v-if="isAssetEditing" class="flex items-center gap-1.5">
          <Kbd>↵</Kbd>
          <span class="ml-1">Select</span>
        </span>
        <span class="flex items-center gap-1.5">
          <Kbd>Ctrl</Kbd>
          <Kbd>M</Kbd>
          <span class="ml-1">Menu</span>
        </span>
        <span class="ml-auto flex items-center gap-1.5">
          <Kbd>Esc</Kbd>
          <span class="ml-1">Close</span>
        </span>
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
            class="shrink-0 text-violet-600 dark:text-violet-400"
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
            class="shrink-0 text-violet-600 dark:text-violet-400"
          />
          <AngleRight
            v-else-if="slotProps.hasSubmenu"
            :size="12"
            class="shrink-0"
          />
        </a>
      </template>
    </ContextMenu>

    <ContextMenu ref="displayMenu" :model="displayMenuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm text-slate-700 dark:text-slate-200"
          @click.prevent
        >
          <component
            :is="slotProps.item.menuIcon || slotProps.icon"
            v-if="slotProps.item.menuIcon || slotProps.icon"
            :size="14"
            class="shrink-0"
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="slotProps.item.active"
            :size="12"
            class="shrink-0 text-violet-600 dark:text-violet-400"
          />
          <AngleRight
            v-else-if="slotProps.hasSubmenu"
            :size="12"
            class="shrink-0"
          />
        </a>
      </template>
    </ContextMenu>
  </Dialog>
</template>
