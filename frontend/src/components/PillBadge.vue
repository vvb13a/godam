<script setup lang="ts">
import { StarFill } from '@primeicons/vue'
import { computed } from 'vue'

const props = withDefaults(
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

// Pick whichever of white or slate-900 has the higher contrast against the
// pill's background.
function contentColor(color: string): string {
  const raw = color.replace('#', '')
  if (raw.length !== 6) return '#ffffff'
  const linear = (offset: number): number => {
    const v = parseInt(raw.slice(offset, offset + 2), 16) / 255
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  }
  const luminance =
    0.2126 * linear(0) + 0.7152 * linear(2) + 0.0722 * linear(4)
  const whiteContrast = 1.05 / (luminance + 0.05)
  const darkContrast = (luminance + 0.05) / (0.0091 + 0.05)
  return whiteContrast >= darkContrast ? '#ffffff' : '#0f172a'
}

const badgeStyle = computed(() => {
  if (!props.color) {
    return props.selected
      ? {
          backgroundColor: 'var(--brand-500)',
          color: 'var(--brand-contrast)',
        }
      : undefined
  }
  if (props.selected) {
    return {
      backgroundColor: props.color,
      borderColor: props.color,
      color: contentColor(props.color),
    }
  }
  return {
    backgroundColor: `${props.color}1a`,
    borderColor: props.color,
  }
})

const starColor = computed(() => {
  if (props.selected) {
    return props.color ? contentColor(props.color) : 'var(--brand-contrast)'
  }
  return props.color ?? '#f97316'
})
</script>

<template>
  <span
    class="pill inline-flex items-center gap-1.5 rounded-md border border-slate-200 bg-slate-100 px-2 py-0.5 text-xs text-slate-700 opacity-80 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
    :class="selected ? 'opacity-100' : ''"
    :style="badgeStyle"
  >
    <StarFill
      v-if="starred"
      :size="12"
      class="shrink-0"
      :style="{ color: starColor }"
    />
    <slot name="label">
      <span class="truncate">{{ name }}</span>
    </slot>
    <slot />
  </span>
</template>

<style scoped>
.pill {
  position: relative;
}

.pill::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: currentColor;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease;
}

.pill:hover::after {
  opacity: 0.08;
}
</style>
