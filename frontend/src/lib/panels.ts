import { reactive, watch } from 'vue'

// Panels tracks whether the left (app) and right (inspector) sidebars are
// collapsed. It is a per-device preference persisted to localStorage, so the
// layout is restored the next time the app loads.
export interface PanelState {
  leftCollapsed: boolean
  rightCollapsed: boolean
}

const PANEL_STORAGE_KEY = 'godam:panels'

const DEFAULT_STATE: PanelState = {
  leftCollapsed: false,
  rightCollapsed: false,
}

function loadState(): PanelState {
  try {
    const raw = localStorage.getItem(PANEL_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_STATE }
    const parsed = JSON.parse(raw) as Partial<PanelState>
    return { ...DEFAULT_STATE, ...parsed }
  } catch {
    return { ...DEFAULT_STATE }
  }
}

export const panels = reactive<PanelState>(loadState())

watch(
  panels,
  (value) => {
    try {
      localStorage.setItem(PANEL_STORAGE_KEY, JSON.stringify(value))
    } catch {
      // Storage can be unavailable (private mode); preferences are non-critical.
    }
  },
  { deep: true },
)

export function toggleLeftPanel(): void {
  panels.leftCollapsed = !panels.leftCollapsed
}

export function toggleRightPanel(): void {
  panels.rightCollapsed = !panels.rightCollapsed
}
