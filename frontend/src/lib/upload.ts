import { reactive } from 'vue'
import * as api from '../api/client'
import { formatSize } from './format'
import { library } from './library'
import type { Asset } from '../types'

// Files above this size use the chunked transport. The server streams direct
// uploads to disk too, but chunking keeps each request small and gives large
// files predictable memory characteristics.
const CHUNKED_UPLOAD_THRESHOLD = 8 * 1024 * 1024

export type UploadStatus =
  | 'queued'
  | 'uploading'
  | 'processing'
  | 'done'
  | 'error'
  | 'canceled'

export interface UploadItem {
  id: string
  file: File
  name: string
  previewUrl: string | null
  status: UploadStatus
  progress: number
  error: string | null
  assetId: string | null
  retryable: boolean
}

export interface UploadCallbacks {
  onProgress?: (fraction: number) => void
  onProcessing?: () => void
}

// upload is the shared staging area for the UploadDialog. Files dropped on the
// window (or picked in the dialog) accumulate here until the user confirms.
export const upload = reactive({
  visible: false,
  items: [] as UploadItem[],
  destinationCollectionIds: [] as string[],
  destinationTagIds: [] as string[],
})

// openUpload stages the given files (if any) and opens the dialog. The
// collection and tag defaults come from the view the user is currently in, so
// uploads are automatically associated with the active collection/tag filter.
export function openUpload(
  files: File[] = [],
  destination: string | null = null,
  destinationTag: string | null = null,
): void {
  for (const file of files) {
    upload.items.push(createUploadItem(file))
  }
  upload.destinationCollectionIds = destination ? [destination] : []
  upload.destinationTagIds = destinationTag ? [destinationTag] : []
  upload.visible = true
}

export function createUploadItem(file: File): UploadItem {
  const item: UploadItem = {
    id: newItemId(),
    file,
    name: file.name,
    previewUrl: file.type.startsWith('image/')
      ? URL.createObjectURL(file)
      : null,
    status: 'queued',
    progress: 0,
    error: null,
    assetId: null,
    retryable: true,
  }

  const limit = library.max_upload_bytes
  if (limit > 0 && file.size > limit) {
    item.status = 'error'
    item.error = `Exceeds the ${formatSize(limit)} limit for this library.`
    item.retryable = false
    return item
  }
  if (!mimeAllowed(file)) {
    item.status = 'error'
    item.error = `File type “${file.type}” is not allowed in this library.`
    item.retryable = false
  }
  return item
}

export function fileForUpload(item: UploadItem): File {
  if (item.name === item.file.name) return item.file
  return new File([item.file], item.name, {
    type: item.file.type,
    lastModified: item.file.lastModified,
  })
}

export function disposeUploadItem(item: UploadItem): void {
  if (item.previewUrl) {
    URL.revokeObjectURL(item.previewUrl)
    item.previewUrl = null
  }
}

export function clearUploadItems(): void {
  for (const item of upload.items) {
    disposeUploadItem(item)
  }
  upload.items = []
}

function newItemId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

// mimeAllowed mirrors the backend allow-list: exact types plus "type/*"
// wildcards. An empty or unknown browser type is left for the server to sniff.
function mimeAllowed(file: File): boolean {
  const allowed = library.allowed_mime_types
  if (!allowed.length) return true
  const mime = file.type.split(';')[0].trim().toLowerCase()
  if (!mime || mime === 'application/octet-stream') return true
  return allowed.some((entry) => {
    const pattern = entry.trim().toLowerCase()
    if (!pattern) return false
    if (pattern.endsWith('/*')) return mime.startsWith(pattern.slice(0, -1))
    return pattern === mime
  })
}

export function uploadFile(
  file: File,
  callbacks: UploadCallbacks = {},
  signal?: AbortSignal,
): Promise<Asset> {
  if (signal?.aborted) return Promise.reject(abortError())
  if (file.size <= CHUNKED_UPLOAD_THRESHOLD) {
    return api.uploadAsset(file, undefined, callbacks.onProgress, {
      signal,
      onUploadComplete: callbacks.onProcessing,
    })
  }
  return uploadFileInChunks(file, callbacks, signal)
}

async function uploadFileInChunks(
  file: File,
  callbacks: UploadCallbacks,
  signal?: AbortSignal,
): Promise<Asset> {
  const session = await api.startUpload({
    filename: file.name,
    mime_type: file.type || undefined,
    size: file.size,
  })
  const chunkSize = session.chunk_size || CHUNKED_UPLOAD_THRESHOLD

  try {
    let offset = 0
    while (offset < file.size) {
      if (signal?.aborted) throw abortError()
      const end = Math.min(offset + chunkSize, file.size)
      const base = offset
      offset = await api.uploadChunk(
        session.id,
        offset,
        file.slice(base, end),
        (fraction) => {
          callbacks.onProgress?.((base + fraction * (end - base)) / file.size)
        },
        { signal },
      )
    }
    if (signal?.aborted) throw abortError()
    callbacks.onProcessing?.()
    return await api.completeUpload(session.id)
  } catch (e) {
    await api.cancelUpload(session.id).catch(() => undefined)
    throw e
  }
}

function abortError(): DOMException {
  return new DOMException('Upload canceled', 'AbortError')
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}
