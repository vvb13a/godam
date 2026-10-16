import { reactive } from 'vue'
import type { SmartCollectionFilter } from '../types'

// Client-side only state: the smart collection the user last opened from the
// sidebar. Smart collections are just saved filters, so there is no URL param;
// the Inspector shows this collection while the current filters still match it,
// and any other filter change clears the selection.
export const smartSelection = reactive<{
  id: string | null
  filter: SmartCollectionFilter | null
}>({
  id: null,
  filter: null,
})

export function selectSmartCollection(
  id: string,
  filter: SmartCollectionFilter,
): void {
  smartSelection.id = id
  smartSelection.filter = filter
}

export function clearSmartSelection(): void {
  smartSelection.id = null
  smartSelection.filter = null
}
