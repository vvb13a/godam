<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputGroup from 'primevue/inputgroup'
import InputGroupAddon from 'primevue/inputgroupaddon'
import Message from 'primevue/message'
import Select from 'primevue/select'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import {
  Check,
  CloudUpload,
  ExclamationCircle,
  Plus,
  Refresh,
  SortAmountDown,
  SortAmountUp,
  Times,
  Trash,
  Undo,
  Upload,
} from '@primeicons/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as api from '../api/client'
import { hasCapability } from '../lib/capabilities'
import { formatDuration, formatSize } from '../lib/format'
import { notifyLibraryChanged } from '../lib/library'
import { ACCEPT_STRING } from '../lib/mimeTypes'
import { theme } from '../lib/theme'
import {
  clearUploadItems,
  createUploadItem,
  disposeUploadItem,
  fileForUpload,
  hasCategoryUploadLimits,
  isAbortError,
  upload,
  uploadFile,
} from '../lib/upload'
import type { UploadItem } from '../lib/upload'
import FileTypePreview from './FileTypePreview.vue'

const UPLOAD_CONCURRENCY = 3

type SortKey = 'name' | 'size' | 'type' | 'status'
type SortDirection = 'asc' | 'desc'

const SORT_OPTIONS: Array<{ label: string; value: SortKey }> = [
  { label: 'Name', value: 'name' },
  { label: 'Size', value: 'size' },
  { label: 'Type', value: 'type' },
  { label: 'Status', value: 'status' },
]

const toast = useToast()
const confirm = useConfirm()
const running = ref(false)
const error = ref<string | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const cancelers = new Map<string, AbortController>()
const runStartedAt = ref(0)
const runStartBytes = ref(0)
const nowTick = ref(Date.now())
const displayProgress = ref(0)
let ticker: number | undefined
let progressFrame: number | undefined

const sortKey = ref<SortKey>('name')
const sortDirection = ref<SortDirection>('asc')
const editingId = ref<string | null>(null)
const editingName = ref('')
const renameInput = ref<HTMLInputElement | null>(null)
const dragActive = ref(false)
let dragDepth = 0

const categoryLimits = computed(() => hasCategoryUploadLimits())
const canDeleteAssets = computed(() => hasCapability('assets:delete'))

const totalBytes = computed(() =>
  upload.items.reduce((sum, item) => sum + item.file.size, 0),
)

const doneCount = computed(
  () => upload.items.filter((item) => item.status === 'done').length,
)

const errorCount = computed(
  () => upload.items.filter((item) => item.status === 'error').length,
)

const queuedCount = computed(
  () => upload.items.filter((item) => item.status === 'queued').length,
)

const activeCount = computed(
  () =>
    upload.items.filter(
      (item) => item.status === 'uploading' || item.status === 'processing',
    ).length,
)

const canceledCount = computed(
  () => upload.items.filter((item) => item.status === 'canceled').length,
)

const retryableFailedCount = computed(
  () =>
    upload.items.filter(
      (item) =>
        (item.status === 'error' || item.status === 'canceled') &&
        item.retryable,
    ).length,
)

const pendingCount = computed(
  () => queuedCount.value + retryableFailedCount.value,
)

const stagedCount = computed(
  () =>
    upload.items.filter(
      (item) =>
        item.status === 'queued' ||
        item.status === 'error' ||
        item.status === 'canceled',
    ).length,
)

const fillGrid = computed(
  () => upload.items.length > 0 && upload.items.length <= 20,
)

const gridStyle = computed(() => {
  if (!fillGrid.value) return undefined
  const count = upload.items.length
  const columns = Math.ceil(Math.sqrt(count))
  const rows = Math.ceil(count / columns)
  return {
    gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
    gridTemplateRows: `repeat(${rows}, minmax(0, 1fr))`,
  }
})

const allSucceeded = computed(
  () =>
    upload.items.length > 0 &&
    upload.items.every((item) => item.status === 'done'),
)

const uploadedBytes = computed(() =>
  upload.items.reduce((sum, item) => {
    if (item.status === 'done') return sum + item.file.size
    return sum + item.file.size * Math.min(Math.max(item.progress, 0), 1)
  }, 0),
)

const overallProgress = computed(() => {
  const total = upload.items.length
  if (!total) return 0
  const completed = upload.items.reduce((sum, item) => {
    if (item.status === 'done') return sum + 1
    if (item.status === 'processing') return sum + 1
    if (item.status === 'uploading') {
      return sum + Math.min(Math.max(item.progress, 0), 1)
    }
    return sum
  }, 0)
  const percent = Math.round((completed / total) * 100)
  return Math.min(100, Math.max(0, percent))
})

const pendingBytes = computed(() =>
  upload.items.reduce((sum, item) => {
    if (
      item.status !== 'queued' &&
      item.status !== 'uploading' &&
      item.status !== 'processing'
    ) {
      return sum
    }
    return sum + item.file.size * (1 - Math.min(Math.max(item.progress, 0), 1))
  }, 0),
)

const uploadSpeed = computed(() => {
  if (!running.value || !runStartedAt.value) return 0
  const elapsed = (nowTick.value - runStartedAt.value) / 1000
  if (elapsed < 0.5) return 0
  return Math.max(0, (uploadedBytes.value - runStartBytes.value) / elapsed)
})

const etaSeconds = computed(() => {
  if (!uploadSpeed.value) return 0
  return Math.ceil(pendingBytes.value / uploadSpeed.value)
})

const brandColor = computed(() => (theme.resolved === 'dark' ? '#fb923c' : '#6366f1'))

const stateRing = computed(() => {
  const total = upload.items.length
  if (!total) return 'conic-gradient(#e2e8f0 0deg 360deg)'
  const segments: Array<[number, string]> = [
    [doneCount.value, '#22c55e'],
    [activeCount.value, brandColor.value],
    [errorCount.value, '#ef4444'],
    [queuedCount.value, '#cbd5e1'],
    [canceledCount.value, '#94a3b8'],
  ]
  let angle = 0
  const stops: string[] = []
  for (const [count, color] of segments) {
    if (!count) continue
    const end = angle + (count / total) * 360
    stops.push(`${color} ${angle}deg ${end}deg`)
    angle = end
  }
  return `conic-gradient(${stops.join(', ')})`
})

const stateLegend = computed(() => [
  { key: 'done', label: 'Uploaded', count: doneCount.value, color: '#22c55e' },
  { key: 'active', label: 'Uploading', count: activeCount.value, color: brandColor.value },
  { key: 'error', label: 'Failed', count: errorCount.value, color: '#ef4444' },
  { key: 'queued', label: 'Waiting', count: queuedCount.value, color: '#cbd5e1' },
  { key: 'canceled', label: 'Canceled', count: canceledCount.value, color: '#94a3b8' },
])

const primaryLabel = computed(() => {
  if (running.value) return 'Uploading…'
  if (!upload.items.length) return 'Choose files'
  if (retryableFailedCount.value && !queuedCount.value) return 'Retry'
  if (pendingCount.value) return 'Upload'
  return 'Close'
})

const showCancel = computed(() => running.value || pendingCount.value > 0)

const unuploadedCount = computed(
  () => upload.items.filter((item) => item.status !== 'done').length,
)

const dialogVisible = computed({
  get: () => upload.visible,
  set: (value: boolean) => {
    if (value) {
      upload.visible = true
      return
    }
    requestClose()
  },
})

const sortedItems = computed(() => {
  const direction = sortDirection.value === 'asc' ? 1 : -1
  const items = [...upload.items]
  items.sort((a, b) => {
    let result = 0
    switch (sortKey.value) {
      case 'name':
        result = a.name.localeCompare(b.name)
        break
      case 'size':
        result = a.file.size - b.file.size
        break
      case 'type':
        result = extensionOf(a.name).localeCompare(extensionOf(b.name))
        break
      case 'status':
        result = statusRank(a.status) - statusRank(b.status)
        break
    }
    if (result === 0) result = a.name.localeCompare(b.name)
    return result * direction
  })
  return items
})

watch(
  () => upload.visible,
  (open) => {
    if (!open) cleanup()
  },
)

watch(overallProgress, (target) => animateProgress(target))

onBeforeUnmount(cleanup)

onMounted(() => {
  window.addEventListener('dragenter', onWindowDragEnter)
  window.addEventListener('dragover', onWindowDragOver)
  window.addEventListener('dragleave', onWindowDragLeave)
  window.addEventListener('drop', onWindowDrop)
  window.addEventListener('dragend', resetDragState)
})

onBeforeUnmount(() => {
  window.removeEventListener('dragenter', onWindowDragEnter)
  window.removeEventListener('dragover', onWindowDragOver)
  window.removeEventListener('dragleave', onWindowDragLeave)
  window.removeEventListener('drop', onWindowDrop)
  window.removeEventListener('dragend', resetDragState)
})

function resetDragState(): void {
  dragDepth = 0
  dragActive.value = false
}

function cleanup(): void {
  stopTicker()
  stopProgressAnimation()
  for (const controller of cancelers.values()) {
    controller.abort()
  }
  cancelers.clear()
  clearUploadItems()
  editingId.value = null
  running.value = false
  error.value = null
  displayProgress.value = 0
  resetDragState()
  if (fileInput.value) fileInput.value.value = ''
}

function animateProgress(target: number): void {
  stopProgressAnimation()
  const start = displayProgress.value
  if (start === target) return
  const started = performance.now()
  const duration = Math.min(320, 90 + Math.abs(target - start) * 2)
  const step = (now: number) => {
    const t = Math.min(1, Math.max(0, (now - started) / duration))
    const value = Math.round(start + (target - start) * t)
    displayProgress.value = Math.min(100, Math.max(0, value))
    if (t < 1) {
      progressFrame = requestAnimationFrame(step)
    } else {
      progressFrame = undefined
    }
  }
  progressFrame = requestAnimationFrame(step)
}

function stopProgressAnimation(): void {
  if (progressFrame !== undefined) {
    cancelAnimationFrame(progressFrame)
    progressFrame = undefined
  }
}

function startTicker(): void {
  stopTicker()
  nowTick.value = Date.now()
  ticker = window.setInterval(() => {
    nowTick.value = Date.now()
  }, 1000)
}

function stopTicker(): void {
  if (ticker !== undefined) {
    window.clearInterval(ticker)
    ticker = undefined
  }
}

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function openPicker(): void {
  if (!running.value) fileInput.value?.click()
}

function fileKey(file: File): string {
  return `${file.name}\u0000${file.size}\u0000${file.lastModified}`
}

function addFiles(files: File[]): void {
  const existing = new Set(upload.items.map((item) => fileKey(item.file)))
  let skipped = 0
  for (const file of files) {
    const key = fileKey(file)
    if (existing.has(key)) {
      skipped++
      continue
    }
    existing.add(key)
    upload.items.push(createUploadItem(file))
  }
  if (skipped > 0) {
    toast.add({
      severity: 'info',
      summary: 'Duplicate skipped',
      detail:
        skipped === 1
          ? 'That file is already in the queue.'
          : `${skipped} duplicate files were skipped.`,
      life: 3000,
    })
  }
}

function onFiles(event: Event): void {
  const input = event.target as HTMLInputElement
  if (input.files) addFiles(Array.from(input.files))
  input.value = ''
}

function hasFiles(event: DragEvent): boolean {
  const types = event.dataTransfer?.types
  return !!types && Array.from(types).includes('Files')
}

function onWindowDragEnter(event: DragEvent): void {
  if (!upload.visible || running.value || !hasFiles(event)) return
  event.preventDefault()
  dragDepth++
  dragActive.value = true
}

function onWindowDragOver(event: DragEvent): void {
  if (!upload.visible || !hasFiles(event)) return
  event.preventDefault()
}

function onWindowDragLeave(): void {
  if (!upload.visible) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragActive.value = false
}

function onWindowDrop(event: DragEvent): void {
  if (!upload.visible || !hasFiles(event)) return
  event.preventDefault()
  resetDragState()
  if (running.value) return
  const files = event.dataTransfer?.files
    ? Array.from(event.dataTransfer.files)
    : []
  if (files.length) addFiles(files)
}


function extensionOf(name: string): string {
  const dot = name.lastIndexOf('.')
  if (dot <= 0 || dot === name.length - 1) return 'FILE'
  return name.slice(dot + 1).toUpperCase()
}

function statusRank(status: UploadItem['status']): number {
  switch (status) {
    case 'queued':
      return 0
    case 'uploading':
      return 1
    case 'processing':
      return 2
    case 'done':
      return 3
    case 'error':
      return 4
    case 'canceled':
      return 5
  }
}


function canRename(item: UploadItem): boolean {
  return (
    item.status === 'queued' ||
    item.status === 'error' ||
    item.status === 'canceled'
  )
}

function bindRenameInput(el: unknown): void {
  renameInput.value = el instanceof HTMLInputElement ? el : null
}

async function startRename(item: UploadItem): Promise<void> {
  if (!canRename(item)) return
  editingId.value = item.id
  editingName.value = item.name
  await nextTick()
  renameInput.value?.focus()
  renameInput.value?.select()
}

function commitRename(item: UploadItem): void {
  if (editingId.value !== item.id) return
  const name = editingName.value.trim()
  if (name) item.name = name
  editingId.value = null
}

function cancelRename(): void {
  editingId.value = null
}


function itemBorder(item: UploadItem): string {
  if (item.status === 'done') {
    return 'border-green-300 bg-green-50/40 dark:border-green-900 dark:bg-green-950/10'
  }
  if (item.status === 'error') {
    return 'border-red-300 bg-red-50/40 dark:border-red-900 dark:bg-red-950/10'
  }
  if (item.status === 'canceled') {
    return 'border-slate-300 dark:border-slate-700'
  }
  return 'border-slate-200 dark:border-slate-800'
}

function cancelItem(item: UploadItem): void {
  cancelers.get(item.id)?.abort()
}

function removeFile(item: UploadItem): void {
  if (running.value) return
  if (item.status === 'uploading') cancelItem(item)
  if (item.status === 'processing') return
  if (editingId.value === item.id) editingId.value = null
  disposeUploadItem(item)
  const index = upload.items.indexOf(item)
  if (index >= 0) upload.items.splice(index, 1)
}

function clearStaged(): void {
  if (running.value) return
  for (const item of upload.items) {
    if (
      item.status === 'queued' ||
      item.status === 'error' ||
      item.status === 'canceled'
    ) {
      disposeUploadItem(item)
    }
  }
  upload.items = upload.items.filter(
    (item) =>
      item.status !== 'queued' &&
      item.status !== 'error' &&
      item.status !== 'canceled',
  )
}

function requestRemoveItem(item: UploadItem, event: Event): void {
  if (running.value) return
  if (item.status !== 'done') {
    removeFile(item)
    return
  }
  if (!canDeleteAssets.value) return
  confirm.require({
    group: 'upload',
    target: event.currentTarget as HTMLElement,
    header: 'Revert upload?',
    message: `Permanently delete “${item.name}” from the library? This cannot be undone.`,
    acceptLabel: 'Revert',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => void revertItem(item),
    reject: () => undefined,
  })
}

async function revertItem(item: UploadItem): Promise<void> {
  if (!item.assetId || running.value || !canDeleteAssets.value) return
  try {
    await api.deleteAsset(item.assetId)
    item.assetId = null
    disposeUploadItem(item)
    const index = upload.items.indexOf(item)
    if (index >= 0) upload.items.splice(index, 1)
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary: 'Reverted',
      detail: item.name,
      life: 3000,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not revert upload',
      detail: toMessage(e),
      life: 6000,
    })
  }
}

function requestRevertUploaded(event: Event): void {
  const uploaded = upload.items.filter(
    (item) => item.status === 'done' && item.assetId,
  )
  if (running.value || !uploaded.length || !canDeleteAssets.value) return
  confirm.require({
    group: 'upload',
    target: event.currentTarget as HTMLElement,
    header: 'Revert uploaded assets?',
    message: `Permanently delete ${uploaded.length} uploaded ${
      uploaded.length === 1 ? 'asset' : 'assets'
    } from the library? This cannot be undone.`,
    acceptLabel: 'Revert',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => void revertUploaded(),
    reject: () => undefined,
  })
}

async function revertUploaded(): Promise<void> {
  if (running.value || !canDeleteAssets.value) return
  const uploaded = upload.items.filter(
    (item) => item.status === 'done' && item.assetId,
  )
  if (!uploaded.length) return
  const results = await Promise.allSettled(
    uploaded.map((item) => api.deleteAsset(item.assetId as string)),
  )
  let reverted = 0
  let failed = 0
  uploaded.forEach((item, index) => {
    if (results[index]?.status === 'fulfilled') {
      reverted++
      item.assetId = null
      disposeUploadItem(item)
      const itemIndex = upload.items.indexOf(item)
      if (itemIndex >= 0) upload.items.splice(itemIndex, 1)
    } else {
      failed++
    }
  })
  if (reverted > 0) {
    notifyLibraryChanged()
    toast.add({
      severity: 'success',
      summary: 'Uploads reverted',
      detail:
        reverted === 1
          ? '1 uploaded asset was deleted.'
          : `${reverted} uploaded assets were deleted.`,
      life: 3500,
    })
  }
  if (failed > 0) {
    toast.add({
      severity: 'error',
      summary: 'Some uploads could not be reverted',
      detail:
        failed === 1
          ? '1 asset could not be deleted.'
          : `${failed} assets could not be deleted.`,
      life: 6000,
    })
  }
}

function retryItem(item: UploadItem): void {
  if (!item.retryable || item.status === 'uploading' || item.status === 'processing') {
    return
  }
  item.status = 'queued'
  item.progress = 0
  item.error = null
  item.assetId = null
  if (!running.value) void runQueue()
}

function cancelAll(): void {
  for (const item of upload.items) {
    if (item.status === 'queued') {
      item.status = 'canceled'
      item.error = 'Upload canceled'
    } else if (item.status === 'uploading') {
      cancelItem(item)
    }
  }
}

async function processItem(item: UploadItem): Promise<boolean> {
  const controller = new AbortController()
  cancelers.set(item.id, controller)
  item.status = 'uploading'
  item.progress = 0
  item.error = null
  item.assetId = null

  try {
    const asset = await uploadFile(
      fileForUpload(item),
      {
        onProgress: (fraction) => {
          item.progress = fraction
        },
        onProcessing: () => {
          item.status = 'processing'
          item.progress = 1
        },
      },
      controller.signal,
    )
    item.assetId = asset.id
    item.progress = 1
    item.status = 'done'
    return true
  } catch (e) {
    if (controller.signal.aborted || isAbortError(e)) {
      item.status = 'canceled'
      item.error = 'Upload canceled'
    } else {
      const message = toMessage(e)
      item.status = 'error'
      item.error = message
      toast.add({
        severity: 'error',
        summary: 'Upload failed',
        detail: `${item.name}: ${message}`,
        life: 6000,
      })
    }
    return false
  } finally {
    cancelers.delete(item.id)
  }
}

async function runQueue(): Promise<void> {
  if (running.value) return
  const queue = sortedItems.value.filter(
    (item) =>
      item.status === 'queued' ||
      ((item.status === 'error' || item.status === 'canceled') &&
        item.retryable),
  )
  if (!queue.length) return

  for (const item of queue) {
    item.status = 'queued'
    item.progress = 0
    item.error = null
  }

  running.value = true
  error.value = null
  runStartBytes.value = uploadedBytes.value
  runStartedAt.value = Date.now()
  nowTick.value = Date.now()
  startTicker()
  const completedIds: string[] = []
  let next = 0

  const worker = async (): Promise<void> => {
    while (next < queue.length) {
      const item = queue[next++]
      if (item.status !== 'queued') continue
      const ok = await processItem(item)
      if (ok && item.assetId) completedIds.push(item.assetId)
    }
  }

  try {
    const workers = Math.min(UPLOAD_CONCURRENCY, queue.length)
    await Promise.all(Array.from({ length: workers }, worker))
    if (completedIds.length) {
      notifyLibraryChanged()
      toast.add({
        severity: 'success',
        summary: 'Upload complete',
        detail:
          completedIds.length === 1
            ? '1 file uploaded'
            : `${completedIds.length} files uploaded`,
        life: 3500,
      })
    }
  } catch (e) {
    const message = toMessage(e)
    error.value = message
    toast.add({
      severity: 'error',
      summary: 'Upload finishing failed',
      detail: message,
      life: 6000,
    })
  } finally {
    running.value = false
    runStartedAt.value = 0
    stopTicker()
  }

  if (upload.items.some((item) => item.status === 'queued')) {
    void runQueue()
  }
}

function requestClose(): void {
  if (running.value) return
  if (!unuploadedCount.value) {
    upload.visible = false
    return
  }
  confirm.require({
    group: 'upload-discard',
    header: 'Discard staged files?',
    message:
      unuploadedCount.value === 1
        ? '1 file has not been uploaded and will be lost. Uploaded assets stay in the library.'
        : `${unuploadedCount.value} files have not been uploaded and will be lost. Uploaded assets stay in the library.`,
    acceptLabel: 'Discard',
    rejectLabel: 'Keep editing',
    acceptProps: { severity: 'danger' },
    rejectProps: { text: true },
    accept: () => {
      upload.visible = false
    },
  })
}

function onPrimary(): void {
  if (running.value) return
  if (!upload.items.length) {
    openPicker()
    return
  }
  if (pendingCount.value) {
    void runQueue()
    return
  }
  requestClose()
}
</script>

<template>
  <Dialog
    v-model:visible="dialogVisible"
    modal
    :show-header="false"
    :style="{ width: 'min(90rem, 97vw)' }"
    :content-style="{ height: 'min(54rem, 90vh)', padding: '0' }"
    :closable="false"
    :close-on-escape="!running"
    :dismissable-mask="!running"
  >
    <div class="flex h-full min-h-0">
      <section class="relative flex min-w-0 flex-1 flex-col overflow-hidden bg-slate-50/40 dark:bg-slate-950/20">
        <div class="min-h-0 flex-1 overflow-y-auto p-4">
          <div
            v-if="!upload.items.length"
            class="flex h-full min-h-64 items-center justify-center"
          >
            <button
              type="button"
              class="flex w-full max-w-xl flex-col items-center justify-center rounded-xl border-2 border-dashed border-slate-300 px-8 py-14 text-center transition hover:border-brand-400 hover:bg-brand-50/50 dark:border-slate-700 dark:hover:bg-brand-950/20"
              @click="openPicker"
            >
              <CloudUpload :size="32" class="text-slate-400" />
              <span
                class="mt-3 text-base font-medium text-brand-700 dark:text-brand-400"
              >
                Choose files
              </span>
              <span class="mt-1 text-sm text-slate-400">
                or drop them here
              </span>
              <span v-if="categoryLimits" class="mt-3 text-xs text-slate-400">
                Size limits vary by file type in this library.
              </span>
            </button>
          </div>

          <div
            v-else
            class="grid gap-3"
            :class="
              fillGrid
                ? 'h-full'
                : 'grid-cols-[repeat(auto-fill,minmax(10rem,1fr))]'
            "
            :style="gridStyle"
          >
            <article
              v-for="item in sortedItems"
              :key="item.id"
              class="group relative rounded-lg border bg-white dark:bg-slate-900"
              :class="[itemBorder(item), fillGrid ? 'h-full' : '']"
            >
              <div
                class="relative overflow-hidden rounded-lg bg-slate-100 dark:bg-slate-800"
                :class="fillGrid ? 'h-full' : 'aspect-square'"
              >
                <img
                  v-if="item.previewUrl"
                  :src="item.previewUrl"
                  :alt="item.name"
                  class="h-full w-full object-cover"
                />
                <FileTypePreview
                  v-else
                  :mime-type="item.file.type"
                  :filename="item.name"
                />

                <div class="absolute top-1.5 right-1.5 flex gap-1">
                  <button
                    v-if="item.status === 'uploading'"
                    class="inline-flex h-7 w-7 items-center justify-center rounded-md border border-white/20 bg-black/60 text-white shadow-sm backdrop-blur transition hover:bg-red-600"
                    title="Cancel upload"
                    aria-label="Cancel upload"
                    @click="cancelItem(item)"
                  >
                    <Times :size="13" />
                  </button>
                  <button
                    v-else-if="
                      item.retryable &&
                      (item.status === 'error' || item.status === 'canceled')
                    "
                    class="inline-flex h-7 w-7 items-center justify-center rounded-md border border-white/20 bg-black/60 text-white shadow-sm backdrop-blur transition hover:bg-indigo-500 hover:text-white dark:hover:bg-orange-500 dark:hover:text-orange-950"
                    title="Retry"
                    aria-label="Retry upload"
                    @click="retryItem(item)"
                  >
                    <Refresh :size="13" />
                  </button>
                  <button
                    v-if="
                      item.status === 'queued' ||
                      item.status === 'error' ||
                      item.status === 'canceled' ||
                      (item.status === 'done' && canDeleteAssets)
                    "
                    class="inline-flex h-7 w-7 items-center justify-center rounded-md border border-white/20 bg-black/60 text-white shadow-sm backdrop-blur transition hover:bg-red-600 disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="running"
                    :title="
                      item.status === 'done'
                        ? 'Revert uploaded asset'
                        : 'Remove from queue'
                    "
                    :aria-label="
                      item.status === 'done'
                        ? 'Revert uploaded asset'
                        : 'Remove from queue'
                    "
                    @click="requestRemoveItem(item, $event)"
                  >
                    <Undo v-if="item.status === 'done'" :size="13" />
                    <Trash v-else :size="13" />
                  </button>
                </div>

                <span
                  v-if="item.status === 'done'"
                  class="absolute top-1.5 left-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-green-500 text-white shadow"
                >
                  <Check :size="12" />
                </span>
                <span
                  v-else-if="item.status === 'error'"
                  class="absolute top-1.5 left-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-white shadow"
                >
                  <ExclamationCircle :size="12" />
                </span>

                <div
                  v-if="item.status === 'done'"
                  class="pointer-events-none absolute inset-0 ring-2 ring-inset ring-green-500"
                />
                <div
                  v-else-if="item.status === 'error'"
                  class="pointer-events-none absolute inset-0 ring-2 ring-inset ring-red-500"
                />

                <div
                  class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/40 to-transparent p-2 text-white"
                >
                  <input
                    v-if="editingId === item.id"
                    :ref="bindRenameInput"
                    v-model="editingName"
                    type="text"
                    class="w-full rounded border border-white/40 bg-black/50 px-1.5 py-0.5 text-xs text-white outline-none"
                    @click.stop
                    @dblclick.stop
                    @keydown.enter.prevent="commitRename(item)"
                    @keydown.esc.prevent="cancelRename"
                    @blur="commitRename(item)"
                  />
                  <p
                    v-else
                    class="truncate text-xs font-medium"
                    :title="item.name"
                    @dblclick.stop="startRename(item)"
                  >
                    {{ item.name }}
                  </p>
                  <p
                    v-if="item.status === 'error'"
                    class="mt-1 line-clamp-2 rounded bg-red-600/95 px-1.5 py-1 text-[10px] font-medium leading-snug text-white"
                    :title="item.error ?? 'Upload failed'"
                  >
                    {{ item.error ?? 'Upload failed' }}
                  </p>
                  <div
                    v-else
                    class="mt-0.5 flex items-center justify-between gap-2 text-[10px] text-white/75"
                  >
                    <span>{{ formatSize(item.file.size) }}</span>
                    <span>{{ extensionOf(item.name) }}</span>
                  </div>
                </div>

                <div
                  v-if="item.status === 'uploading'"
                  class="absolute inset-x-0 bottom-0 h-1 bg-black/25"
                >
                  <div
                    class="h-full bg-brand-400 transition-all"
                    :style="{ width: `${Math.round(item.progress * 100)}%` }"
                  />
                </div>
                <div
                  v-else-if="item.status === 'processing'"
                  class="absolute inset-x-0 bottom-0 h-1 overflow-hidden bg-black/25"
                >
                  <div class="h-full w-1/3 animate-pulse bg-brand-400" />
                </div>
              </div>
            </article>
          </div>

        </div>

      </section>

      <aside
        class="flex w-80 shrink-0 flex-col gap-4 overflow-y-auto border-l border-slate-200 p-4 dark:border-slate-800"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold">Upload assets</h2>
            <p class="text-xs text-slate-400">
              Stage, review, then upload.
            </p>
          </div>
          <button
            class="inline-flex shrink-0 text-slate-400 hover:text-slate-700 disabled:opacity-40 dark:hover:text-slate-200"
            title="Close"
            :disabled="running"
            @click="requestClose"
          >
            <Times :size="16" />
          </button>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <div class="rounded-lg border border-slate-200 p-2.5 dark:border-slate-800">
            <p class="text-[10px] font-medium tracking-wide text-slate-400 uppercase">
              Files
            </p>
            <p class="mt-0.5 text-lg font-semibold">{{ upload.items.length }}</p>
          </div>
          <div class="rounded-lg border border-slate-200 p-2.5 dark:border-slate-800">
            <p class="text-[10px] font-medium tracking-wide text-slate-400 uppercase">
              Total size
            </p>
            <p class="mt-0.5 text-sm font-semibold">{{ formatSize(totalBytes) }}</p>
          </div>
        </div>

        <InputGroup class="w-full">
          <Select
            v-model="sortKey"
            :options="SORT_OPTIONS"
            option-label="label"
            option-value="value"
            class="min-w-0 flex-1"
          />
          <InputGroupAddon class="!p-0">
            <div class="flex h-full items-center">
              <button
                type="button"
                class="inline-flex h-full w-10 items-center justify-center transition"
                :class="
                  sortDirection === 'asc'
                    ? 'bg-indigo-600 text-white dark:bg-orange-500 dark:text-orange-950'
                    : 'text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800'
                "
                title="Sort ascending"
                aria-label="Sort ascending"
                @click="sortDirection = 'asc'"
              >
                <SortAmountUp :size="15" />
              </button>
              <button
                type="button"
                class="inline-flex h-full w-10 items-center justify-center transition"
                :class="
                  sortDirection === 'desc'
                    ? 'bg-indigo-600 text-white dark:bg-orange-500 dark:text-orange-950'
                    : 'text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800'
                "
                title="Sort descending"
                aria-label="Sort descending"
                @click="sortDirection = 'desc'"
              >
                <SortAmountDown :size="15" />
              </button>
            </div>
          </InputGroupAddon>
        </InputGroup>

        <div class="flex flex-col items-center gap-3 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
          <div
            class="relative h-32 w-32 rounded-full"
            :style="{ background: stateRing }"
          >
            <div
              class="absolute inset-[14px] flex flex-col items-center justify-center rounded-full bg-white dark:bg-slate-900"
            >
              <span class="text-2xl font-semibold">{{ displayProgress }}%</span>
              <span class="text-[10px] font-medium tracking-wide text-slate-400 uppercase">
                uploaded
              </span>
            </div>
          </div>
          <div class="grid w-full grid-cols-2 gap-x-4 gap-y-1 text-xs">
            <div
              v-for="state in stateLegend"
              :key="state.key"
              class="flex items-center justify-between gap-2"
            >
              <span class="flex items-center gap-1.5 text-slate-500 dark:text-slate-400">
                <span
                  class="h-2 w-2 rounded-full"
                  :style="{ backgroundColor: state.color }"
                />
                {{ state.label }}
              </span>
              <span class="font-medium">{{ state.count }}</span>
            </div>
          </div>
          <p v-if="running" class="text-xs text-slate-400">
            {{ formatSize(uploadSpeed) }}/s
            <template v-if="etaSeconds">
              · {{ formatDuration(etaSeconds) }} left
            </template>
          </p>
        </div>

        <Message v-if="error" severity="error">{{ error }}</Message>

        <div class="mt-auto space-y-2 border-t border-slate-200 pt-4 dark:border-slate-800">
          <Button
            v-if="upload.items.length"
            label="Select files"
            severity="secondary"
            outlined
            class="w-full"
            :disabled="running"
            @click="openPicker"
          >
            <template #icon><Plus :size="16" /></template>
          </Button>
          <Button
            v-if="stagedCount"
            label="Clear staged"
            severity="secondary"
            outlined
            class="w-full"
            :disabled="running"
            @click="clearStaged"
          >
            <template #icon><Trash :size="16" /></template>
          </Button>
          <Button
            v-if="doneCount && canDeleteAssets"
            label="Revert uploaded"
            severity="danger"
            outlined
            class="w-full"
            :disabled="running"
            @click="requestRevertUploaded($event)"
          >
            <template #icon><Undo :size="16" /></template>
          </Button>
          <div class="flex items-center gap-2 pt-1">
            <Button
              v-if="showCancel"
              :label="running ? 'Cancel all' : 'Cancel'"
              text
              class="min-w-0 flex-1 whitespace-nowrap"
              @click="running ? cancelAll() : requestClose()"
            />
            <Button
              :label="primaryLabel"
              class="min-w-0 flex-1 whitespace-nowrap"
              :loading="running"
              :disabled="running"
              @click="onPrimary"
            >
              <template #icon>
                <Plus v-if="!upload.items.length" :size="16" />
                <Refresh v-else-if="retryableFailedCount" :size="16" />
                <Check v-else-if="allSucceeded" :size="16" />
                <Upload v-else :size="16" />
              </template>
            </Button>
          </div>
        </div>

        <input
          ref="fileInput"
          type="file"
          multiple
          :accept="ACCEPT_STRING"
          :disabled="running"
          class="hidden"
          @change="onFiles"
        />
      </aside>
    </div>
  </Dialog>

  <Teleport to="body">
    <div
      v-if="dragActive"
      class="pointer-events-none fixed inset-0 z-[9999] flex items-center justify-center bg-brand-600/10 backdrop-blur-sm"
    >
      <div
        class="rounded-xl border-2 border-dashed border-brand-400 bg-white px-12 py-10 text-center shadow-2xl dark:bg-slate-900"
      >
        <CloudUpload :size="40" class="mx-auto text-brand-500" />
        <p class="mt-3 text-lg font-semibold text-brand-700 dark:text-brand-300">
          Drop to add files
        </p>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          They will be added to this upload.
        </p>
      </div>
    </div>
  </Teleport>
</template>
