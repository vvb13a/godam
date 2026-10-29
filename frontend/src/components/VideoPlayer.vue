<script setup lang="ts">
import Popover from 'primevue/popover'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'
import { Camera, Cog } from '@primeicons/vue'
import { onMounted, ref, watch } from 'vue'
import * as api from '../api/client'
import type { Asset } from '../types'

defineOptions({ name: 'VideoAssetPlayer' })

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ saved: [asset: Asset] }>()

const toast = useToast()

const AUTOPLAY_KEY = 'godam:video:autoplay'

function loadAutoplay(): boolean {
  try {
    const raw = localStorage.getItem(AUTOPLAY_KEY)
    return raw === null ? true : raw === 'true'
  } catch {
    return true
  }
}

const videoEl = ref<HTMLVideoElement | null>(null)
const settingsPopover = ref<InstanceType<typeof Popover> | null>(null)
const failed = ref(false)
const posterBusy = ref(false)
const autoplay = ref(loadAutoplay())

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

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  const minutes = Math.floor(total / 60)
  const secs = total % 60
  return `${minutes}:${String(secs).padStart(2, '0')}`
}

async function useCurrentFrameAsPoster(): Promise<void> {
  if (posterBusy.value) return
  const timestamp = videoEl.value?.currentTime ?? 0
  posterBusy.value = true
  try {
    const updated = await api.regeneratePreview(props.asset.id, { timestamp })
    emit('saved', updated)
    toast.add({
      severity: 'success',
      summary: `Poster set to ${formatTime(timestamp)}`,
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update poster',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  } finally {
    posterBusy.value = false
  }
}

onMounted(async () => {
  // Loaded lazily so Video.js only ships when a video is opened.
  await import('@videojs/html/video/player')
  await import('@videojs/html/video/skin')
})
</script>

<template>
  <div
    class="relative flex h-full w-full flex-col items-center justify-center gap-3 bg-slate-100 p-4 dark:bg-slate-800"
  >
    <div class="absolute right-4 top-4 z-10 flex items-center gap-2">
      <button
        class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/80 text-slate-500 shadow-sm backdrop-blur hover:bg-slate-100 disabled:opacity-40 dark:bg-slate-900/80 dark:text-slate-400 dark:hover:bg-slate-800"
        :disabled="posterBusy"
        :title="`Use ${formatTime(videoEl?.currentTime ?? 0)} as poster`"
        @click="useCurrentFrameAsPoster"
      >
        <Camera :size="16" />
      </button>
      <button
        class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/80 text-slate-500 shadow-sm backdrop-blur hover:bg-slate-100 dark:bg-slate-900/80 dark:text-slate-400 dark:hover:bg-slate-800"
        title="Player settings"
        @click="toggleSettings"
      >
        <Cog :size="16" />
      </button>
    </div>

    <video-player class="video-player">
      <video-skin class="video-skin">
        <video
          ref="videoEl"
          :src="api.assetContentUrl(props.asset.id)"
          :poster="api.assetPreviewUrl(props.asset.id, props.asset.updated_at)"
          :autoplay="autoplay"
          playsinline
          @error="failed = true"
        />
      </video-skin>
    </video-player>

    <p v-if="failed" class="text-sm text-rose-500">
      This video file could not be played in your browser.
    </p>

    <Popover ref="settingsPopover">
      <div class="w-64 space-y-3">
        <div class="flex items-center justify-between gap-4">
          <span class="text-sm font-medium text-slate-700 dark:text-slate-200">Autoplay</span>
          <ToggleSwitch v-model="autoplay" />
        </div>
        <p class="text-xs text-slate-400">
          Play video automatically when opened or navigated to.
        </p>
      </div>
    </Popover>
  </div>
</template>

<style scoped>
.video-player {
  display: flex;
  justify-content: center;
  width: 100%;
  max-width: 72rem;
  max-height: 100%;
}

.video-skin {
  display: block;
  width: 100%;
  max-height: 100%;
  aspect-ratio: 16 / 9;
}

.video-player:not(:defined),
.video-skin:not(:defined) {
  visibility: hidden;
}
</style>
