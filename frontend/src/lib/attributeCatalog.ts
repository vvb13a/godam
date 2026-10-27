import { reactive, watch } from 'vue'
import * as api from '../api/client'
import { attributesRevision } from './attributesDialog'
import type { AttributeFacet } from '../types'

// The catalog includes usage facets plus metadata-only values, so the dialog
// and inspector can offer values that no asset uses yet.
export const attributeCatalog = reactive({
  items: [] as AttributeFacet[],
  loaded: false,
  loading: false,
})

let inflight: Promise<void> | null = null

export function loadAttributeCatalog(force = false): Promise<void> {
  if (!force && attributeCatalog.loaded) return Promise.resolve()
  if (inflight) return inflight
  attributeCatalog.loading = true
  inflight = api
    .listAttributeCatalog()
    .then((items) => {
      attributeCatalog.items = items
      attributeCatalog.loaded = true
    })
    .catch(() => {
      attributeCatalog.loaded = false
    })
    .finally(() => {
      attributeCatalog.loading = false
      inflight = null
    })
  return inflight
}

watch(attributesRevision, () => {
  void loadAttributeCatalog(true)
})

export function catalogFacet(key: string): AttributeFacet | undefined {
  return attributeCatalog.items.find((facet) => facet.key === key)
}
