import { reactive } from 'vue'

// assetDrag tracks an in-flight drag of one or more library assets so sidebar
// rows can act as drop targets for the whole selection.
export const assetDrag = reactive({
  assetIds: [] as string[],
  active: false,
})

export function startAssetDrag(ids: string[]): void {
  assetDrag.assetIds = ids
  assetDrag.active = true
}

export function endAssetDrag(): void {
  assetDrag.assetIds = []
  assetDrag.active = false
}
