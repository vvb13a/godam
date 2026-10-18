<script setup lang="ts">
import Dialog from 'primevue/dialog'
import { reactive, ref, watch } from 'vue'
import { useToast } from 'primevue/usetoast'
import * as api from '../api/client'
import { library } from '../lib/library'
import { DEFAULT_ALLOWED, MIME_CATEGORIES } from '../lib/mimeTypes'
import { uploadSettingsDialog } from '../lib/uploadSettings'

const toast = useToast()
const saving = ref(false)
const selected = reactive<Record<string, boolean>>({})
const maxMb = ref(0)

watch(
  () => uploadSettingsDialog.visible,
  (visible) => {
    if (!visible) return
    const allowed =
      library.allowed_mime_types && library.allowed_mime_types.length
        ? library.allowed_mime_types
        : DEFAULT_ALLOWED
    for (const key of Object.keys(selected)) delete selected[key]
    for (const mime of allowed) selected[mime] = true
    maxMb.value = library.max_upload_bytes
      ? Math.round(library.max_upload_bytes / (1024 * 1024))
      : 0
  },
)

function toggleAll(categoryOptions: { value: string }[], on: boolean): void {
  for (const option of categoryOptions) selected[option.value] = on
}

async function save(): Promise<void> {
  const allowed = Object.keys(selected).filter((mime) => selected[mime])
  const maxBytes = Math.max(0, Math.round(maxMb.value)) * 1024 * 1024
  saving.value = true
  try {
    const updated = await api.setInstanceUploadSettings(allowed, maxBytes)
    Object.assign(library, updated)
    toast.add({ severity: 'success', summary: 'Upload settings saved', life: 2500 })
    uploadSettingsDialog.visible = false
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not save settings',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Dialog
    v-model:visible="uploadSettingsDialog.visible"
    header="Upload settings"
    modal
    :draggable="false"
    :style="{ width: '36rem' }"
  >
    <div class="space-y-5">
      <div>
        <p class="mb-2 text-sm font-medium text-slate-700 dark:text-slate-200">Allowed file types</p>
        <p class="mb-3 text-xs text-slate-400">
          Leave this library without an explicit list to inherit the app default.
        </p>
        <div class="max-h-72 space-y-3 overflow-y-auto pr-1">
          <div v-for="category in MIME_CATEGORIES" :key="category.label">
            <div class="mb-1 flex items-center justify-between">
              <span class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
                {{ category.label }}
              </span>
              <div class="flex gap-2 text-xs">
                <button
                  class="text-violet-600 dark:text-violet-400 hover:underline"
                  @click="toggleAll(category.options, true)"
                >
                  All
                </button>
                <button
                  class="text-slate-400 hover:underline"
                  @click="toggleAll(category.options, false)"
                >
                  None
                </button>
              </div>
            </div>
            <label
              v-for="option in category.options"
              :key="option.value"
              class="flex cursor-pointer items-center gap-2 py-0.5 text-sm text-slate-600 dark:text-slate-300"
            >
              <input v-model="selected[option.value]" type="checkbox" />
              {{ option.label }}
              <span class="text-xs text-slate-400">{{ option.value }}</span>
            </label>
          </div>
        </div>
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-200" for="max-upload">
          Max upload size (MB)
        </label>
        <input
          id="max-upload"
          v-model.number="maxMb"
          type="number"
          min="0"
          step="1"
          class="w-32 rounded border border-slate-300 dark:border-slate-700 px-3 py-1.5 text-sm"
        />
        <p class="mt-1 text-xs text-slate-400">0 keeps the app default.</p>
      </div>
    </div>

    <template #footer>
      <button
        class="rounded px-3 py-1.5 text-sm text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
        @click="uploadSettingsDialog.visible = false"
      >
        Cancel
      </button>
      <button
        class="rounded bg-violet-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-violet-700 disabled:opacity-50"
        :disabled="saving"
        @click="save"
      >
        Save
      </button>
    </template>
  </Dialog>
</template>
