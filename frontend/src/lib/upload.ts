import { reactive } from 'vue'

// upload is the shared staging area for the UploadDialog. Files dropped on the
// window (or picked in the dialog) accumulate here until the user confirms.
export const upload = reactive({
  visible: false,
  files: [] as File[],
  destinationCollectionId: null as string | null,
})

// openUpload stages the given files (if any) and opens the dialog. The
// destination defaults to the collection the user is currently viewing.
export function openUpload(
  files: File[] = [],
  destination: string | null = null,
): void {
  if (files.length) {
    upload.files.push(...files)
  }
  upload.destinationCollectionId = destination
  upload.visible = true
}
