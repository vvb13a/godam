<script setup lang="ts">
import { Eye, EyeSlash } from '@primeicons/vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import { reactive, ref, watch } from 'vue'
import { useToast } from 'primevue/usetoast'
import * as api from '../api/client'
import { library } from '../lib/library'
import { DEFAULT_ALLOWED, MIME_CATEGORIES } from '../lib/mimeTypes'
import { uploadSettingsDialog } from '../lib/uploadSettings'
import type { UploadLimits } from '../types'

const toast = useToast()
const saving = ref(false)
const selected = reactive<Record<string, boolean>>({})
const limitMb = reactive<Record<keyof UploadLimits, number | null>>({
  image_bytes: 0,
  audio_bytes: 0,
  video_bytes: 0,
  pdf_bytes: 0,
})

function mbToBytes(value: number | null | undefined): number {
  return Math.max(0, Math.round(value ?? 0)) * 1024 * 1024
}

function bytesToMb(value: number | undefined): number {
  return value ? Math.round(value / (1024 * 1024)) : 0
}

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
    const limits = library.upload_limits ?? {}
    for (const category of MIME_CATEGORIES) {
      limitMb[category.limitKey] = bytesToMb(limits[category.limitKey])
    }
  },
)

function allSelected(categoryOptions: { value: string }[]): boolean {
  return categoryOptions.every((option) => selected[option.value])
}

function toggleAll(categoryOptions: { value: string }[]): void {
  const next = !allSelected(categoryOptions)
  for (const option of categoryOptions) selected[option.value] = next
}

function close(): void {
  uploadSettingsDialog.visible = false
}

async function save(): Promise<void> {
  const allowed = Object.keys(selected).filter((mime) => selected[mime])
  const limits = {} as UploadLimits
  for (const category of MIME_CATEGORIES) {
    limits[category.limitKey] = mbToBytes(limitMb[category.limitKey])
  }
  saving.value = true
  try {
    const updated = await api.setInstanceUploadSettings(allowed, limits)
    Object.assign(library, updated)
    toast.add({ severity: 'success', summary: 'Upload settings saved', life: 2500 })
    close()
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
    :style="{ width: '48rem', maxWidth: '95vw' }"
  >
    <div class="space-y-3">
      <p class="text-sm text-slate-500 dark:text-slate-400">
        Choose the allowed formats and max size per type. 0 MB means unlimited.
      </p>

      <div
        class="hidden grid-cols-[6rem_1fr_8rem] gap-3 px-3 text-xs font-semibold tracking-wide text-slate-400 uppercase md:grid"
      >
        <span>Format</span>
        <span>Allowed</span>
        <span class="text-right">Max size</span>
      </div>

      <div
        v-for="category in MIME_CATEGORIES"
        :key="category.label"
        class="flex flex-col gap-2 rounded-lg border border-slate-200 p-3 md:grid md:grid-cols-[6rem_1fr_8rem] md:items-center md:gap-3 dark:border-slate-700"
      >
        <div class="flex items-center gap-1">
          <span class="text-sm font-medium text-slate-700 dark:text-slate-200">
            {{ category.label }}
          </span>
          <Button
            text
            rounded
            size="small"
            severity="secondary"
            class="!h-6 !w-6 !p-0"
            :aria-label="allSelected(category.options) ? 'Deselect all' : 'Select all'"
            @click="toggleAll(category.options)"
          >
            <template #icon>
              <Eye v-if="allSelected(category.options)" :size="14" />
              <EyeSlash v-else :size="14" />
            </template>
          </Button>
        </div>

        <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5">
          <div
            v-for="option in category.options"
            :key="option.value"
            class="flex items-center gap-1.5"
          >
            <Checkbox
              :input-id="option.value"
              v-model="selected[option.value]"
              binary
            />
            <label
              :for="option.value"
              class="cursor-pointer text-sm text-slate-700 dark:text-slate-200"
            >
              {{ option.label }}
            </label>
          </div>
        </div>

        <div class="flex items-center justify-between gap-2 md:justify-end">
          <span class="text-xs text-slate-400 md:hidden">Max size</span>
          <div class="w-28 shrink-0">
            <InputNumber
              v-model="limitMb[category.limitKey]"
              :min="0"
              :max-fraction-digits="0"
              suffix=" MB"
              fluid
            />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <Button label="Cancel" text severity="secondary" @click="close" />
      <Button label="Save" :loading="saving" @click="save" />
    </template>
  </Dialog>
</template>
