<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import type { MenuItem } from 'primevue/menuitem'
import { ExternalLink, Filter, Times } from '@primeicons/vue'
import type { Component } from 'vue'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { clearAllFilters, filterState } from '../lib/filters'
import { library } from '../lib/library'
import {
  quicklinks,
  toggleQuicklink,
  type Quicklink,
  type QuicklinkType,
} from '../lib/quicklinks'
import { selectSmartCollection, smartSelection } from '../lib/smartSelection'
import type { SmartCollectionFilter } from '../types'

const router = useRouter()
const menu = ref<InstanceType<typeof ContextMenu> | null>(null)
const contextItem = ref<ResolvedQuicklink | null>(null)

interface ResolvedQuicklink extends Quicklink {
  count?: number
}

const items = computed<ResolvedQuicklink[]>(() =>
  quicklinks.items.map((item) => {
    const smart = library.smartCollections.find((c) => c.id === item.id)
    return {
      ...item,
      name: smart?.name ?? item.name,
      color: smart?.color ? `#${smart.color}` : item.color,
      count: smart?.asset_count,
    }
  }),
)

function iconFor(_type: QuicklinkType): Component {
  return Filter
}

function isActive(item: ResolvedQuicklink): boolean {
  return smartSelection.id === item.id
}

function navigateFilter(filter: SmartCollectionFilter): void {
  clearAllFilters()
  if (filter.type) filterState.types = [filter.type]
  filterState.sort = filter.sort || 'date-desc'
  void router.push({ name: 'library' })
}

function open(item: ResolvedQuicklink): void {
  const smart = library.smartCollections.find((c) => c.id === item.id)
  if (!smart) return
  navigateFilter(smart.filter)
  selectSmartCollection(smart.id, smart.filter)
}

function unpin(item: Quicklink): void {
  toggleQuicklink(item)
}

interface QuicklinkMenuItem extends MenuItem {
  menuIcon?: Component
}

function openMenu(event: MouseEvent, item: ResolvedQuicklink): void {
  contextItem.value = item
  menu.value?.show(event)
}

const menuItems = computed<QuicklinkMenuItem[]>(() => {
  const item = contextItem.value
  if (!item) return []
  return [
    {
      label: 'Open',
      menuIcon: ExternalLink,
      command: () => open(item),
    },
    {
      label: 'Unpin from Quicklinks',
      menuIcon: Times,
      command: () => unpin(item),
    },
  ]
})
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
        Pin smart collections from their context menus.
      </p>
      <div
        v-for="item in items"
        :key="`${item.type}:${item.id}`"
        class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-sm select-none"
        :class="
          isActive(item)
            ? 'bg-brand-50 text-brand-700 dark:bg-brand-950/50 dark:text-brand-300'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white'
        "
        title="Right-click for options"
        @click="open(item)"
        @contextmenu.prevent="openMenu($event, item)"
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
      </div>
    </div>

    <ContextMenu ref="menu" :model="menuItems">
      <template #item="slotProps">
        <a
          data-pc-section="itemlink"
          class="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm"
          :class="slotProps.item.class ?? 'text-slate-700 dark:text-slate-200'"
          @click.prevent
        >
          <component
            :is="slotProps.item.menuIcon"
            v-if="slotProps.item.menuIcon"
            :size="14"
            class="shrink-0"
          />
          <span class="flex-1">{{ slotProps.item.label }}</span>
        </a>
      </template>
    </ContextMenu>
  </div>
</template>
