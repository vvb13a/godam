import { reactive, ref } from 'vue'

export const attributesDialog = reactive({
  visible: false,
})

// attributesRevision lets the sidebar refresh labels/sort after the dialog
// changes metadata without reloading the whole library.
export const attributesRevision = ref(0)

export function openAttributesDialog(): void {
  attributesDialog.visible = true
}

export function notifyAttributesChanged(): void {
  attributesRevision.value++
}
