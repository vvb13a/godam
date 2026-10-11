<script setup lang="ts">
import Message from 'primevue/message'
import { onMounted, ref } from 'vue'
import * as api from '../api/client'
import { formatSize } from '../lib/format'
import type { Stats } from '../types'
import MetricTile from './MetricTile.vue'

const stats = ref<Stats | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

function share(bytes: number): number {
  const total = stats.value?.storage_bytes ?? 0
  return total > 0 ? Math.max(2, (bytes / total) * 100) : 0
}

onMounted(async () => {
  try {
    stats.value = await api.getStats()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="mx-auto h-full max-w-5xl overflow-y-auto">
    <header class="mb-6">
      <h1 class="text-2xl font-semibold tracking-tight">Dashboard</h1>
      <p class="text-sm text-slate-500">Overview of your asset library.</p>
    </header>

    <Message v-if="error" severity="error" class="mb-4">{{ error }}</Message>
    <p v-if="loading" class="text-sm text-slate-400">Loading…</p>

    <template v-if="stats">
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <MetricTile label="Assets" :value="stats.assets" />
        <MetricTile
          label="Storage"
          :value="formatSize(stats.storage_bytes)"
        />
        <MetricTile label="Collections" :value="stats.collections" />
        <MetricTile label="Tags" :value="stats.tags" />
      </div>

      <section class="mt-6 rounded border border-slate-200 bg-white">
        <div class="border-b border-slate-200 px-4 py-3 text-sm font-semibold">
          Assets by type
        </div>
        <div v-if="stats.types.length" class="divide-y divide-slate-100">
          <div v-for="type in stats.types" :key="type.mime_type" class="px-4 py-3">
            <div class="flex items-center justify-between text-sm">
              <span class="font-medium">{{ type.mime_type }}</span>
              <span class="text-slate-500">
                {{ type.count }} · {{ formatSize(type.bytes) }}
              </span>
            </div>
            <div class="mt-2 h-1.5 w-full overflow-hidden rounded bg-slate-100">
              <div
                class="h-full rounded bg-violet-500"
                :style="{ width: share(type.bytes) + '%' }"
              />
            </div>
          </div>
        </div>
        <p v-else class="px-4 py-6 text-sm text-slate-400">No assets yet.</p>
      </section>
    </template>
  </div>
</template>
