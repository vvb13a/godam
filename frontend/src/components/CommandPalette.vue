<script setup lang="ts">
import CommandMenu from 'primevue/commandmenu'
import type { CommandMenuItem } from 'primevue/commandmenu'
import Dialog from 'primevue/dialog'
import {
  CloudUpload,
  Cog,
  Filter,
  Folder,
  FolderOpen,
  Images,
  Palette,
  SignOut,
  SlidersH,
  Tag,
  Trash,
  UserEdit,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, nextTick, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { logout } from '../lib/auth'
import { openBackupSettings } from '../lib/backupSettings'
import { commandMenu } from '../lib/command'
import { library } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import { SIDEBAR_TOGGLES, sidebarPrefs } from '../lib/sidebar'
import { clearSmartSelection, selectSmartCollection } from '../lib/smartSelection'
import { openTagsDialog } from '../lib/tagsDialog'
import { THEME_OPTIONS, setThemeMode } from '../lib/theme'
import { openUploadSettings } from '../lib/uploadSettings'
import { openUserSettings } from '../lib/userSettings'
import type { SmartCollectionFilter } from '../types'
import Kbd from './Kbd.vue'

type Scope = 'all' | 'commands' | 'collections' | 'tags' | 'smart'

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

const commandRef = ref<{ $el?: HTMLElement } | null>(null)
const scope = ref<Scope>('all')
const search = ref('')

const scopes: { value: Scope; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'commands', label: 'Commands' },
  { value: 'collections', label: 'Collections' },
  { value: 'tags', label: 'Tags' },
  { value: 'smart', label: 'Smart' },
]

function focusInput(): void {
  commandRef.value?.$el?.querySelector<HTMLInputElement>('input')?.focus()
}

function setScope(value: Scope): void {
  scope.value = value
  search.value = ''
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
  const query: Record<string, string> = {}
  if (filter.collection) query.collection = filter.collection
  if (filter.tag) query.tag = filter.tag
  if (filter.type) query.type = filter.type
  if (filter.sort) query.sort = filter.sort
  void router.push({ name: 'all', query })
}

const groups = computed<PaletteGroup[]>(() => {
  const commands: PaletteItem[] = [
    {
      label: 'Manage tags',
      icon: Tag,
      keywords: ['tags', 'manage', 'create tag'],
      command: () => {
        close()
        openTagsDialog()
      },
    },
    {
      label: 'Open upload settings',
      icon: Cog,
      keywords: ['upload', 'settings', 'mime', 'size'],
      command: () => {
        close()
        openUploadSettings()
      },
    },
    {
      label: 'Open storage & backup settings',
      icon: CloudUpload,
      keywords: ['backup', 'storage', 'sftp', 'settings'],
      command: () => {
        close()
        openBackupSettings()
      },
    },
    {
      label: 'Open user settings',
      icon: UserEdit,
      keywords: ['user', 'account', 'profile', 'password'],
      command: () => {
        close()
        openUserSettings()
      },
    },
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
    {
      label: 'Sign out',
      icon: SignOut,
      keywords: ['logout', 'sign out', 'exit'],
      command: async () => {
        close()
        await logout()
        await router.push({ name: 'login' })
      },
    },
  ]

  const result: PaletteGroup[] = [
    {
      label: 'Navigation',
      scope: 'commands',
      items: [
        {
          label: 'All assets',
          icon: Images,
          keywords: ['all', 'library', 'assets', 'everything'],
          command: () => run({ name: 'all' }),
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
        command: () =>
          run({ name: 'all', query: { collection: collection.id } }),
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
          command: () => run({ name: 'all', query: { tag: tag.id } }),
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
          selectSmartCollection(smart.id, smart.filter)
          navigateFilter(smart.filter)
        },
      })),
    })
  }

  return result
})

const model = computed<PaletteGroup[]>(() => {
  if (scope.value === 'all') return groups.value
  return groups.value.filter((group) => group.scope === scope.value)
})
</script>

<template>
  <Dialog
    v-model:visible="commandMenu.visible"
    modal
    :show-header="false"
    :draggable="false"
    :style="{ width: '44rem' }"
    :pt="{ content: { class: '!p-0' } }"
    @show="onShow"
  >
    <div
      @keydown.tab.prevent.stop="cycleScope"
      @keydown.esc.prevent="close()"
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
              ? 'bg-violet-50 text-violet-700 dark:bg-violet-950/50 dark:text-violet-300'
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
        empty-filter-message="No matching results."
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
</template>
