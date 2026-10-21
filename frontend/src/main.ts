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

// Indigo primary in light mode, orange in dark. Scale entries are mode-aware;
// the light 500/600/700 steps are shifted darker for contrast on white.
const GodamPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: 'light-dark({indigo.50}, {orange.50})',
      100: 'light-dark({indigo.100}, {orange.100})',
      200: 'light-dark({indigo.200}, {orange.200})',
      300: 'light-dark({indigo.300}, {orange.300})',
      400: 'light-dark({indigo.400}, {orange.400})',
      500: 'light-dark({indigo.600}, {orange.500})',
      600: 'light-dark({indigo.700}, {orange.600})',
      700: 'light-dark({indigo.800}, {orange.700})',
      800: 'light-dark({indigo.900}, {orange.800})',
      900: 'light-dark({indigo.950}, {orange.900})',
      950: 'light-dark({indigo.950}, {orange.950})',
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
