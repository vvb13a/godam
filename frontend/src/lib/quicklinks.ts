import { reactive, watch } from 'vue'

export type QuicklinkType = 'collection' | 'smart' | 'tag'

export interface Quicklink {
  type: QuicklinkType
  id: string
  name: string
  color?: string
}

const STORAGE_KEY = 'godam:quicklinks'

function load(): Quicklink[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is Quicklink =>
        item &&
        typeof item.id === 'string' &&
        typeof item.name === 'string' &&
        (item.type === 'collection' ||
          item.type === 'smart' ||
          item.type === 'tag'),
    )
  } catch {
    return []
  }
}

export const quicklinks = reactive({
  items: load() as Quicklink[],
})

watch(
  quicklinks,
  (value) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value.items))
    } catch {
      // Storage unavailable: quicklinks are per-session only.
    }
  },
  { deep: true },
)

export function isPinned(type: QuicklinkType, id: string): boolean {
  return quicklinks.items.some((item) => item.type === type && item.id === id)
}

export function toggleQuicklink(link: Quicklink): void {
  const index = quicklinks.items.findIndex(
    (item) => item.type === link.type && item.id === link.id,
  )
  if (index >= 0) quicklinks.items.splice(index, 1)
  else quicklinks.items.push(link)
}
