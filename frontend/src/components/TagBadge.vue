<script setup lang="ts">
import { StarFill } from '@primeicons/vue'

withDefaults(
  defineProps<{
    name: string
    color?: string
    starred?: boolean
    selected?: boolean
  }>(),
  {
    color: undefined,
    starred: false,
    selected: false,
  },
)
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-md border border-slate-200 bg-slate-100 px-2 py-0.5 text-xs text-slate-700 opacity-80 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
    :class="
      selected
        ? 'opacity-100 ring-1 ring-violet-400 ring-inset dark:ring-violet-500'
        : ''
    "
    :style="
      color
        ? { backgroundColor: `${color}1a`, borderColor: color }
        : undefined
    "
  >
    <StarFill
      v-if="starred"
      :size="12"
      class="shrink-0"
      :style="{ color: color ?? '#f59e0b' }"
    />
    <slot name="label">
      <span class="truncate">{{ name }}</span>
    </slot>
    <slot />
  </span>
</template>
