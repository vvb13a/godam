import { reactive, watch } from 'vue'
import * as api from '../api/client'
import { attributesRevision } from './attributesDialog'
import type { AttributeTree } from '../types'

export const attributeTrees = reactive({
  items: [] as AttributeTree[],
  loaded: false,
  loading: false,
})

let inflight: Promise<void> | null = null

export function loadAttributeTrees(force = false): Promise<void> {
  if (!force && attributeTrees.loaded) return Promise.resolve()
  if (inflight) return inflight
  attributeTrees.loading = true
  inflight = api
    .listAttributeTree()
    .then((items) => {
      attributeTrees.items = items
      attributeTrees.loaded = true
    })
    .catch(() => {
      attributeTrees.loaded = false
    })
    .finally(() => {
      attributeTrees.loading = false
      inflight = null
    })
  return inflight
}

watch(attributesRevision, () => {
  void loadAttributeTrees(true)
})

export function attributeTreeFor(key: string): AttributeTree | undefined {
  return attributeTrees.items.find((tree) => tree.key === key)
}
