<script setup lang="ts">
import { ChartPie, Images } from '@primeicons/vue'
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

const route = useRoute()

const isDashboard = computed(() => route.name === 'dashboard')
const isLibrary = computed(
  () => route.name === 'library' || route.name === 'collection',
)
</script>

<template>
  <aside class="flex w-56 shrink-0 flex-col border-r border-slate-200 bg-white">
    <div class="flex h-16 items-center border-b border-slate-200 px-5">
      <span class="text-lg font-bold tracking-tight">GoDam</span>
    </div>

    <nav class="flex-1 space-y-1 p-3">
      <RouterLink
        :to="{ name: 'dashboard' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          isDashboard
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <ChartPie :size="16" />
        Dashboard
      </RouterLink>
      <RouterLink
        :to="{ name: 'library' }"
        class="flex items-center gap-2 rounded px-3 py-2 text-sm font-medium"
        :class="
          isLibrary
            ? 'bg-violet-50 text-violet-700'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
        "
      >
        <Images :size="16" />
        Library
      </RouterLink>
    </nav>

    <div class="border-t border-slate-200 p-3 text-xs text-slate-400">
      Digital Asset Manager
    </div>
  </aside>
</template>
