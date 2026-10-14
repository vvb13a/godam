<script setup lang="ts">
import { reactive, watch } from 'vue'
import * as api from '../api/client'
import { formatSize } from '../lib/format'
import type { Asset } from '../types'

// MosaicWall renders a justified, full-width wall: each item's width comes from
// its real aspect ratio (flex-grow/flex-basis), so a landscape spans roughly
// twice the width of a portrait. Every row fills the full width; a trailing
// spacer keeps the final row from being stretched.
const props = withDefaults(
  defineProps<{
    assets: Asset[]
    rowHeight: string
    selectedIds?: string[]
    showExtension?: boolean
    showName?: boolean
    showSize?: boolean
  }>(),
  {
    selectedIds: () => [],
    showExtension: false,
    showName: false,
    showSize: false,
  },
)

defineEmits<{
  open: [asset: Asset, event: MouseEvent]
  menu: [event: MouseEvent, asset: Asset]
  dragstart: [asset: Asset, event: DragEvent]
  dragend: []
}>()

const aspects = reactive<Record<string, number>>({})

function initialAspect(asset: Asset): number {
  const { width, height } = asset.metadata ?? {}
  if (width && height) return width / height
  return 1
}

watch(
  () => props.assets,
  (list) => {
    for (const asset of list) {
      if (!(asset.id in aspects)) {
        aspects[asset.id] = initialAspect(asset)
      }
    }
  },
  { immediate: true },
)

function itemStyle(asset: Asset): Record<string, string> {
  const aspect = aspects[asset.id] ?? 1
  return {
    flexGrow: String(aspect),
    flexBasis: `calc(${props.rowHeight} * ${aspect})`,
    height: props.rowHeight,
  }
}

function onLoad(asset: Asset, event: Event): void {
  const img = event.target as HTMLImageElement
  if (img.naturalWidth > 0 && img.naturalHeight > 0) {
    aspects[asset.id] = img.naturalWidth / img.naturalHeight
  }
}

function extensionOf(asset: Asset): string {
  const name = asset.filename
  const dot = name.lastIndexOf('.')
  if (dot > 0 && dot < name.length - 1) {
    return name.slice(dot + 1).toUpperCase()
  }
  const slash = asset.mime_type.indexOf('/')
  return slash >= 0 ? asset.mime_type.slice(slash + 1).toUpperCase() : ''
}
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <button
      v-for="asset in assets"
      :key="asset.id"
      type="button"
      draggable="true"
      :data-asset-id="asset.id"
      class="group relative overflow-hidden rounded bg-slate-100 focus:ring-2 focus:ring-violet-400 focus:outline-none"
      :class="selectedIds.includes(asset.id) ? 'ring-2 ring-violet-500' : ''"
      :style="itemStyle(asset)"
      @click.stop="$emit('open', asset, $event)"
      @contextmenu.prevent="$emit('menu', $event, asset)"
      @dragstart="$emit('dragstart', asset, $event)"
      @dragend="$emit('dragend')"
    >
      <img
        :src="api.assetPreviewUrl(asset.id)"
        :alt="asset.filename"
        loading="lazy"
        draggable="false"
        class="h-full w-full object-cover transition group-hover:opacity-90"
        @load="onLoad(asset, $event)"
      />
      <span
        v-if="showExtension && extensionOf(asset)"
        class="pointer-events-none absolute top-1 left-1 rounded bg-slate-900/60 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-white uppercase"
      >
        {{ extensionOf(asset) }}
      </span>
      <div
        v-if="showName || showSize"
        class="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-slate-900/80 to-transparent px-2 pt-6 pb-1.5 text-white"
      >
        <p v-if="showName" class="truncate text-xs font-medium">
          {{ asset.filename }}
        </p>
        <p v-if="showSize" class="text-[10px] text-white/70">
          {{ formatSize(asset.byte_size) }}
        </p>
      </div>
    </button>
    <span
      aria-hidden="true"
      style="flex-grow: 9999; flex-basis: 0; height: 0"
    ></span>
  </div>
</template>
