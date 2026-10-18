<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import ConfirmPopup from 'primevue/confirmpopup'
import Toast from 'primevue/toast'
import { useConfirm } from 'primevue/useconfirm'
import { CloudUpload } from '@primeicons/vue'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import CommandPalette from './components/CommandPalette.vue'
import LibraryOnboarding from './components/LibraryOnboarding.vue'
import LibraryUploadSettingsDialog from './components/LibraryUploadSettingsDialog.vue'
import TagsDialog from './components/TagsDialog.vue'
import UploadDialog from './components/UploadDialog.vue'
import { auth } from './lib/auth'
import { commandMenu } from './lib/command'
import { editorDirty, setLeaveConfirm } from './lib/editorState'
import { embed, startEmbedBridge } from './lib/embed'
import { initLibrary, library, resetLibrary } from './lib/library'
import { openUpload } from './lib/upload'

const route = useRoute()
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

function currentCollectionId(): string | null {
  const collection = route.query.collection
  if (typeof collection !== 'string' || collection === 'uncategorized') return null
  return collection
}

// currentTagId returns the tag filter's id only when it is a single, concrete
// tag (not the "untagged" pseudo-filter or a multi-tag selection).
function currentTagId(): string | null {
  const tag = route.query.tag
  if (typeof tag !== 'string' || !tag) return null
  if (tag === 'untagged' || tag.includes(',')) return null
  return tag
}

function onDragEnter(event: DragEvent): void {
  if (!hasFiles(event)) return
  dragDepth++
  dragging.value = true
}

function onDragOver(event: DragEvent): void {
  if (!hasFiles(event)) return
  event.preventDefault()
}

function onDragLeave(event: DragEvent): void {
  if (!hasFiles(event)) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragging.value = false
}

function onDrop(event: DragEvent): void {
  if (!hasFiles(event)) return
  event.preventDefault()
  dragDepth = 0
  dragging.value = false
  const files = event.dataTransfer?.files
    ? Array.from(event.dataTransfer.files)
    : []
  if (files.length) {
    openUpload(files, currentCollectionId(), currentTagId())
  }
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    commandMenu.visible = !commandMenu.visible
  }
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

// Load (or clear) the library scope whenever the signed-in user changes. An
// embedding host pins the app to the requested project.
watch(
  () => auth.user,
  (user) => {
    if (user) {
      void initLibrary(
        embed.active && embed.libraryId ? embed.libraryId : undefined,
      )
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
  <div class="flex h-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-100">
    <RouterView v-if="!auth.user" />

    <template v-else>
      <div
        v-if="!library.loaded"
        class="flex h-full w-full items-center justify-center text-sm text-slate-400"
      >
        Loading…
      </div>

      <p
        v-else-if="embed.active && !library.libraries.length"
        class="flex h-full w-full items-center justify-center text-sm text-slate-400"
      >
        No libraries are available in this workspace.
      </p>

      <LibraryOnboarding v-else-if="!library.libraries.length" />

      <template v-else>
        <AppSidebar :embed="embed.active" />
        <main class="min-h-0 flex-1 overflow-hidden">
          <RouterView />
        </main>

        <div
          v-if="dragging"
          class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-violet-600/10 backdrop-blur-sm"
        >
          <div
            class="rounded-lg border-2 border-dashed border-violet-400 bg-white dark:bg-slate-900 px-12 py-10 text-center shadow-lg"
          >
            <CloudUpload :size="40" class="mx-auto text-violet-500" />
            <p class="mt-3 text-lg font-semibold text-violet-700 dark:text-violet-300">
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
      <TagsDialog />
      <LibraryUploadSettingsDialog />
    </template>

    <ConfirmDialog group="navigation" />
    <ConfirmPopup group="editor" />
    <Toast />
  </div>
</template>
