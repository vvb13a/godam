import { reactive, watch } from 'vue'
import type { View, ViewFilter } from '../types'

const STORAGE_KEY = 'godam:views'

function isView(value: unknown): value is View {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<View>
  return (
    typeof item.id === 'string' &&
    typeof item.name === 'string' &&
    typeof item.color === 'string' &&
    typeof item.sort === 'number' &&
    !!item.filter &&
    typeof item.filter === 'object'
  )
}

function load(): View[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isView)
  } catch {
    return []
  }
}

export const views = reactive({
  items: load() as View[],
})

watch(
  views,
  (value) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value.items))
    } catch {
      // Storage unavailable: views are per-session only.
    }
  },
  { deep: true },
)

function uniqueName(base: string): string {
  const names = new Set(views.items.map((item) => item.name))
  if (!names.has(base)) return base
  let index = 2
  while (names.has(`${base} ${index}`)) index++
  return `${base} ${index}`
}

export function createView(
  name: string,
  filter: ViewFilter,
  color = 'f97316',
): View {
  const now = new Date().toISOString()
  const sort = views.items.reduce(
    (highest, item) => Math.max(highest, item.sort + 1),
    0,
  )
  const view: View = {
    id:
      globalThis.crypto?.randomUUID?.() ??
      `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    name: uniqueName(name.trim() || 'View'),
    description: '',
    color,
    sort,
    filter,
    created_at: now,
    updated_at: now,
  }
  views.items.push(view)
  return view
}

export function updateView(
  id: string,
  patch: Partial<Pick<View, 'name' | 'description' | 'color'>>,
): View | undefined {
  const view = views.items.find((item) => item.id === id)
  if (!view) return undefined
  if (patch.name !== undefined) view.name = patch.name.trim()
  if (patch.description !== undefined) view.description = patch.description
  if (patch.color !== undefined) view.color = patch.color
  view.updated_at = new Date().toISOString()
  return view
}

export function deleteView(id: string): void {
  views.items = views.items.filter((item) => item.id !== id)
}

export function reorderViews(order: { id: string; sort: number }[]): void {
  for (const item of order) {
    const view = views.items.find((entry) => entry.id === item.id)
    if (view) {
      view.sort = item.sort
      view.updated_at = new Date().toISOString()
    }
  }
}
