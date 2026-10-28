<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import ConfirmPopup from 'primevue/confirmpopup'
import Toast from 'primevue/toast'
import { useConfirm } from 'primevue/useconfirm'
import { CloudUpload } from '@primeicons/vue'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterView } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import AppSidebarAttributes from './components/AppSidebarAttributes.vue'
import AttributesDialog from './components/AttributesDialog.vue'
import CommandPalette from './components/CommandPalette.vue'
import LibraryBackupDialog from './components/LibraryBackupDialog.vue'
import LibraryUploadSettingsDialog from './components/LibraryUploadSettingsDialog.vue'
import PresetsDialog from './components/PresetsDialog.vue'
import WarmPresetsProgress from './components/WarmPresetsProgress.vue'
import UploadDialog from './components/UploadDialog.vue'
import { auth } from './lib/auth'
import { hasCapability } from './lib/capabilities'
import { commandMenu } from './lib/command'
import { editorDirty, setLeaveConfirm } from './lib/editorState'
import { sidebarPrefs } from './lib/sidebar'
import { embed, startEmbedBridge } from './lib/embed'
import { initLibrary, library, resetLibrary } from './lib/library'
import { applyUploadSettings, openUpload, upload } from './lib/upload'

const confirm = useConfirm()
const dragging = ref(false)
let dragDepth = 0

// A single app-native confirmation for leaving the editor with unsaved changes,
// used by the router guard and the editor's Back control alike.
setLeaveConfirm(
  () =>
    new Promise<boolean>((resolve) => {
      confirm.require({
        group: 'navigation',
        header: 'Discard changes?',
        message: 'You have unsaved changes. Leave without saving?',
        acceptLabel: 'Discard',
        rejectLabel: 'Cancel',
        acceptProps: { severity: 'danger' },
        rejectProps: { text: true },
        accept: () => resolve(true),
        reject: () => resolve(false),
        // Dismissing (Escape/overlay) fires onHide rather than reject; resolve
        // false so the navigation guard never hangs. resolve is idempotent.
        onHide: () => resolve(false),
      })
    }),
)

// The browser's unload hook can only show its own generic prompt, not an app
// dialog; use it to still protect tab close/reload while editing.
function onBeforeUnload(event: BeforeUnloadEvent): void {
  if (!editorDirty.value) return
  event.preventDefault()
  event.returnValue = ''
}

function hasFiles(event: DragEvent): boolean {
  const types = event.dataTransfer?.types
  return !!types && Array.from(types).includes('Files')
}

function resetGlobalDrag(): void {
  dragDepth = 0
  dragging.value = false
}

function onDragEnter(event: DragEvent): void {
  if (upload.visible) {
    resetGlobalDrag()
    return
  }
  if (!hasFiles(event)) return
  dragDepth++
  dragging.value = true
}

function onDragOver(event: DragEvent): void {
  if (upload.visible) return
  if (!hasFiles(event)) return
  event.preventDefault()
}

function onDragLeave(event: DragEvent): void {
  if (upload.visible) {
    resetGlobalDrag()
    return
  }
  if (!hasFiles(event)) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragging.value = false
}

function onDrop(event: DragEvent): void {
  if (upload.visible) return
  if (!hasFiles(event)) return
  if (!hasCapability('assets:upload')) return
  event.preventDefault()
  dragDepth = 0
  dragging.value = false
  const files = event.dataTransfer?.files
    ? Array.from(event.dataTransfer.files)
    : []
  if (files.length) {
    openUpload(files)
  }
}

const DOUBLE_SHIFT_MS = 350
let lastShiftAt = 0

function isTypingTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  return (
    el.tagName === 'INPUT' ||
    el.tagName === 'TEXTAREA' ||
    el.tagName === 'SELECT' ||
    el.isContentEditable
  )
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    commandMenu.visible = !commandMenu.visible
    return
  }
  if (event.key !== 'Shift') {
    lastShiftAt = 0
    return
  }
  if (event.repeat || event.metaKey || event.ctrlKey || event.altKey) return
  if (isTypingTarget(event.target)) {
    lastShiftAt = 0
    return
  }
  const now = performance.now()
  if (now - lastShiftAt <= DOUBLE_SHIFT_MS) {
    lastShiftAt = 0
    event.preventDefault()
    commandMenu.visible = !commandMenu.visible
    return
  }
  lastShiftAt = now
}

onMounted(() => {
  window.addEventListener('dragenter', onDragEnter)
  window.addEventListener('dragover', onDragOver)
  window.addEventListener('dragleave', onDragLeave)
  window.addEventListener('drop', onDrop)
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('beforeunload', onBeforeUnload)
})

startEmbedBridge()

// The picker is rendered inside a host-owned modal. Keep the iframe canvas
// transparent so the consuming application's backdrop and rounded container
// show through instead of a godam-coloured rectangle.
watch(
  () => embed.active && embed.mode !== 'library',
  (picker) => {
    document.documentElement.classList.toggle('embed-picker', picker)
  },
  { immediate: true },
)

watch(
  () => upload.visible,
  (visible) => {
    if (visible) resetGlobalDrag()
  },
)

// Load (or clear) the library scope whenever the signed-in user changes. An
// embedding host pins the app to the requested project.
watch(
  () => auth.user,
  (user) => {
    if (user) {
      void initLibrary().then(() => {
        applyUploadSettings(user.scope?.upload_settings)
      })
    } else {
      resetLibrary()
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  window.removeEventListener('dragenter', onDragEnter)
  window.removeEventListener('dragover', onDragOver)
  window.removeEventListener('dragleave', onDragLeave)
  window.removeEventListener('drop', onDrop)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('beforeunload', onBeforeUnload)
})
</script>

<template>
  <div
    class="flex h-screen text-slate-800 dark:text-slate-100"
    :class="
      embed.active && embed.mode !== 'library'
        ? 'bg-transparent'
        : 'bg-slate-50 dark:bg-[#0b1220]'
    "
  >
    <div
      v-if="!library.loaded"
      class="flex h-full w-full items-center justify-center text-sm text-slate-400"
    >
      Loading…
    </div>

    <p
      v-else-if="!library.id"
      class="flex h-full w-full items-center justify-center text-sm text-slate-400"
    >
      No library is configured for this instance.
    </p>

    <template v-else>
      <div class="flex min-h-0 min-w-0 flex-1 py-4">
        <AppSidebarAttributes v-if="sidebarPrefs.mode === 'attributes'" />
        <AppSidebar v-else />
        <main class="min-h-0 flex-1 overflow-hidden">
          <RouterView />
        </main>
      </div>

      <div
        v-if="dragging"
        class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-brand-600/10 backdrop-blur-sm"
      >
        <div
          class="rounded-lg border-2 border-dashed border-brand-400 bg-white dark:bg-slate-900 px-12 py-10 text-center shadow-lg"
        >
          <CloudUpload :size="40" class="mx-auto text-brand-500" />
          <p class="mt-3 text-lg font-semibold text-brand-700 dark:text-brand-300">
            Drop files to upload
          </p>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
            They'll be staged for review before uploading.
          </p>
        </div>
      </div>
    </template>

    <UploadDialog />
    <CommandPalette />
    <AttributesDialog />
    <PresetsDialog />
    <WarmPresetsProgress />
    <LibraryUploadSettingsDialog />
    <LibraryBackupDialog />

    <ConfirmDialog group="navigation" />
    <ConfirmDialog group="upload-discard" />
    <ConfirmDialog group="attributes" />
    <ConfirmPopup group="editor" />
    <ConfirmPopup group="upload" />
    <Toast />
  </div>
</template>
