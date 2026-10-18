import * as api from '../api/client'
import type { Asset } from '../types'

const MAX_ORIGINAL_BYTES = 8 * 1024 * 1024
const SEARCHABLE_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
])

interface SearchImage {
  blob: Blob
  filename: string
  mime: string
}

export async function searchAssetWithGoogleLens(asset: Asset): Promise<void> {
  if (!asset.mime_type.startsWith('image/')) {
    throw new Error('Only images can be searched with Google Lens')
  }

  const lensWindow = window.open('about:blank', '_blank')
  if (!lensWindow) {
    throw new Error('Allow popups for this site to use Google Lens')
  }
  lensWindow.opener = null
  try {
    lensWindow.document.title = 'Google Lens'
    lensWindow.document.body.textContent = 'Preparing image for Google Lens…'
  } catch {
  }

  try {
    const image = await loadSearchImage(asset)
    const base64 = await blobToBase64(image.blob)
    openGoogleLensUpload(lensWindow, base64, image.filename, image.mime)
  } catch (error) {
    if (!lensWindow.closed) lensWindow.close()
    throw error
  }
}

async function loadSearchImage(asset: Asset): Promise<SearchImage> {
  try {
    const response = await fetch(api.assetContentUrl(asset.id), {
      credentials: 'same-origin',
    })
    if (response.ok) {
      const type = (response.headers.get('content-type') ?? '')
        .split(';')[0]
        .trim()
        .toLowerCase()
      const length = Number(response.headers.get('content-length') ?? 0)
      if (
        SEARCHABLE_TYPES.has(type) &&
        length > 0 &&
        length <= MAX_ORIGINAL_BYTES
      ) {
        return {
          blob: await response.blob(),
          filename: `image${extensionFor(type)}`,
          mime: type,
        }
      }
      await response.body?.cancel()
    }
  } catch {
  }

  const preview = await fetch(api.assetPreviewUrl(asset.id, asset.updated_at), {
    credentials: 'same-origin',
  })
  if (!preview.ok) throw new Error('Could not load the image preview')
  const blob = await preview.blob()
  const type = (blob.type || 'image/jpeg').toLowerCase()
  return {
    blob,
    filename: `image${extensionFor(type)}`,
    mime: type,
  }
}

function extensionFor(mime: string): string {
  switch (mime) {
    case 'image/png':
      return '.png'
    case 'image/webp':
      return '.webp'
    case 'image/gif':
      return '.gif'
    default:
      return '.jpg'
  }
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = typeof reader.result === 'string' ? reader.result : ''
      const comma = result.indexOf(',')
      const base64 = comma >= 0 ? result.slice(comma + 1) : ''
      if (!base64) {
        reject(new Error('Could not prepare the image'))
        return
      }
      resolve(base64)
    }
    reader.onerror = () =>
      reject(reader.error ?? new Error('Could not prepare the image'))
    reader.readAsDataURL(blob)
  })
}

function openGoogleLensUpload(
  lensWindow: Window,
  base64: string,
  filename: string,
  mime: string,
): void {
  const form = `<!doctype html>
<form id="f" action="https://www.google.com/searchbyimage/upload" method="POST" enctype="multipart/form-data" target="_top">
  <input type="file" name="encoded_image" id="i">
  <input type="hidden" name="image_content" value="">
</form>
<script>
  const binary = atob(${JSON.stringify(base64)});
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  const file = new File([bytes], ${JSON.stringify(filename)}, { type: ${JSON.stringify(mime)} });
  const transfer = new DataTransfer();
  transfer.items.add(file);
  document.getElementById('i').files = transfer.files;
  document.getElementById('f').submit();
</script>`

  const page = `<!doctype html>
<meta charset="utf-8">
<title>Google Lens</title>
<p style="font-family: system-ui, sans-serif; padding: 2rem">Preparing image for Google Lens…</p>
<iframe sandbox="allow-forms allow-scripts allow-top-navigation" srcdoc="${escapeHtml(form)}" style="display:none"></iframe>`

  const url = URL.createObjectURL(new Blob([page], { type: 'text/html' }))
  lensWindow.location.replace(url)
  window.setTimeout(() => URL.revokeObjectURL(url), 60000)
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}
