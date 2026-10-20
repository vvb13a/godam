import { reactive } from 'vue'

export type TagAssetMode = 'replace' | 'add'

// tagsDialog holds the visibility of the tag manager. When assetId is set the
// dialog edits a single asset's tags. When assetIds/mode are set it is used for
// a bulk asset tagging operation: "replace" sets the selection as the exact
// tag set, "add" merges the selection into the existing tags.
export const tagsDialog = reactive({
  visible: false,
  assetId: null as string | null,
  assetIds: [] as string[],
  mode: null as TagAssetMode | null,
  upload: false,
})

export function openTagsDialog(): void {
  tagsDialog.assetId = null
  tagsDialog.assetIds = []
  tagsDialog.mode = null
  tagsDialog.upload = false
  tagsDialog.visible = true
}

export function openAssetTags(assetId: string): void {
  tagsDialog.assetId = assetId
  tagsDialog.assetIds = []
  tagsDialog.mode = null
  tagsDialog.upload = false
  tagsDialog.visible = true
}

export function openBulkAssetTags(
  assetIds: string[],
  mode: TagAssetMode,
): void {
  tagsDialog.assetId = null
  tagsDialog.assetIds = [...assetIds]
  tagsDialog.mode = mode
  tagsDialog.upload = false
  tagsDialog.visible = true
}

// openUploadTags reuses the tag manager as a picker for assets that are still
// staged in the upload dialog.
export function openUploadTags(): void {
  tagsDialog.assetId = null
  tagsDialog.assetIds = []
  tagsDialog.mode = null
  tagsDialog.upload = true
  tagsDialog.visible = true
}
