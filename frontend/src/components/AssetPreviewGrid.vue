<script setup lang="ts">
import { computed, reactive } from 'vue'
import * as api from '../api/client'
import type { Asset } from '../types'

const props = withDefaults(
  defineProps<{
    assets: Asset[]
    limit?: number
  }>(),
  {
    limit: 3,
  },
)

const previewAssets = computed(() => props.assets.slice(0, props.limit))
const remaining = computed(() =>
  Math.max(0, props.assets.length - props.limit),
)

// Previews that fail to load (documents, videos without a still, …) fall back
// to the plain slate tile rather than a broken-image icon.
const failedIds = reactive(new Set<string>())
function markFailed(id: string): void {
  failedIds.add(id)
}
</script>

<template>
  <div class="grid grid-cols-2 gap-1">
    <div
      v-for="asset in previewAssets"
      :key="asset.id"
      class="aspect-square overflow-hidden rounded bg-slate-100 dark:bg-slate-800"
    >
      <img
        v-if="!failedIds.has(asset.id)"
        :src="api.assetPreviewUrl(asset.id, asset.updated_at)"
        :alt="asset.filename"
        class="h-full w-full object-cover"
        @error="markFailed(asset.id)"
      />
    </div>
    <div
      v-if="remaining > 0"
      class="flex aspect-square items-center justify-center rounded bg-slate-100 dark:bg-slate-800 text-xl font-semibold text-slate-500 dark:text-slate-400"
    >
      +{{ remaining }}
    </div>
  </div>
</template>
