import type { Asset, Collection, Stats } from '../types'

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

export function listAssets(collectionId?: string): Promise<Asset[]> {
  const query = collectionId
    ? `?collection_id=${encodeURIComponent(collectionId)}`
    : ''
  return getJSON<Asset[]>(`/api/assets${query}`)
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

export function addTag(assetId: string, name: string): Promise<Asset> {
  return sendJSON<Asset>(
    `/api/assets/${encodeURIComponent(assetId)}/tags`,
    'POST',
    { name },
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
