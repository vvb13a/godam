<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Camera,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Download,
  Expand,
  Print,
  Refresh,
  Search,
  SearchMinus,
  SearchPlus,
  Times,
  Undo,
  WindowMaximize,
} from '@primeicons/vue'
import { useToast } from 'primevue/usetoast'
import type {
  EventBus as EventBusInstance,
  PDFFindController as PDFFindControllerInstance,
  PDFLinkService as PDFLinkServiceInstance,
  PDFViewer as PDFViewerInstance,
} from 'pdfjs-dist/web/pdf_viewer.mjs'
import * as api from '../api/client'
import type { Asset } from '../types'

const props = defineProps<{ asset: Asset }>()
const emit = defineEmits<{ saved: [asset: Asset] }>()

const toast = useToast()
const container = ref<HTMLDivElement | null>(null)
const viewerEl = ref<HTMLDivElement | null>(null)
const page = ref(1)
const pages = ref(0)
const scale = ref(1)
const scrollMode = ref(0) // ScrollMode.VERTICAL
const spreadMode = ref(0) // SpreadMode.NONE
const findQuery = ref('')
const matchCurrent = ref(0)
const matchTotal = ref(0)
const findNotFound = ref(false)
const posterBusy = ref(false)

const zoomPercent = computed(() => Math.round(scale.value * 100))

let pdfViewer: PDFViewerInstance | null = null
let eventBus: EventBusInstance | null = null
let linkService: PDFLinkServiceInstance | null = null
let findController: PDFFindControllerInstance | null = null
let loadingTask: { destroy: () => Promise<void> } | null = null
let findTimer: number | null = null
let disposed = false

function contentUrl(): string {
  return api.cacheBust(api.assetContentUrl(props.asset.id))
}

async function mount(): Promise<void> {
  if (!container.value || !viewerEl.value) return
  const pdfjs = await import('pdfjs-dist')
  const workerUrl = (
    await import('pdfjs-dist/build/pdf.worker.min.mjs?url')
  ).default
  pdfjs.GlobalWorkerOptions.workerSrc = workerUrl
  // Loaded lazily so the viewer CSS/JS only ships when a PDF is opened.
  await import('pdfjs-dist/web/pdf_viewer.css')
  const viewer = await import('pdfjs-dist/web/pdf_viewer.mjs')
  if (disposed || !container.value || !viewerEl.value) return

  eventBus = new viewer.EventBus()
  linkService = new viewer.PDFLinkService({ eventBus })
  findController = new viewer.PDFFindController({ eventBus, linkService })
  pdfViewer = new viewer.PDFViewer({
    container: container.value,
    viewer: viewerEl.value,
    eventBus,
    linkService,
    findController,
  })
  linkService.setViewer(pdfViewer)

  const { FindState } = viewer
  eventBus.on('pagesinit', () => {
    // Fit the whole page on load so the entire first page is visible.
    if (pdfViewer) pdfViewer.currentScaleValue = 'page-fit'
    pages.value = pdfViewer?.pagesCount ?? 0
  })
  eventBus.on('pagechanging', (event: { pageNumber: number }) => {
    page.value = event.pageNumber
  })
  eventBus.on('scalechanging', (event: { scale: number }) => {
    scale.value = event.scale
  })
  eventBus.on(
    'updatefindmatchescount',
    (event: { matchesCount: { current: number; total: number } }) => {
      matchCurrent.value = event.matchesCount.current
      matchTotal.value = event.matchesCount.total
    },
  )
  eventBus.on(
    'updatefindcontrolstate',
    (event: {
      state: number
      matchesCount?: { current: number; total: number }
    }) => {
      findNotFound.value = event.state === FindState.NOT_FOUND
      // The current/total match count is delivered on every match navigation
      // through this event; updatefindmatchescount only fires during extraction.
      if (event.matchesCount) {
        matchCurrent.value = event.matchesCount.current
        matchTotal.value = event.matchesCount.total
      }
    },
  )

  const task = pdfjs.getDocument({ url: contentUrl() })
  const loaded = await task.promise
  if (disposed) {
    void task.destroy()
    return
  }
  loadingTask = task
  pdfViewer.setDocument(loaded)
  linkService.setDocument(loaded, null)
  pages.value = pdfViewer.pagesCount
}

function prevPage(): void {
  if (pdfViewer && pdfViewer.currentPageNumber > 1) {
    pdfViewer.currentPageNumber -= 1
  }
}

function nextPage(): void {
  if (pdfViewer && pdfViewer.currentPageNumber < pdfViewer.pagesCount) {
    pdfViewer.currentPageNumber += 1
  }
}

function onPageInput(event: Event): void {
  if (!pdfViewer) return
  const value = Number((event.target as HTMLInputElement).value)
  pdfViewer.currentPageNumber = Math.min(
    Math.max(1, Math.floor(value) || 1),
    pdfViewer.pagesCount,
  )
}

function zoomIn(): void {
  if (pdfViewer) pdfViewer.currentScale = Math.min(5, pdfViewer.currentScale * 1.15)
}

function zoomOut(): void {
  if (pdfViewer) pdfViewer.currentScale = Math.max(0.1, pdfViewer.currentScale / 1.15)
}

function fitWidth(): void {
  if (pdfViewer) pdfViewer.currentScaleValue = 'page-width'
}

function fitScreen(): void {
  if (pdfViewer) pdfViewer.currentScaleValue = 'page-fit'
}

function rotate(delta: number): void {
  if (pdfViewer) {
    pdfViewer.pagesRotation = (pdfViewer.pagesRotation + delta + 360) % 360
  }
}

function applyScroll(): void {
  if (pdfViewer) pdfViewer.scrollMode = scrollMode.value
}

function applySpread(): void {
  if (pdfViewer) pdfViewer.spreadMode = spreadMode.value
}

function dispatchFind(type: '' | 'again', findPrevious = false): void {
  eventBus?.dispatch('find', {
    source: null,
    type,
    query: findQuery.value,
    caseSensitive: false,
    entireWord: false,
    highlightAll: true,
    findPrevious,
    matchDiacritics: false,
  })
}

function onFindInput(): void {
  if (findTimer !== null) window.clearTimeout(findTimer)
  if (!findQuery.value) {
    clearFind()
    return
  }
  findTimer = window.setTimeout(() => {
    findTimer = null
    findNotFound.value = false
    dispatchFind('')
  }, 250)
}

function findNext(): void {
  if (findQuery.value) dispatchFind('again', false)
}

function findPrevious(): void {
  if (findQuery.value) dispatchFind('again', true)
}

function clearFind(): void {
  if (findTimer !== null) {
    window.clearTimeout(findTimer)
    findTimer = null
  }
  findQuery.value = ''
  matchCurrent.value = 0
  matchTotal.value = 0
  findNotFound.value = false
  eventBus?.dispatch('findbarclose', { source: null })
}

function onFindKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter') {
    event.preventDefault()
    if (event.shiftKey) findPrevious()
    else findNext()
  } else if (event.key === 'Escape') {
    event.preventDefault()
    clearFind()
  }
}

async function useCurrentPageAsPoster(): Promise<void> {
  if (posterBusy.value || !pages.value) return
  posterBusy.value = true
  try {
    const updated = await api.regeneratePreview(props.asset.id, {
      page: page.value,
    })
    emit('saved', updated)
    toast.add({
      severity: 'success',
      summary: `Poster set to page ${page.value}`,
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not update poster',
      detail: e instanceof Error ? e.message : String(e),
      life: 4000,
    })
  } finally {
    posterBusy.value = false
  }
}

function download(): void {
  const anchor = document.createElement('a')
  anchor.href = api.assetContentUrl(props.asset.id)
  anchor.download = props.asset.filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

// Printing uses the browser's own PDF printing via a hidden same-origin iframe.
function print(): void {
  const frame = document.createElement('iframe')
  frame.style.position = 'fixed'
  frame.style.width = '0'
  frame.style.height = '0'
  frame.style.border = '0'
  frame.src = contentUrl()
  frame.onload = () => {
    try {
      frame.contentWindow?.focus()
      frame.contentWindow?.print()
    } catch {
      // Ignore cross-origin/blocked printing; the user can use Download instead.
    }
  }
  document.body.appendChild(frame)
  window.setTimeout(() => frame.remove(), 60000)
}

function dispose(): void {
  disposed = true
  if (findTimer !== null) {
    window.clearTimeout(findTimer)
    findTimer = null
  }
  pdfViewer?.cleanup()
  if (loadingTask) {
    void loadingTask.destroy()
    loadingTask = null
  }
  pdfViewer = null
  eventBus = null
  linkService = null
  findController = null
}

onMounted(mount)
onBeforeUnmount(dispose)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div
      class="flex flex-wrap items-center gap-1 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-3 py-1.5 text-sm text-slate-600 dark:text-slate-300"
    >
      <button
        class="pdf-btn"
        :disabled="page <= 1"
        title="Previous page"
        @click="prevPage"
      >
        <ChevronLeft :size="16" />
      </button>
      <input
        class="w-12 rounded border border-slate-300 dark:border-slate-700 px-1 py-0.5 text-center text-xs tabular-nums"
        type="number"
        :value="page"
        min="1"
        :max="pages"
        @change="onPageInput"
      />
      <span class="text-xs tabular-nums text-slate-400">/ {{ pages }}</span>
      <button
        class="pdf-btn"
        :disabled="page >= pages"
        title="Next page"
        @click="nextPage"
      >
        <ChevronRight :size="16" />
      </button>

      <span class="pdf-sep" />

      <button class="pdf-btn" title="Zoom out" @click="zoomOut">
        <SearchMinus :size="16" />
      </button>
      <span class="w-12 text-center text-xs tabular-nums">{{ zoomPercent }}%</span>
      <button class="pdf-btn" title="Zoom in" @click="zoomIn">
        <SearchPlus :size="16" />
      </button>
      <button class="pdf-btn" title="Fit to screen" @click="fitScreen">
        <WindowMaximize :size="16" />
      </button>
      <button class="pdf-btn" title="Fit width" @click="fitWidth">
        <Expand :size="16" />
      </button>

      <span class="pdf-sep" />

      <button class="pdf-btn" title="Rotate left" @click="rotate(-90)">
        <Undo :size="16" />
      </button>
      <button class="pdf-btn" title="Rotate right" @click="rotate(90)">
        <Refresh :size="16" />
      </button>

      <span class="pdf-sep" />

      <select
        v-model.number="scrollMode"
        class="rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs"
        title="Scrolling mode"
        @change="applyScroll"
      >
        <option :value="0">Vertical</option>
        <option :value="1">Horizontal</option>
        <option :value="2">Wrapped</option>
      </select>
      <select
        v-model.number="spreadMode"
        class="rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs"
        title="Page layout"
        @change="applySpread"
      >
        <option :value="0">Single page</option>
        <option :value="1">Odd spreads</option>
        <option :value="2">Even spreads</option>
      </select>

      <span class="pdf-sep" />

      <div
        class="flex items-center gap-1 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5"
      >
        <Search :size="14" class="shrink-0 text-slate-400" />
        <input
          v-model="findQuery"
          type="text"
          placeholder="Search"
          class="w-28 bg-transparent text-xs text-slate-700 dark:text-slate-200 outline-none"
          @input="onFindInput"
          @keydown="onFindKeydown"
        />
        <span
          v-if="findQuery"
          class="min-w-9 text-center text-xs tabular-nums"
          :class="findNotFound ? 'text-red-500' : 'text-slate-400'"
        >
          {{ matchTotal ? `${matchCurrent}/${matchTotal}` : '0/0' }}
        </span>
        <button
          class="pdf-btn-sm"
          title="Previous match"
          :disabled="!findQuery"
          @click="findPrevious"
        >
          <ChevronUp :size="14" />
        </button>
        <button
          class="pdf-btn-sm"
          title="Next match"
          :disabled="!findQuery"
          @click="findNext"
        >
          <ChevronDown :size="14" />
        </button>
        <button
          v-if="findQuery"
          class="pdf-btn-sm"
          title="Clear search"
          @click="clearFind"
        >
          <Times :size="14" />
        </button>
      </div>

      <div class="ml-auto flex items-center gap-1">
        <button
          class="pdf-btn"
          :disabled="posterBusy || !pages"
          :title="`Use page ${page} as poster`"
          @click="useCurrentPageAsPoster"
        >
          <Camera :size="16" />
        </button>
        <button class="pdf-btn" title="Download" @click="download">
          <Download :size="16" />
        </button>
        <button class="pdf-btn" title="Print" @click="print">
          <Print :size="16" />
        </button>
      </div>
    </div>

    <div class="relative min-h-0 flex-1">
      <div ref="container" class="absolute inset-0 overflow-auto bg-slate-200 dark:bg-slate-700">
        <div ref="viewerEl" class="pdfViewer" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.pdf-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 1.75rem;
  width: 1.75rem;
  border-radius: 0.375rem;
  color: #475569;
}
.pdf-btn:hover:not(:disabled) {
  background-color: #f1f5f9;
  color: #0f172a;
}
.pdf-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.pdf-btn-sm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 1.25rem;
  width: 1.25rem;
  border-radius: 0.25rem;
  color: #64748b;
}
.pdf-btn-sm:hover:not(:disabled) {
  background-color: #f1f5f9;
  color: #0f172a;
}
.pdf-btn-sm:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.pdf-sep {
  width: 1px;
  height: 1.25rem;
  background-color: #e2e8f0;
  margin: 0 0.25rem;
}
</style>

:global(.dark) .pdf-btn {
  color: #cbd5e1;
}

:global(.dark) .pdf-btn:hover:not(:disabled) {
  background-color: #1e293b;
  color: #f8fafc;
}

:global(.dark) .pdf-btn-sm {
  color: #94a3b8;
}

:global(.dark) .pdf-btn-sm:hover:not(:disabled) {
  background-color: #1e293b;
  color: #f8fafc;
}

:global(.dark) .pdf-sep {
  background-color: #334155;
}
