<script setup lang="ts">
import { Folder, FolderOpen, FolderPlus } from '@primeicons/vue'
import { libraryColorHex } from '../lib/libraryColors'
import Tree from 'primevue/tree'
import type { TreeNode } from 'primevue/treenode'
import { computed, ref, watch } from 'vue'
import type { AttributeTreeNode } from '../types'

const props = withDefaults(
  defineProps<{
    values: AttributeTreeNode[]
    selected?: Record<string, string[]>
    filter?: string
  }>(),
  {
    selected: () => ({}),
    filter: '',
  },
)

const emit = defineEmits<{
  toggle: [key: string, value: string]
}>()

interface UINode extends TreeNode {
  key: string
  label: string
  data: AttributeTreeNode
  children: UINode[]
}

function toNodes(
  values: AttributeTreeNode[],
  path = '',
): UINode[] {
  return values.map((value) => {
    const key = `${path}\u0000${value.key}\u0000${value.value}`
    return {
      key,
      label: value.label || value.value,
      data: value,
      children: toNodes(value.children ?? [], key),
    }
  })
}

function filterNodes(
  values: AttributeTreeNode[],
  query: string,
): AttributeTreeNode[] {
  if (!query) return values
  const result: AttributeTreeNode[] = []
  for (const value of values) {
    const children = filterNodes(value.children ?? [], query)
    const matches =
      (value.label || value.value).toLowerCase().includes(query) ||
      value.value.toLowerCase().includes(query)
    if (matches) result.push(value)
    else if (children.length) result.push({ ...value, children })
  }
  return result
}

const nodes = computed(() =>
  toNodes(
    filterNodes(
      props.values,
      (props.filter ?? '').trim().toLowerCase(),
    ),
  ),
)
const expandedKeys = ref<Record<string, boolean>>({})

watch(
  nodes,
  (list) => {
    const next = { ...expandedKeys.value }
    let changed = false
    const collect = (items: UINode[]): void => {
      for (const item of items) {
        if (item.children.length && !(item.key in next)) {
          next[item.key] = true
          changed = true
        }
        collect(item.children)
      }
    }
    collect(list)
    if (changed) expandedKeys.value = next
  },
  { immediate: true, deep: true },
)

function isSelected(node: AttributeTreeNode): boolean {
  return (props.selected[node.key] ?? []).includes(node.value)
}

function hasChildren(node: TreeNode): boolean {
  return (node.children?.length ?? 0) > 0
}

function folderIcon(node: TreeNode) {
  if (!hasChildren(node)) return Folder
  return expandedKeys.value[String(node.key)] ? FolderOpen : FolderPlus
}

function toggleNode(node: TreeNode): void {
  const key = String(node.key)
  expandedKeys.value = {
    ...expandedKeys.value,
    [key]: !expandedKeys.value[key],
  }
}

function onFolderClick(event: MouseEvent, node: TreeNode): void {
  if (!hasChildren(node)) return
  event.stopPropagation()
  toggleNode(node)
}

const treePt = {
  nodeContent: { class: '!p-0' },
  nodeToggleButton: { class: '!hidden' },
  nodeLabel: { class: '!min-w-0 !flex-1' },
  // Expose the parent's colour to the connector line drawn for its children.
  nodeChildren: (options: {
    instance?: { node?: { data?: { color?: string } | null } }
  }) => ({
    style: {
      '--guide-color': libraryColorHex(options?.instance?.node?.data?.color ?? ''),
    },
  }),
}
</script>

<template>
  <Tree
    :value="nodes"
    v-model:expanded-keys="expandedKeys"
    :meta-key-selection="false"
    :pt="treePt"
    class="!border-0 !bg-transparent !px-0 !pt-1"
  >
    <template #empty>
      <p class="px-2 py-2 text-xs text-slate-400">No values</p>
    </template>
    <template #default="{ node }">
      <div
        class="flex w-full min-w-0 items-center gap-2 rounded-md px-2 py-1 text-sm transition"
        :class="
          isSelected(node.data)
            ? 'bg-brand-50/60 text-brand-700 dark:bg-brand-950/25 dark:text-brand-300'
            : 'text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800'
        "
        v-tooltip="{ value: node.data.description || '' }"
        @click="emit('toggle', node.data.key, node.data.value)"
      >
        <component
          :is="folderIcon(node)"
          :size="20"
          class="shrink-0"
          :style="{ color: libraryColorHex(node.data.color ?? '') }"
          @click="onFolderClick($event, node)"
        />
        <span class="min-w-0 flex-1 truncate leading-5">{{ node.label }}</span>
        <span
          v-if="node.data.count"
          class="shrink-0 text-xs text-slate-400"
        >
          {{ node.data.count }}
        </span>
      </div>
    </template>
  </Tree>
</template>


<style scoped>
/* Tree guide lines: a dotted line drops from the parent folder icon centre and
   turns into a small L that stops just before the last child's icon. The colour
   is the parent folder's, exposed as --guide-color. */
:deep(.p-tree-node-children > .p-tree-node) {
  position: relative;
}

:deep(.p-tree-node-children > .p-tree-node)::before {
  content: '';
  position: absolute;
  left: -0.2rem;
  top: 0;
  bottom: -2px;
  width: 2px;
  background-image: repeating-linear-gradient(
    to bottom,
    var(--guide-color, transparent) 0 2px,
    transparent 2px 4px
  );
  opacity: 0.8;
}

:deep(.p-tree-node-children > .p-tree-node:first-child)::before {
  top: -0.5rem;
}

:deep(.p-tree-node-children > .p-tree-node:last-child)::before {
  bottom: 0.875rem;
}

:deep(.p-tree-node-children > .p-tree-node:last-child)::after {
  content: '';
  position: absolute;
  left: -0.2rem;
  top: 0.875rem;
  width: 0.7rem;
  height: 2px;
  background-image: repeating-linear-gradient(
    to right,
    var(--guide-color, transparent) 0 2px,
    transparent 2px 4px
  );
  opacity: 0.8;
}
</style>
