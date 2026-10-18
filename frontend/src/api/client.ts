import type {
  Asset,
  Collection,
  Library,
  LibraryBackup,
  LibraryBackupStatus,
  SmartCollection,
  SmartCollectionFilter,
  Stats,
  Tag,
  TagGroup,
  User,
  UserOption,
} from '../types'

// Auth endpoints legitimately return 401 (bad/absent credentials); every other
// 401 means the session expired, which the app handles by returning to login.
let unauthorizedHandler: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
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
  if (res.status === 401 && !res.url.includes('/api/auth/')) {
    unauthorizedHandler?.()
  }
  throw new Error(message)
}

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, { credentials: 'same-origin', ...init })
  await ensureOk(res)
  return (await res.json()) as T
}

async function requestVoid(url: string, init?: RequestInit): Promise<void> {
  const res = await fetch(url, { credentials: 'same-origin', ...init })
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

export function listUsers(): Promise<UserOption[]> {
  return getJSON<UserOption[]>('/api/users')
}

export function login(email: string, password: string): Promise<User> {
  return sendJSON<User>('/api/auth/login', 'POST', { email, password })
}

export function devLogin(): Promise<User> {
  return requestJSON<User>('/api/auth/dev-login', { method: 'POST' })
}

export function logout(): Promise<void> {
  return requestVoid('/api/auth/logout', { method: 'POST' })
}

export function updateProfile(name: string, email: string): Promise<User> {
  return sendJSON<User>('/api/auth/profile', 'PUT', { name, email })
}

export function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  return requestVoid('/api/auth/password', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  })
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
  maxUploadBytes: number,
): Promise<Library> {
  return sendJSON<Library>('/api/instance/upload-settings', 'PUT', {
    allowed_mime_types: allowedMimeTypes,
    max_upload_bytes: maxUploadBytes,
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
  collectionIds?: string[]
  tagIds?: string[]
  uncategorized?: boolean
  untagged?: boolean
  search?: string
  types?: string[]
  colors?: string[]
  mimeTypes?: string[]
  uploadedBy?: string[]
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

export function listAssets(params: AssetListParams): Promise<AssetPage> {
  const query = new URLSearchParams()
  if (params.collectionIds?.length) {
    query.set('collection_ids', params.collectionIds.join(','))
  }
  if (params.tagIds?.length) query.set('tag_ids', params.tagIds.join(','))
  if (params.uncategorized) query.set('uncategorized', 'true')
  if (params.untagged) query.set('untagged', 'true')
  if (params.search) query.set('search', params.search)
  if (params.types?.length) query.set('types', params.types.join(','))
  if (params.colors?.length) query.set('colors', params.colors.join(','))
  if (params.mimeTypes?.length) query.set('mime_types', params.mimeTypes.join(','))
  if (params.uploadedBy?.length) {
    query.set('uploaded_by', params.uploadedBy.join(','))
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

export function importAsset(source: string): Promise<Asset> {
  return sendJSON<Asset>('/api/assets/import', 'POST', { source })
}

// uploadAsset posts a multipart form with XMLHttpRequest so the caller can
// report upload progress (fetch has no upload progress event).
export function uploadAsset(
  file: File,
  driver?: string,
  onProgress?: (fraction: number) => void,
): Promise<Asset> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    if (driver) form.append('driver', driver)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/assets/upload')
    xhr.upload.onprogress = (event) => {
      if (onProgress && event.lengthComputable) {
        onProgress(event.loaded / event.total)
      }
    }
    xhr.onload = () => {
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
    xhr.onerror = () => reject(new Error('Upload failed'))
    xhr.send(form)
  })
}

export function assetContentUrl(id: string): string {
  return `/api/assets/${encodeURIComponent(id)}/content`
}

// assetPreviewUrl builds the preview URL. Pass the asset's updated_at as the
// version so an edited file (which regenerates the preview) is fetched fresh
// instead of being served from the browser cache.
export function assetPreviewUrl(id: string, version?: string): string {
  const base = `/api/assets/${encodeURIComponent(id)}/preview`
  return version ? `${base}?v=${encodeURIComponent(version)}` : base
}

// assetsZipUrl is the streaming download of many assets as a zip archive.
export function assetsZipUrl(ids: string[]): string {
  return `/api/assets/zip?ids=${ids.map(encodeURIComponent).join(',')}`
}

// regeneratePreview re-renders the stored preview for an asset and returns the
// refreshed asset (its updated_at changes so caches are bust).
export function regeneratePreview(id: string): Promise<Asset> {
  return requestJSON<Asset>(
    `/api/assets/${encodeURIComponent(id)}/preview`,
    { method: 'POST' },
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

// ---- Asset tags & collections ----

export function addTag(assetId: string, tagId: string): Promise<Asset> {
  return sendJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/tags`,
    'POST',
    { tag_id: tagId },
  )
}

export function removeTag(assetId: string, tagId: string): Promise<Asset> {
  return requestJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/tags/${encodeURIComponent(tagId)}`,
    { method: 'DELETE' },
  )
}

export function addToCollection(
  assetId: string,
  collectionId: string,
): Promise<Asset> {
  return sendJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/collections`,
    'POST',
    { collection_id: collectionId },
  )
}

export function removeFromCollection(
  assetId: string,
  collectionId: string,
): Promise<Asset> {
  return requestJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/collections/${encodeURIComponent(collectionId)}`,
    { method: 'DELETE' },
  )
}

// addAssetsToCollection links many assets to one collection in a single request.
export function addAssetsToCollection(
  collectionId: string,
  assetIds: string[],
): Promise<void> {
  return requestVoid('/api/assets/collections', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ collection_id: collectionId, asset_ids: assetIds }),
  })
}

// applyAssetsTags adds or replaces tags across many assets in one request.
// "add" merges tagIds into each asset; "replace" makes them the exact tag set.
export function applyAssetsTags(
  assetIds: string[],
  tagIds: string[],
  mode: 'add' | 'replace',
): Promise<void> {
  return requestVoid('/api/assets/tags', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ asset_ids: assetIds, tag_ids: tagIds, mode }),
  })
}

// ---- Tags ----

export function listTags(): Promise<Tag[]> {
  return getJSON<Tag[]>('/api/tags')
}

export function createTag(name: string): Promise<Tag> {
  return sendJSON<Tag>('/api/tags', 'POST', { name })
}

export function updateTag(
  id: string,
  name: string,
  groupId: string | null,
  starred: boolean,
  description: string,
): Promise<Tag> {
  return sendJSON<Tag>(`/api/tags/${encodeURIComponent(id)}`, 'PUT', {
    name,
    description,
    group_id: groupId,
    starred,
  })
}

export function deleteTag(id: string): Promise<void> {
  return requestVoid(`/api/tags/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// ---- Tag groups ----

export function listTagGroups(): Promise<TagGroup[]> {
  return getJSON<TagGroup[]>('/api/tag-groups')
}

export function createTagGroup(
  name: string,
  description: string,
  color: string,
): Promise<TagGroup> {
  return sendJSON<TagGroup>('/api/tag-groups', 'POST', {
    name,
    description,
    color,
  })
}

export function updateTagGroup(
  id: string,
  name: string,
  description: string,
  color: string,
): Promise<TagGroup> {
  return sendJSON<TagGroup>(
    `/api/tag-groups/${encodeURIComponent(id)}`,
    'PUT',
    { name, description, color },
  )
}

export function deleteTagGroup(id: string): Promise<void> {
  return requestVoid(`/api/tag-groups/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

// ---- Collections ----

export function listCollections(): Promise<Collection[]> {
  return getJSON<Collection[]>('/api/collections')
}

export function createCollection(
  name: string,
  color: string,
  parentId: string | null,
): Promise<Collection> {
  return sendJSON<Collection>('/api/collections', 'POST', {
    name,
    color,
    parent_id: parentId,
  })
}

export function updateCollection(
  id: string,
  name: string,
  color: string,
  description: string,
): Promise<Collection> {
  return sendJSON<Collection>(
    `/api/collections/${encodeURIComponent(id)}`,
    'PUT',
    { name, description, color },
  )
}

// moveCollection reparents/reorders a folder after a drag-and-drop.
export function moveCollection(
  id: string,
  parentId: string | null,
  position: number,
): Promise<void> {
  return requestVoid(`/api/collections/${encodeURIComponent(id)}/move`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parent_id: parentId, position }),
  })
}

export function deleteCollection(id: string): Promise<void> {
  return requestVoid(`/api/collections/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

// ---- Smart collections ----

export function listSmartCollections(): Promise<SmartCollection[]> {
  return getJSON<SmartCollection[]>('/api/smart-collections')
}

export function createSmartCollection(
  name: string,
  color: string,
  filter: SmartCollectionFilter,
  description = '',
): Promise<SmartCollection> {
  return sendJSON<SmartCollection>('/api/smart-collections', 'POST', {
    name,
    description,
    color,
    filter,
  })
}

export function updateSmartCollection(
  id: string,
  name: string,
  color: string,
  description: string,
): Promise<SmartCollection> {
  return sendJSON<SmartCollection>(
    `/api/smart-collections/${encodeURIComponent(id)}`,
    'PUT',
    { name, description, color },
  )
}

export function reorderSmartCollections(
  items: { id: string; sort: number }[],
): Promise<void> {
  return requestVoid('/api/smart-collections/sort', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  })
}

export function deleteSmartCollection(id: string): Promise<void> {
  return requestVoid(`/api/smart-collections/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
