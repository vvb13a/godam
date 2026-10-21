<script setup lang="ts">
import {
  File as FileIcon,
  FilePdf,
  Image as ImageIcon,
  Video,
  VolumeUp,
} from '@primeicons/vue'
import type { Component } from 'vue'
import { computed } from 'vue'

interface PreviewKind {
  icon: Component
  gradient: string
}

const KINDS: Record<string, PreviewKind> = {
  pdf: {
    icon: FilePdf,
    gradient: 'from-red-600 via-rose-600 to-red-900',
  },
  audio: {
    icon: VolumeUp,
    gradient: 'from-emerald-500 via-teal-500 to-cyan-600',
  },
  video: {
    icon: Video,
    gradient: 'from-purple-500 via-fuchsia-500 to-pink-600',
  },
  image: {
    icon: ImageIcon,
    gradient: 'from-sky-400 via-cyan-500 to-teal-500',
  },
  generic: {
    icon: FileIcon,
    gradient: 'from-slate-500 via-slate-600 to-slate-800',
  },
}

const props = withDefaults(
  defineProps<{
    mimeType?: string
    filename?: string
    compact?: boolean
  }>(),
  {
    mimeType: '',
    filename: '',
    compact: false,
  },
)

const extension = computed(() => {
  const name = props.filename
  const dot = name.lastIndexOf('.')
  if (dot > 0 && dot < name.length - 1) {
    return name.slice(dot + 1).toUpperCase()
  }
  const mime = props.mimeType.split(';')[0]
  const slash = mime.indexOf('/')
  return slash >= 0 ? mime.slice(slash + 1).toUpperCase() : ''
})

const kind = computed<PreviewKind>(() => {
  const mime = props.mimeType.split(';')[0].trim().toLowerCase()
  const ext = extension.value.toLowerCase()

  if (mime === 'application/pdf' || ext === 'pdf') return KINDS.pdf
  if (mime.startsWith('audio/')) return KINDS.audio
  if (mime.startsWith('video/')) return KINDS.video
  if (mime.startsWith('image/')) return KINDS.image
  if (['mp3', 'wav', 'm4a', 'aac'].includes(ext)) return KINDS.audio
  if (['mp4', 'webm', 'mov'].includes(ext)) return KINDS.video
  if (['jpg', 'jpeg', 'png', 'webp', 'gif'].includes(ext)) return KINDS.image
  return KINDS.generic
})
</script>

<template>
  <div
    class="relative flex h-full w-full items-center justify-center overflow-hidden bg-gradient-to-br"
    :class="kind.gradient"
  >
    <div
      class="absolute -top-8 -left-8 h-28 w-28 rounded-full bg-white/15 blur-2xl"
    />
    <div
      class="absolute -right-6 -bottom-10 h-32 w-32 rounded-full bg-black/15 blur-2xl"
    />
    <div class="relative flex flex-col items-center justify-center gap-2 px-3 text-center text-white">
      <div
        class="flex items-center justify-center rounded-2xl bg-white/15 shadow-lg ring-1 ring-white/25 backdrop-blur-sm"
        :class="compact ? 'h-10 w-10' : 'h-16 w-16'"
      >
        <component :is="kind.icon" :size="compact ? 20 : 30" />
      </div>
    </div>
  </div>
</template>
