// splitNames accepts a batch of names separated by comma or semicolon and
// returns trimmed, non-empty, case-insensitively unique values in order.
export function splitNames(value: string): string[] {
  const seen = new Set<string>()
  const names: string[] = []
  for (const part of value.split(/[,;]/)) {
    const name = part.trim()
    if (!name) continue
    const key = name.toLocaleLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    names.push(name)
  }
  return names
}
