<script setup lang="ts">
import { Folder, Images, Search, Tag, Tags, Trash } from '@primeicons/vue'
import { computed, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { openCommandMenu } from '../lib/command'
import { library, refreshLibrary } from '../lib/library'
import { openTagsDialog } from '../lib/tagsDialog'
import CollectionTree from './CollectionTree.vue'
import Kbd from './Kbd.vue'
import LibrarySwitcher from './LibrarySwitcher.vue'
import SmartCollections from './SmartCollections.vue'
import UserMenu from './UserMenu.vue'

const route = useRoute()

const activeCollection = computed(
  () => (route.query.collection as string | undefined) ?? null,
)
const activeTag = computed(() => (route.query.tag as string | undefined) ?? null)
const allActive = computed(
  () => route.name === 'all' && !activeCollection.value && !activeTag.value,
)
const uncategorizedActive = computed(
  () => route.name === 'all' && activeCollection.value === 'uncategorized',
)
const untaggedActive = computed(
  () => route.name === 'all' && activeTag.value === 'untagged',
)

watch(
  () => library.version,
  () => {
    void refreshLibrary()
  },
)
</script>

<template>
  <aside class="flex w-64 shrink-0 flex-col border-r border-slate-200 bg-white">
    <div class="space-y-2 p-3 pb-0">
      <LibrarySwitcher />
      <button
        class="flex w-full items-center gap-2 rounded border border-slate-200 px-3 py-2 text-sm text-slate-500 hover:border-slate-300 hover:text-slate-700"
        @click="openCommandMenu"
      >
        <Search :size="14" />
        <span class="flex-1 text-left">Search</span>
        <Kbd>⌘K</Kbd>
      </button>
    </div>

    <nav class="space-y-1 p-3">
      <RouterLink
        :to="{ name: 'all' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          allActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="18" />
        <span class="flex-1">All</span>
        <span
          v-if="library.stats?.assets"
          class="shrink-0 text-xs text-slate-400"
        >
          {{ library.stats.assets }}
        </span>
      </RouterLink>
      <RouterLink
        :to="{ name: 'all', query: { collection: 'uncategorized' } }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          uncategorizedActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Folder :size="18" />
        <span class="flex-1">Uncategorized</span>
        <span
          v-if="library.stats?.uncategorized"
          class="shrink-0 text-xs text-slate-400"
        >
          {{ library.stats.uncategorized }}
        </span>
      </RouterLink>
      <RouterLink
        :to="{ name: 'all', query: { tag: 'untagged' } }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          untaggedActive
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Tag :size="18" />
        <span class="flex-1">Untagged</span>
        <span
          v-if="library.stats?.untagged"
          class="shrink-0 text-xs text-slate-400"
        >
          {{ library.stats.untagged }}
        </span>
      </RouterLink>
      <button
        class="flex w-full items-center gap-2 rounded px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900"
        @click="openTagsDialog"
      >
        <Tags :size="18" />
        <span class="flex-1 text-left">Tags</span>
      </button>
      <RouterLink
        :to="{ name: 'trash' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          route.name === 'trash'
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Trash :size="18" />
        <span class="flex-1">Trash</span>
      </RouterLink>
    </nav>

    <div class="min-h-0 flex-1 overflow-y-auto pb-2">
      <CollectionTree />
      <SmartCollections />
    </div>

    <UserMenu />
  </aside>
</template>
