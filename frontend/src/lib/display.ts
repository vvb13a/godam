export type CardSize = 'small' | 'medium' | 'large'
export type CardShape = 'square' | 'avatar' | 'logo' | 'landscape' | 'poster'

export const CARD_SIZES: { value: CardSize; label: string }[] = [
  { value: 'small', label: 'Small' },
  { value: 'medium', label: 'Medium' },
  { value: 'large', label: 'Large' },
]

export const CARD_SHAPES: { value: CardShape; label: string }[] = [
  { value: 'square', label: 'Square' },
  { value: 'avatar', label: 'Avatar' },
  { value: 'logo', label: 'Logo' },
  { value: 'landscape', label: 'Landscape' },
  { value: 'poster', label: 'Poster' },
]
