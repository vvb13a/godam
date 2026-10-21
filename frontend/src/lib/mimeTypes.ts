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
    ],
  },
  {
    label: 'Documents',
    options: [{ value: 'application/pdf', label: 'PDF' }],
  },
  {
    label: 'Audio',
    options: [
      { value: 'audio/mpeg', label: 'MP3' },
      { value: 'audio/wav', label: 'WAV' },
      { value: 'audio/mp4', label: 'M4A / AAC' },
      { value: 'audio/webm', label: 'WebM audio' },
    ],
  },
  {
    label: 'Video',
    options: [
      { value: 'video/mp4', label: 'MP4' },
      { value: 'video/webm', label: 'WebM' },
      { value: 'video/quicktime', label: 'QuickTime (MOV)' },
    ],
  },
]

export const DEFAULT_ALLOWED: string[] = [
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
  'application/pdf',
  'audio/mpeg',
  'audio/wav',
  'audio/mp4',
  'audio/webm',
  'video/mp4',
  'video/webm',
  'video/quicktime',
]

// accept attribute for <input type="file">: extensions plus mime wildcards.
export const ACCEPT_STRING =
  '.jpg,.jpeg,.png,.webp,.gif,.pdf,.mp3,.wav,.m4a,.aac,.mp4,.webm,.mov'

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
