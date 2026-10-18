<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { Check } from '@primeicons/vue'
import { ref } from 'vue'
import * as api from '../api/client'
import { loadLibraries, selectLibrary } from '../lib/library'
import {
  DEFAULT_LIBRARY_COLOR,
  LIBRARY_COLORS,
  libraryColorHex,
} from '../lib/libraryColors'

const name = ref('')
const color = ref(DEFAULT_LIBRARY_COLOR)
const busy = ref(false)
const error = ref<string | null>(null)

async function create(): Promise<void> {
  if (!name.value.trim() || busy.value) return
  busy.value = true
  error.value = null
  try {
    const lib = await api.createLibrary(name.value.trim(), color.value, 0)
    await loadLibraries()
    selectLibrary(lib.id)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex h-full w-full items-center justify-center bg-slate-50 dark:bg-slate-950 p-6">
    <div class="w-full max-w-md rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-8 shadow-sm">
      <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">Create your first library</h1>
      <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
        A library is a self-contained scope for your assets, tags, collections
        and tag groups. You can create more later and switch between them.
      </p>

      <div class="mt-6 space-y-4">
        <div>
          <label class="mb-2 block text-sm font-medium">Name</label>
          <InputText
            v-model="name"
            placeholder="e.g. Personal"
            class="w-full"
            :disabled="busy"
            @keyup.enter="create"
          />
        </div>
        <div>
          <label class="mb-2 block text-sm font-medium">Color</label>
          <div class="grid grid-cols-10 gap-1.5">
            <button
              v-for="option in LIBRARY_COLORS"
              :key="option.value"
              type="button"
              class="flex h-7 w-7 items-center justify-center rounded-full ring-offset-2"
              :class="color === option.value ? 'ring-2 ring-slate-400' : ''"
              :style="{ backgroundColor: libraryColorHex(option.value) }"
              :title="option.name"
              @click="color = option.value"
            >
              <Check v-if="color === option.value" :size="14" class="text-white" />
            </button>
          </div>
        </div>
      </div>

      <p v-if="error" class="mt-4 text-sm text-red-600 dark:text-red-400">{{ error }}</p>

      <Button
        label="Create library"
        class="mt-6 w-full"
        :loading="busy"
        :disabled="!name.trim()"
        @click="create"
      />
    </div>
  </div>
</template>
