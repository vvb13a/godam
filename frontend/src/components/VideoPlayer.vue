<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import * as api from '../api/client'
import type { Asset } from '../types'

const props = defineProps<{ asset: Asset }>()

const videoEl = ref<HTMLVideoElement | null>(null)
let player: { destroy: () => void } | null = null

onMounted(async () => {
  if (!videoEl.value) return
  // Loaded lazily so the Plyr CSS/JS only ships when a video is opened.
  await import('plyr/dist/plyr.css')
  const { default: Plyr } = await import('plyr')
  if (!videoEl.value) return
  player = new Plyr(videoEl.value, {
    controls: [
      'play-large',
      'play',
      'progress',
      'current-time',
      'duration',
      'mute',
      'volume',
      'settings',
      'pip',
      'airplay',
      'fullscreen',
    ],
    settings: ['quality', 'speed', 'captions'],
    seekTime: 10,
    keyboard: { focused: true, global: false },
    tooltips: { controls: true, seek: true },
  })
})

onBeforeUnmount(() => {
  player?.destroy()
  player = null
})
</script>

<template>
  <div class="flex h-full w-full items-center justify-center bg-black p-4">
    <video
      ref="videoEl"
      :src="api.assetContentUrl(asset.id)"
      :poster="api.assetPreviewUrl(asset.id, asset.updated_at)"
      controls
      playsinline
      class="w-full max-w-4xl"
      style="--plyr-color-main: #f59e0b"
    />
  </div>
</template>
