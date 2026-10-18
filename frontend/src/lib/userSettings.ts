import { reactive } from 'vue'

// Global state for the operator account settings dialog so the command menu and
// the user menu can both open it.
export const userSettingsDialog = reactive({
  visible: false,
})

export function openUserSettings(): void {
  userSettingsDialog.visible = true
}
