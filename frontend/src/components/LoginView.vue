<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { Database } from '@primeicons/vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { devLogin, login } from '../lib/auth'

const route = useRoute()
const router = useRouter()

const email = ref('')
const password = ref('')
const busy = ref(false)
const error = ref<string | null>(null)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function destination(): string {
  const redirect = route.query.redirect
  return typeof redirect === 'string' && redirect ? redirect : '/all'
}

async function submit(): Promise<void> {
  if (busy.value || !email.value.trim() || !password.value) return
  busy.value = true
  error.value = null
  try {
    await login(email.value.trim(), password.value)
    await router.push(destination())
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}

async function useDeveloper(): Promise<void> {
  if (busy.value) return
  busy.value = true
  error.value = null
  try {
    await devLogin()
    await router.push(destination())
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex h-full w-full items-center justify-center bg-slate-50 dark:bg-slate-950 p-6">
    <div class="w-full max-w-sm rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-8 shadow-sm">
      <div class="mb-6 flex items-center gap-2">
        <Database :size="24" class="text-brand-700 dark:text-brand-400" />
        <h1 class="text-xl font-semibold text-slate-800 dark:text-slate-100">GoDam</h1>
      </div>

      <form class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="mb-1.5 block text-sm font-medium text-slate-700 dark:text-slate-200">Email</label>
          <InputText
            v-model="email"
            type="email"
            placeholder="you@example.com"
            class="w-full"
            :disabled="busy"
            autocomplete="username"
          />
        </div>
        <div>
          <label class="mb-1.5 block text-sm font-medium text-slate-700 dark:text-slate-200">Password</label>
          <InputText
            v-model="password"
            type="password"
            placeholder="••••••••"
            class="w-full"
            :disabled="busy"
            autocomplete="current-password"
          />
        </div>

        <p v-if="error" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>

        <Button
          type="submit"
          label="Sign in"
          class="w-full"
          :loading="busy"
          :disabled="!email.trim() || !password"
        />
      </form>

      <div class="my-5 flex items-center gap-3 text-xs text-slate-400">
        <span class="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
        <span>or</span>
        <span class="h-px flex-1 bg-slate-200 dark:bg-slate-700" />
      </div>

      <Button
        label="Continue as developer"
        severity="secondary"
        class="w-full"
        :disabled="busy"
        @click="useDeveloper"
      >
        <template #icon><Database :size="16" /></template>
      </Button>
    </div>
  </div>
</template>
