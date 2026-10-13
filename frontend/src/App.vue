<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
import { CloudUpload } from '@primeicons/vue'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import CommandPalette from './components/CommandPalette.vue'
import TagsDialog from './components/TagsDialog.vue'
import UploadDialog from './components/UploadDialog.vue'
import { commandMenu } from './lib/command'
import { openUpload } from './lib/upload'

const route = useRoute()
const dragging = ref(false)
let dragDepth = 0

function hasFiles(event: DragEvent): boolean {
  const types = event.dataTransfer?.types
  return !!types && Array.from(types).includes('Files')
}

function currentCollectionId(): string | null {
  const collection = route.query.collection
  if (typeof collection !== 'string' || collection === 'uncategorized') return null
  return collection
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
    openUpload(files, currentCollectionId())
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
})

onBeforeUnmount(() => {
  window.removeEventListener('dragenter', onDragEnter)
  window.removeEventListener('dragover', onDragOver)
  window.removeEventListener('dragleave', onDragLeave)
  window.removeEventListener('drop', onDrop)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="flex h-screen bg-slate-50 text-slate-800">
    <AppSidebar />
    <main class="min-h-0 flex-1 overflow-hidden">
      <RouterView />
    </main>

    <div
      v-if="dragging"
      class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-violet-600/10 backdrop-blur-sm"
    >
      <div
        class="rounded-lg border-2 border-dashed border-violet-400 bg-white px-12 py-10 text-center shadow-lg"
      >
        <CloudUpload :size="40" class="mx-auto text-violet-500" />
        <p class="mt-3 text-lg font-semibold text-violet-700">
          Drop files to upload
        </p>
        <p class="mt-1 text-sm text-slate-500">
          They'll be staged for review before uploading.
        </p>
      </div>
    </div>

    <UploadDialog />
    <CommandPalette />
    <TagsDialog />
    <ConfirmDialog />
    <Toast />
  </div>
</template>
