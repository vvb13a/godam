<script setup lang="ts">
withDefaults(defineProps<{ name: string; color?: string }>(), {
  color: undefined,
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs text-slate-700"
    :style="{
      backgroundColor: color ? `${color}1a` : '#f1f5f9',
      borderColor: color ?? '#e2e8f0',
    }"
  >
    <span
      class="h-2 w-2 shrink-0 rounded-full"
      :style="{ backgroundColor: color ?? '#94a3b8' }"
    />
    <span class="truncate">{{ name }}</span>
    <slot />
  </span>
</template>
