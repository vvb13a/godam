export type CardShape = 'square' | 'landscape' | 'poster' | 'native'

// Asset preview width in the grid, in pixels. Bounds are chosen to stay
// useful: 80px still shows a recognisable thumbnail, 320px is a large preview.
export const CARD_SIZE_MIN = 80
export const CARD_SIZE_MAX = 320
export const CARD_SIZE_DEFAULT = 132

export const CARD_SHAPES: { value: CardShape; label: string }[] = [
  { value: 'square', label: 'Square' },
  { value: 'landscape', label: 'Landscape' },
  { value: 'poster', label: 'Poster' },
  { value: 'native', label: 'Mosaic' },
]
