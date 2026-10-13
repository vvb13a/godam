import { reactive } from 'vue'

// tagsDialog holds the visibility of the tag manager. When assetId is set the
// dialog is being used to edit a single asset's tags: it loads the asset's
// current tags as the initial selection and saves the selection on close.
export const tagsDialog = reactive({
  visible: false,
  assetId: null as string | null,
})

export function openTagsDialog(): void {
  tagsDialog.assetId = null
  tagsDialog.visible = true
}

export function openAssetTags(assetId: string): void {
  tagsDialog.assetId = assetId
  tagsDialog.visible = true
}
