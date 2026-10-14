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
  }>(),
  { shape: 'square', selected: false },
)

defineEmits<{
  open: [asset: Asset, event: MouseEvent]
  menu: [event: MouseEvent, asset: Asset]
  dragstart: [asset: Asset, event: DragEvent]
  dragend: []
}>()

const imageFailed = ref(false)

const mediaClass = computed(() => {
  const base = 'relative w-full overflow-hidden bg-slate-100'
  switch (props.shape) {
    case 'avatar':
      return `${base} aspect-square p-3`
    case 'logo':
      return `${base} aspect-square bg-white p-4`
    case 'landscape':
      return `${base} aspect-video`
    case 'poster':
      return `${base} aspect-[2/3]`
    default:
      return `${base} aspect-square`
  }
})

const imageClass = computed(() => {
  switch (props.shape) {
    case 'avatar':
      return 'h-full w-full rounded-full object-cover'
    case 'logo':
      return 'h-full w-full object-contain'
    default:
      return 'h-full w-full object-cover'
  }
})
</script>

<template>
  <button
    type="button"
    draggable="true"
    :data-asset-id="asset.id"
    class="group overflow-hidden rounded border bg-white text-left transition hover:border-violet-300 hover:shadow-sm focus:ring-2 focus:ring-violet-400 focus:outline-none"
    :class="selected ? 'border-violet-500 ring-2 ring-violet-500' : 'border-slate-200'"
    @click.stop="$emit('open', asset, $event)"
    @contextmenu.prevent="$emit('menu', $event, asset)"
    @dragstart="$emit('dragstart', asset, $event)"
    @dragend="$emit('dragend')"
  >
    <div :class="mediaClass">
      <img
        v-if="!imageFailed"
        :src="api.assetPreviewUrl(asset.id)"
        :alt="asset.filename"
        loading="lazy"
        draggable="false"
        :class="imageClass"
        @error="imageFailed = true"
      />
      <div
        v-else
        class="flex h-full w-full items-center justify-center text-xs text-slate-400"
      >
        No preview
      </div>
    </div>
    <div class="p-2.5">
      <p class="truncate text-sm font-medium" :title="asset.filename">
        {{ asset.filename }}
      </p>
      <p class="mt-0.5 text-xs text-slate-400">{{ formatSize(asset.byte_size) }}</p>
    </div>
  </button>
</template>
