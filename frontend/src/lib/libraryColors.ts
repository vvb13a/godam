// LIBRARY_COLORS is the curated palette for library swatches. Colors are stored
// without the leading '#' (matching tag-group colors) and rendered as `#${hex}`.
export interface LibraryColor {
  name: string
  value: string
}

export const LIBRARY_COLORS: LibraryColor[] = [
  { name: 'Violet', value: '8b5cf6' },
  { name: 'Indigo', value: '6366f1' },
  { name: 'Blue', value: '3b82f6' },
  { name: 'Sky', value: '0ea5e9' },
  { name: 'Teal', value: '14b8a6' },
  { name: 'Emerald', value: '10b981' },
  { name: 'Amber', value: 'f59e0b' },
  { name: 'Orange', value: 'f97316' },
  { name: 'Rose', value: 'f43f5e' },
  { name: 'Pink', value: 'ec4899' },
]

export const DEFAULT_LIBRARY_COLOR = 'f97316'

export function libraryColorHex(value: string): string {
  return value ? `#${value}` : '#94a3b8'
}
