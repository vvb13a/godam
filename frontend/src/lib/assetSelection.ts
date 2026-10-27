import { computed, ref } from 'vue'
import { embed } from './embed'

const localSelectedIds = ref<string[]>([])

// selectedIds is shared by the grid/inspector and the command palette so bulk
// actions can operate on the current selection from either surface.
export const selectedIds = computed<string[]>({
  get: () => (embed.active ? embed.selectedIds : localSelectedIds.value),
  set: (value) => {
    if (embed.active) embed.selectedIds = value
    else localSelectedIds.value = value
  },
})
