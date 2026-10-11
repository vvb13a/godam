import { reactive } from 'vue'
import * as api from '../api/client'
import type { Collection } from '../types'

// library holds the collections shared by the sidebar and the views, plus a
// version counter that bumped on any library-wide change (e.g. a drag-and-drop
// upload) so mounted views know to reload.
export const library = reactive({
  collections: [] as Collection[],
  version: 0,
})

export async function loadCollections(): Promise<void> {
  library.collections = await api.listCollections()
}

export function notifyLibraryChanged(): void {
  library.version++
}
