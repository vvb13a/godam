import { reactive } from 'vue'

// upload is the shared staging area for the UploadDialog. Files dropped on the
// window (or picked in the dialog) accumulate here until the user confirms.
export const upload = reactive({
  visible: false,
  files: [] as File[],
  destinationCollectionId: null as string | null,
  destinationTagId: null as string | null,
})

// openUpload stages the given files (if any) and opens the dialog. The
// collection and tag defaults come from the view the user is currently in, so
// uploads are automatically associated with the active collection/tag filter.
export function openUpload(
  files: File[] = [],
  destination: string | null = null,
  destinationTag: string | null = null,
): void {
  if (files.length) {
    upload.files.push(...files)
  }
  upload.destinationCollectionId = destination
  upload.destinationTagId = destinationTag
  upload.visible = true
}
