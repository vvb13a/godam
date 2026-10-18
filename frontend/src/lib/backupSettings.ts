import { reactive } from 'vue'

// Global state for the instance backup settings dialog.
export const backupSettingsDialog = reactive({
  visible: false,
})

export function openBackupSettings(): void {
  backupSettingsDialog.visible = true
}
