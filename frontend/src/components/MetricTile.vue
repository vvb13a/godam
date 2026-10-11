<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  hint?: string
}>()
</script>

<template>
  <div class="rounded border border-slate-200 bg-white p-4">
    <div class="text-xs font-medium tracking-wide text-slate-400 uppercase">
      {{ label }}
    </div>
    <div class="mt-1 text-2xl font-semibold tracking-tight">{{ value }}</div>
    <div v-if="hint" class="mt-0.5 text-xs text-slate-400">{{ hint }}</div>
  </div>
</template>
