import type {
  Asset,
  AttributeFacet,
  AttributeTree,
  Instance,
  Conversion,
  ImgProxyCatalog,
  Library,
  LibraryBackup,
  LibraryBackupStatus,
  Preset,
  PresetParameters,
  Stats,
  UploadLimits,
  User,
} from '../types'

// Embed sessions authenticate with a scoped JWT. The token
// is kept in memory and attached to API requests; URL-based asset references
// append it as a query parameter because <img>/<video> cannot send headers.
let authToken = ''

export function setAuthToken(token: string): void {
  authToken = token.trim()
}

export function getAuthToken(): string {
  return authToken
}

export function authHeaders(): Record<string, string> {
  return authToken ? { Authorization: `Bearer ${authToken}` } : {}
}

export function withAuthToken(url: string): string {
  if (!authToken) return url
  const parsed = new URL(url, window.location.origin)
  parsed.searchParams.set('token', authToken)
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

// cacheBust keeps the existing query (including a JWT) and adds a timestamp.
export function cacheBust(url: string): string {
  const parsed = new URL(url, window.location.origin)
  parsed.searchParams.set('t', String(Date.now()))
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

function withAuth(init?: RequestInit): RequestInit {
  if (!authToken) return init ?? {}
  const headers = new Headers(init?.headers)
  if (!headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${authToken}`)
  }
  return { ...init, headers }
}

async function ensureOk(res: Response): Promise<void> {
  if (res.ok) return
  let message = `Request failed (${res.status})`
  try {
    const body = (await res.json()) as { error?: string }
    if (body.error) {
      message = body.error
    }
  } catch {
    // Non-JSON error body: keep the status message.
  }
  throw new Error(message)
}

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    credentials: 'same-origin',
    ...withAuth(init),
  })
  await ensureOk(res)
  return (await res.json()) as T
}

async function requestVoid(url: string, init?: RequestInit): Promise<void> {
  const res = await fetch(url, {
    credentials: 'same-origin',
    ...withAuth(init),
  })
  await ensureOk(res)
}

function getJSON<T>(url: string): Promise<T> {
  return requestJSON<T>(url)
}

function sendJSON<T>(url: string, method: string, body: unknown): Promise<T> {
  return requestJSON<T>(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

// ---- Auth ----

export function getMe(): Promise<User> {
  return getJSON<User>('/api/auth/me')
}

// ---- Instance ----

export function getInstance(): Promise<Library> {
  return getJSON<Library>('/api/instance')
}

export function updateInstance(name: string, color: string): Promise<Library> {
  return sendJSON<Library>('/api/instance', 'PUT', { name, color })
}

export function setInstanceCover(assetId: string): Promise<Library> {
  return sendJSON<Library>('/api/instance/cover', 'PUT', { asset_id: assetId })
}

export function setInstanceWatermark(assetId: string): Promise<Library> {
  return sendJSON<Library>('/api/instance/watermark', 'PUT', {
    asset_id: assetId,
  })
}

export function setInstanceUploadSettings(
  allowedMimeTypes: string[],
  uploadLimits: UploadLimits,
): Promise<Library> {
  return sendJSON<Library>('/api/instance/upload-settings', 'PUT', {
    allowed_mime_types: allowedMimeTypes,
    upload_limits: uploadLimits,
  })
}

export interface LibraryBackupUpdate {
  enabled: boolean
  driver: string
  host: string
  port: number
  user: string
  password: string
  clear_password: boolean
  key_path: string
  base_path: string
  host_key_fingerprint: string
}

export function getLibraryBackup(): Promise<LibraryBackup> {
  return getJSON<LibraryBackup>('/api/instance/backup')
}

export function setLibraryBackup(
  payload: LibraryBackupUpdate,
): Promise<LibraryBackup> {
  return sendJSON<LibraryBackup>('/api/instance/backup', 'PUT', payload)
}

export function testLibraryBackup(payload: LibraryBackupUpdate): Promise<void> {
  return requestVoid('/api/instance/backup/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export function runLibraryBackup(): Promise<{ queued: number }> {
  return requestJSON<{ queued: number }>('/api/instance/backup/run', {
    method: 'POST',
  })
}

export function getLibraryBackupStatus(): Promise<LibraryBackupStatus> {
  return getJSON<LibraryBackupStatus>('/api/instance/backup/status')
}

// ---- Assets ----

export function getStats(): Promise<Stats> {
  return getJSON<Stats>('/api/stats')
}

export interface AssetListParams {
  search?: string
  types?: string[]
  colors?: string[]
  mimeTypes?: string[]
  attributes?: Record<string, string[]>
  minDuration?: number | null
  maxDuration?: number | null
  minPages?: number | null
  maxPages?: number | null
  minWidth?: number | null
  maxWidth?: number | null
  minHeight?: number | null
  maxHeight?: number | null
  minSize?: number | null
  maxSize?: number | null
  addedAfter?: string
  addedBefore?: string
  modifiedAfter?: string
  modifiedBefore?: string
  sort?: string
  cursor?: string
  limit?: number
}

export interface AssetPage {
  items: Asset[]
  next_cursor?: string
  total: number
  total_bytes: number
  total_capped: boolean
}

export function listAttributeFacets(): Promise<AttributeFacet[]> {
  return requestJSON<{ attributes: AttributeFacet[] }>('/api/attributes').then(
    (response) => response.attributes ?? [],
  )
}

export function listAttributeCatalog(): Promise<AttributeFacet[]> {
  return requestJSON<{ attributes: AttributeFacet[] }>(
    '/api/attributes/catalog',
  ).then((response) => response.attributes ?? [])
}

export function createAttributeValues(
  key: string,
  values: string[],
): Promise<void> {
  return requestVoid('/api/attributes/metadata/values', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, values }),
  })
}

export function createAttribute(
  key: string,
  label = '',
): Promise<void> {
  return requestVoid('/api/attributes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, label }),
  })
}

export function updateAttributeDescription(
  key: string,
  value: string,
  description: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/description', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value, description }),
  })
}

export function updateAttributeColor(
  key: string,
  value: string,
  color: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/color', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value, color }),
  })
}

export function updateAttributeValueOrder(
  key: string,
  values: string[],
): Promise<void> {
  return requestVoid('/api/attributes/metadata/value-order', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, values }),
  })
}

export function updateAttributeKeyOrder(keys: string[]): Promise<void> {
  return requestVoid('/api/attributes/metadata/key-order', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keys }),
  })
}

export function updateAttributeValuesLocked(
  key: string,
  locked: boolean,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/values-lock', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, locked }),
  })
}

export function updateAttributeValueSearch(
  key: string,
  enabled: boolean,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/value-search', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, enabled }),
  })
}

export function updateAttributeStamping(
  keys: boolean,
  values: boolean,
): Promise<Instance> {
  return sendJSON<Instance>('/api/instance/attribute-stamping', 'PUT', {
    keys,
    values,
  })
}

export function updateAttributeLabel(
  key: string,
  value: string,
  label: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/label', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value, label }),
  })
}

export function updateAttributeSort(
  key: string,
  sort: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/sort', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, sort }),
  })
}

export function updateAttributeParent(
  key: string,
  parent: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/parent', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, parent }),
  })
}

export function listAttributeTree(): Promise<AttributeTree[]> {
  return requestJSON<{ trees: AttributeTree[] }>('/api/attributes/tree').then(
    (response) => response.trees ?? [],
  )
}

export function updateAttributeRender(
  key: string,
  render: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/render', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, render }),
  })
}

export function updateAttributeSearchable(
  key: string,
  searchable: boolean,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/search', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, searchable }),
  })
}

export function updateAttributeMulti(
  key: string,
  multi: boolean,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/multi', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, multi }),
  })
}

export function updateAttributeUnassigned(
  key: string,
  includeUnassigned: boolean,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/unassigned', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      key,
      include_unassigned: includeUnassigned,
    }),
  })
}

export function updateAttributeUnassignedLabel(
  key: string,
  label: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/unassigned-label', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, label }),
  })
}

export function deleteAttributeValue(
  key: string,
  value: string,
): Promise<void> {
  return requestVoid('/api/attributes/metadata/value', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value }),
  })
}

export function deleteAttributeKey(key: string): Promise<void> {
  return requestVoid('/api/attributes/metadata/key', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key }),
  })
}

export function listAssets(params: AssetListParams): Promise<AssetPage> {
  const query = new URLSearchParams()
  if (params.search) query.set('search', params.search)
  if (params.types?.length) query.set('types', params.types.join(','))
  if (params.colors?.length) query.set('colors', params.colors.join(','))
  if (params.mimeTypes?.length) query.set('mime_types', params.mimeTypes.join(','))
  if (params.attributes && Object.keys(params.attributes).length) {
    query.set('attributes', JSON.stringify(params.attributes))
  }
  setNumberParam(query, 'min_duration', params.minDuration)
  setNumberParam(query, 'max_duration', params.maxDuration)
  setNumberParam(query, 'min_pages', params.minPages)
  setNumberParam(query, 'max_pages', params.maxPages)
  setNumberParam(query, 'min_width', params.minWidth)
  setNumberParam(query, 'max_width', params.maxWidth)
  setNumberParam(query, 'min_height', params.minHeight)
  setNumberParam(query, 'max_height', params.maxHeight)
  setNumberParam(query, 'min_size', params.minSize)
  setNumberParam(query, 'max_size', params.maxSize)
  if (params.addedAfter) query.set('added_after', params.addedAfter)
  if (params.addedBefore) query.set('added_before', params.addedBefore)
  if (params.modifiedAfter) query.set('modified_after', params.modifiedAfter)
  if (params.modifiedBefore) query.set('modified_before', params.modifiedBefore)
  if (params.sort) query.set('sort', params.sort)
  if (params.cursor) query.set('cursor', params.cursor)
  if (params.limit) query.set('limit', String(params.limit))
  return getJSON<AssetPage>(`/api/assets?${query.toString()}`)
}

function setNumberParam(
  query: URLSearchParams,
  key: string,
  value: number | null | undefined,
): void {
  if (value !== null && value !== undefined && Number.isFinite(value)) {
    query.set(key, String(value))
  }
}

export async function listAllAssets(
  params: AssetListParams,
  max = 2000,
): Promise<Asset[]> {
  const items: Asset[] = []
  let cursor: string | undefined
  do {
    const page = await listAssets({ ...params, cursor, limit: 200 })
    items.push(...page.items)
    cursor = page.next_cursor || undefined
  } while (cursor && items.length < max)
  return items.slice(0, max)
}

export function getAsset(id: string): Promise<Asset> {
  return getJSON<Asset>(`/api/assets/${encodeURIComponent(id)}`)
}

// listAssetConversions returns the conversions currently stored for an asset,
// already sorted by file size by the server.
export function listAssetConversions(id: string): Promise<Conversion[]> {
  return getJSON<Conversion[]>(
    `/api/assets/${encodeURIComponent(id)}/conversions`,
  )
}

// purgeConversions removes every generated conversion from the cache.
export function purgeConversions(): Promise<{ removed: number }> {
  return requestJSON<{ removed: number }>('/api/conversions/purge', {
    method: 'POST',
  })
}

// ---- Image presets ----

export function listPresets(): Promise<Preset[]> {
  return getJSON<Preset[]>('/api/presets')
}

export function getPresetCatalog(): Promise<ImgProxyCatalog> {
  return getJSON<ImgProxyCatalog>('/api/presets/catalog')
}

export interface PresetInput {
  label: string
  name: string
  parameters: PresetParameters
}

export function createPreset(input: PresetInput): Promise<Preset> {
  return sendJSON<Preset>('/api/presets', 'POST', input)
}

export function updatePreset(
  id: string,
  input: PresetInput,
): Promise<Preset> {
  return sendJSON<Preset>(
    `/api/presets/${encodeURIComponent(id)}`,
    'PUT',
    input,
  )
}

export function deletePreset(id: string): Promise<void> {
  return requestVoid(`/api/presets/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

// previewPreset renders unsaved preset parameters for one asset. The response
// is an image blob and is never written to the conversion cache.
export async function previewPreset(
  assetId: string,
  parameters: PresetParameters,
  signal?: AbortSignal,
): Promise<Blob> {
  const res = await fetch(
    `/api/assets/${encodeURIComponent(assetId)}/preset-preview`,
    {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify(parameters),
      signal,
    },
  )
  await ensureOk(res)
  const blob = await res.blob()
  if (!blob.size) {
    throw new Error('Preview returned an empty image')
  }
  return blob
}

export interface SignedImgproxyRequest {
  url: string
  segment: string
}

// signImgproxy signs an ad-hoc imgproxy processing pipeline for an asset. The
// returned URL writes through the same conversion cache as named presets.
export function signImgproxy(
  assetId: string,
  format: string,
  processing: string,
): Promise<SignedImgproxyRequest> {
  return sendJSON<SignedImgproxyRequest>('/api/imgproxy/sign', 'POST', {
    asset_id: assetId,
    format,
    processing,
  })
}

export function deleteAsset(id: string): Promise<void> {
  return requestVoid(`/api/assets/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export interface UpdateAssetInput {
  title: string
  note: string
}

// updateAsset changes the editable title and note, returning the refreshed
// asset so the caller can replace its copy without a reload.
export function updateAsset(
  id: string,
  input: UpdateAssetInput,
): Promise<Asset> {
  return sendJSON<Asset>(`/api/assets/${encodeURIComponent(id)}`, 'PUT', input)
}

export function setAssetAttributeValues(
  id: string,
  key: string,
  values: string[],
): Promise<Asset> {
  return sendJSON<Asset>(
    `/api/assets/${encodeURIComponent(id)}/attributes`,
    'PUT',
    { key, values },
  )
}

export function bulkSetAssetAttributeValues(
  assetIds: string[],
  key: string,
  values: string[],
  mode: 'add' | 'replace',
): Promise<void> {
  return requestVoid('/api/assets/attributes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      asset_ids: assetIds,
      key,
      values,
      mode,
    }),
  })
}

export interface UploadSession {
  id: string
  filename: string
  mime_type: string
  size: number
  received: number
  chunk_size: number
  created_at: string
}

export interface StartUploadInput {
  filename: string
  mime_type?: string
  size: number
  driver?: string
}

// startUpload opens a resumable session for a large file. The returned
// chunk_size is the maximum number of bytes accepted by one uploadChunk call.
export function startUpload(input: StartUploadInput): Promise<UploadSession> {
  return sendJSON<UploadSession>('/api/assets/uploads', 'POST', input)
}

export interface UploadRequestOptions {
  signal?: AbortSignal
  onUploadComplete?: () => void
}

function abortError(): DOMException {
  return new DOMException('Upload canceled', 'AbortError')
}

// uploadChunk sends one byte range with XMLHttpRequest so progress can be
// reported. It resolves with the server's new byte offset.
export function uploadChunk(
  id: string,
  offset: number,
  chunk: Blob,
  onProgress?: (fraction: number) => void,
  options: UploadRequestOptions = {},
): Promise<number> {
  if (options.signal?.aborted) return Promise.reject(abortError())
  return new Promise((resolve, reject) => {
    const url = `/api/assets/uploads/${encodeURIComponent(id)}/chunks?offset=${offset}`
    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    const cleanup = () => options.signal?.removeEventListener('abort', abort)

    xhr.open('PUT', url)
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    if (authToken) xhr.setRequestHeader('Authorization', `Bearer ${authToken}`)
    xhr.upload.onprogress = (event) => {
      if (onProgress && event.lengthComputable && event.total > 0) {
        onProgress(event.loaded / event.total)
      }
    }
    xhr.onload = () => {
      cleanup()
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const body = JSON.parse(xhr.responseText) as { offset?: number }
          resolve(typeof body.offset === 'number' ? body.offset : offset + chunk.size)
        } catch {
          resolve(offset + chunk.size)
        }
        return
      }
      let message = `Upload failed (${xhr.status})`
      try {
        const body = JSON.parse(xhr.responseText) as { error?: string }
        if (body.error) message = body.error
      } catch {
        // Keep the status message.
      }
      reject(new Error(message))
    }
    xhr.onerror = () => {
      cleanup()
      reject(new Error('Upload failed'))
    }
    xhr.onabort = () => {
      cleanup()
      reject(abortError())
    }
    options.signal?.addEventListener('abort', abort)
    xhr.send(chunk)
  })
}

export function completeUpload(id: string, signal?: AbortSignal): Promise<Asset> {
  return requestJSON<Asset>(
    `/api/assets/uploads/${encodeURIComponent(id)}/complete`,
    { method: 'POST', signal },
  )
}

export function cancelUpload(id: string): Promise<void> {
  return requestVoid(`/api/assets/uploads/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

// uploadAsset posts a multipart form with XMLHttpRequest so the caller can
// report upload progress (fetch has no upload progress event).
export function uploadAsset(
  file: File,
  driver?: string,
  onProgress?: (fraction: number) => void,
  options: UploadRequestOptions = {},
): Promise<Asset> {
  if (options.signal?.aborted) return Promise.reject(abortError())
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    if (driver) form.append('driver', driver)

    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    const cleanup = () => options.signal?.removeEventListener('abort', abort)

    xhr.open('POST', '/api/assets/upload')
    if (authToken) xhr.setRequestHeader('Authorization', `Bearer ${authToken}`)
    xhr.upload.onprogress = (event) => {
      if (onProgress && event.lengthComputable && event.total > 0) {
        onProgress(event.loaded / event.total)
      }
    }
    xhr.upload.onload = () => options.onUploadComplete?.()
    xhr.onload = () => {
      cleanup()
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(JSON.parse(xhr.responseText) as Asset)
        return
      }
      let message = `Upload failed (${xhr.status})`
      try {
        const body = JSON.parse(xhr.responseText) as { error?: string }
        if (body.error) message = body.error
      } catch {
        // Keep the status message.
      }
      reject(new Error(message))
    }
    xhr.onerror = () => {
      cleanup()
      reject(new Error('Upload failed'))
    }
    xhr.onabort = () => {
      cleanup()
      reject(abortError())
    }
    options.signal?.addEventListener('abort', abort)
    xhr.send(form)
  })
}

export function assetContentUrl(id: string): string {
  return withAuthToken(`/api/assets/${encodeURIComponent(id)}/content`)
}

// Bump when the preview pipeline changes so existing browser-cached previews
// are not reused. The server ignores this parameter.
const PREVIEW_REVISION = '2'

// assetPreviewUrl builds the preview URL. Pass the asset's updated_at as the
// version so an edited file (which regenerates the preview) is fetched fresh
// instead of being served from the browser cache.
export function assetPreviewUrl(id: string, version?: string): string {
  const base = `/api/assets/${encodeURIComponent(id)}/preview`
  const params = new URLSearchParams({ rev: PREVIEW_REVISION })
  if (version) params.set('v', version)
  return withAuthToken(`${base}?${params.toString()}`)
}

// assetsZipUrl is the streaming download of many assets as a zip archive.
export function assetsZipUrl(ids: string[]): string {
  return withAuthToken(
    `/api/assets/zip?ids=${ids.map(encodeURIComponent).join(',')}`,
  )
}

export interface PreviewOptions {
  page?: number
  timestamp?: number
}

// regeneratePreview re-renders the stored preview for an asset and returns the
// refreshed asset (its updated_at changes so caches are bust). Pass a page for
// PDFs or a timestamp for videos to choose which frame becomes the preview.
export function regeneratePreview(
  id: string,
  options?: PreviewOptions,
): Promise<Asset> {
  const init: RequestInit = { method: 'POST' }
  if (options) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(options)
  }
  return requestJSON<Asset>(
    `/api/assets/${encodeURIComponent(id)}/preview`,
    init,
  )
}

// replaceAssetContent overwrites the stored file with edited bytes (e.g. from
// the in-browser image editor). The server regenerates the preview and metadata.
export function replaceAssetContent(
  assetId: string,
  file: Blob,
  filename?: string,
): Promise<Asset> {
  const form = new FormData()
  form.append('file', file, filename ?? 'edited')
  return requestJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/content`,
    { method: 'PUT', body: form },
  )
}
