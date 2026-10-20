<script setup lang="ts">
import { Times } from '@primeicons/vue'
import { computed, onBeforeUnmount, watch } from 'vue'
import {
  cancelWarm,
  dismissWarm,
  warmProgress,
} from '../lib/warmPresets'

const percent = computed(() => {
  if (!warmProgress.totalConversions) return 0
  return Math.min(
    100,
    Math.round((warmProgress.done / warmProgress.totalConversions) * 100),
  )
})

let closeTimer: number | undefined

watch(
  () => warmProgress.finished,
  (finished) => {
    if (closeTimer !== undefined) {
      window.clearTimeout(closeTimer)
      closeTimer = undefined
    }
    if (!finished) return
    closeTimer = window.setTimeout(() => {
      closeTimer = undefined
      dismissWarm()
    }, 3000)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (closeTimer !== undefined) window.clearTimeout(closeTimer)
})
</script>

<template>
  <div
    v-if="warmProgress.active || warmProgress.finished"
    class="fixed bottom-4 right-4 z-50 w-80 rounded-xl border border-slate-200 bg-white p-4 shadow-xl dark:border-slate-700 dark:bg-slate-900"
  >
    <div class="flex items-start justify-between gap-3">
      <div>
        <p class="text-sm font-semibold">
          {{ warmProgress.active ? 'Warming presets' : 'Warm-up complete' }}
        </p>
        <p class="mt-0.5 text-xs text-slate-400">
          {{ warmProgress.done }} / {{ warmProgress.totalConversions }} conversions
          · {{ warmProgress.totalAssets }} assets
          <span v-if="warmProgress.failed" class="text-amber-600 dark:text-amber-400">
            · {{ warmProgress.failed }} failed
          </span>
          <span v-if="warmProgress.skipped" class="text-slate-400">
            · {{ warmProgress.skipped }} skipped
          </span>
        </p>
      </div>
      <button
        v-if="warmProgress.active"
        class="shrink-0 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
        title="Cancel"
        @click="cancelWarm"
      >
        <Times :size="14" />
      </button>
    </div>

    <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
      <div
        class="h-full rounded-full bg-violet-600 transition-[width] duration-200"
        :style="{ width: `${percent}%` }"
      />
    </div>

    <p
      v-if="warmProgress.current"
      class="mt-2 truncate font-mono text-[11px] text-slate-400"
    >
      {{ warmProgress.current }}
    </p>

    <ul
      v-if="warmProgress.errors.length"
      class="mt-2 max-h-20 space-y-0.5 overflow-y-auto text-[11px] text-amber-600 dark:text-amber-400"
    >
      <li v-for="(message, index) in warmProgress.errors" :key="index" class="truncate">
        {{ message }}
      </li>
    </ul>

  </div>
</template>
