import { ref } from 'vue'

// Bumped whenever the conversion cache changes globally (purge or bulk warm),
// so an open inspector can reload its conversion list.
export const conversionCacheVersion = ref(0)

export function notifyConversionCacheChanged(): void {
  conversionCacheVersion.value++
}
