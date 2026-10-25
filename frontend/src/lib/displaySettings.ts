import { ref } from 'vue'

// The display controls live in AssetsView, but the trigger also exists in the
// attributes sidebar. Share the request and its anchor so the popover opens
// against the clicked button.
export const displaySettingsRequest = ref(0)
export const displaySettingsAnchor = ref<HTMLElement | null>(null)

export function openDisplaySettings(event: Event): void {
  displaySettingsAnchor.value = event.currentTarget as HTMLElement | null
  displaySettingsRequest.value++
}
