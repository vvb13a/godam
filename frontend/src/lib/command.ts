import { reactive } from 'vue'

// commandMenu holds the visibility of the global command palette so any
// component can open it (e.g. the sidebar trigger or the Cmd/Ctrl+K shortcut).
export const commandMenu = reactive({
  visible: false,
})

export function openCommandMenu(): void {
  commandMenu.visible = true
}
