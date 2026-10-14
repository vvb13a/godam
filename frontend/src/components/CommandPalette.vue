<script setup lang="ts">
import CommandMenu from 'primevue/commandmenu'
import type { CommandMenuItem } from 'primevue/commandmenu'
import Dialog from 'primevue/dialog'
import { Folder, FolderOpen, Images, Tag, Trash } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { commandMenu } from '../lib/command'
import { library } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import { openTagsDialog } from '../lib/tagsDialog'
import Kbd from './Kbd.vue'

interface PaletteItem extends CommandMenuItem {
  folderIcon?: Component
  color?: string
  dot?: string
}

const router = useRouter()

const commandRef = ref<{ $el?: HTMLElement } | null>(null)

function focusInput(): void {
  commandRef.value?.$el?.querySelector<HTMLInputElement>('input')?.focus()
}

function run(to: RouteLocationRaw): void {
  commandMenu.visible = false
  void router.push(to)
}

const model = computed<PaletteItem[]>(() => {
  const items: PaletteItem[] = [
    {
      label: 'Navigation',
      items: [
        {
          label: 'All',
          icon: Images,
          keywords: ['all', 'library', 'assets', 'everything'],
          command: () => run({ name: 'all' }),
        },
        {
          label: 'Uncategorized',
          icon: Folder,
          keywords: ['uncategorized', 'no collection'],
          command: () =>
            run({ name: 'all', query: { collection: 'uncategorized' } }),
        },
        {
          label: 'Untagged',
          icon: Tag,
          keywords: ['untagged', 'no tags'],
          command: () => run({ name: 'all', query: { tag: 'untagged' } }),
        },
        {
          label: 'Trash',
          icon: Trash,
          keywords: ['trash', 'deleted'],
          command: () => run({ name: 'trash' }),
        },
      ],
    },
    {
      label: 'Organization',
      items: [
        {
          label: 'Manage tags',
          icon: Tag,
          keywords: ['tags', 'manage', 'create tag'],
          command: () => {
            commandMenu.visible = false
            openTagsDialog()
          },
        },
      ],
    },
  ]

  if (library.collections.length) {
    const collectionItems: PaletteItem[] = library.collections.map(
      (collection) => ({
        label: collection.name,
        keywords: ['collection'],
        folderIcon: library.collections.some(
          (item) => item.parent_id === collection.id,
        )
          ? FolderOpen
          : Folder,
        color: libraryColorHex(collection.color),
        command: () =>
          run({ name: 'all', query: { collection: collection.id } }),
      }),
    )
    items.push({ label: 'Collections', items: collectionItems })
  }

  if (library.tags.length) {
    const tagItems: PaletteItem[] = library.tags.map((tag) => {
      const group = tag.group_id
        ? library.tagGroups.find((item) => item.id === tag.group_id)
        : undefined
      return {
        label: tag.name,
        keywords: ['tag'],
        dot: group?.color ? `#${group.color}` : '#94a3b8',
        command: () => run({ name: 'all', query: { tag: tag.id } }),
      }
    })
    items.push({ label: 'Tags', items: tagItems })
  }

  return items
})
</script>

<template>
  <Dialog
    v-model:visible="commandMenu.visible"
    modal
    :show-header="false"
    :draggable="false"
    :style="{ width: '32rem' }"
    :pt="{ content: { class: '!p-0' } }"
    @show="focusInput"
  >
    <CommandMenu
      ref="commandRef"
      :model="model"
      placeholder="Type a command or search…"
      aria-label="Command menu"
      empty-filter-message="No matching commands."
    >
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm"
          @click.prevent
        >
          <span
            v-if="slotProps.item.dot"
            class="h-2.5 w-2.5 shrink-0 rounded-full ring-1 ring-slate-300"
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
        <div class="flex items-center gap-2 text-slate-400">
          <Kbd>↑</Kbd>
          <Kbd>↓</Kbd>
          <Kbd>↵</Kbd>
        </div>
      </template>
    </CommandMenu>
  </Dialog>
</template>
