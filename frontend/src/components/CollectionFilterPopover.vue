<script setup lang="ts">
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import { ChevronDown, Search } from '@primeicons/vue'
import { computed, ref } from 'vue'
import { library } from '../lib/library'
import { libraryColorHex } from '../lib/libraryColors'
import CollectionTree from './CollectionTree.vue'
import TagBadge from './TagBadge.vue'

const props = withDefaults(
  defineProps<{
    modelValue: string[]
    disabled?: boolean
    lockedTitle?: string
  }>(),
  {
    disabled: false,
    lockedTitle: 'This filter is locked by the host application',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
  show: []
  hide: []
}>()

const popover = ref<InstanceType<typeof Popover> | null>(null)
const search = ref('')
const draft = ref<string[]>([])

const options = computed(() => [
  {
    id: 'uncategorized',
    label: 'Uncategorized',
    color: undefined as string | undefined,
  },
  ...library.collections.map((collection) => ({
    id: collection.id,
    label: collection.name,
    color: libraryColorHex(collection.color),
  })),
])

const selectedItems = computed(() => {
  const ids = new Set(props.modelValue)
  return options.value.filter((item) => ids.has(item.id))
})

const visibleItems = computed(() => selectedItems.value.slice(0, 2))
const overflowCount = computed(() =>
  Math.max(0, selectedItems.value.length - visibleItems.value.length),
)
function toggle(event: Event): void {
  if (props.disabled) return
  draft.value = [...props.modelValue]
  search.value = ''
  popover.value?.toggle(event)
}

function apply(): void {
  const current = props.modelValue.join(',')
  const next = draft.value.join(',')
  if (current !== next) emit('update:modelValue', [...draft.value])
  popover.value?.hide()
}

function clear(): void {
  draft.value = []
}
</script>

<template>
  <button
    type="button"
    class="flex h-9 w-full min-w-0 items-center gap-2 rounded-md border border-slate-300 bg-white px-2 text-left text-sm text-slate-700 shadow-sm transition hover:border-brand-300 disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
    :disabled="disabled"
    :title="disabled ? lockedTitle : 'Filter by collection'"
    @click="toggle"
  >
    <span class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
      <template v-if="selectedItems.length">
        <TagBadge
          v-for="item in visibleItems"
          :key="item.id"
          :name="item.label"
          :color="item.color"
          class="max-w-[7rem]"
        />
        <span v-if="overflowCount" class="shrink-0 text-xs text-slate-400">
          +{{ overflowCount }}
        </span>
      </template>
      <span v-else class="truncate text-slate-400">All collections</span>
    </span>
    <ChevronDown :size="14" class="shrink-0 text-slate-400" />
  </button>

  <Popover
    ref="popover"
    :pt="{ content: { class: '!p-0' } }"
    @show="emit('show')"
    @hide="emit('hide')"
  >
    <div class="w-80">
      <div
        class="flex items-center gap-2 border-b border-slate-100 px-2 pt-2 pb-2 dark:border-slate-800"
      >
        <Search :size="15" class="shrink-0 text-slate-400" />
        <input
          v-model="search"
          type="text"
          placeholder="Search collections…"
          class="min-w-0 flex-1 bg-transparent text-sm outline-none"
        />
      </div>

      <div class="mt-2 max-h-96 overflow-y-auto">
        <CollectionTree
          v-model:selected-ids="draft"
          selectable
          :navigable="false"
          :manage="false"
          :show-header="false"
          :defer-refresh="true"
          :filter="search"
        />
      </div>

      <div
        class="mt-2 flex items-center justify-between border-t border-slate-100 px-2 pt-2 pb-2 dark:border-slate-800"
      >
        <span class="text-xs text-slate-400">
          {{ draft.length }} selected
        </span>
        <div class="flex items-center gap-2">
          <Button label="Clear" text size="small" @click="clear" />
          <Button label="Done" size="small" @click="apply" />
        </div>
      </div>
    </div>
  </Popover>
</template>
