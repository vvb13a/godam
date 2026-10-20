import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Ref } from 'vue'

// useAnimatedHeight gives an auto-sized dialog body an explicit height that can
// be transitioned. The content ref keeps its natural/max-height sizing; the
// wrapper is clipped and animated to match it.
export function useAnimatedHeight(content: Ref<HTMLElement | null>) {
  const wrapper = ref<HTMLElement | null>(null)
  const height = ref<number | null>(null)
  const ready = ref(false)
  let observer: ResizeObserver | null = null

  function setWrapper(el: unknown): void {
    wrapper.value = el instanceof HTMLElement ? el : null
  }

  function measure(): void {
    const el = content.value
    if (!el) return
    const next = el.offsetHeight
    if (height.value !== null && Math.abs(next - height.value) < 0.5) return
    height.value = next
    if (!ready.value) {
      requestAnimationFrame(() => {
        ready.value = true
      })
    }
  }

  watch(
    content,
    (el) => {
      observer?.disconnect()
      observer = null
      if (!el) {
        height.value = null
        ready.value = false
        return
      }
      measure()
      observer = new ResizeObserver(measure)
      observer.observe(el)
    },
    { flush: 'post' },
  )

  onBeforeUnmount(() => observer?.disconnect())

  const style = computed(() => ({
    height: height.value === null ? undefined : `${height.value}px`,
    transition: ready.value ? 'height 200ms ease-out' : 'none',
  }))

  return { setWrapper, style }
}
