<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import * as api from '../api/client'
import { editorDirty } from '../lib/editorState'
import { currentLibrary } from '../lib/library'
import { isCsvMime, isFontMime, isTextMime } from '../lib/mimeTypes'
import type { Asset } from '../types'
import AudioPlayer from './AudioPlayer.vue'
import PdfViewer from './PdfViewer.vue'
import VideoPlayer from './VideoPlayer.vue'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ close: []; saved: [asset: Asset] }>()

const confirm = useConfirm()
const toast = useToast()
const imageHost = ref<HTMLElement | null>(null)
const saving = ref(false)
const watermarkPickerVisible = ref(false)
const watermarkAssets = ref<Asset[]>([])
const watermarkLoading = ref(false)
let loadWatermarkImage:
  | ((url: string, revokeObjectUrl?: boolean) => void)
  | null = null

// Text / CSV editor state.
const textContent = ref('')
const originalText = ref('')
const textLoading = ref(false)
// Font specimen state.
const fontFamily = ref('')
let fontFace: FontFace | null = null

type Mode = 'image' | 'pdf' | 'video' | 'audio' | 'font' | 'text' | 'csv' | 'other'

const mode = computed<Mode>(() => {
  const mime = props.asset.mime_type
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/pdf') return 'pdf'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  if (isFontMime(mime)) return 'font'
  if (isCsvMime(mime)) return 'csv'
  if (isTextMime(mime)) return 'text'
  return 'other'
})

const csvRows = computed<string[][]>(() =>
  textContent.value ? parseCsv(textContent.value) : [],
)

function parseCsv(text: string): string[][] {
  const rows: string[][] = []
  let row: string[] = []
  let field = ''
  let inQuotes = false
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i]
    if (inQuotes) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"'
          i += 1
        } else {
          inQuotes = false
        }
      } else {
        field += ch
      }
    } else if (ch === '"') {
      inQuotes = true
    } else if (ch === ',') {
      row.push(field)
      field = ''
    } else if (ch === '\n') {
      row.push(field)
      rows.push(row)
      row = []
      field = ''
    } else if (ch !== '\r') {
      field += ch
    }
  }
  if (field !== '' || row.length) {
    row.push(field)
    rows.push(row)
  }
  return rows
}

interface FilerobotInstance {
  render: (config?: Record<string, unknown>) => void
  terminate: () => void
  getCurrentImgData: (
    info: { name?: string; extension?: string; quality?: number },
    pixelRatio?: boolean | number,
    keepLoadingSpinnerShown?: boolean,
  ) => {
    imageData?: { imageBase64?: string; mimeType?: string; fullName?: string }
    hideLoadingSpinner?: () => void
  }
}

let imageEditor: FilerobotInstance | null = null
let disposed = false

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// The editor returns the saved image as a data URL; turn it back into a Blob
// for the multipart upload.
function base64ToBlob(base64?: string, mime = 'image/png'): Blob {
  const data = base64?.split(',')[1] ?? ''
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
  return new Blob([bytes], { type: mime })
}

const SAVED_IMAGE_TYPES = ['jpg', 'jpeg', 'png', 'webp'] as const
type SavedImageType = (typeof SAVED_IMAGE_TYPES)[number]

function savedImageType(mime: string): SavedImageType {
  const sub = mime.split('/')[1] as SavedImageType
  return SAVED_IMAGE_TYPES.includes(sub) ? sub : 'png'
}

function fileBase(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(0, dot) : name
}

// The content endpoint is cacheable, so bust the cache each time the editor is
// (re)opened to always load the current file.
function contentUrl(): string {
  return `${api.assetContentUrl(props.asset.id)}?t=${Date.now()}`
}

// The library's default watermark asset, shown in the watermark gallery so it
// can be reused with one click.
function watermarkGallery(): { url: string; previewUrl: string }[] {
  const id = currentLibrary()?.watermark_asset_id
  if (!id) return []
  return [{ url: api.assetContentUrl(id), previewUrl: api.assetPreviewUrl(id) }]
}

// Opens a picker of the library's image assets for the watermark tool, so
// watermarks are reused from the DAM rather than uploaded from disk.
async function openWatermarkPicker(
  loadAndSet: (url: string, revokeObjectUrl?: boolean) => void,
): Promise<void> {
  loadWatermarkImage = loadAndSet
  watermarkPickerVisible.value = true
  watermarkLoading.value = true
  try {
    const all = await api.listAllAssets({ types: ['image'] })
    watermarkAssets.value = all.filter((a) => a.mime_type.startsWith('image/'))
  } catch {
    watermarkAssets.value = []
  } finally {
    watermarkLoading.value = false
  }
}

function chooseWatermark(asset: Asset): void {
  loadWatermarkImage?.(api.assetContentUrl(asset.id))
  watermarkPickerVisible.value = false
}

// The library's image assets, offered as the annotate "add image" gallery so
// images are reused from the DAM rather than uploaded from disk.
async function libraryImageGallery(): Promise<
  { originalUrl: string; previewUrl: string }[]
> {
  try {
    const all = await api.listAllAssets({ types: ['image'] })
    return all
      .filter(
        (a) => a.mime_type.startsWith('image/') && a.id !== props.asset.id,
      )
      .map((a) => ({
        originalUrl: api.assetContentUrl(a.id),
        previewUrl: api.assetPreviewUrl(a.id, a.updated_at),
      }))
  } catch {
    return []
  }
}

async function mountImage(): Promise<void> {
  if (!imageHost.value) return
  const { default: FilerobotImageEditor } = await import('filerobot-image-editor')
  if (disposed || !imageHost.value) return
  const imageGallery = await libraryImageGallery()
  if (disposed || !imageHost.value) return
  const Ctor = FilerobotImageEditor as unknown as new (
    element: HTMLElement,
    config: Record<string, unknown>,
  ) => FilerobotInstance
  const editor = new Ctor(imageHost.value, {
    source: contentUrl(),
    defaultSavedImageType: savedImageType(props.asset.mime_type),
    // Open on Filters (sorted first) with its own tool selected, so the crop
    // tool does not mount and apply an implicit crop on load. Filerobot's
    // defaultToolId defaults to 'Crop' regardless of the tab, so it must be set
    // explicitly; the crop stays neutral until the user opens the crop tool.
    defaultTabId: 'Filters',
    defaultToolId: 'Filters',
    tabsIds: [
      'Filters',
      'Finetune',
      'Adjust',
      'Watermark',
      'Annotate',
      'Resize',
    ],
    Crop: {
      presetsItems: [
        { titleKey: 'classicTv', descriptionKey: '4:3', ratio: 4 / 3 },
        { titleKey: 'cinemascope', descriptionKey: '21:9', ratio: 21 / 9 },
      ],
    },
    translations: {
      classicTv: 'Classic TV',
      cinemascope: 'Cinemascope',
    },
    // Watermarks come from the DAM, never the local file system: show the
    // library's default watermark asset in the gallery, and route the "upload"
    // button to a picker of the library's image assets.
    Watermark: {
      gallery: watermarkGallery(),
      onUploadWatermarkImgClick: (
        loadAndSet: (url: string, revokeObjectUrl?: boolean) => void,
      ) => {
        void openWatermarkPicker(loadAndSet)
      },
    },
    // The annotate "add image" tool only offers upload or a gallery; disable the
    // upload and provide the library's images as the gallery.
    Image: {
      disableUpload: true,
      gallery: imageGallery,
    },
    // Use the bundled English strings instead of fetching them from the remote
    // translation service (which is a large external request on load).
    useBackendTranslations: false,
    language: 'en',
    // We drive save/close from our own floating controls: hide the built-in
    // save button (and its options modal) and track unsaved changes ourselves.
    removeSaveButton: true,
    avoidChangesNotSavedAlertOnLeave: true,
    onModify: (state: { haveNotSavedChanges?: boolean }) => {
      editorDirty.value = Boolean(state?.haveNotSavedChanges)
    },
  })
  // No onClose config => Filerobot's own close button (and its discard dialog)
  // are not rendered.
  editor.render()
  imageEditor = editor
}

async function loadText(): Promise<void> {
  textLoading.value = true
  try {
    const response = await fetch(api.assetContentUrl(props.asset.id), {
      credentials: 'same-origin',
    })
    if (!response.ok) throw new Error(`Failed to load (${response.status})`)
    textContent.value = await response.text()
    originalText.value = textContent.value
    editorDirty.value = false
  } catch (e) {
    textContent.value = ''
    originalText.value = ''
    toast.add({
      severity: 'error',
      summary: 'Could not load file',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    textLoading.value = false
  }
}

async function loadFont(): Promise<void> {
  try {
    const family = `godam-font-${props.asset.id}`
    const face = new FontFace(family, `url(${api.assetContentUrl(props.asset.id)})`)
    await face.load()
    document.fonts.add(face)
    fontFace = face
    fontFamily.value = family
  } catch {
    fontFamily.value = ''
  }
}

watch(textContent, () => {
  if (mode.value === 'text') {
    editorDirty.value = textContent.value !== originalText.value
  }
})

async function performImageSave(): Promise<void> {
  const editor = imageEditor
  if (!editor || saving.value) return
  saving.value = true
  let hideSpinner: (() => void) | undefined
  try {
    const result = editor.getCurrentImgData(
      {
        name: fileBase(props.asset.filename),
        extension: savedImageType(props.asset.mime_type),
        quality: 0.92,
      },
      true,
      true,
    )
    hideSpinner = result.hideLoadingSpinner
    const imageData = result.imageData
    if (!imageData?.imageBase64) throw new Error('Could not read the edited image')
    const blob = base64ToBlob(imageData.imageBase64, imageData.mimeType ?? 'image/png')
    const updated = await api.replaceAssetContent(props.asset.id, blob, imageData.fullName)
    editorDirty.value = false
    emit('saved', updated)
    toast.add({ severity: 'success', summary: 'Image saved', life: 2500 })
    emit('close')
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Save failed',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    hideSpinner?.()
    saving.value = false
  }
}

async function performTextSave(): Promise<void> {
  if (saving.value) return
  saving.value = true
  try {
    const blob = new Blob([textContent.value], { type: props.asset.mime_type })
    const updated = await api.replaceAssetContent(
      props.asset.id,
      blob,
      props.asset.filename,
    )
    originalText.value = textContent.value
    editorDirty.value = false
    emit('saved', updated)
    toast.add({ severity: 'success', summary: 'File saved', life: 2500 })
    emit('close')
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Save failed',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    saving.value = false
  }
}

// Called by the floating control's Save button: a simple confirm, then save.
function save(event?: MouseEvent): void {
  const target = mode.value
  if (saving.value) return
  if (target === 'image' && !imageEditor) return
  if (target !== 'image' && target !== 'text') return
  confirm.require({
    group: 'editor',
    target: event?.currentTarget as HTMLElement,
    header: 'Save changes',
    message: 'Save changes? This replaces the stored file.',
    acceptLabel: 'Save',
    rejectLabel: 'Cancel',
    rejectProps: { text: true },
    accept: () => {
      if (target === 'image') void performImageSave()
      else void performTextSave()
    },
  })
}

defineExpose({ save })

function dispose(): void {
  disposed = true
  imageEditor?.terminate()
  imageEditor = null
  if (fontFace) {
    document.fonts.delete(fontFace)
    fontFace = null
  }
  fontFamily.value = ''
  editorDirty.value = false
}

function init(): void {
  disposed = false
  editorDirty.value = false
  if (mode.value === 'image') void mountImage()
  else if (mode.value === 'text' || mode.value === 'csv') void loadText()
  else if (mode.value === 'font') void loadFont()
}

onMounted(init)
watch(
  () => props.asset.id,
  () => {
    dispose()
    init()
  },
)
onBeforeUnmount(dispose)
</script>

<template>
  <div class="min-h-0 flex-1 overflow-hidden">
    <div v-if="mode === 'image'" ref="imageHost" class="h-full w-full" />
    <PdfViewer v-else-if="mode === 'pdf'" :key="asset.id" :asset="asset" />

    <div
      v-else-if="mode === 'font'"
      class="h-full w-full overflow-y-auto bg-slate-100 dark:bg-slate-800 p-6"
    >
      <div class="mx-auto max-w-3xl rounded-lg bg-white dark:bg-slate-900 p-6 shadow-sm">
        <p
          class="mb-5 text-5xl leading-tight break-words text-slate-800 dark:text-slate-100"
          :style="{ fontFamily: fontFamily || 'inherit' }"
        >
          The quick brown fox jumps over the lazy dog
        </p>
        <p
          class="mb-3 text-3xl break-words text-slate-700 dark:text-slate-200"
          :style="{ fontFamily: fontFamily || 'inherit' }"
        >
          ABCDEFGHIJKLMNOPQRSTUVWXYZ
        </p>
        <p
          class="mb-3 text-3xl break-words text-slate-700 dark:text-slate-200"
          :style="{ fontFamily: fontFamily || 'inherit' }"
        >
          abcdefghijklmnopqrstuvwxyz
        </p>
        <p
          class="text-lg text-slate-500 dark:text-slate-400"
          :style="{ fontFamily: fontFamily || 'inherit' }"
        >
          Aa Bb Cc — 0123456789 !@#$%^&*()
        </p>
        <p v-if="!fontFamily" class="mt-4 text-sm text-slate-400">
          This font could not be loaded for preview.
        </p>
      </div>
      <div class="mx-auto mt-4 max-w-3xl">
        <a
          :href="api.assetContentUrl(asset.id)"
          target="_blank"
          rel="noreferrer"
          class="text-sm text-violet-600 dark:text-violet-400 hover:underline"
        >
          Download font
        </a>
      </div>
    </div>

    <AudioPlayer v-else-if="mode === 'audio'" :asset="asset" />

    <VideoPlayer
      v-else-if="mode === 'video'"
      :key="asset.id"
      :asset="asset"
    />

    <div v-else-if="mode === 'text'" class="h-full w-full bg-slate-100 dark:bg-slate-800 p-4">
      <p v-if="textLoading" class="text-sm text-slate-400">Loading…</p>
      <textarea
        v-else
        v-model="textContent"
        spellcheck="false"
        class="h-full w-full resize-none rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-4 font-mono text-sm leading-relaxed text-slate-800 dark:text-slate-100 outline-none focus:border-violet-400"
      />
    </div>

    <div
      v-else-if="mode === 'csv'"
      class="h-full w-full overflow-auto bg-slate-100 dark:bg-slate-800 p-4"
    >
      <p v-if="textLoading" class="text-sm text-slate-400">Loading…</p>
      <table v-else class="min-w-full border-collapse bg-white dark:bg-slate-900 text-sm">
        <tbody>
          <tr
            v-for="(row, rowIndex) in csvRows"
            :key="rowIndex"
            :class="rowIndex === 0 ? 'font-medium text-slate-800 dark:text-slate-100' : 'text-slate-600 dark:text-slate-300'"
          >
            <td
              v-for="(cell, cellIndex) in row"
              :key="cellIndex"
              class="border border-slate-200 dark:border-slate-800 px-2 py-1 whitespace-nowrap"
            >
              {{ cell }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div
      v-else
      class="flex h-full flex-col items-center justify-center gap-3 text-sm text-slate-400"
    >
      <p>No inline editor for this file type.</p>
      <a
        :href="api.assetContentUrl(asset.id)"
        target="_blank"
        rel="noreferrer"
        class="text-violet-600 dark:text-violet-400 hover:underline"
      >
        Open original
      </a>
    </div>

    <Dialog
      v-model:visible="watermarkPickerVisible"
      header="Choose watermark"
      modal
      :draggable="false"
      :style="{ width: '42rem' }"
    >
      <div
        v-if="watermarkAssets.length"
        class="grid max-h-96 grid-cols-3 gap-3 overflow-y-auto sm:grid-cols-4"
      >
        <button
          v-for="image in watermarkAssets"
          :key="image.id"
          class="overflow-hidden rounded border border-slate-200 dark:border-slate-800 text-left hover:border-violet-400"
          @click="chooseWatermark(image)"
        >
          <img
            :src="api.assetPreviewUrl(image.id, image.updated_at)"
            :alt="image.title || image.filename"
            class="aspect-square w-full bg-slate-100 dark:bg-slate-800 object-cover"
          />
          <span class="block truncate px-1.5 py-1 text-xs text-slate-500 dark:text-slate-400">
            {{ image.title || image.filename }}
          </span>
        </button>
      </div>
      <p v-else-if="watermarkLoading" class="text-sm text-slate-400">
        Loading…
      </p>
      <p v-else class="text-sm text-slate-400">
        No image assets in this library yet.
      </p>
    </Dialog>
  </div>
</template>
