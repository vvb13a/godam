import { reactive } from 'vue'

// Global state for the instance upload settings dialog.
export const uploadSettingsDialog = reactive({
  visible: false,
})

export function openUploadSettings(): void {
  uploadSettingsDialog.visible = true
}
