import type {
  Asset,
  Collection,
  Library,
  SmartCollection,
  SmartCollectionFilter,
  Stats,
  Tag,
  TagGroup,
  User,
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

// ---- Libraries ----

export function listLibraries(): Promise<Library[]> {
  return getJSON<Library[]>('/api/libraries')
}

export function createLibrary(
  name: string,
  color: string,
  sort: number,
): Promise<Library> {
  return sendJSON<Library>('/api/libraries', 'POST', { name, color, sort })
}

export function updateLibrary(
  id: string,
  name: string,
  color: string,
  sort: number,
): Promise<Library> {
  return sendJSON<Library>(`/api/libraries/${encodeURIComponent(id)}`, 'PUT', {
    name,
    color,
    sort,
  })
}

export function deleteLibrary(id: string): Promise<void> {
  return requestVoid(`/api/libraries/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export function reorderLibraries(
  items: { id: string; sort: number }[],
): Promise<void> {
  return requestVoid('/api/libraries/sort', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ items }),
  })
}

// ---- Assets ----

export function getStats(libraryId: string): Promise<Stats> {
  return getJSON<Stats>(`/api/stats?library_id=${encodeURIComponent(libraryId)}`)
}

export interface AssetListParams {
  libraryId: string
  collectionId?: string
  tagIds?: string[]
  uncategorized?: boolean
  untagged?: boolean
}

export function listAssets(params: AssetListParams): Promise<Asset[]> {
  const query = new URLSearchParams()
  query.set('library_id', params.libraryId)
  if (params.collectionId) query.set('collection_id', params.collectionId)
  if (params.tagIds?.length) query.set('tag_ids', params.tagIds.join(','))
  if (params.uncategorized) query.set('uncategorized', 'true')
  if (params.untagged) query.set('untagged', 'true')
  return getJSON<Asset[]>(`/api/assets?${query.toString()}`)
}

export function getAsset(id: string): Promise<Asset> {
  return getJSON<Asset>(`/api/assets/${encodeURIComponent(id)}`)
}

export function deleteAsset(id: string): Promise<void> {
  return requestVoid(`/api/assets/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export function importAsset(libraryId: string, source: string): Promise<Asset> {
  return sendJSON<Asset>('/api/assets/import', 'POST', {
    library_id: libraryId,
    source,
  })
}

// uploadAsset posts a multipart form with XMLHttpRequest so the caller can
// report upload progress (fetch has no upload progress event).
export function uploadAsset(
  libraryId: string,
  file: File,
  driver?: string,
  onProgress?: (fraction: number) => void,
): Promise<Asset> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    form.append('library_id', libraryId)
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

export function assetPreviewUrl(id: string): string {
  return `/api/assets/${encodeURIComponent(id)}/preview`
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

// ---- Tags ----

export function listTags(libraryId: string): Promise<Tag[]> {
  return getJSON<Tag[]>(`/api/tags?library_id=${encodeURIComponent(libraryId)}`)
}

export function createTag(libraryId: string, name: string): Promise<Tag> {
  return sendJSON<Tag>('/api/tags', 'POST', { library_id: libraryId, name })
}

export function updateTag(
  id: string,
  name: string,
  groupId: string | null,
  starred: boolean,
): Promise<Tag> {
  return sendJSON<Tag>(`/api/tags/${encodeURIComponent(id)}`, 'PUT', {
    name,
    group_id: groupId,
    starred,
  })
}

export function deleteTag(id: string): Promise<void> {
  return requestVoid(`/api/tags/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// ---- Tag groups ----

export function listTagGroups(libraryId: string): Promise<TagGroup[]> {
  return getJSON<TagGroup[]>(
    `/api/tag-groups?library_id=${encodeURIComponent(libraryId)}`,
  )
}

export function createTagGroup(
  libraryId: string,
  name: string,
  description: string,
  color: string,
): Promise<TagGroup> {
  return sendJSON<TagGroup>('/api/tag-groups', 'POST', {
    library_id: libraryId,
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

export function listCollections(libraryId: string): Promise<Collection[]> {
  return getJSON<Collection[]>(
    `/api/collections?library_id=${encodeURIComponent(libraryId)}`,
  )
}

export function createCollection(
  libraryId: string,
  name: string,
  color: string,
  parentId: string | null,
): Promise<Collection> {
  return sendJSON<Collection>('/api/collections', 'POST', {
    library_id: libraryId,
    name,
    color,
    parent_id: parentId,
  })
}

export function updateCollection(
  id: string,
  name: string,
  color: string,
): Promise<Collection> {
  return sendJSON<Collection>(
    `/api/collections/${encodeURIComponent(id)}`,
    'PUT',
    { name, color },
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

export function listSmartCollections(
  libraryId: string,
): Promise<SmartCollection[]> {
  return getJSON<SmartCollection[]>(
    `/api/smart-collections?library_id=${encodeURIComponent(libraryId)}`,
  )
}

export function createSmartCollection(
  libraryId: string,
  name: string,
  color: string,
  filter: SmartCollectionFilter,
): Promise<SmartCollection> {
  return sendJSON<SmartCollection>('/api/smart-collections', 'POST', {
    library_id: libraryId,
    name,
    color,
    filter,
  })
}

export function updateSmartCollection(
  id: string,
  name: string,
  color: string,
): Promise<SmartCollection> {
  return sendJSON<SmartCollection>(
    `/api/smart-collections/${encodeURIComponent(id)}`,
    'PUT',
    { name, color },
  )
}

export function deleteSmartCollection(id: string): Promise<void> {
  return requestVoid(`/api/smart-collections/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
