// Catalog of uploadable MIME types, shared by the library upload settings and
// the upload dialog. Keep DEFAULT_ALLOWED in sync with the backend
// service.DefaultConfig allow-list.

export interface MimeOption {
  value: string
  label: string
}

export interface MimeCategory {
  label: string
  options: MimeOption[]
}

export const MIME_CATEGORIES: MimeCategory[] = [
  {
    label: 'Images',
    options: [
      { value: 'image/jpeg', label: 'JPEG' },
      { value: 'image/png', label: 'PNG' },
      { value: 'image/webp', label: 'WebP' },
      { value: 'image/gif', label: 'GIF' },
      { value: 'image/tiff', label: 'TIFF' },
      { value: 'image/bmp', label: 'BMP' },
      { value: 'image/svg+xml', label: 'SVG' },
    ],
  },
  {
    label: 'Documents',
    options: [{ value: 'application/pdf', label: 'PDF' }],
  },
  {
    label: 'Fonts',
    options: [
      { value: 'font/sfnt', label: 'TrueType / OpenType' },
      { value: 'font/woff', label: 'WOFF' },
      { value: 'font/woff2', label: 'WOFF2' },
    ],
  },
  {
    label: 'Text & code',
    options: [
      { value: 'text/plain', label: 'Plain text' },
      { value: 'text/csv', label: 'CSV' },
      { value: 'text/css', label: 'CSS' },
      { value: 'text/html', label: 'HTML' },
      { value: 'text/javascript', label: 'JavaScript' },
      { value: 'application/json', label: 'JSON' },
      { value: 'application/xml', label: 'XML' },
      { value: 'text/markdown', label: 'Markdown' },
    ],
  },
  {
    label: 'Audio',
    options: [{ value: 'audio/*', label: 'All audio' }],
  },
  {
    label: 'Video',
    options: [{ value: 'video/*', label: 'All video' }],
  },
]

export const DEFAULT_ALLOWED: string[] = [
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
  'image/tiff',
  'image/bmp',
  'image/svg+xml',
  'application/pdf',
  'font/sfnt',
  'font/woff',
  'font/woff2',
  'text/plain',
  'text/csv',
  'text/css',
  'text/html',
  'text/javascript',
  'application/json',
  'application/xml',
  'text/markdown',
  'audio/*',
  'video/*',
  'application/ogg',
]

// accept attribute for <input type="file">: extensions plus mime wildcards.
export const ACCEPT_STRING =
  '.jpg,.jpeg,.png,.webp,.gif,.tif,.tiff,.bmp,.svg,.pdf,.ttf,.otf,.woff,.woff2,.txt,.csv,.css,.js,.json,.xml,.md,.mp3,.wav,.ogg,.flac,.m4a,.mp4,.webm,.mov,image/*,audio/*,video/*'

export function isFontMime(mime: string): boolean {
  return (
    mime.startsWith('font/') ||
    mime.startsWith('application/font') ||
    mime.startsWith('application/x-font') ||
    mime === 'application/vnd.ms-fontobject'
  )
}

export function isCsvMime(mime: string): boolean {
  return mime === 'text/csv' || mime === 'application/csv'
}

export function isTextMime(mime: string): boolean {
  return (
    (mime.startsWith('text/') && !isCsvMime(mime)) ||
    mime === 'application/json' ||
    mime === 'application/xml' ||
    mime === 'application/javascript'
  )
}

// Asset types for which the backend produces a real visual preview (image,
// audio waveform, video frame, or PDF first page). Everything else uses the
// app's default file-type artwork instead.
export function hasMediaPreview(mime: string): boolean {
  const clean = mime.split(';')[0].trim().toLowerCase()
  return (
    clean.startsWith('image/') ||
    clean.startsWith('audio/') ||
    clean === 'application/ogg' ||
    clean.startsWith('video/') ||
    clean === 'application/pdf'
  )
}
