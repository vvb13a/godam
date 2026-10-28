<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import * as api from '../api/client'
import { editorDirty } from '../lib/editorState'
import {
  darkImageEditorTheme,
  lightImageEditorTheme,
} from '../lib/imageEditorTheme'
import { currentLibrary, library } from '../lib/library'
import { theme as appTheme } from '../lib/theme'
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

type Mode = 'image' | 'pdf' | 'video' | 'audio' | 'other'

const mode = computed<Mode>(() => {
  const mime = props.asset.mime_type
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/pdf') return 'pdf'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'
  return 'other'
})

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
  return api.cacheBust(api.assetContentUrl(props.asset.id))
}

// Image assets power both the watermark picker and Filerobot's image gallery.
// Cache the full list until the library version changes so opening or stepping
// through image editors does not refetch the whole library each time.
let imageAssetsCache: { version: number; items: Asset[] } | null = null
let imageAssetsPromise: Promise<Asset[]> | null = null

function loadImageAssets(): Promise<Asset[]> {
  const version = library.version
  if (imageAssetsCache?.version === version) {
    return Promise.resolve(imageAssetsCache.items)
  }
  if (imageAssetsPromise) return imageAssetsPromise
  imageAssetsPromise = api
    .listAllAssets({ types: ['image'] })
    .then((items) => {
      imageAssetsCache = { version, items }
      return items
    })
    .catch(() => [])
    .finally(() => {
      imageAssetsPromise = null
    })
  return imageAssetsPromise
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
    const all = await loadImageAssets()
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
    const all = await loadImageAssets()
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
    theme:
      appTheme.resolved === 'dark'
        ? darkImageEditorTheme
        : lightImageEditorTheme,
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

// Called by the floating control's Save button: a simple confirm, then save.
function save(event?: MouseEvent): void {
  if (saving.value || mode.value !== 'image' || !imageEditor) return
  confirm.require({
    group: 'editor',
    target: event?.currentTarget as HTMLElement,
    header: 'Save changes',
    message: 'Save changes? This replaces the stored file.',
    acceptLabel: 'Save',
    rejectLabel: 'Cancel',
    rejectProps: { text: true },
    accept: () => void performImageSave(),
  })
}

defineExpose({ save })

function dispose(): void {
  disposed = true
  imageEditor?.terminate()
  imageEditor = null
  editorDirty.value = false
}

function init(): void {
  disposed = false
  editorDirty.value = false
  if (mode.value === 'image') void mountImage()
}

onMounted(init)
watch(
  () => props.asset.id,
  () => {
    dispose()
    init()
  },
)
// Re-render with the other palette without remounting, so the current edits
// survive a theme change while the editor is open.
watch(
  () => appTheme.resolved,
  (resolved) => {
    if (mode.value !== 'image' || !imageEditor) return
    imageEditor.render({
      theme: resolved === 'dark' ? darkImageEditorTheme : lightImageEditorTheme,
    })
  },
)
onBeforeUnmount(dispose)
</script>

<template>
  <div class="min-h-0 flex-1 overflow-hidden">
    <div v-if="mode === 'image'" ref="imageHost" class="h-full w-full" />
    <PdfViewer v-else-if="mode === 'pdf'" :key="asset.id" :asset="asset" />

    <AudioPlayer v-else-if="mode === 'audio'" :asset="asset" />

    <VideoPlayer
      v-else-if="mode === 'video'"
      :key="asset.id"
      :asset="asset"
    />

    <div
      v-else
      class="flex h-full flex-col items-center justify-center gap-3 text-sm text-slate-400"
    >
      <p>No inline editor for this file type.</p>
      <a
        :href="api.assetContentUrl(asset.id)"
        target="_blank"
        rel="noreferrer"
        class="text-brand-700 dark:text-brand-400 hover:underline"
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
          class="overflow-hidden rounded border border-slate-200 dark:border-slate-800 text-left hover:border-brand-400"
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
