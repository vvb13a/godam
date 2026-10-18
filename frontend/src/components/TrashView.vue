<script setup lang="ts">
import { Trash } from '@primeicons/vue'
</script>

<template>
  <div
    class="flex h-full flex-col items-center justify-center text-center text-slate-400"
  >
    <Trash :size="40" class="text-slate-300" />
    <h1 class="mt-4 text-lg font-semibold text-slate-700 dark:text-slate-200">Trash</h1>
    <p class="mt-1 text-sm">Deleted assets will appear here. Coming soon.</p>
  </div>
</template>
