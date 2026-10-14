import { reactive } from 'vue'
import * as api from '../api/client'
import type { User } from '../types'

// auth holds the signed-in user. `ready` flips once the initial session probe
// finishes so the router guard knows whether it can decide.
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

export async function login(email: string, password: string): Promise<void> {
  auth.user = await api.login(email, password)
}

export async function devLogin(): Promise<void> {
  auth.user = await api.devLogin()
}

export async function logout(): Promise<void> {
  try {
    await api.logout()
  } finally {
    auth.user = null
  }
}

export async function updateProfile(name: string, email: string): Promise<void> {
  auth.user = await api.updateProfile(name, email)
}

export async function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<void> {
  await api.changePassword(currentPassword, newPassword)
}
