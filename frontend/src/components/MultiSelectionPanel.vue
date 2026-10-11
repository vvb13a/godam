<script setup lang="ts">
import Button from 'primevue/button'
import { Times, Trash } from '@primeicons/vue'
import { computed } from 'vue'
import { formatSize } from '../lib/format'
import type { Asset } from '../types'

const props = defineProps<{ assets: Asset[] }>()
const emit = defineEmits<{ close: []; remove: [] }>()

const totalBytes = computed(() =>
  props.assets.reduce((sum, asset) => sum + asset.byte_size, 0),
)

const typeCounts = computed(() => {
  const counts = new Map<string, number>()
  for (const asset of props.assets) {
    counts.set(asset.mime_type, (counts.get(asset.mime_type) ?? 0) + 1)
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1])
})
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center justify-between gap-2 border-b border-slate-200 px-3 py-2"
    >
      <span class="truncate text-sm font-semibold">
        {{ assets.length }} assets selected
      </span>
      <button
        class="inline-flex shrink-0 text-slate-400 hover:text-slate-700"
        title="Clear selection"
        @click="emit('close')"
      >
        <Times :size="16" />
      </button>
    </div>

    <div class="min-h-0 flex-1 space-y-5 overflow-y-auto p-3">
      <dl class="space-y-1.5 text-sm">
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Assets</dt>
          <dd>{{ assets.length }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Total size</dt>
          <dd>{{ formatSize(totalBytes) }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Types</dt>
          <dd>{{ typeCounts.length }}</dd>
        </div>
      </dl>

      <section>
        <h3 class="mb-2 text-sm font-semibold">By type</h3>
        <div class="space-y-1 text-sm">
          <div
            v-for="[type, count] in typeCounts"
            :key="type"
            class="flex justify-between gap-4"
          >
            <span class="truncate text-slate-600">{{ type }}</span>
            <span class="text-slate-400">{{ count }}</span>
          </div>
        </div>
      </section>

      <Button severity="danger" outlined class="w-full" @click="emit('remove')">
        <template #icon><Trash :size="16" /></template>
        Delete selected
      </Button>
    </div>
  </div>
</template>
