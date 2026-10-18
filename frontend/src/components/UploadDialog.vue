<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import ProgressBar from 'primevue/progressbar'
import { Times, Upload } from '@primeicons/vue'
import { computed, ref, watch } from 'vue'
import * as api from '../api/client'
import { formatSize } from '../lib/format'
import { library, notifyLibraryChanged } from '../lib/library'
import { ACCEPT_STRING } from '../lib/mimeTypes'
import { upload } from '../lib/upload'

const sourceUrl = ref('')
const uploading = ref(false)
const progress = ref(0)
const current = ref('')
const error = ref<string | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

const currentLibrary = computed(
  () => library.libraries.find((item) => item.id === library.currentId) ?? null,
)
const maxUploadBytes = computed(() => currentLibrary.value?.max_upload_bytes ?? 0)

watch(
  () => upload.visible,
  (open) => {
    if (!open) cleanup()
  },
)

function cleanup(): void {
  upload.files = []
  upload.destinationCollectionId = null
  upload.destinationTagId = null
  sourceUrl.value = ''
  uploading.value = false
  progress.value = 0
  current.value = ''
  error.value = null
  if (fileInput.value) fileInput.value.value = ''
}

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function onFiles(event: Event): void {
  const input = event.target as HTMLInputElement
  if (input.files) {
    upload.files.push(...Array.from(input.files))
  }
  input.value = ''
}

function removeFile(index: number): void {
  upload.files.splice(index, 1)
}

const destinationTagName = computed(() => {
  const id = upload.destinationTagId
  if (!id) return null
  return library.tags.find((tag) => tag.id === id)?.name ?? null
})

async function finish(assetIds: string[]): Promise<void> {
  const destination = upload.destinationCollectionId
  const tagId = upload.destinationTagId
  const libraryId = library.currentId
  if (destination) {
    await api.addAssetsToCollection(destination, assetIds)
  }
  if (tagId && libraryId) {
    await api.applyAssetsTags(libraryId, assetIds, [tagId], 'add')
  }
  notifyLibraryChanged()
  upload.visible = false
}

async function uploadFiles(): Promise<void> {
  if (!upload.files.length || uploading.value) return
  const libraryId = library.currentId
  if (!libraryId) return
  const limit = maxUploadBytes.value
  if (limit > 0) {
    const tooLarge = upload.files.find((file) => file.size > limit)
    if (tooLarge) {
      error.value = `“${tooLarge.name}” exceeds the ${formatSize(limit)} limit for this library.`
      return
    }
  }
  uploading.value = true
  error.value = null
  const total = upload.files.length
  try {
    const ids: string[] = []
    let done = 0
    for (const file of upload.files) {
      current.value = file.name
      const asset = await api.uploadAsset(libraryId, file, undefined, (fraction) => {
        progress.value = Math.round(((done + fraction) / total) * 100)
      })
      ids.push(asset.id)
      done++
      progress.value = Math.round((done / total) * 100)
    }
    await finish(ids)
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    uploading.value = false
  }
}

async function importUrl(): Promise<void> {
  const source = sourceUrl.value.trim()
  if (!source || uploading.value) return
  const libraryId = library.currentId
  if (!libraryId) return
  uploading.value = true
  error.value = null
  current.value = source
  progress.value = 0
  try {
    const asset = await api.importAsset(libraryId, source)
    await finish([asset.id])
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <Dialog
    v-model:visible="upload.visible"
    modal
    header="Upload assets"
    :style="{ width: '34rem' }"
    :close-on-escape="!uploading"
    :dismissable-mask="!uploading"
  >
    <div class="space-y-6">
      <div v-if="upload.files.length">
        <label class="mb-2 block text-sm font-medium">
          Staged files ({{ upload.files.length }})
        </label>
        <ul class="max-h-40 space-y-1 overflow-y-auto">
          <li
            v-for="(file, index) in upload.files"
            :key="`${file.name}-${index}`"
            class="flex items-center justify-between rounded border border-slate-200 dark:border-slate-800 px-2 py-1 text-sm"
          >
            <span class="truncate">{{ file.name }}</span>
            <button
              class="inline-flex shrink-0 text-slate-400 hover:text-red-500"
              title="Remove"
              @click="removeFile(index)"
            >
              <Times :size="14" />
            </button>
          </li>
        </ul>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium">Add files</label>
        <input
          ref="fileInput"
          type="file"
          multiple
          :accept="ACCEPT_STRING"
          class="block w-full text-sm text-slate-600 dark:text-slate-300 file:mr-3 file:rounded file:border-0 file:bg-violet-50 dark:file:bg-violet-950/50 file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-violet-700 dark:file:text-violet-300 hover:file:bg-violet-100"
          @change="onFiles"
        />
        <p v-if="maxUploadBytes" class="mt-1 text-xs text-slate-400">
          Max {{ formatSize(maxUploadBytes) }} per file in this library.
        </p>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium">From a URL</label>
        <div class="flex gap-2">
          <InputText
            v-model="sourceUrl"
            placeholder="https://example.com/image.jpg"
            class="flex-1"
            @keyup.enter="importUrl"
          />
          <Button
            label="Import"
            severity="secondary"
            :disabled="uploading || !sourceUrl.trim()"
            @click="importUrl"
          />
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium">Destination</label>
        <select
          v-model="upload.destinationCollectionId"
          class="w-full rounded border border-slate-300 dark:border-slate-700 px-3 py-2 text-sm"
        >
          <option :value="null">No collection</option>
          <option
            v-for="collection in library.collections"
            :key="collection.id"
            :value="collection.id"
          >
            {{ collection.name }}
          </option>
        </select>
        <p v-if="destinationTagName" class="mt-1 text-xs text-slate-500 dark:text-slate-400">
          Tag “{{ destinationTagName }}” will be applied to the uploaded assets.
        </p>
      </div>

      <div v-if="uploading" class="space-y-2">
        <ProgressBar :value="progress" />
        <p class="truncate text-xs text-slate-500 dark:text-slate-400">{{ current }}</p>
      </div>

      <Message v-if="error" severity="error">{{ error }}</Message>
    </div>

    <template #footer>
      <Button label="Cancel" text :disabled="uploading" @click="upload.visible = false" />
      <Button
        label="Upload"
        :loading="uploading"
        :disabled="!upload.files.length"
        @click="uploadFiles"
      >
        <template #icon><Upload :size="16" /></template>
      </Button>
    </template>
  </Dialog>
</template>
