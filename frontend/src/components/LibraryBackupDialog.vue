<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import * as api from '../api/client'
import { backupSettingsDialog } from '../lib/backupSettings'
import type { LibraryBackup, LibraryBackupStatus } from '../types'

const toast = useToast()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const running = ref(false)
const hasPassword = ref(false)
const clearPassword = ref(false)
const status = ref<LibraryBackupStatus | null>(null)

const form = reactive({
  enabled: false,
  host: '',
  port: 22,
  user: '',
  password: '',
  keyPath: '',
  basePath: '',
  hostKeyFingerprint: '',
})

let poll: number | null = null

function toMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function applySettings(settings: LibraryBackup): void {
  form.enabled = settings.enabled
  form.host = settings.host
  form.port = settings.port || 22
  form.user = settings.user
  form.keyPath = settings.key_path
  form.basePath = settings.base_path
  form.hostKeyFingerprint = settings.host_key_fingerprint
  form.password = ''
  hasPassword.value = settings.has_password
  clearPassword.value = false
}

function payload(): api.LibraryBackupUpdate {
  return {
    enabled: form.enabled,
    driver: 'sftp',
    host: form.host.trim(),
    port: Math.max(0, Math.round(Number(form.port) || 0)),
    user: form.user.trim(),
    password: form.password,
    clear_password: clearPassword.value,
    key_path: form.keyPath.trim(),
    base_path: form.basePath.trim(),
    host_key_fingerprint: form.hostKeyFingerprint.trim(),
  }
}

async function loadStatus(): Promise<void> {
  const id = backupSettingsDialog.libraryId
  if (!id) return
  try {
    status.value = await api.getLibraryBackupStatus(id)
  } catch {
    // The status panel is best-effort; saving/loading reports real errors.
  }
}

async function load(): Promise<void> {
  const id = backupSettingsDialog.libraryId
  if (!id) return
  loading.value = true
  try {
    const [settings] = await Promise.all([
      api.getLibraryBackup(id),
      loadStatus(),
    ])
    applySettings(settings)
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not load backup settings',
      detail: toMessage(e),
      life: 4000,
    })
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  const id = backupSettingsDialog.libraryId
  if (!id) return
  saving.value = true
  try {
    const updated = await api.setLibraryBackup(id, payload())
    applySettings(updated)
    toast.add({
      severity: 'success',
      summary: 'Backup settings saved',
      life: 2500,
    })
    await loadStatus()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not save backup settings',
      detail: toMessage(e),
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function test(): Promise<void> {
  const id = backupSettingsDialog.libraryId
  if (!id) return
  testing.value = true
  try {
    await api.testLibraryBackup(id, payload())
    toast.add({
      severity: 'success',
      summary: 'SFTP connection successful',
      life: 2500,
    })
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'SFTP connection failed',
      detail: toMessage(e),
      life: 6000,
    })
  } finally {
    testing.value = false
  }
}

async function backupNow(): Promise<void> {
  const id = backupSettingsDialog.libraryId
  if (!id) return
  running.value = true
  try {
    const result = await api.runLibraryBackup(id)
    toast.add({
      severity: 'success',
      summary: `Queued ${result.queued} asset(s)`,
      detail: 'Backups run in the background.',
      life: 3000,
    })
    await loadStatus()
  } catch (e) {
    toast.add({
      severity: 'error',
      summary: 'Could not start backup',
      detail: toMessage(e),
      life: 5000,
    })
  } finally {
    running.value = false
  }
}

function startPolling(): void {
  stopPolling()
  poll = window.setInterval(() => void loadStatus(), 3000)
}

function stopPolling(): void {
  if (poll !== null) {
    window.clearInterval(poll)
    poll = null
  }
}

watch(
  () => backupSettingsDialog.visible,
  (visible) => {
    if (visible) {
      void load()
      startPolling()
    } else {
      stopPolling()
    }
  },
)

onBeforeUnmount(stopPolling)
</script>

<template>
  <Dialog
    v-model:visible="backupSettingsDialog.visible"
    header="Storage & backup"
    modal
    :draggable="false"
    :style="{ width: '42rem' }"
  >
    <div class="space-y-5">
      <div
        class="flex items-center justify-between gap-4 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 dark:border-slate-800 dark:bg-slate-900/60"
      >
        <div>
          <p class="text-sm font-medium text-slate-700 dark:text-slate-200">
            Back up originals to SFTP
          </p>
          <p class="mt-0.5 text-xs text-slate-400">
            Previews stay local. Only original files are copied.
          </p>
        </div>
        <ToggleSwitch v-model="form.enabled" />
      </div>

      <div class="grid grid-cols-2 gap-3">
        <label class="col-span-2 field-label">
          Host
          <input
            v-model="form.host"
            class="field-input"
            placeholder="backup.example.com"
          />
        </label>
        <label class="field-label">
          Port
          <input
            v-model.number="form.port"
            class="field-input"
            type="number"
            min="1"
            max="65535"
          />
        </label>
        <label class="field-label">
          User
          <input v-model="form.user" class="field-input" placeholder="backup" />
        </label>
        <label class="field-label">
          Password
          <input
            v-model="form.password"
            class="field-input"
            type="password"
            autocomplete="new-password"
            :placeholder="
              hasPassword ? 'Stored — leave blank to keep' : 'Password'
            "
          />
        </label>
        <label class="field-label">
          SSH key path
          <input
            v-model="form.keyPath"
            class="field-input"
            placeholder="~/.ssh/id_ed25519"
          />
        </label>
        <label class="col-span-2 field-label">
          Base path
          <input
            v-model="form.basePath"
            class="field-input"
            placeholder="/backups/godam"
          />
        </label>
        <label class="col-span-2 field-label">
          Host key fingerprint
          <input
            v-model="form.hostKeyFingerprint"
            class="field-input"
            placeholder="SHA256:… (optional)"
          />
        </label>
        <label
          v-if="hasPassword"
          class="col-span-2 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400"
        >
          <input v-model="clearPassword" type="checkbox" />
          Clear the stored password
        </label>
      </div>

      <div
        v-if="status"
        class="rounded-lg border border-slate-200 px-3 py-2 dark:border-slate-800"
      >
        <div class="flex items-center justify-between gap-3">
          <p class="text-xs font-semibold tracking-wide text-slate-400 uppercase">
            Status
          </p>
          <Button
            label="Refresh"
            severity="secondary"
            size="small"
            text
            @click="loadStatus"
          />
        </div>
        <div class="mt-2 grid grid-cols-4 gap-2 text-center">
          <div>
            <p class="text-lg font-semibold text-slate-800 dark:text-slate-100">
              {{ status.total }}
            </p>
            <p class="text-xs text-slate-400">Total</p>
          </div>
          <div>
            <p class="text-lg font-semibold text-emerald-600 dark:text-emerald-400">
              {{ status.synced }}
            </p>
            <p class="text-xs text-slate-400">Synced</p>
          </div>
          <div>
            <p class="text-lg font-semibold text-amber-600 dark:text-amber-400">
              {{ status.pending }}
            </p>
            <p class="text-xs text-slate-400">Pending</p>
          </div>
          <div>
            <p class="text-lg font-semibold text-red-600 dark:text-red-400">
              {{ status.failed }}
            </p>
            <p class="text-xs text-slate-400">Failed</p>
          </div>
        </div>
        <p
          v-if="status.last_error"
          class="mt-2 rounded bg-red-50 px-2 py-1 text-xs text-red-700 dark:bg-red-950/40 dark:text-red-300"
        >
          {{ status.last_error }}
        </p>
        <p v-if="status.last_run_at" class="mt-2 text-xs text-slate-400">
          Last successful run: {{ new Date(status.last_run_at).toLocaleString() }}
        </p>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <Button
          label="Test connection"
          severity="secondary"
          :loading="testing"
          @click="test"
        />
        <Button
          label="Back up now"
          severity="secondary"
          :loading="running"
          :disabled="!status?.enabled"
          @click="backupNow"
        />
        <Button
          label="Save"
          class="ml-auto"
          :loading="saving"
          @click="save"
        />
      </div>
    </div>
  </Dialog>
</template>

<style scoped>
.field-label {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: #475569;
}

:global(.dark) .field-label {
  color: #cbd5e1;
}

.field-input {
  width: 100%;
  padding: 0.5rem 0.6rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.4rem;
  background: #fff;
  color: #1e293b;
  font: inherit;
  font-size: 0.85rem;
  font-weight: 400;
}

.field-input:focus {
  border-color: #a78bfa;
  outline: 2px solid #ddd6fe;
  outline-offset: 0;
}

:global(.dark) .field-input {
  border-color: #334155;
  background: #0f172a;
  color: #f1f5f9;
}

:global(.dark) .field-input:focus {
  border-color: #7c3aed;
  outline-color: #4c1d95;
}
</style>
