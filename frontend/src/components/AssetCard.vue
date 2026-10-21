<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import * as api from '../api/client'
import type { CardShape } from '../lib/display'
import { formatDuration, formatSize } from '../lib/format'
import { hasMediaPreview } from '../lib/mimeTypes'
import type { Asset } from '../types'
import FileTypePreview from './FileTypePreview.vue'

const props = withDefaults(
  defineProps<{
    asset: Asset
    shape?: CardShape
    selected?: boolean
    dragging?: boolean
    showExtension?: boolean
    showName?: boolean
    showSize?: boolean
    showDimensions?: boolean
    showDuration?: boolean
    fill?: boolean
    autoplayAudio?: boolean
    autoplayVideo?: boolean
  }>(),
  {
    shape: 'square',
    selected: false,
    dragging: false,
    showExtension: false,
    showName: false,
    showSize: false,
    showDimensions: false,
    showDuration: false,
    fill: false,
    autoplayAudio: false,
    autoplayVideo: false,
  },
)

const emit = defineEmits<{
  open: [asset: Asset, event: MouseEvent]
  menu: [event: MouseEvent, asset: Asset]
  dragstart: [asset: Asset, event: DragEvent]
  dragend: []
}>()

const imageFailed = ref(false)
const loadedAspect = ref(0)

// Hover preview: play audio/video inline after a longer hover.
const mediaEl = ref<HTMLMediaElement | null>(null)
const seekBarEl = ref<HTMLElement | null>(null)
const playing = ref(false)
const currentTime = ref(0)
const duration = ref(0)
let hoverTimer: number | null = null
let seeking = false
const HOVER_DELAY = 700

const useTypePreview = computed(() => !hasMediaPreview(props.asset.mime_type))
const isAudio = computed(() => props.asset.mime_type.startsWith('audio/'))
const isVideo = computed(() => props.asset.mime_type.startsWith('video/'))
const progress = computed(() =>
  duration.value > 0 ? Math.min(1, currentTime.value / duration.value) : 0,
)

function onTimeUpdate(): void {
  currentTime.value = mediaEl.value?.currentTime ?? 0
}

function onMetadata(): void {
  duration.value = mediaEl.value?.duration ?? 0
}

function seekTo(event: MouseEvent): void {
  const el = mediaEl.value
  const bar = seekBarEl.value
  if (!el || !bar || !duration.value) return
  const rect = bar.getBoundingClientRect()
  const fraction = Math.min(Math.max(0, (event.clientX - rect.left) / rect.width), 1)
  el.currentTime = fraction * duration.value
  currentTime.value = el.currentTime
}

function onSeekMove(event: MouseEvent): void {
  if (seeking) seekTo(event)
}

function onSeekEnd(): void {
  seeking = false
  window.removeEventListener('mousemove', onSeekMove)
  window.removeEventListener('mouseup', onSeekEnd)
}

function onSeekStart(event: MouseEvent): void {
  if (!mediaEl.value || !duration.value) return
  event.preventDefault()
  seeking = true
  seekTo(event)
  window.addEventListener('mousemove', onSeekMove)
  window.addEventListener('mouseup', onSeekEnd)
}

function startPlayback(): void {
  const el = mediaEl.value
  if (!el) return
  el.currentTime = 0
  el.play().catch(() => {
    // Autoplay with sound may be blocked; video can at least play muted.
    if (isVideo.value) {
      el.muted = true
      el.play().catch(() => undefined)
    }
  })
}

function onHoverStart(): void {
  const enabled = isAudio.value
    ? props.autoplayAudio
    : isVideo.value
      ? props.autoplayVideo
      : false
  if (!enabled) return
  if (hoverTimer !== null) window.clearTimeout(hoverTimer)
  hoverTimer = window.setTimeout(() => {
    hoverTimer = null
    playing.value = true
    void nextTick(startPlayback)
  }, HOVER_DELAY)
}

function stopPlayback(): void {
  if (hoverTimer !== null) {
    window.clearTimeout(hoverTimer)
    hoverTimer = null
  }
  onSeekEnd()
  const el = mediaEl.value
  if (el) {
    el.pause()
    el.currentTime = 0
  }
  playing.value = false
  currentTime.value = 0
  duration.value = 0
}

function onDragStart(event: DragEvent): void {
  stopPlayback()
  emit('dragstart', props.asset, event)
}

onBeforeUnmount(stopPlayback)

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

const dimensions = computed(() => {
  const { width, height } = props.asset.metadata ?? {}
  return width && height ? `${width} × ${height}` : ''
})

const durationText = computed(() => {
  const seconds = props.asset.metadata?.duration_sec
  return seconds ? formatDuration(seconds) : ''
})

const mediaClass = computed(() => {
  if (props.fill) return 'relative h-full w-full overflow-hidden'
  const base = 'relative w-full overflow-hidden'
  switch (props.shape) {
    case 'landscape':
      return `${base} bg-slate-100 dark:bg-slate-800 aspect-video`
    case 'poster':
      return `${base} bg-slate-100 dark:bg-slate-800 aspect-[2/3]`
    case 'native':
      return base
    default:
      return `${base} bg-slate-100 dark:bg-slate-800 aspect-square`
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
  // While an asset is being dragged it gets a dotted outline (and, when it is
  // part of a larger selection, the ring is softened) so the source is clear.
  if (props.dragging) {
    return props.fill
      ? 'h-full rounded border border-dashed border-brand-500 bg-white dark:bg-slate-900 opacity-60'
      : 'rounded border border-dashed border-brand-500 bg-white dark:bg-slate-900 opacity-60'
  }
  const base = props.selected
    ? 'rounded border border-brand-500 bg-white dark:bg-slate-900 ring-2 ring-brand-500'
    : 'rounded border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 hover:border-brand-300'
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
    class="group block w-full overflow-hidden text-left transition focus:ring-2 focus:ring-brand-400 focus:outline-none"
    :class="cardClass"
    @click.stop="$emit('open', asset, $event)"
    @contextmenu.prevent="$emit('menu', $event, asset)"
    @mouseenter="onHoverStart"
    @mouseleave="stopPlayback"
    @dragstart="onDragStart"
    @dragend="$emit('dragend')"
  >
    <div :class="mediaClass" :style="mediaStyle">
      <FileTypePreview
        v-if="useTypePreview"
        :mime-type="asset.mime_type"
        :filename="asset.filename"
      />
      <img
        v-else-if="!imageFailed"
        :src="api.assetPreviewUrl(asset.id, asset.updated_at)"
        :alt="asset.filename"
        loading="lazy"
        draggable="false"
        class="h-full w-full object-cover"
        @load="onImageLoad"
        @error="imageFailed = true"
      />
      <FileTypePreview
        v-else
        :mime-type="asset.mime_type"
        :filename="asset.filename"
      />

      <video
        v-if="playing && isVideo"
        ref="mediaEl"
        :src="api.assetContentUrl(asset.id)"
        class="absolute inset-0 h-full w-full bg-black object-cover"
        playsinline
        @timeupdate="onTimeUpdate"
        @loadedmetadata="onMetadata"
        @durationchange="onMetadata"
        @ended="stopPlayback"
      />
      <audio
        v-else-if="playing && isAudio"
        ref="mediaEl"
        :src="api.assetContentUrl(asset.id)"
        class="hidden"
        @timeupdate="onTimeUpdate"
        @loadedmetadata="onMetadata"
        @durationchange="onMetadata"
        @ended="stopPlayback"
      />

      <template v-if="playing && duration > 0">
        <div
          ref="seekBarEl"
          class="absolute inset-x-0 bottom-0 z-10 flex h-3 cursor-pointer items-end"
          @mousedown.stop.prevent="onSeekStart"
          @click.stop
        >
          <div class="h-1.5 w-full bg-slate-900/25">
            <div class="h-full bg-brand-500" :style="{ width: `${progress * 100}%` }" />
          </div>
        </div>
        <span
          class="pointer-events-none absolute bottom-2.5 left-1.5 z-10 rounded bg-slate-900/70 px-1.5 py-0.5 text-[10px] tabular-nums text-white"
        >
          {{ formatDuration(currentTime) }}
        </span>
        <span
          class="pointer-events-none absolute right-1.5 bottom-2.5 z-10 rounded bg-slate-900/70 px-1.5 py-0.5 text-[10px] tabular-nums text-white"
        >
          {{ formatDuration(duration) }}
        </span>
      </template>

      <span
        v-if="showExtension && extension"
        :class="extensionClass"
      >
        {{ extension }}
      </span>

      <div
        v-if="!playing && (showName || showSize || showDimensions)"
        class="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-slate-900/80 to-transparent px-2 pt-6 pb-1.5 text-white"
      >
        <p v-if="showName" class="truncate text-xs font-medium">
          {{ asset.filename }}
        </p>
        <p v-if="showSize" class="text-[10px] text-white/70">
          {{ formatSize(asset.byte_size) }}
        </p>
        <p v-if="showDimensions && dimensions" class="text-[10px] text-white/70">
          {{ dimensions }}
        </p>
      </div>

      <span
        v-if="showDuration && durationText && !playing"
        class="pointer-events-none absolute right-1.5 bottom-1.5 z-10 rounded bg-slate-900/70 px-1.5 py-0.5 text-[10px] tabular-nums text-white"
      >
        {{ durationText }}
      </span>
    </div>
  </button>
</template>
