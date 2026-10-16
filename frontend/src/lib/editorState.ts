import { ref } from 'vue'

// Shared edit-session state. The detail editor marks itself dirty while it holds
// unsaved edits, and the router guard consults this so *every* way of leaving
// the editor (sidebar clicks, other routes, the editor's Back control) is
// protected by the same app-native confirmation.
export const editorDirty = ref(false)

let leaveConfirm: (() => Promise<boolean>) | null = null
let closeEditor: (() => void) | null = null

export function setLeaveConfirm(fn: (() => Promise<boolean>) | null): void {
  leaveConfirm = fn
}

export function setEditorCloser(fn: (() => void) | null): void {
  closeEditor = fn
}

// Resolves true when it is safe to leave. The editor is closed once the user
// confirms (or immediately when there is nothing to confirm).
export async function leaveEditor(): Promise<boolean> {
  if (!editorDirty.value) {
    closeEditor?.()
    return true
  }
  const confirmed = leaveConfirm ? await leaveConfirm() : true
  if (!confirmed) return false
  editorDirty.value = false
  closeEditor?.()
  return true
}
