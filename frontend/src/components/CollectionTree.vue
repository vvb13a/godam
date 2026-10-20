<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import Popover from 'primevue/popover'
import Tree from 'primevue/tree'
import { useToast } from 'primevue/usetoast'
import type { MenuItem } from 'primevue/menuitem'
import {
  AngleDoubleDown,
  AngleDoubleUp,
  Bookmark,
  Check,
  CheckCircle,
  ChevronDown,
  ChevronUp,
  Folder,
  FolderOpen,
  FolderPlus,
  Palette,
  Pencil,
  Plus,
  SortAlt,
} from '@primeicons/vue'
import type { Component } from 'vue'
import type { TreeNode } from 'primevue/treenode'
import type { TreeNodeDropEvent } from 'primevue/tree'
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as api from '../api/client'
import { assetDrag } from '../lib/assetDrag'
import { library, notifyLibraryChanged } from '../lib/library'
import { LIBRARY_COLORS, libraryColorHex } from '../lib/libraryColors'
import { confirmDeleteMenuItem } from '../lib/menu'
import { splitNames } from '../lib/names'
import { isPinned, toggleQuicklink } from '../lib/quicklinks'
import { openCollectionsDialog } from '../lib/collectionsDialog'
import { clearSmartSelection } from '../lib/smartSelection'
import type { Collection } from '../types'

interface CollectionNode {
  key: string
  label: string
  data: Collection | null
  children: CollectionNode[]
  draggable?: boolean
  selectable?: boolean
}

interface TreeMenuItem extends MenuItem {
  menuIcon?: Component
  color?: string
  colorOption?: boolean
}

type SortMode = 'custom' | 'name-asc' | 'name-desc' | 'count-asc' | 'count-desc'

const DEFAULT_COLLECTION_COLOR = '8b5cf6'
const DRAFT_KEY = '__draft__'
const EXPANDED_STORAGE_KEY = 'godam:collections:expanded'
const SORT_STORAGE_KEY = 'godam:collections:sort'
const SORT_MODES: SortMode[] = ['custom', 'name-asc', 'name-desc', 'count-asc', 'count-desc']

function loadExpanded(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(EXPANDED_STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : null
    return parsed && typeof parsed === 'object'
      ? (parsed as Record<string, boolean>)
      : {}
  } catch {
    return {}
  }
}

function loadSortMode(): SortMode {
  const raw = localStorage.getItem(SORT_STORAGE_KEY)
  return SORT_MODES.includes(raw as SortMode) ? (raw as SortMode) : 'custom'
}

const props = withDefaults(
  defineProps<{
    selectable?: boolean
    selectedIds?: string[]
    filter?: string
    showHeader?: boolean
    navigable?: boolean
    deferRefresh?: boolean
    manage?: boolean
  }>(),
  {
    selectable: false,
    selectedIds: () => [],
    filter: '',
    showHeader: true,
    navigable: true,
    deferRefresh: false,
    manage: true,
  },
)

const emit = defineEmits<{
  'update:selectedIds': [ids: string[]]
  change: []
  open: [collection: Collection]
}>()

const route = useRoute()
const router = useRouter()
const toast = useToast()

const root = ref<HTMLElement | null>(null)
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const sortPopover = ref<InstanceType<typeof Popover> | null>(null)
const contextCollection = ref<Collection | null>(null)

const expandedKeys = ref<Record<string, boolean>>(loadExpanded())
const sortMode = ref<SortMode>(loadSortMode())
const editingId = ref<string | null>(null)
const editingName = ref('')
const creating = ref(false)
const createParent = ref<string | null>(null)
const draftName = ref('')
const dragOverId = ref<string | null>(null)

watch(
  expandedKeys,
  (value) => {
    try {
      localStorage.setItem(EXPANDED_STORAGE_KEY, JSON.stringify(value))
    } catch {
      // Storage unavailable (private mode): expansion simply won't persist.
    }
  },
  { deep: true },
)

watch(sortMode, (value) => {
  try {
    localStorage.setItem(SORT_STORAGE_KEY, value)
  } catch {
    // Storage unavailable: the choice simply won't persist.
  }
})

const activeCollection = computed(() => {
  const id = route.query.collection
  return typeof id === 'string' ? id : null
})

const selectionKeys = computed<Record<string, boolean>>(() =>
  !props.selectable && activeCollection.value
    ? { [activeCollection.value]: true }
    : {},
)

const selectedSet = computed(() => new Set(props.selectedIds))

// Build the flat collection list into the tree the PrimeVue Tree consumes.
const nodes = computed<CollectionNode[]>(() => {
  const byParent = new Map<string | null, Collection[]>()
  for (const collection of library.collections) {
    const key = collection.parent_id ?? null
    const list = byParent.get(key)
    if (list) list.push(collection)
    else byParent.set(key, [collection])
  }
  for (const list of byParent.values()) {
    list.sort(compareCollections)
  }
  const build = (parent: string | null): CollectionNode[] => {
    const children: CollectionNode[] = (byParent.get(parent) ?? []).map(
      (collection) => ({
        key: collection.id,
        label: collection.name,
        data: collection,
        children: build(collection.id),
      }),
    )
    // The inline "new folder" draft lives only in the tree until it is named,
    // so cancelling never creates anything.
    if (creating.value && (createParent.value ?? null) === parent) {
      children.push({
        key: DRAFT_KEY,
        label: '',
        data: null,
        children: [],
        draggable: false,
        selectable: false,
      })
    }
    return children
  }
  return build(null)
})

const treeNodes = computed<CollectionNode[]>(() => {
  const query = props.filter.trim().toLowerCase()
  if (!query) return nodes.value
  const filterTree = (list: CollectionNode[]): CollectionNode[] => {
    const result: CollectionNode[] = []
    for (const node of list) {
      if (node.key === DRAFT_KEY) continue
      const children = filterTree(node.children)
      if (node.label.toLowerCase().includes(query) || children.length) {
        result.push({ ...node, children })
      }
    }
    return result
  }
  return filterTree(nodes.value)
})

watch(
  () => props.filter,
  (value) => {
    if (!value.trim()) return
    const next = { ...expandedKeys.value }
    for (const parent of parentCollections.value) {
      next[parent.id] = true
    }
    expandedKeys.value = next
  },
)

// Expand any newly seen parent once, so subfolders are visible by default.
watch(
  () => library.collections,
  (list) => {
    const next = { ...expandedKeys.value }
    let changed = false
    for (const collection of list) {
      const isParent = list.some((item) => item.parent_id === collection.id)
      if (isParent && !(collection.id in next)) {
        next[collection.id] = true
        changed = true
      }
    }
    if (changed) expandedKeys.value = next
  },
  { immediate: true },
)

// Reveal the active collection by expanding its ancestors (e.g. when selecting
// a nested folder from the command menu).
watch(
  [activeCollection, () => library.collections],
  () => {
    const id = activeCollection.value
    if (!id || id === 'uncategorized') return
    const byId = new Map(library.collections.map((item) => [item.id, item]))
    const next = { ...expandedKeys.value }
    let changed = false
    let parent = byId.get(id)?.parent_id ?? null
    while (parent) {
      if (!next[parent]) {
        next[parent] = true
        changed = true
      }
      parent = byId.get(parent)?.parent_id ?? null
    }
    if (changed) expandedKeys.value = next
  },
  { immediate: true },
)

watch(
  () => assetDrag.active,
  (active) => {
    if (!active) dragOverId.value = null
  },
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function refreshLibrary(): void {
  if (props.deferRefresh) emit('change')
  else notifyLibraryChanged()
}

// Sibling order depends on the chosen display mode; "custom" keeps the stored
// drag-and-drop order.
function compareCollections(a: Collection, b: Collection): number {
  switch (sortMode.value) {
    case 'name-asc':
      return a.name.localeCompare(b.name)
    case 'name-desc':
      return b.name.localeCompare(a.name)
    case 'count-asc':
      return a.asset_count - b.asset_count || a.name.localeCompare(b.name)
    case 'count-desc':
      return b.asset_count - a.asset_count || a.name.localeCompare(b.name)
    default:
      return a.sort - b.sort || a.name.localeCompare(b.name)
  }
}

function colorOf(collection: Collection): string {
  return libraryColorHex(collection.color)
}

function hasChildren(node: TreeNode): boolean {
  return (node.children?.length ?? 0) > 0
}

// The folder icon encodes state: plain folder for leaves, open folder for
// expanded parents, folder-plus for collapsed parents.
function folderIcon(node: TreeNode): Component {
  if (!hasChildren(node)) return Folder
  return expandedKeys.value[node.key] ? FolderOpen : FolderPlus
}

function toggleNodeKey(id: string): void {
  expandedKeys.value = { ...expandedKeys.value, [id]: !expandedKeys.value[id] }
}

function toggleNode(node: TreeNode): void {
  toggleNodeKey(node.key)
}

// The folder icon doubles as the expand/collapse control for parents; for
// leaves it falls through so the row still selects on click.
function onFolderClick(event: MouseEvent, node: TreeNode): void {
  if (!hasChildren(node)) return
  event.stopPropagation()
  toggleNode(node)
}

// A new sub-folder inherits its parent's colour; top-level folders default.
const draftColor = computed(() => {
  const parentId = createParent.value
  if (parentId) {
    const parent = library.collections.find((item) => item.id === parentId)
    if (parent?.color) return parent.color
  }
  return DEFAULT_COLLECTION_COLOR
})

function childrenOf(parentId: string | null): Collection[] {
  return library.collections
    .filter((collection) => (collection.parent_id ?? null) === parentId)
    .sort(compareCollections)
}

// descendantIds includes the folder itself and every nested subfolder.
function descendantIds(id: string): Set<string> {
  const result = new Set<string>([id])
  let grew = true
  while (grew) {
    grew = false
    for (const collection of library.collections) {
      if (
        collection.parent_id &&
        result.has(collection.parent_id) &&
        !result.has(collection.id)
      ) {
        result.add(collection.id)
        grew = true
      }
    }
  }
  return result
}

function replaceLocal(updated: Collection): void {
  const index = library.collections.findIndex((item) => item.id === updated.id)
  if (index >= 0) library.collections[index] = updated
  else library.collections.push(updated)
}

function clearActiveIf(gone: Set<string>): void {
  const active = activeCollection.value
  if (active && gone.has(active)) {
    const query = { ...route.query }
    delete query.collection
    void router.push({ name: 'all', query })
  }
}

function focusRename(id: string): void {
  nextTick(() => {
    const el = root.value?.querySelector<HTMLInputElement>(
      `[data-collection-rename="${id}"]`,
    )
    el?.focus()
    el?.select()
  })
}

// addCollection opens an inline draft row. Nothing is persisted until a name is
// entered, so cancelling or leaving it blank creates no folder at all.
function addCollection(parentId: string | null): void {
  if (!props.manage) return
  editingId.value = null
  createParent.value = parentId
  draftName.value = ''
  creating.value = true
  if (parentId) {
    expandedKeys.value = { ...expandedKeys.value, [parentId]: true }
  }
  nextTick(() => {
    root.value
      ?.querySelector<HTMLInputElement>('[data-collection-draft]')
      ?.focus()
  })
}

function cancelDraft(): void {
  creating.value = false
  draftName.value = ''
}

async function createCollection(
  name: string,
  parentId: string | null = null,
  color = DEFAULT_COLLECTION_COLOR,
): Promise<Collection | null> {
  if (!props.manage) return null
  const trimmed = name.trim()
  if (!trimmed) return null
  const existing = library.collections.find(
    (collection) =>
      (collection.parent_id ?? null) === parentId &&
      collection.name.trim().toLowerCase() === trimmed.toLowerCase(),
  )
  if (existing) return existing
  try {
    const collection = await api.createCollection(trimmed, color, parentId)
    library.collections.push(collection)
    if (props.selectable) {
      emit('update:selectedIds', [
        ...new Set([...props.selectedIds, collection.id]),
      ])
    }
    if (parentId) {
      expandedKeys.value = { ...expandedKeys.value, [parentId]: true }
    }
    refreshLibrary()
    return collection
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create folder',
      detail: toMessage(e),
      life: 4000,
    })
    return null
  }
}

async function commitDraft(): Promise<void> {
  if (!creating.value) return
  const parentId = createParent.value
  const names = splitNames(draftName.value)
  creating.value = false
  draftName.value = ''
  for (const name of names) {
    await createCollection(name, parentId, draftColor.value)
  }
}

function startRename(collection: Collection): void {
  menu.value?.hide()
  creating.value = false
  editingId.value = collection.id
  editingName.value = collection.name
  focusRename(collection.id)
}

function cancelRename(): void {
  editingId.value = null
}

async function commitRename(): Promise<void> {
  const id = editingId.value
  if (!id) return
  editingId.value = null
  const collection = library.collections.find((item) => item.id === id)
  const name = editingName.value.trim()
  if (!collection || !name || name === collection.name) return
  try {
    replaceLocal(
      await api.updateCollection(id, name, collection.color, collection.description),
    )
    refreshLibrary()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Rename failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function setColor(color: string): Promise<void> {
  const collection = contextCollection.value
  menu.value?.hide()
  if (!collection) return
  try {
    replaceLocal(
      await api.updateCollection(collection.id, collection.name, color, collection.description),
    )
    refreshLibrary()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Update failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

async function remove(collection: Collection): Promise<void> {
  menu.value?.hide()
  const gone = descendantIds(collection.id)
  try {
    await api.deleteCollection(collection.id)
    library.collections = library.collections.filter(
      (item) => !gone.has(item.id),
    )
    if (props.selectable) {
      emit(
        'update:selectedIds',
        props.selectedIds.filter((id) => !gone.has(id)),
      )
    }
    refreshLibrary()
    clearActiveIf(gone)
    toast.add({ severity: 'success', summary: 'Folder deleted', life: 2500 })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Delete failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function openMenu(event: MouseEvent, collection: Collection): void {
  contextCollection.value = collection
  menu.value?.show(event)
}

const contextHasChildren = computed(() => {
  const collection = contextCollection.value
  return (
    !!collection &&
    library.collections.some((item) => item.parent_id === collection.id)
  )
})

const contextExpanded = computed(() => {
  const collection = contextCollection.value
  return !!collection && !!expandedKeys.value[collection.id]
})

const menuItems = computed<TreeMenuItem[]>(() => {
  const collection = contextCollection.value
  if (!collection) return []
  const items: TreeMenuItem[] = [
    {
      label: 'Open',
      menuIcon: FolderOpen,
      command: () => openCollection(collection),
    },
    {
      label: isPinned('collection', collection.id)
        ? 'Unpin from Quicklinks'
        : 'Pin to Quicklinks',
      menuIcon: Bookmark,
      command: () =>
        toggleQuicklink({
          type: 'collection',
          id: collection.id,
          name: collection.name,
          color: collection.color ? `#${collection.color}` : undefined,
        }),
    },
    { separator: true },
  ]
  if (props.selectable) {
    items.push({
      label: selectedSet.value.has(collection.id) ? 'Deselect' : 'Select',
      menuIcon: Check,
      command: () => toggleSelection(collection.id),
    })
    if (contextHasChildren.value) {
      const ids = descendantIds(collection.id)
      const allSelected = [...ids].every((id) => selectedSet.value.has(id))
      items.push({
        label: allSelected ? 'Deselect all' : 'Select all',
        menuIcon: CheckCircle,
        command: () => {
          const next = new Set(props.selectedIds)
          if (allSelected) ids.forEach((id) => next.delete(id))
          else ids.forEach((id) => next.add(id))
          emit('update:selectedIds', [...next])
        },
      })
    }
  }
  items.push(
    {
      label: 'Rename',
      menuIcon: Pencil,
      command: () => startRename(collection),
    },
  )
  if (contextHasChildren.value) {
    items.push({
      label: contextExpanded.value ? 'Collapse' : 'Expand',
      menuIcon: contextExpanded.value ? ChevronUp : ChevronDown,
      command: () => toggleNodeKey(collection.id),
    })
  }
  items.push(
    {
      label: 'Color',
      menuIcon: Palette,
      items: LIBRARY_COLORS.map((option) => ({
        label: option.name,
        color: libraryColorHex(option.value),
        colorOption: true,
        command: () => void setColor(option.value),
      })),
    },
    { separator: true },
    {
      label: 'Create',
      menuIcon: Plus,
      command: () => void addCollection(collection.parent_id ?? null),
    },
    {
      label: 'Create subfolder',
      menuIcon: FolderOpen,
      command: () => void addCollection(collection.id),
    },
    { separator: true },
    confirmDeleteMenuItem('Delete', () => void remove(collection)),
  )
  return items
})

const SORT_OPTIONS: { value: SortMode; label: string }[] = [
  { value: 'custom', label: 'Custom order' },
  { value: 'name-asc', label: 'Name A–Z' },
  { value: 'name-desc', label: 'Name Z–A' },
  { value: 'count-desc', label: 'Most assets' },
  { value: 'count-asc', label: 'Fewest assets' },
]

function toggleSort(event: Event): void {
  sortPopover.value?.toggle(event)
}

function setSort(mode: SortMode): void {
  sortMode.value = mode
  sortPopover.value?.hide()
}

const parentCollections = computed(() =>
  library.collections.filter((collection) =>
    library.collections.some((item) => item.parent_id === collection.id),
  ),
)

const allExpanded = computed(() =>
  parentCollections.value.every((item) => expandedKeys.value[item.id]),
)

function toggleExpandAll(): void {
  const expand = !allExpanded.value
  const next = { ...expandedKeys.value }
  for (const parent of parentCollections.value) {
    next[parent.id] = expand
  }
  expandedKeys.value = next
}

function toggleSelection(id: string): void {
  const next = new Set(props.selectedIds)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  emit('update:selectedIds', [...next])
}

function onRowClick(node: TreeNode): void {
  if (!props.selectable) return
  if (node.data) toggleSelection(node.data.id)
}

function onRowDoubleClick(node: TreeNode): void {
  if (!props.manage || props.selectable || !node.data) return
  startRename(node.data)
}

function openCollection(collection: Collection): void {
  clearSmartSelection()
  const query: Record<string, string> = { collection: collection.id }
  if (typeof route.query.sort === 'string' && route.query.sort) {
    query.sort = route.query.sort
  }
  emit('open', collection)
  void router.push({ name: 'all', query })
}

function onSelect(node: TreeNode): void {
  if (props.selectable || !props.navigable) return
  if (editingId.value) return
  // Selecting a collection drops any smart-collection Inspector selection and
  // resets the other filters (locking comes later).
  clearSmartSelection()
  const query: Record<string, string> = { collection: node.key }
  if (typeof route.query.sort === 'string' && route.query.sort) {
    query.sort = route.query.sort
  }
  void router.push({ name: 'all', query })
}

async function onDrop(event: TreeNodeDropEvent): Promise<void> {
  if (!props.manage) return
  const drag = (event.dragNode as TreeNode | undefined)?.data as
    | Collection
    | undefined
  const drop = (event.dropNode as TreeNode | undefined)?.data as
    | Collection
    | undefined
  if (!drag || !drop || drag.id === drop.id) return
  const position = event.dropPosition
  let parentId: string | null
  let index: number
  if (position === 0) {
    parentId = drop.id
    index = childrenOf(parentId).filter((item) => item.id !== drag.id).length
  } else {
    parentId = drop.parent_id ?? null
    const siblings = childrenOf(parentId).filter((item) => item.id !== drag.id)
    const at = siblings.findIndex((item) => item.id === drop.id)
    index = position < 0 ? at : at + 1
  }
  try {
    await api.moveCollection(drag.id, parentId, index)
    applyLocalMove(drag, parentId, index)
    refreshLibrary()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Move failed',
      detail: toMessage(e),
      life: 4000,
    })
  }
}

function applyLocalMove(
  drag: Collection,
  parentId: string | null,
  index: number,
): void {
  const siblings = library.collections
    .filter(
      (item) =>
        item.id !== drag.id && (item.parent_id ?? null) === parentId,
    )
    .sort(compareCollections)
  drag.parent_id = parentId ?? undefined
  siblings.splice(Math.min(index, siblings.length), 0, drag)
  siblings.forEach((item, position) => {
    item.sort = position
  })
}

// treePt tags each row with its collection id so delegated events can resolve
// the whole row, drops the built-in toggle (the folder icon doubles as the
// expand/collapse control) and moves the row padding onto the slot content so
// the whole card can be styled.
const treePt = {
  nodeContent: (options: {
    context: { node: { data: { id?: string } | null } }
  }) => {
    const id = options.context.node.data?.id
    return {
      class: '!p-0',
      ...(id ? { 'data-collection-id': id } : {}),
    }
  },
  nodeToggleButton: { class: '!hidden' },
  nodeLabel: { class: '!min-w-0 !flex-1' },
  // Expose the parent's colour to the connector line drawn for its children.
  nodeChildren: (options: {
    instance?: { node?: { data?: { color?: string } | null } }
  }) => ({
    style: {
      '--guide-color': libraryColorHex(options?.instance?.node?.data?.color ?? ''),
    },
  }),
}

function collectionFromTarget(target: EventTarget | null): Collection | null {
  const el =
    target instanceof Element
      ? target.closest<HTMLElement>('[data-collection-id]')
      : null
  const id = el?.getAttribute('data-collection-id')
  if (!id) return null
  return library.collections.find((collection) => collection.id === id) ?? null
}

function onRootContextMenu(event: MouseEvent): void {
  if (!props.manage) return
  const collection = collectionFromTarget(event.target)
  if (!collection) return
  event.preventDefault()
  openMenu(event, collection)
}

function onAssetDragOver(event: DragEvent): void {
  if (!props.manage || !assetDrag.active) return
  const collection = collectionFromTarget(event.target)
  if (!collection) return
  // Take over from the Tree's own drag-and-drop while assets are dragged.
  event.preventDefault()
  event.stopPropagation()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dragOverId.value = collection.id
}

async function onAssetDrop(event: DragEvent): Promise<void> {
  if (!props.manage || !assetDrag.active) return
  const collection = collectionFromTarget(event.target)
  if (!collection) return
  event.preventDefault()
  event.stopPropagation()
  dragOverId.value = null
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
  if (!ids.length) return
  try {
    await api.addAssetsToCollection(collection.id, ids)
    refreshLibrary()
    toast.add({
      severity: 'success',
      summary: ids.length > 1 ? `Added ${ids.length} assets` : 'Added to folder',
      detail: collection.name,
      life: 2000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not add to folder',
      detail: toMessage(e),
      life: 4000,
    })
  }
}
defineExpose({
  addCollection,
  createCollection,
  toggleExpandAll,
  toggleSort,
  allExpanded,
})

</script>

<template>
  <div
    ref="root"
    @contextmenu="onRootContextMenu"
    @dragover.capture="onAssetDragOver"
    @drop.capture="onAssetDrop"
  >
    <div
      v-if="showHeader"
      class="sticky top-0 z-10 flex items-center justify-between bg-white dark:bg-slate-900 pt-2 pb-0 pr-3 pl-[1.375rem]"
    >
      <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
        Collections
        <template v-if="library.collections.length">
          ({{ library.collections.length }})
        </template>
      </span>
      <div class="flex items-center gap-1">
        <button
          class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
          title="Manage collections"
          @click="openCollectionsDialog"
        >
          <FolderOpen :size="14" />
        </button>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
          :title="allExpanded ? 'Collapse all' : 'Expand all'"
          @click="toggleExpandAll"
        >
          <AngleDoubleUp v-if="allExpanded" :size="14" />
          <AngleDoubleDown v-else :size="14" />
        </button>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
          title="Sort collections"
          @click="toggleSort"
        >
          <SortAlt :size="14" />
        </button>
        <button
          class="inline-flex text-slate-400 hover:text-violet-600 dark:hover:text-violet-400"
          title="New collection"
          @click="addCollection(null)"
        >
          <Plus :size="14" />
        </button>
      </div>
    </div>

    <Popover ref="sortPopover">
      <div class="w-56">
        <button
          v-for="option in SORT_OPTIONS"
          :key="option.value"
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm"
          :class="
            sortMode === option.value
              ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300'
              : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
          "
          @click="setSort(option.value)"
        >
          <span class="flex-1 text-left">{{ option.label }}</span>
          <Check v-if="sortMode === option.value" :size="12" class="shrink-0" />
        </button>
      </div>
    </Popover>

    <Tree
      :value="treeNodes"
      :selection-keys="selectionKeys"
      :selection-mode="selectable || !navigable ? undefined : 'single'"
      v-model:expanded-keys="expandedKeys"
      :draggable-nodes="manage"
      :droppable-nodes="manage"
      :validate-drop="true"
      :pt="treePt"
      class="!border-0 !bg-transparent !pt-1"
      @node-select="onSelect"
      @node-drop="onDrop"
    >
      <template #empty>
        <p class="px-2 py-6 text-center text-sm text-slate-400">
          {{ filter.trim() ? 'No matching collections' : 'No collections' }}
        </p>
      </template>
      <template #default="{ node }">
        <div
          class="flex w-full min-w-0 items-center gap-2 rounded-md py-1"
          :class="[
            'px-2',
            dragOverId === node.data?.id
              ? 'bg-slate-100 dark:bg-slate-800'
              : selectable && node.data && selectedSet.has(node.data.id)
                ? 'bg-violet-50/60 dark:bg-violet-950/25'
                : 'hover:bg-slate-100 dark:hover:bg-slate-800',
          ]"
          @click="onRowClick(node)"
          @dblclick="onRowDoubleClick(node)"
        >
          <template v-if="node.key === DRAFT_KEY">
            <component
              :is="folderIcon(node)"
              :size="20"
              class="shrink-0"
              :style="{ color: libraryColorHex(draftColor) }"
              @click="onFolderClick($event, node)"
              @dblclick.stop
            />
            <input
              v-model="draftName"
              data-collection-draft
              type="text"
              placeholder="New folder (use , or ; to batch)"
              class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none placeholder:text-slate-300 dark:placeholder:text-slate-500"
              @click.stop
              @keydown.stop
              @keydown.enter="commitDraft"
              @keydown.esc="cancelDraft"
              @blur="commitDraft"
            />
          </template>
          <template v-else>
            <component
              :is="folderIcon(node)"
              :size="20"
              class="shrink-0"
              :style="{ color: colorOf(node.data) }"
              @click="onFolderClick($event, node)"
              @dblclick.stop
            />
            <input
              v-if="editingId === node.data.id"
              v-model="editingName"
              :data-collection-rename="node.data.id"
              type="text"
              class="min-w-0 flex-1 border-b border-violet-400 bg-transparent p-0 text-sm leading-5 text-slate-800 dark:text-slate-100 outline-none"
              @click.stop
              @dblclick.stop
              @dragstart.stop.prevent
              @keydown.stop
              @keydown.enter="commitRename"
              @keydown.esc="cancelRename"
              @blur="commitRename"
            />
            <span
              v-else
              class="min-w-0 flex-1 truncate border-b border-transparent leading-5"
            >
              {{ node.data.name }}
            </span>
            <CheckCircle
              v-if="selectable && selectedSet.has(node.data.id)"
              :size="14"
              class="shrink-0 text-violet-600 dark:text-violet-400"
            />
            <span
              v-if="node.data.asset_count"
              class="shrink-0 text-xs text-slate-400"
            >
              {{ node.data.asset_count }}
            </span>
          </template>
        </div>
      </template>
    </Tree>

    <ContextMenu ref="menu" :model="menuItems">
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
          />
          <span class="flex-1">{{ slotProps.label }}</span>
          <Check
            v-if="
              slotProps.item.colorOption &&
              contextCollection?.color === slotProps.item.color.replace('#', '')
            "
            :size="12"
            class="shrink-0 text-violet-600 dark:text-violet-400"
          />
        </a>
      </template>
    </ContextMenu>
  </div>
</template>

<style scoped>
/* Tree guide lines: a dotted line drops from the parent folder icon centre and
   turns into a small L that stops just before the last child's icon. The colour
   is the parent folder's, exposed as --guide-color. */
:deep(.p-tree-node-children > .p-tree-node) {
  position: relative;
}

:deep(.p-tree-node-children > .p-tree-node)::before {
  content: '';
  position: absolute;
  left: -0.2rem;
  top: 0;
  bottom: -2px;
  width: 2px;
  background-image: repeating-linear-gradient(
    to bottom,
    var(--guide-color, transparent) 0 2px,
    transparent 2px 4px
  );
  opacity: 0.8;
}

:deep(.p-tree-node-children > .p-tree-node:first-child)::before {
  top: -0.5rem;
}

:deep(.p-tree-node-children > .p-tree-node:last-child)::before {
  bottom: 0.875rem;
}

:deep(.p-tree-node-children > .p-tree-node:last-child)::after {
  content: '';
  position: absolute;
  left: -0.2rem;
  top: 0.875rem;
  width: 0.7rem;
  height: 2px;
  background-image: repeating-linear-gradient(
    to right,
    var(--guide-color, transparent) 0 2px,
    transparent 2px 4px
  );
  opacity: 0.8;
}
</style>
