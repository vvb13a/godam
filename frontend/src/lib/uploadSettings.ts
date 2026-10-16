import { reactive } from 'vue'

// Global state for the per-library upload settings dialog.
export const uploadSettingsDialog = reactive<{
  visible: boolean
  libraryId: string | null
}>({
  visible: false,
  libraryId: null,
})

export function openUploadSettings(libraryId: string): void {
  uploadSettingsDialog.libraryId = libraryId
  uploadSettingsDialog.visible = true
}
