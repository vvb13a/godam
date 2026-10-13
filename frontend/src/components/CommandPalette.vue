<script setup lang="ts">
import CommandMenu from 'primevue/commandmenu'
import type { CommandMenuItem } from 'primevue/commandmenu'
import Dialog from 'primevue/dialog'
import { Folder, Images, Tag, Trash } from '@primeicons/vue'
import { computed, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { commandMenu } from '../lib/command'
import { library } from '../lib/library'
import { openTagsDialog } from '../lib/tagsDialog'
import Kbd from './Kbd.vue'

const router = useRouter()

const commandRef = ref<{ $el?: HTMLElement } | null>(null)

function focusInput(): void {
  commandRef.value?.$el?.querySelector<HTMLInputElement>('input')?.focus()
}

function run(to: RouteLocationRaw): void {
  commandMenu.visible = false
  void router.push(to)
}

const model = computed<CommandMenuItem[]>(() => {
  const items: CommandMenuItem[] = [
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
    items.push({
      label: 'Collections',
      items: library.collections.map((collection) => ({
        label: collection.name,
        keywords: ['collection'],
        command: () =>
          run({ name: 'all', query: { collection: collection.id } }),
      })),
    })
  }

  if (library.tags.length) {
    items.push({
      label: 'Tags',
      items: library.tags.map((tag) => ({
        label: tag.name,
        keywords: ['tag'],
        command: () => run({ name: 'all', query: { tag: tag.id } }),
      })),
    })
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
