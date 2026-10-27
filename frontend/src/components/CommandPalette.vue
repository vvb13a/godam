<script setup lang="ts">
import CommandMenu from 'primevue/commandmenu'
import type { CommandMenuItem } from 'primevue/commandmenu'
import ConfirmDialog from 'primevue/confirmdialog'
import Dialog from 'primevue/dialog'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import {
  CloudUpload,
  Cog,
  Filter,
  Folder,
  FolderOpen,
  FolderPlus,
  Images,
  ListCheck,
  Palette,
  Search,
  SlidersH,
  Tag,
  Times,
  Trash,
  Upload,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import * as api from '../api/client'
import { selectedIds } from '../lib/assetSelection'
import { attributeCatalog, loadAttributeCatalog } from '../lib/attributeCatalog'
import { effectiveAttributeSort, sortAttributeValues } from '../lib/attributeSort'
import { notifyAttributesChanged, openAttributesDialog } from '../lib/attributesDialog'
import { openBackupSettings } from '../lib/backupSettings'
import { notifyConversionCacheChanged } from '../lib/conversionCache'
import { openCollectionsDialog } from '../lib/collectionsDialog'
import { hasCapability } from '../lib/capabilities'
import { commandMenu } from '../lib/command'
import {
  applyCollectionFilter,
  applyTagFilter,
  clearAllFilters,
  filterState,
} from '../lib/filters'
import { library, loadStats, notifyLibraryChanged } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import { openPresetsDialog } from '../lib/presetsDialog'
import { SIDEBAR_TOGGLES, sidebarPrefs } from '../lib/sidebar'
import { clearSmartSelection, selectSmartCollection } from '../lib/smartSelection'
import { openTagsDialog } from '../lib/tagsDialog'
import { THEME_OPTIONS, setThemeMode } from '../lib/theme'
import { openUpload } from '../lib/upload'
import { openUploadSettings } from '../lib/uploadSettings'
import { ATTRIBUTE_UNASSIGNED } from '../types'
import type { SmartCollectionFilter } from '../types'
import Kbd from './Kbd.vue'

type Scope =
  | 'all'
  | 'commands'
  | 'collections'
  | 'tags'
  | 'smart'
  | 'attributes'
  | 'create'

interface PaletteItem extends CommandMenuItem {
  folderIcon?: Component
  color?: string
  dot?: string
}

interface PaletteGroup extends CommandMenuItem {
  scope: Exclude<Scope, 'all'>
  items: PaletteItem[]
}

const router = useRouter()
const toast = useToast()
const confirm = useConfirm()

const commandRef = ref<{ $el?: HTMLElement } | null>(null)
const scope = ref<Scope>('all')
const search = ref('')
void loadAttributeCatalog()

const scopes: { value: Scope; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'commands', label: 'Commands' },
  { value: 'collections', label: 'Collections' },
  { value: 'tags', label: 'Tags' },
  { value: 'smart', label: 'Smart' },
  { value: 'attributes', label: 'Attributes' },
  { value: 'create', label: 'Create' },
]

function focusInput(): void {
  commandRef.value?.$el?.querySelector<HTMLInputElement>('input')?.focus()
}

function setScope(value: Scope): void {
  scope.value = value
  void nextTick(focusInput)
}

function cycleScope(event: KeyboardEvent): void {
  const index = scopes.findIndex((item) => item.value === scope.value)
  const direction = event.shiftKey ? -1 : 1
  const next = (index + direction + scopes.length) % scopes.length
  setScope(scopes[next].value)
}

function onShow(): void {
  scope.value = 'all'
  search.value = ''
  void nextTick(focusInput)
}

function run(to: RouteLocationRaw): void {
  commandMenu.visible = false
  clearSmartSelection()
  void router.push(to)
}

function close(): void {
  commandMenu.visible = false
}

function navigateFilter(filter: SmartCollectionFilter): void {
  clearAllFilters()
  if (filter.collection) filterState.collections = [filter.collection]
  if (filter.tag) filterState.tags = [filter.tag]
  if (filter.type) filterState.types = [filter.type]
  filterState.sort = filter.sort || 'date-desc'
  void router.push({ name: 'library' })
}

const hasFilters = computed(
  () =>
    filterState.collections.length > 0 ||
    filterState.tags.length > 0 ||
    filterState.types.length > 0,
)

function currentCollectionId(): string | null {
  return filterState.collections.length === 1
    ? filterState.collections[0]
    : null
}

function currentTagId(): string | null {
  return filterState.tags.length === 1 ? filterState.tags[0] : null
}

function currentFilter(): SmartCollectionFilter {
  const filter: SmartCollectionFilter = {}
  if (filterState.collections.length === 1) {
    filter.collection = filterState.collections[0]
  }
  if (filterState.tags.length === 1) filter.tag = filterState.tags[0]
  if (filterState.types.length === 1) filter.type = filterState.types[0]
  if (filterState.sort !== 'date-desc') filter.sort = filterState.sort
  return filter
}

async function createCollection(name: string): Promise<void> {
  close()
  try {
    await api.createCollection(name, 'f97316', null)
    notifyLibraryChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create collection',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
}

async function createTag(name: string): Promise<void> {
  close()
  try {
    await api.createTag(name)
    notifyLibraryChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create tag',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
}

async function createSmartCollection(name: string): Promise<void> {
  close()
  try {
    await api.createSmartCollection(name, 'f97316', currentFilter())
    notifyLibraryChanged()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not create smart collection',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
}

async function purgeCache(): Promise<void> {
  try {
    const result = await api.purgeConversions()
    notifyConversionCacheChanged()
    await loadStats()
    toast.add({
      severity: 'success',
      summary: 'Conversion cache purged',
      detail: `${result.removed} file${result.removed === 1 ? '' : 's'} removed`,
      life: 3000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not purge the conversion cache',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
}

async function applyBulkAttribute(
  key: string,
  values: string[],
  mode: 'add' | 'replace',
): Promise<void> {
  const ids = selectedIds.value.slice()
  if (!ids.length) return
  try {
    await api.bulkSetAssetAttributeValues(ids, key, values, mode)
    notifyLibraryChanged()
    notifyAttributesChanged()
    toast.add({
      severity: 'success',
      summary:
        mode === 'add'
          ? `Added attribute to ${ids.length} asset${ids.length === 1 ? '' : 's'}`
          : `Updated attributes on ${ids.length} asset${ids.length === 1 ? '' : 's'}`,
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update attributes',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  }
}

function attributePaletteItems(
  mode: 'add' | 'replace',
): PaletteItem[] {
  const items: PaletteItem[] = []
  for (const facet of attributeCatalog.items) {
    const label = facet.label || facet.key
    const values = sortAttributeValues(
      facet.values.filter((value) => value.value !== ATTRIBUTE_UNASSIGNED),
      effectiveAttributeSort(facet.key, facet.sort),
    )
    if (mode === 'replace' && values.length) {
      items.push({
        label: `Clear ${label}`,
        icon: Times,
        keywords: ['attributes', 'clear', label, facet.key],
        command: () => void applyBulkAttribute(facet.key, [], 'replace'),
      })
    }
    for (const value of values) {
      const valueLabel = value.label || value.value
      items.push({
        label:
          mode === 'add'
            ? `Add ${valueLabel} → ${label}`
            : `Set ${label} = ${valueLabel}`,
        icon: ListCheck,
        keywords: [
          'attributes',
          mode,
          label,
          facet.key,
          valueLabel,
          value.value,
        ],
        command: () =>
          void applyBulkAttribute(facet.key, [value.value], mode),
      })
    }
  }
  return items
}

const groups = computed<PaletteGroup[]>(() => {
  const commands: PaletteItem[] = []
  const term = search.value.trim()
  if (term) {
    commands.push({
      label: `Search assets for "${term}"`,
      icon: Search,
      keywords: [term, 'search', 'find', 'assets'],
      command: () => {
        filterState.search = term
        run({ name: 'library' })
      },
    })
  }
  if (hasCapability('assets:upload')) {
    commands.push({
      label: 'Upload assets',
      icon: Upload,
      keywords: ['upload', 'add', 'import', 'files', 'assets'],
      command: () => {
        close()
        openUpload([], currentCollectionId(), currentTagId())
      },
    })
  }
  if (hasCapability('library:update')) {
    commands.push({
      label: 'Manage collections',
      icon: Folder,
      keywords: ['collections', 'folders', 'manage', 'select'],
      command: () => {
        close()
        openCollectionsDialog()
      },
    })
    commands.push({
      label: 'Manage tags',
      icon: Tag,
      keywords: ['tags', 'manage', 'create tag'],
      command: () => {
        close()
        openTagsDialog()
      },
    })
  }
  if (hasCapability('settings:attributes')) {
    commands.push({
      label: 'Manage attributes',
      icon: Cog,
      keywords: ['attributes', 'manage', 'rename', 'sort', 'labels'],
      command: () => {
        close()
        openAttributesDialog()
      },
    })
  }
  if (hasCapability('settings:presets')) {
    commands.push({
      label: 'Manage image presets',
      icon: Images,
      keywords: ['presets', 'imgproxy', 'conversions', 'image', 'manipulations'],
      command: () => {
        close()
        openPresetsDialog()
      },
    })
  }
  if (hasCapability('conversions:purge')) {
    commands.push({
      label: 'Purge conversion cache',
      icon: Trash,
      keywords: ['cache', 'purge', 'conversions', 'clean', 'delete'],
      command: () => {
        close()
        confirm.require({
          group: 'cache',
          header: 'Purge conversion cache?',
          message:
            'Every generated conversion will be deleted. Originals and preset definitions are kept.',
          acceptLabel: 'Purge',
          rejectLabel: 'Cancel',
          acceptProps: { severity: 'danger' },
          rejectProps: { text: true },
          accept: () => void purgeCache(),
        })
      },
    })
  }
  if (hasCapability('settings:upload')) {
    commands.push({
      label: 'Open upload settings',
      icon: Cog,
      keywords: ['upload', 'settings', 'mime', 'size'],
      command: () => {
        close()
        openUploadSettings()
      },
    })
  }
  if (hasCapability('settings:storage')) {
    commands.push({
      label: 'Open storage & backup settings',
      icon: CloudUpload,
      keywords: ['backup', 'storage', 'sftp', 'settings'],
      command: () => {
        close()
        openBackupSettings()
      },
    })
  }
  commands.push(
    ...SIDEBAR_TOGGLES.map((toggle) => ({
      label: `${sidebarPrefs[toggle.key] ? 'Hide' : 'Show'} ${toggle.label}`,
      icon: SlidersH,
      keywords: ['sidebar', 'panel', 'toggle', toggle.label],
      command: () => {
        close()
        sidebarPrefs[toggle.key] = !sidebarPrefs[toggle.key]
      },
    })),
    ...THEME_OPTIONS.map((option) => ({
      label: `Theme: ${option.label}`,
      icon: Palette,
      keywords: ['theme', 'appearance', option.label],
      command: () => {
        close()
        setThemeMode(option.value)
      },
    })),
  )

  const result: PaletteGroup[] = [
    {
      label: 'Navigation',
      scope: 'commands',
      items: [
        {
          label: 'All assets',
          icon: Images,
          keywords: ['all', 'library', 'assets', 'everything'],
          command: () => {
            clearAllFilters()
            run({ name: 'library' })
          },
        },
        {
          label: 'Trash',
          icon: Trash,
          keywords: ['trash', 'deleted'],
          command: () => run({ name: 'trash' }),
        },
      ],
    },
    { label: 'Commands', scope: 'commands', items: commands },
  ]

  if (library.collections.length) {
    result.push({
      label: 'Collections',
      scope: 'collections',
      items: library.collections.map((collection) => ({
        label: collection.name,
        keywords: ['collection', 'folder'],
        folderIcon: library.collections.some(
          (item) => item.parent_id === collection.id,
        )
          ? FolderOpen
          : Folder,
        color: libraryColorHex(collection.color),
        command: () => {
          applyCollectionFilter(collection.id)
          run({ name: 'library' })
        },
      })),
    })
  }

  if (library.tags.length) {
    result.push({
      label: 'Tags',
      scope: 'tags',
      items: library.tags.map((tag) => {
        const group = tag.group_id
          ? library.tagGroups.find((item) => item.id === tag.group_id)
          : undefined
        return {
          label: tag.name,
          keywords: ['tag'],
          dot: group?.color ? `#${group.color}` : '#94a3b8',
          command: () => {
            applyTagFilter(tag.id)
            run({ name: 'library' })
          },
        }
      }),
    })
  }

  if (library.smartCollections.length) {
    result.push({
      label: 'Smart Collections',
      scope: 'smart',
      items: library.smartCollections.map((smart) => ({
        label: smart.name,
        icon: Filter,
        keywords: ['smart', 'filter'],
        color: libraryColorHex(smart.color),
        command: () => {
          close()
          navigateFilter(smart.filter)
          selectSmartCollection(smart.id, smart.filter)
        },
      })),
    })
  }

  if (
    hasCapability('attributes:select') &&
    selectedIds.value.length &&
    attributeCatalog.items.length
  ) {
    const addItems = attributePaletteItems('add')
    const replaceItems = attributePaletteItems('replace')
    if (addItems.length) {
      result.push({
        label: 'Attributes · Add',
        scope: 'attributes',
        items: addItems,
      })
    }
    if (replaceItems.length) {
      result.push({
        label: 'Attributes · Replace',
        scope: 'attributes',
        items: replaceItems,
      })
    }
  }

  return result
})

const model = computed<PaletteGroup[]>(() => {
  if (scope.value === 'create') {
    const value = search.value.trim()
    if (!value) return []
    const items: PaletteItem[] = [
      {
        label: `Create collection "${value}"`,
        icon: FolderPlus,
        keywords: [value, 'create', 'new', 'collection'],
        command: () => void createCollection(value),
      },
      {
        label: `Create tag "${value}"`,
        icon: Tag,
        keywords: [value, 'create', 'new', 'tag'],
        command: () => void createTag(value),
      },
    ]
    if (hasFilters.value) {
      items.push({
        label: `Create smart collection "${value}"`,
        icon: Filter,
        keywords: [value, 'create', 'new', 'smart', 'filter'],
        command: () => void createSmartCollection(value),
      })
    }
    return [{ label: 'Create', scope: 'create', items }]
  }
  if (scope.value === 'all') return groups.value
  return groups.value.filter((group) => group.scope === scope.value)
})

const emptyMessage = computed(() =>
  scope.value === 'create'
    ? 'Type a name to create a collection, tag, or smart collection.'
    : 'No results.',
)

const emptyFilterMessage = computed(() =>
  scope.value === 'create'
    ? 'Type a name to create something.'
    : 'No matching results.',
)
</script>

<template>
  <Dialog
    v-model:visible="commandMenu.visible"
    modal
    dismissable-mask
    :show-header="false"
    :draggable="false"
    :style="{ width: '44rem' }"
    :pt="{ content: { class: '!p-0' } }"
    @show="onShow"
  >
    <div
      @keydown.tab.prevent.stop="cycleScope"
      @keydown.esc.prevent="close()"
      @keydown.space.capture.stop
    >
      <div
        class="flex items-center gap-1 border-b border-slate-100 px-3 py-2 dark:border-slate-800"
      >
        <button
          v-for="item in scopes"
          :key="item.value"
          type="button"
          class="rounded-md px-2 py-1 text-xs font-medium transition-colors"
          :class="
            scope === item.value
              ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/50 dark:text-brand-300'
              : 'text-slate-500 hover:bg-slate-100 hover:text-slate-800 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100'
          "
          @click="setScope(item.value)"
        >
          {{ item.label }}
        </button>
        <span class="ml-auto flex items-center gap-1.5 text-[11px] text-slate-400">
          <Kbd>Tab</Kbd>
          <span>Switch</span>
        </span>
      </div>

      <CommandMenu
        ref="commandRef"
        v-model:search="search"
        :model="model"
        placeholder="Type a command or search…"
        aria-label="Command menu"
        :empty-message="emptyMessage"
        :empty-filter-message="emptyFilterMessage"
      >
        <template #item="slotProps">
          <a
            data-pc-section="itemlink"
            class="flex cursor-pointer items-center gap-2 px-3 py-1 text-sm"
            @click.prevent
          >
            <span
              v-if="slotProps.item.dot"
              class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300 dark:ring-slate-700"
              :style="{ backgroundColor: slotProps.item.dot }"
            />
            <component
              :is="slotProps.item.folderIcon || slotProps.item.icon"
              v-else-if="slotProps.item.folderIcon || slotProps.item.icon"
              :size="14"
              class="shrink-0"
              :style="
                slotProps.item.color ? { color: slotProps.item.color } : undefined
              "
            />
            <span class="flex-1">{{ slotProps.label }}</span>
          </a>
        </template>
        <template #footer>
          <div class="flex items-center gap-3 text-[11px] text-slate-400">
            <span class="flex items-center gap-1.5">
              <Kbd>↑</Kbd>
              <Kbd>↓</Kbd>
              <span>Navigate</span>
            </span>
            <span class="flex items-center gap-1.5">
              <Kbd>↵</Kbd>
              <span>Open</span>
            </span>
            <span class="flex items-center gap-1.5">
              <Kbd>Tab</Kbd>
              <span>Scope</span>
            </span>
            <span class="ml-auto flex items-center gap-1.5">
              <Kbd>Esc</Kbd>
              <span>Close</span>
            </span>
          </div>
        </template>
      </CommandMenu>
    </div>
  </Dialog>
  <ConfirmDialog group="cache" />
</template>
