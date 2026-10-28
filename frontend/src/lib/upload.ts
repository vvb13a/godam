import { reactive } from 'vue'
import * as api from '../api/client'
import { formatSize } from './format'
import { library } from './library'
import type { Asset, UploadLimits, UploadSetting } from '../types'

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
})

// openUpload stages the given files (if any) and opens the dialog.
export function openUpload(files: File[] = []): void {
  for (const file of files) {
    upload.items.push(createUploadItem(file))
  }
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

  const limit = uploadLimitFor(file.type)
  if (limit > 0 && file.size > limit) {
    item.status = 'error'
    item.error = `Exceeds the ${formatSize(limit)} limit for this file type.`
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

// uploadLimitFor returns the byte limit for a file's category. Zero means the
// category has no size limit.
export function uploadLimitFor(mime: string): number {
  const clean = mime.split(';')[0].trim().toLowerCase()
  let key: keyof UploadLimits | null = null
  if (clean.startsWith('image/')) key = 'image_bytes'
  else if (clean.startsWith('audio/')) key = 'audio_bytes'
  else if (clean.startsWith('video/')) key = 'video_bytes'
  else if (clean === 'application/pdf') key = 'pdf_bytes'
  return key ? (library.upload_limits?.[key] ?? 0) : 0
}

const LIMIT_KEYS: (keyof UploadLimits)[] = [
  'image_bytes',
  'audio_bytes',
  'video_bytes',
  'pdf_bytes',
]

function formatPatterns(format?: string): string[] {
  switch ((format ?? '*').toLowerCase()) {
    case 'image':
      return ['image/*']
    case 'audio':
      return ['audio/*']
    case 'video':
      return ['video/*']
    case 'pdf':
      return ['application/pdf']
    default:
      return ['*/*']
  }
}

function formatLimitKeys(format?: string): (keyof UploadLimits)[] {
  switch ((format ?? '*').toLowerCase()) {
    case 'image':
      return ['image_bytes']
    case 'audio':
      return ['audio_bytes']
    case 'video':
      return ['video_bytes']
    case 'pdf':
      return ['pdf_bytes']
    default:
      return LIMIT_KEYS
  }
}

function mimePatternsIntersect(a: string, b: string): boolean {
  a = a.trim().toLowerCase()
  b = b.trim().toLowerCase()
  if (!a || !b) return false
  if (a === '*' || a === '*/*' || b === '*' || b === '*/*') return true
  if (a === b) return true
  if (a.endsWith('/*')) return b.startsWith(a.slice(0, -1))
  if (b.endsWith('/*')) return a.startsWith(b.slice(0, -1))
  return false
}

function lowerLimit(base: number, token: number): number {
  if (token <= 0) return base
  if (base <= 0 || token < base) return token
  return base
}

// applyUploadSettings narrows the library's upload UI to the token's
// upload_settings. It mirrors the server: settings can only make uploads
// stricter, never broader.
export function applyUploadSettings(settings?: UploadSetting[]): void {
  if (!settings?.length) return

  const tokenPatterns = new Set<string>()
  const tokenLimits: UploadLimits = {}
  for (const setting of settings) {
    const patterns = setting.mime_types?.length
      ? setting.mime_types
      : formatPatterns(setting.format)
    for (const pattern of patterns) {
      if (pattern.trim()) tokenPatterns.add(pattern.trim().toLowerCase())
    }
    if ((setting.max_bytes ?? 0) > 0) {
      for (const key of formatLimitKeys(setting.format)) {
        tokenLimits[key] = lowerLimit(
          tokenLimits[key] ?? 0,
          setting.max_bytes ?? 0,
        )
      }
    }
  }

  if (!tokenPatterns.has('*/*') && !tokenPatterns.has('*')) {
    const base = library.allowed_mime_types ?? []
    library.allowed_mime_types = base.length
      ? base.filter((pattern) =>
          [...tokenPatterns].some((token) => mimePatternsIntersect(pattern, token)),
        )
      : [...tokenPatterns]
  }

  const limits: UploadLimits = { ...(library.upload_limits ?? {}) }
  for (const key of LIMIT_KEYS) {
    const token = tokenLimits[key] ?? 0
    if (token > 0) limits[key] = lowerLimit(limits[key] ?? 0, token)
  }
  library.upload_limits = limits
}

export function hasCategoryUploadLimits(): boolean {
  const limits = library.upload_limits
  if (!limits) return false
  return (
    (limits.image_bytes ?? 0) > 0 ||
    (limits.audio_bytes ?? 0) > 0 ||
    (limits.video_bytes ?? 0) > 0 ||
    (limits.pdf_bytes ?? 0) > 0
  )
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
