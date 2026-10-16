<script setup lang="ts">
import { StarFill } from '@primeicons/vue'

withDefaults(defineProps<{ name: string; color?: string; starred?: boolean }>(), {
  color: undefined,
  starred: false,
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs text-slate-700 opacity-80"
    :style="{
      backgroundColor: color ? `${color}1a` : '#f1f5f9',
      borderColor: color ?? '#e2e8f0',
    }"
  >
    <StarFill
      v-if="starred"
      :size="12"
      class="shrink-0"
      :style="{ color: color ?? '#f59e0b' }"
    />
    <span class="truncate">{{ name }}</span>
    <slot />
  </span>
</template>
