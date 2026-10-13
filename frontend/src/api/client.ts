import type { Asset, Collection, Stats, Tag, TagGroup } from '../types'

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
  const res = await fetch(url, init)
  await ensureOk(res)
  return (await res.json()) as T
}

async function requestVoid(url: string, init?: RequestInit): Promise<void> {
  const res = await fetch(url, init)
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

// ---- Assets ----

export function getStats(): Promise<Stats> {
  return getJSON<Stats>('/api/stats')
}

export interface AssetListParams {
  collectionId?: string
  tagIds?: string[]
  uncategorized?: boolean
  untagged?: boolean
}

export function listAssets(params: AssetListParams = {}): Promise<Asset[]> {
  const query = new URLSearchParams()
  if (params.collectionId) query.set('collection_id', params.collectionId)
  if (params.tagIds?.length) query.set('tag_ids', params.tagIds.join(','))
  if (params.uncategorized) query.set('uncategorized', 'true')
  if (params.untagged) query.set('untagged', 'true')
  const qs = query.toString()
  return getJSON<Asset[]>(`/api/assets${qs ? `?${qs}` : ''}`)
}

export function getAsset(id: string): Promise<Asset> {
  return getJSON<Asset>(`/api/assets/${encodeURIComponent(id)}`)
}

export function deleteAsset(id: string): Promise<void> {
  return requestVoid(`/api/assets/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
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
  description: string,
  icon: string,
): Promise<Collection> {
  return sendJSON<Collection>('/api/collections', 'POST', {
    name,
    description,
    icon,
  })
}

export function updateCollection(
  id: string,
  name: string,
  description: string,
  icon: string,
): Promise<Collection> {
  return sendJSON<Collection>(
    `/api/collections/${encodeURIComponent(id)}`,
    'PUT',
    { name, description, icon },
  )
}

export function duplicateCollection(id: string): Promise<Collection> {
  return requestJSON<Collection>(
    `/api/collections/${encodeURIComponent(id)}/duplicate`,
    { method: 'POST' },
  )
}

export function deleteCollection(id: string): Promise<void> {
  return requestVoid(`/api/collections/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}
