import { reactive } from 'vue'

// presetsDialog is the global image-preset configurator. presetId selects an
// existing preset; null starts a new one.
export const presetsDialog = reactive({
  visible: false,
  presetId: null as string | null,
  assetId: null as string | null,
})

export function openPresetsDialog(
  presetId: string | null = null,
  assetId: string | null = null,
): void {
  presetsDialog.presetId = presetId
  presetsDialog.assetId = assetId
  presetsDialog.visible = true
}
