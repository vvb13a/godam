import { auth } from './auth'

// hasCapability mirrors the server's exact, namespace wildcard, and global
// wildcard matching. It is for UI affordances only; the server is authoritative.
export function hasCapability(capability: string): boolean {
  const capabilities = auth.user?.capabilities ?? []
  for (const raw of capabilities) {
    const value = raw.trim()
    if (value === '*' || value === capability) return true
    if (value.endsWith(':*')) {
      const prefix = value.slice(0, -1)
      if (capability.startsWith(prefix)) return true
    }
  }
  return false
}

export function hasAnyCapability(capabilities: string[]): boolean {
  return capabilities.some(hasCapability)
}
