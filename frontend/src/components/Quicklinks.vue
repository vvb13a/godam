<script setup lang="ts">
import { Folder, Filter, Tag as TagIcon, Times } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { library } from '../lib/library'
import {
  quicklinks,
  toggleQuicklink,
  type Quicklink,
  type QuicklinkType,
} from '../lib/quicklinks'
import { clearSmartSelection, selectSmartCollection, smartSelection } from '../lib/smartSelection'
import type { SmartCollectionFilter } from '../types'

const route = useRoute()
const router = useRouter()

interface ResolvedQuicklink extends Quicklink {
  count?: number
}

const items = computed<ResolvedQuicklink[]>(() =>
  quicklinks.items.map((item) => {
    if (item.type === 'collection') {
      const collection = library.collections.find((c) => c.id === item.id)
      return {
        ...item,
        name: collection?.name ?? item.name,
        color: collection?.color ? `#${collection.color}` : item.color,
        count: collection?.asset_count,
      }
    }
    if (item.type === 'smart') {
      const smart = library.smartCollections.find((c) => c.id === item.id)
      return {
        ...item,
        name: smart?.name ?? item.name,
        color: smart?.color ? `#${smart.color}` : item.color,
        count: smart?.asset_count,
      }
    }
    const tag = library.tags.find((t) => t.id === item.id)
    const group = tag?.group_id
      ? library.tagGroups.find((g) => g.id === tag.group_id)
      : undefined
    return {
      ...item,
      name: tag?.name ?? item.name,
      color: group?.color ? `#${group.color}` : item.color,
      count: tag?.asset_count,
    }
  }),
)

function iconFor(type: QuicklinkType): Component {
  if (type === 'collection') return Folder
  if (type === 'smart') return Filter
  return TagIcon
}

function isActive(item: ResolvedQuicklink): boolean {
  if (item.type === 'collection') {
    return route.query.collection === item.id
  }
  if (item.type === 'tag') {
    return route.query.tag === item.id
  }
  return smartSelection.id === item.id
}

function navigateFilter(filter: SmartCollectionFilter): void {
  const query: Record<string, string> = {}
  if (filter.collection) query.collection = filter.collection
  if (filter.tag) query.tag = filter.tag
  if (filter.type) query.type = filter.type
  if (filter.sort) query.sort = filter.sort
  void router.push({ name: 'all', query })
}

function open(item: ResolvedQuicklink): void {
  if (item.type === 'collection') {
    clearSmartSelection()
    void router.push({ name: 'all', query: { collection: item.id } })
    return
  }
  if (item.type === 'tag') {
    clearSmartSelection()
    void router.push({ name: 'all', query: { tag: item.id } })
    return
  }
  const smart = library.smartCollections.find((c) => c.id === item.id)
  if (!smart) return
  selectSmartCollection(smart.id, smart.filter)
  navigateFilter(smart.filter)
}

function unpin(item: Quicklink): void {
  toggleQuicklink(item)
}
</script>

<template>
  <div>
    <div
      class="sticky top-0 z-10 flex items-center justify-between bg-white pt-3 pb-0 pr-3 pl-[1.375rem] dark:bg-slate-900"
    >
      <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
        Quicklinks
        <template v-if="items.length">({{ items.length }})</template>
      </span>
    </div>

    <div class="space-y-0.5 px-[0.875rem] py-1">
      <p
        v-if="!items.length"
        class="px-2 py-2 text-xs leading-relaxed text-slate-400"
      >
        Pin collections, smart collections or tags from their context menus.
      </p>
      <div
        v-for="item in items"
        :key="`${item.type}:${item.id}`"
        class="group flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm select-none"
        :class="
          isActive(item)
            ? 'bg-violet-50 text-violet-700 dark:bg-violet-950/50 dark:text-violet-300'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white'
        "
        @click="open(item)"
      >
        <component
          :is="iconFor(item.type)"
          :size="16"
          class="shrink-0"
          :style="item.color ? { color: item.color } : undefined"
        />
        <span class="min-w-0 flex-1 truncate">{{ item.name }}</span>
        <span v-if="item.count !== undefined" class="shrink-0 text-xs text-slate-400">
          {{ item.count }}
        </span>
        <button
          type="button"
          class="inline-flex shrink-0 text-slate-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-red-500"
          title="Unpin"
          @click.stop="unpin(item)"
        >
          <Times :size="12" />
        </button>
      </div>
    </div>
  </div>
</template>
