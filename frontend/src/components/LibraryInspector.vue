<script setup lang="ts">
import Message from 'primevue/message'
import { Image, Times } from '@primeicons/vue'
import { computed, ref } from 'vue'
import * as api from '../api/client'
import { formatSize } from '../lib/format'
import { currentLibrary, library, loadLibraries } from '../lib/library'

const busy = ref(false)
const error = ref<string | null>(null)
const previewFailed = ref(false)

const stats = computed(() => library.stats)
const coverAssetId = computed(() => currentLibrary()?.cover_asset_id ?? '')
const typeCounts = computed(() => stats.value?.types ?? [])

async function removeCover(): Promise<void> {
  busy.value = true
  error.value = null
  try {
    await api.setInstanceCover('')
    previewFailed.value = false
    await loadLibraries()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-center justify-end px-3 py-2">
      <button
        v-if="coverAssetId"
        class="inline-flex shrink-0 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
        title="Remove cover"
        :disabled="busy"
        @click="removeCover"
      >
        <Times :size="16" />
      </button>
    </div>

    <div class="min-h-0 flex-1 space-y-6 overflow-y-auto px-3 pb-4">
      <div
        class="flex aspect-video items-center justify-center overflow-hidden rounded bg-slate-100 dark:bg-slate-800"
      >
        <img
          v-if="coverAssetId && !previewFailed"
          :src="api.assetPreviewUrl(coverAssetId)"
          alt="Library cover"
          class="h-full w-full object-cover"
          @error="previewFailed = true"
        />
        <Image v-else :size="28" class="text-slate-300" />
      </div>

      <section>
        <h3 class="mb-2 text-sm font-semibold">Library</h3>
        <dl class="space-y-1.5 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Assets</dt>
            <dd class="text-right">{{ stats?.assets ?? '—' }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Size</dt>
            <dd class="text-right">
              {{ stats ? formatSize(stats.storage_bytes) : '—' }}
            </dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Collections</dt>
            <dd class="text-right">{{ stats?.collections ?? '—' }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Smart collections</dt>
            <dd class="text-right">{{ library.smartCollections.length }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-slate-400">Tags</dt>
            <dd class="text-right">{{ stats?.tags ?? '—' }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="typeCounts.length">
        <h3 class="mb-2 text-sm font-semibold">By type</h3>
        <div class="space-y-1.5 text-sm">
          <div
            v-for="type in typeCounts"
            :key="type.mime_type"
            class="flex justify-between gap-4"
          >
            <span class="truncate text-slate-600 dark:text-slate-300">{{ type.mime_type }}</span>
            <span class="shrink-0 text-slate-400">
              {{ type.count }} · {{ formatSize(type.bytes) }}
            </span>
          </div>
        </div>
      </section>

      <p v-if="!coverAssetId" class="text-xs text-slate-400">
        Set a library cover from an asset's right-click menu.
      </p>

      <Message v-if="error" severity="error">{{ error }}</Message>
    </div>
  </div>
</template>
