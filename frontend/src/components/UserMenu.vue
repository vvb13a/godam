<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Popover from 'primevue/popover'
import SelectButton from 'primevue/selectbutton'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'
import { ChevronRight, SignOut, SlidersH, UserEdit } from '@primeicons/vue'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { auth, changePassword, logout, updateProfile } from '../lib/auth'
import { openCommandMenu } from '../lib/command'
import { SIDEBAR_TOGGLES, sidebarPrefs } from '../lib/sidebar'
import {
  THEME_OPTIONS,
  setThemeMode,
  theme,
  type ThemeMode,
} from '../lib/theme'
import Kbd from './Kbd.vue'

const router = useRouter()
const toast = useToast()

const menu = ref<InstanceType<typeof Popover> | null>(null)
const displayMenu = ref<InstanceType<typeof Popover> | null>(null)

const themeMode = computed<ThemeMode>({
  get: () => theme.mode,
  set: (mode) => setThemeMode(mode),
})

const dialogVisible = ref(false)
const name = ref('')
const email = ref('')
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const busy = ref(false)
const error = ref<string | null>(null)

const initial = computed(() =>
  (auth.user?.name?.trim()?.[0] ?? '?').toUpperCase(),
)

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function toggle(event: Event): void {
  menu.value?.toggle(event)
}

function openDisplayMenu(event: Event): void {
  menu.value?.hide()
  displayMenu.value?.toggle(event)
}

function open(): void {
  name.value = auth.user?.name ?? ''
  email.value = auth.user?.email ?? ''
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  error.value = null
  menu.value?.hide()
  dialogVisible.value = true
}

async function save(): Promise<void> {
  if (busy.value) return
  const nextName = name.value.trim()
  const nextEmail = email.value.trim()
  if (!nextName || !nextEmail) {
    error.value = 'Name and email are required'
    return
  }
  const wantsPassword =
    !!currentPassword.value || !!newPassword.value || !!confirmPassword.value
  if (wantsPassword) {
    if (newPassword.value.length < 8) {
      error.value = 'New password must be at least 8 characters'
      return
    }
    if (newPassword.value !== confirmPassword.value) {
      error.value = 'Passwords do not match'
      return
    }
  }

  error.value = null
  busy.value = true
  try {
    await updateProfile(nextName, nextEmail)
    if (wantsPassword) {
      await changePassword(currentPassword.value, newPassword.value)
    }
    toast.add({ severity: 'success', summary: 'Account updated', life: 2000 })
    dialogVisible.value = false
  } catch (e) {
    error.value = toMessage(e)
  } finally {
    busy.value = false
  }
}

async function doLogout(): Promise<void> {
  menu.value?.hide()
  await logout()
  await router.push({ name: 'login' })
}
</script>

<template>
  <div class="border-t border-slate-200 dark:border-slate-800 p-2">
    <div class="flex items-center gap-2 rounded px-2 py-2 hover:bg-slate-100 dark:hover:bg-slate-800">
      <button
        type="button"
        class="flex min-w-0 flex-1 items-center gap-2 text-left"
        title="Account"
        @click="toggle"
      >
        <span
          class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-violet-100 dark:bg-violet-900/50 text-xs font-semibold text-violet-700 dark:text-violet-300"
        >
          {{ initial }}
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm font-medium text-slate-700 dark:text-slate-200">
            {{ auth.user?.name }}
          </span>
          <span class="block truncate text-xs text-slate-400">
            {{ auth.user?.email }}
          </span>
        </span>
      </button>
      <button
        type="button"
        class="shrink-0"
        title="Open command menu"
        @click="openCommandMenu"
      >
        <Kbd>⌘K</Kbd>
      </button>
    </div>

    <Popover ref="menu">
      <div class="w-64">
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
          @click="open"
        >
          <UserEdit :size="14" />
          <span>Account settings</span>
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
          @click="openDisplayMenu"
        >
          <SlidersH :size="14" />
          <span class="flex-1 text-left">Panel display</span>
          <ChevronRight :size="12" class="shrink-0 text-slate-400" />
        </button>
        <div class="my-1 border-t border-slate-100 dark:border-slate-800" />
        <div class="px-2 py-1">
          <p
            class="mb-1.5 text-xs font-semibold tracking-wide text-slate-400 uppercase"
          >
            Theme
          </p>
          <SelectButton
            v-model="themeMode"
            :options="THEME_OPTIONS"
            option-label="label"
            option-value="value"
            :allow-empty="false"
            size="small"
            class="w-full"
          />
        </div>
        <div class="my-1 border-t border-slate-100 dark:border-slate-800" />
        <button
          type="button"
          class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/40"
          @click="doLogout"
        >
          <SignOut :size="14" />
          <span>Sign out</span>
        </button>
      </div>
    </Popover>

    <Popover ref="displayMenu">
      <div class="w-56">
        <p
          class="px-2 pt-1 pb-2 text-xs font-semibold tracking-wide text-slate-400 uppercase"
        >
          Panels
        </p>
        <div
          v-for="toggle in SIDEBAR_TOGGLES"
          :key="toggle.key"
          class="flex items-center justify-between gap-3 rounded px-2 py-1.5"
        >
          <span class="text-sm text-slate-600 dark:text-slate-300">{{ toggle.label }}</span>
          <ToggleSwitch v-model="sidebarPrefs[toggle.key]" />
        </div>
      </div>
    </Popover>

    <Dialog
      v-model:visible="dialogVisible"
      modal
      header="Account settings"
      :style="{ width: '26rem' }"
    >
      <div class="space-y-4">
        <div>
          <label class="mb-1.5 block text-sm font-medium text-slate-700 dark:text-slate-200">Name</label>
          <InputText v-model="name" placeholder="Name" class="w-full" />
        </div>
        <div>
          <label class="mb-1.5 block text-sm font-medium text-slate-700 dark:text-slate-200">Email</label>
          <InputText v-model="email" type="email" placeholder="Email" class="w-full" />
        </div>

        <div class="border-t border-slate-100 dark:border-slate-800 pt-4">
          <p class="mb-1 text-sm font-medium text-slate-700 dark:text-slate-200">Change password</p>
          <p class="mb-3 text-xs text-slate-400">
            Leave these blank to keep your current password.
          </p>
          <div class="space-y-3">
            <InputText
              v-model="currentPassword"
              type="password"
              placeholder="Current password"
              class="w-full"
              autocomplete="current-password"
            />
            <InputText
              v-model="newPassword"
              type="password"
              placeholder="New password"
              class="w-full"
              autocomplete="new-password"
            />
            <InputText
              v-model="confirmPassword"
              type="password"
              placeholder="Confirm new password"
              class="w-full"
              autocomplete="new-password"
              @keyup.enter="save"
            />
          </div>
        </div>

        <p v-if="error" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      </div>
      <template #footer>
        <Button label="Cancel" text :disabled="busy" @click="dialogVisible = false" />
        <Button label="Save" :loading="busy" @click="save" />
      </template>
    </Dialog>
  </div>
</template>
