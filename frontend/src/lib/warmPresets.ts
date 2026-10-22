import { reactive } from 'vue'
import { authHeaders } from '../api/client'
import { loadStats } from './library'

interface WarmEvent {
  type: 'start' | 'progress' | 'error' | 'done'
  total_assets?: number
  total_conversions?: number
  skipped?: number
  asset_id?: string
  preset?: string
  done?: number
  failed?: number
  message?: string
}

export const warmProgress = reactive({
  active: false,
  finished: false,
  totalAssets: 0,
  totalConversions: 0,
  done: 0,
  failed: 0,
  skipped: 0,
  current: '',
  errors: [] as string[],
})

let controller: AbortController | null = null

function applyEvent(event: WarmEvent): void {
  if (event.type === 'start') {
    warmProgress.totalAssets = event.total_assets ?? 0
    warmProgress.totalConversions = event.total_conversions ?? 0
    warmProgress.skipped = event.skipped ?? 0
    return
  }
  if (event.type === 'progress' || event.type === 'error') {
    warmProgress.done = event.done ?? warmProgress.done
    warmProgress.failed = event.failed ?? warmProgress.failed
    warmProgress.current = event.preset
      ? `${event.preset} · ${event.asset_id ?? ''}`
      : ''
    if (event.type === 'error' && event.message) {
      warmProgress.errors.push(event.message)
      if (warmProgress.errors.length > 8) warmProgress.errors.shift()
    }
    return
  }
  if (event.type === 'done') {
    warmProgress.done = event.done ?? warmProgress.done
    warmProgress.failed = event.failed ?? warmProgress.failed
    warmProgress.skipped = event.skipped ?? warmProgress.skipped
    warmProgress.current = ''
  }
}

// warmPresets streams warm-up progress for the selected assets. The promise
// resolves when the server finishes or the request is cancelled.
export async function warmPresets(assetIds: string[]): Promise<void> {
  if (warmProgress.active || !assetIds.length) return

  warmProgress.active = true
  warmProgress.finished = false
  warmProgress.totalAssets = 0
  warmProgress.totalConversions = 0
  warmProgress.done = 0
  warmProgress.failed = 0
  warmProgress.skipped = 0
  warmProgress.current = ''
  warmProgress.errors = []

  controller = new AbortController()
  try {
    const res = await fetch('/api/assets/presets/warm', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ asset_ids: assetIds }),
      signal: controller.signal,
    })
    if (!res.ok || !res.body) {
      let message = `Warm-up failed (${res.status})`
      try {
        const body = (await res.json()) as { error?: string }
        if (body.error) message = body.error
      } catch {
        // Keep the status message.
      }
      throw new Error(message)
    }

    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const chunk = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        const data = chunk
          .split('\n')
          .filter((line) => line.startsWith('data:'))
          .map((line) => line.slice(5).trim())
          .join('\n')
        if (data) {
          try {
            applyEvent(JSON.parse(data) as WarmEvent)
          } catch {
            // Ignore malformed frames.
          }
        }
        boundary = buffer.indexOf('\n\n')
      }
    }
  } catch (e) {
    if (!(e instanceof DOMException && e.name === 'AbortError')) {
      warmProgress.errors.push(e instanceof Error ? e.message : String(e))
    }
  } finally {
    warmProgress.active = false
    warmProgress.finished = true
    warmProgress.current = ''
    controller = null
    void loadStats()
  }
}

export function cancelWarm(): void {
  controller?.abort()
}

export function dismissWarm(): void {
  warmProgress.finished = false
  warmProgress.totalAssets = 0
  warmProgress.totalConversions = 0
  warmProgress.done = 0
  warmProgress.failed = 0
  warmProgress.skipped = 0
  warmProgress.errors = []
}
