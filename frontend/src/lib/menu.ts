import { ExclamationTriangle, Trash } from '@primeicons/vue'
import type { MenuItem } from 'primevue/menuitem'

// A destructive "Delete" entry that reveals a nested "Confirm" item. Shared by
// every context menu so the action looks and behaves identically: callers only
// supply the label and the action that runs once confirmed.
export function confirmDeleteMenuItem(
  label: string,
  command: () => void,
): MenuItem {
  return {
    label,
    icon: Trash,
    class: 'text-red-600',
    items: [
      {
        label: 'Confirm',
        icon: ExclamationTriangle,
        class: 'text-red-600',
        command,
      },
    ],
  }
}
