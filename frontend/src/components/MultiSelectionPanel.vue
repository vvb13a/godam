<script setup lang="ts">
import { Times } from '@primeicons/vue'
import { computed } from 'vue'
import { formatSize } from '../lib/format'
import type { Asset } from '../types'
import AssetPreviewGrid from './AssetPreviewGrid.vue'

const props = defineProps<{ assets: Asset[] }>()
const emit = defineEmits<{ close: [] }>()

const totalBytes = computed(() =>
  props.assets.reduce((sum, asset) => sum + asset.byte_size, 0),
)

interface TypeSummary {
  mimeType: string
  count: number
  bytes: number
}

const typeSummaries = computed<TypeSummary[]>(() => {
  const map = new Map<string, TypeSummary>()
  for (const asset of props.assets) {
    const entry = map.get(asset.mime_type) ?? {
      mimeType: asset.mime_type,
      count: 0,
      bytes: 0,
    }
    entry.count += 1
    entry.bytes += asset.byte_size
    map.set(asset.mime_type, entry)
  }
  return [...map.values()].sort(
    (a, b) => b.count - a.count || b.bytes - a.bytes,
  )
})
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-center justify-end px-3 py-2">
      <button
        class="inline-flex shrink-0 text-slate-400 hover:text-slate-700"
        title="Clear selection"
        @click="emit('close')"
      >
        <Times :size="16" />
      </button>
    </div>

    <div class="min-h-0 flex-1 space-y-5 overflow-y-auto px-3 pb-4">
      <AssetPreviewGrid :assets="assets" />

      <dl class="space-y-1.5 text-sm">
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Assets</dt>
          <dd class="text-right">{{ assets.length }}</dd>
        </div>
        <div class="flex justify-between gap-4">
          <dt class="text-slate-400">Size</dt>
          <dd class="text-right">{{ formatSize(totalBytes) }}</dd>
        </div>
      </dl>

      <section>
        <h3 class="mb-2 text-sm font-semibold">By type</h3>
        <div class="space-y-1.5 text-sm">
          <div
            v-for="type in typeSummaries"
            :key="type.mimeType"
            class="flex justify-between gap-4"
          >
            <span class="truncate text-slate-600">{{ type.mimeType }}</span>
            <span class="shrink-0 text-slate-400">
              {{ type.count }} · {{ formatSize(type.bytes) }}
            </span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
