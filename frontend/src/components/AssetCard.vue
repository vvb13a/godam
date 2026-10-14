<script setup lang="ts">
import { computed, ref } from 'vue'
import * as api from '../api/client'
import type { CardShape } from '../lib/display'
import { formatSize } from '../lib/format'
import type { Asset } from '../types'

const props = withDefaults(
  defineProps<{
    asset: Asset
    shape?: CardShape
    selected?: boolean
    showExtension?: boolean
    showName?: boolean
    showSize?: boolean
    fill?: boolean
  }>(),
  {
    shape: 'square',
    selected: false,
    showExtension: false,
    showName: false,
    showSize: false,
    fill: false,
  },
)

defineEmits<{
  open: [asset: Asset, event: MouseEvent]
  menu: [event: MouseEvent, asset: Asset]
  dragstart: [asset: Asset, event: DragEvent]
  dragend: []
}>()

const imageFailed = ref(false)
const loadedAspect = ref(0)

// Native shape keeps each preview's real aspect ratio (metadata first, then the
// decoded image as a fallback).
const aspect = computed(() => {
  const { width, height } = props.asset.metadata ?? {}
  if (width && height) return width / height
  if (loadedAspect.value) return loadedAspect.value
  return 1
})

const extension = computed(() => {
  const name = props.asset.filename
  const dot = name.lastIndexOf('.')
  if (dot > 0 && dot < name.length - 1) {
    return name.slice(dot + 1).toUpperCase()
  }
  const slash = props.asset.mime_type.indexOf('/')
  return slash >= 0 ? props.asset.mime_type.slice(slash + 1).toUpperCase() : ''
})

const mediaClass = computed(() => {
  if (props.fill) return 'relative h-full w-full overflow-hidden'
  const base = 'relative w-full overflow-hidden'
  switch (props.shape) {
    case 'landscape':
      return `${base} bg-slate-100 aspect-video`
    case 'poster':
      return `${base} bg-slate-100 aspect-[2/3]`
    case 'native':
      return base
    default:
      return `${base} bg-slate-100 aspect-square`
  }
})

const mediaStyle = computed(() =>
  props.shape === 'native' && !props.fill
    ? { aspectRatio: String(aspect.value) }
    : undefined,
)

function onImageLoad(event: Event): void {
  const img = event.target as HTMLImageElement
  if (img.naturalWidth > 0 && img.naturalHeight > 0) {
    loadedAspect.value = img.naturalWidth / img.naturalHeight
  }
}

const cardClass = computed(() => {
  const base = props.selected
    ? 'rounded border border-violet-500 bg-white ring-2 ring-violet-500'
    : 'rounded border border-slate-200 bg-white hover:border-violet-300 hover:shadow-sm'
  return props.fill ? `${base} h-full` : base
})

const extensionClass =
  'pointer-events-none absolute top-1 left-1 rounded bg-slate-900/60 px-1.5 py-0.5 text-[10px] font-semibold tracking-wide text-white uppercase'
</script>

<template>
  <button
    type="button"
    draggable="true"
    :data-asset-id="asset.id"
    class="group block w-full overflow-hidden text-left transition focus:ring-2 focus:ring-violet-400 focus:outline-none"
    :class="cardClass"
    @click.stop="$emit('open', asset, $event)"
    @contextmenu.prevent="$emit('menu', $event, asset)"
    @dragstart="$emit('dragstart', asset, $event)"
    @dragend="$emit('dragend')"
  >
    <div :class="mediaClass" :style="mediaStyle">
      <img
        v-if="!imageFailed"
        :src="api.assetPreviewUrl(asset.id)"
        :alt="asset.filename"
        loading="lazy"
        draggable="false"
        class="h-full w-full object-cover"
        @load="onImageLoad"
        @error="imageFailed = true"
      />
      <div
        v-else
        class="flex h-full w-full items-center justify-center text-xs text-slate-400"
      >
        No preview
      </div>

      <span
        v-if="showExtension && extension"
        :class="extensionClass"
      >
        {{ extension }}
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
    </div>
  </button>
</template>
