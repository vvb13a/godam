import { reactive, watch } from 'vue'
import type { AttributeRenderSetting } from '../types'

export type AttributeRenderMode = 'select' | 'checkbox' | 'tags' | 'tree'

const STORAGE_KEY = 'godam:attributes:render'
const MODES: AttributeRenderMode[] = ['select', 'checkbox', 'tags', 'tree']

export const USER_RENDER_OPTIONS: {
  value: AttributeRenderMode
  label: string
}[] = [
  { value: 'select', label: 'Select' },
  { value: 'checkbox', label: 'Checkbox' },
  { value: 'tags', label: 'Tags' },
  { value: 'tree', label: 'Tree' },
]

export const RENDER_OPTIONS: {
  value: AttributeRenderSetting
  label: string
}[] = [
  { value: 'open', label: 'Open (users choose)' },
  ...USER_RENDER_OPTIONS,
]

function isMode(value: unknown): value is AttributeRenderMode {
  return MODES.includes(value as AttributeRenderMode)
}

function load(): Record<string, AttributeRenderMode> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const result: Record<string, AttributeRenderMode> = {}
    for (const [key, mode] of Object.entries(parsed)) {
      if (isMode(mode)) result[key] = mode
    }
    return result
  } catch {
    return {}
  }
}

export const attributeRenderModes = reactive<
  Record<string, AttributeRenderMode>
>(load())

watch(
  attributeRenderModes,
  (value) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
    } catch {
      // Storage unavailable: the choice simply won't persist.
    }
  },
  { deep: true },
)

export function attributeRenderMode(key: string): AttributeRenderMode {
  const mode = attributeRenderModes[key]
  return isMode(mode) ? mode : 'select'
}

export function setAttributeRenderMode(
  key: string,
  mode: AttributeRenderMode,
): void {
  attributeRenderModes[key] = mode
}

// effectiveAttributeRender resolves the metadata setting. "open" (or missing
// metadata) lets the user choose; a fixed setting always wins.
export function effectiveAttributeRender(
  key: string,
  setting: AttributeRenderSetting | undefined,
): AttributeRenderMode {
  if (setting === 'open' || setting === undefined) {
    return attributeRenderMode(key)
  }
  return isMode(setting) ? setting : attributeRenderMode(key)
}
