import { reactive } from 'vue'
import * as api from '../api/client'
import type { User } from '../types'

// auth holds the current actor. Embed sessions carry a scope; direct access
// resolves to an unscoped actor.
export const auth = reactive({
  user: null as User | null,
  ready: false,
})

export async function loadMe(): Promise<void> {
  try {
    auth.user = await api.getMe()
  } catch {
    auth.user = null
  } finally {
    auth.ready = true
  }
}
