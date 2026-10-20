import { reactive } from 'vue'

// collectionsDialog is the shared collection picker. With upload=true it edits
// the upload dialog's destination collections; with assetId set it edits the
// collections of a single asset.
export const collectionsDialog = reactive({
  visible: false,
  assetId: null as string | null,
  assetIds: [] as string[],
  mode: null as 'add' | 'replace' | null,
  upload: false,
})

export function openAssetCollections(assetId: string): void {
  collectionsDialog.assetId = assetId
  collectionsDialog.assetIds = []
  collectionsDialog.mode = null
  collectionsDialog.upload = false
  collectionsDialog.visible = true
}

export function openUploadCollections(): void {
  collectionsDialog.assetId = null
  collectionsDialog.assetIds = []
  collectionsDialog.mode = null
  collectionsDialog.upload = true
  collectionsDialog.visible = true
}

export function openCollectionsDialog(): void {
  collectionsDialog.assetId = null
  collectionsDialog.assetIds = []
  collectionsDialog.mode = null
  collectionsDialog.upload = false
  collectionsDialog.visible = true
}

export function openBulkAssetCollections(
  assetIds: string[],
  mode: 'add' | 'replace',
): void {
  collectionsDialog.assetId = null
  collectionsDialog.assetIds = [...assetIds]
  collectionsDialog.mode = mode
  collectionsDialog.upload = false
  collectionsDialog.visible = true
}
