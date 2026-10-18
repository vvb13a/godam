import { reactive } from 'vue'

// Global state for the per-library backup settings dialog.
export const backupSettingsDialog = reactive<{
  visible: boolean
  libraryId: string | null
}>({
  visible: false,
  libraryId: null,
})

export function openBackupSettings(libraryId: string): void {
  backupSettingsDialog.libraryId = libraryId
  backupSettingsDialog.visible = true
}
