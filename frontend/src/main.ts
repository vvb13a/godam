import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import Tooltip from 'primevue/tooltip'
import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { router } from './router'
import { initTheme } from './lib/theme'

// Violet primary, matching the accent the former TUI used.
const GodamPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '{violet.50}',
      100: '{violet.100}',
      200: '{violet.200}',
      300: '{violet.300}',
      400: '{violet.400}',
      500: '{violet.500}',
      600: '{violet.600}',
      700: '{violet.700}',
      800: '{violet.800}',
      900: '{violet.900}',
      950: '{violet.950}',
    },
  },
})

initTheme()

const app = createApp(App)

app.use(PrimeVue, {
  theme: {
    preset: GodamPreset,
    options: {
      // Class-based dark mode so a manual toggle can drive it.
      darkModeSelector: '.p-dark',
    },
  },
  license: 'eyJpZCI6IjgxODMxNzVhLWI0NjUtNDQzYi1hMjgwLWU2ZTQzZjQ2NTE3NiIsInByb2R1Y3QiOiJwcmltZXVpIiwidGllciI6ImNvbW11bml0eSIsInR5cGUiOiJkZXYiLCJpYXQiOjE3OTA4NzUyMjQsImV4cCI6MTgyMjQxMTIyNH0.1RtAOl9Ww3-aE0yEH5e5r9qh7GVP-t1-eEaCcSpPRZUYbgszfwmC4gR5Om7J7Qd362yctgPQEgfTcLNnd6QPCA',
})

app.use(router)
app.use(ToastService)
app.use(ConfirmationService)
app.directive('tooltip', Tooltip)

// Wait for the initial route to resolve before mounting so a deep link is not
// redirected to the default view on load.
router.isReady().then(() => app.mount('#app'))
