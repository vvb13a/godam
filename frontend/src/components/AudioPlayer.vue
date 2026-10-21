<script setup lang="ts">
import Popover from 'primevue/popover'
import ToggleSwitch from 'primevue/toggleswitch'
import WaveSurfer from 'wavesurfer.js'
import Hover from 'wavesurfer.js/dist/plugins/hover.js'
import Minimap from 'wavesurfer.js/dist/plugins/minimap.js'
import Timeline from 'wavesurfer.js/dist/plugins/timeline.js'
import Zoom from 'wavesurfer.js/dist/plugins/zoom.js'
import {
  Cog,
  Download,
  Pause,
  Play,
  StepBackward,
  StepForward,
  VolumeOff,
  VolumeUp,
} from '@primeicons/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as api from '../api/client'
import { theme } from '../lib/theme'
import type { Asset } from '../types'

const props = defineProps<{ asset: Asset }>()

const AUTOPLAY_KEY = 'godam:audio:autoplay'

function loadAutoplay(): boolean {
  try {
    const raw = localStorage.getItem(AUTOPLAY_KEY)
    return raw === null ? true : raw === 'true'
  } catch {
    return true
  }
}

const timelineEl = ref<HTMLElement | null>(null)
const waveformEl = ref<HTMLElement | null>(null)
const minimapEl = ref<HTMLElement | null>(null)
const settingsPopover = ref<InstanceType<typeof Popover> | null>(null)
const playing = ref(false)
const current = ref(0)
const duration = ref(0)
const rate = ref(1)
const volume = ref(1)
const muted = ref(false)
const failed = ref(false)
const autoplay = ref(loadAutoplay())

let wavesurfer: WaveSurfer | null = null

function accentColor(): string {
  return theme.resolved === 'dark' ? '#fb923c' : '#6366f1'
}
let playOnReady = false

const source = computed(() => api.assetContentUrl(props.asset.id))
const silent = computed(() => muted.value || volume.value === 0)

watch(autoplay, (value) => {
  try {
    localStorage.setItem(AUTOPLAY_KEY, String(value))
  } catch {
    // Storage unavailable; the setting just won't persist.
  }
})

function toggleSettings(event: Event): void {
  settingsPopover.value?.toggle(event)
}

function toggle(): void {
  void wavesurfer?.playPause()
}

function skip(delta: number): void {
  wavesurfer?.skip(delta)
}

function applyVolume(): void {
  wavesurfer?.setVolume(volume.value)
  if (volume.value > 0 && muted.value) {
    muted.value = false
    wavesurfer?.setMuted(false)
  }
}

function toggleMute(): void {
  muted.value = !muted.value
  wavesurfer?.setMuted(muted.value)
}

function applyRate(): void {
  wavesurfer?.setPlaybackRate(rate.value)
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  const minutes = Math.floor(total / 60)
  const secs = total % 60
  return `${minutes}:${String(secs).padStart(2, '0')}`
}

onMounted(() => {
  if (!waveformEl.value) return
  wavesurfer = WaveSurfer.create({
    container: waveformEl.value,
    url: source.value,
    height: 128,
    waveColor: '#cbd5e1',
    progressColor: accentColor(),
    cursorColor: accentColor(),
    barWidth: 2,
    barGap: 1,
    barRadius: 2,
    normalize: true,
    dragToSeek: true,
  })
  wavesurfer.setVolume(volume.value)
  playOnReady = autoplay.value

  // Read-only polish: time ruler, overview minimap, wheel/pinch zoom, hover time.
  if (timelineEl.value) {
    wavesurfer.registerPlugin(
      Timeline.create({ container: timelineEl.value, height: 20, formatTimeCallback: formatTime }),
    )
  }
  if (minimapEl.value) {
    wavesurfer.registerPlugin(
      Minimap.create({
        container: minimapEl.value,
        height: 40,
        waveColor: '#cbd5e1',
        progressColor: accentColor(),
        overlayColor: '#0f172a1a',
      }),
    )
  }
  wavesurfer.registerPlugin(Zoom.create({ maxZoom: 200 }))
  wavesurfer.registerPlugin(Hover.create({ formatTimeCallback: formatTime }))

  wavesurfer.on('ready', (d) => {
    duration.value = d
    failed.value = false
    if (playOnReady) {
      playOnReady = false
      void wavesurfer?.play().catch(() => undefined)
    }
  })
  wavesurfer.on('play', () => {
    playing.value = true
  })
  wavesurfer.on('pause', () => {
    playing.value = false
  })
  wavesurfer.on('finish', () => {
    playing.value = false
    current.value = 0
  })
  wavesurfer.on('timeupdate', (t) => {
    current.value = t
  })
  wavesurfer.on('error', () => {
    failed.value = true
  })
})

// Navigating to a different asset resets the player and (when autoplay is on)
// plays it once loaded.
watch(
  () => props.asset.id,
  () => {
    current.value = 0
    duration.value = 0
    playing.value = false
    failed.value = false
    playOnReady = autoplay.value
    void wavesurfer?.load(source.value).catch(() => undefined)
  },
)

onBeforeUnmount(() => {
  wavesurfer?.destroy()
  wavesurfer = null
})
</script>

<template>
  <div class="flex h-full w-full flex-col items-center justify-center gap-6 bg-slate-100 dark:bg-slate-800 p-6">
    <div class="w-full max-w-3xl rounded-lg bg-white dark:bg-slate-900 p-4 shadow-sm">
      <div ref="timelineEl" class="w-full" />
      <div ref="waveformEl" class="w-full" />
      <div ref="minimapEl" class="mt-2 w-full" />
      <p v-if="failed" class="mt-2 text-sm text-rose-500">
        This audio file could not be played in your browser.
      </p>
    </div>

    <div class="flex w-full max-w-3xl flex-wrap items-center gap-3">
      <button
        class="inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40"
        title="Back 10 seconds"
        @click="skip(-10)"
      >
        <StepBackward :size="18" />
      </button>
      <button
        class="inline-flex h-11 w-11 items-center justify-center rounded-full bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40 dark:bg-orange-500 dark:text-orange-950 dark:hover:bg-orange-400"
        :disabled="failed"
        :title="playing ? 'Pause' : 'Play'"
        @click="toggle"
      >
        <Pause v-if="playing" :size="20" />
        <Play v-else :size="20" />
      </button>
      <button
        class="inline-flex h-9 w-9 items-center justify-center rounded-full text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40"
        title="Forward 10 seconds"
        @click="skip(10)"
      >
        <StepForward :size="18" />
      </button>

      <span class="text-sm tabular-nums text-slate-500 dark:text-slate-400">
        {{ formatTime(current) }} / {{ formatTime(duration) }}
      </span>

      <div class="ml-auto flex items-center gap-2">
        <a
          :href="source"
          target="_blank"
          rel="noreferrer"
          class="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
          title="Download"
        >
          <Download :size="16" />
        </a>
        <button
          class="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
          title="Player settings"
          @click="toggleSettings"
        >
          <Cog :size="16" />
        </button>
        <button
          class="inline-flex h-8 w-8 items-center justify-center rounded-full text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
          :title="silent ? 'Unmute' : 'Mute'"
          @click="toggleMute"
        >
          <component :is="silent ? VolumeOff : VolumeUp" :size="16" />
        </button>
        <input
          v-model.number="volume"
          type="range"
          min="0"
          max="1"
          step="0.05"
          class="h-1 w-24 cursor-pointer accent-brand-600"
          title="Volume"
          @input="applyVolume"
        />
        <select
          v-model.number="rate"
          class="rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-600 dark:text-slate-300"
          title="Playback speed"
          @change="applyRate"
        >
          <option :value="0.5">0.5×</option>
          <option :value="0.75">0.75×</option>
          <option :value="1">1×</option>
          <option :value="1.25">1.25×</option>
          <option :value="1.5">1.5×</option>
          <option :value="2">2×</option>
        </select>
      </div>
    </div>

    <Popover ref="settingsPopover">
      <div class="w-64 space-y-3">
        <div class="flex items-center justify-between gap-4">
          <span class="text-sm font-medium text-slate-700 dark:text-slate-200">Autoplay</span>
          <ToggleSwitch v-model="autoplay" />
        </div>
        <p class="text-xs text-slate-400">
          Play audio automatically when opened or navigated to.
        </p>
      </div>
    </Popover>
  </div>
</template>
