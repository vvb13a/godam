// Filerobot Image Editor ships an indigo light palette. Its ThemeProvider
// merges these flat palette overrides over the defaults. Both palettes carry
// the same keys so switching modes can fully replace the previous overrides.

export const lightImageEditorTheme = {
  palette: {
    // Surfaces
    'bg-secondary': '#ffffff',
    'bg-primary': '#f3f5f7',
    'bg-primary-light': '#f3f5f7',
    'bg-primary-hover': '#eff3f6',
    'bg-primary-active': '#e0e7ff',
    'bg-primary-stateless': '#ccd6de',
    'bg-primary-0-5-opacity': 'rgba(255, 255, 255, 0.5)',
    'bg-stateless': '#ffffff',
    'bg-active': '#f3f7fa',
    'bg-hover': '#f9fbfc',
    'bg-grey': '#d8e0e8',
    'bg-base-light': '#ebeffc',
    'bg-base-medium': '#e3e7fb',
    'bg-tooltip': '#4f6276',
    'active-secondary': '#ffffff',
    'active-secondary-hover': 'rgba(79, 70, 229, 0.06)',

    // Text
    'txt-primary': '#37414b',
    'txt-secondary': '#768a9f',
    'txt-secondary-invert': '#d5d8dc',
    'txt-placeholder': '#a9b6c2',
    'txt-warning': '#c46e00',
    'txt-error': '#b9212e',
    'txt-info': '#2375ab',

    // Icons
    'icon-primary': '#768a9f',
    'icons-primary-opacity-0-6': 'rgba(93, 109, 126, 0.6)',
    'icons-secondary': '#97a6b6',
    'icons-placeholder': '#dfe7ed',
    'icons-invert': '#ffffff',
    'icons-muted': '#a9b6c2',
    'icons-primary-hover': '#4f6276',
    'icons-secondary-hover': '#768a9f',

    // Borders
    'borders-primary': '#ccd6de',
    'borders-primary-hover': '#687686',
    'borders-secondary': '#ecf0f3',
    'borders-strong': '#ccd6de',
    'borders-invert': '#556373',
    'border-primary-stateless': '#ccd6de',
    'borders-item': '#e2e8ec',
    'borders-base-light': '#d3dbfa',
    'borders-base-medium': '#a4afe3',
    'borders-button': '#97a6b6',

    // Links and buttons
    'link-primary': '#4338ca',
    'link-stateless': '#4338ca',
    'link-hover': '#3730a3',
    'link-active': '#4f46e5',
    'link-pressed': '#4338ca',
    'link-muted': '#a9b6c2',
    'btn-secondary-text': '#000000',

    // Accent (indigo)
    'accent-primary': '#4f46e5',
    'accent-primary-hover': '#4338ca',
    'accent-primary-active': '#4338ca',
    'accent-primary-disabled': '#e9eef2',
    'accent-secondary-disabled': '#f9fbfc',
    'accent-stateless': '#4f46e5',
    'accent-stateless_0_4_opacity': 'rgba(79, 70, 229, 0.4)',
    'accent_0_5_5_opacity': 'rgba(67, 56, 202, 0.55)',
    'accent_0_7_opacity': 'rgba(67, 56, 202, 0.7)',
    'accent_0_5_opacity': 'rgba(79, 70, 229, 0.05)',
    'accent_1_2_opacity': 'rgba(79, 70, 229, 0.12)',
    'accent_1_8_opacity': 'rgba(79, 70, 229, 0.18)',
    'accent_2_8_opacity': 'rgba(79, 70, 229, 0.28)',
    'accent_4_0_opacity': 'rgba(79, 70, 229, 0.4)',
    modified: '#4f46e5',

    // Tinted status surfaces
    'bg-red': '#fdf4f2',
    'bg-red-light': '#fff5f5',
    'background-red-medium': '#f7e4e6',
    'bg-green': '#edfaf4',
    'bg-green-medium': '#e3faef',
    'bg-blue': '#f4f9ff',
    'bg-orange': '#fff9f2',

    // Edge gradients used by scrollable rows
    'gradient-right':
      'linear-gradient(270deg, #ffffff 1.56%, rgba(255, 255, 255, 0.89) 52.4%, rgba(255, 255, 255, 0.532165) 76.04%, rgba(255, 255, 255, 0) 100%)',
    'gradient-right-active':
      'linear-gradient(270deg, #f3f7fa 1.56%, #f3f7fa 52.4%, rgba(243, 247, 250, 0.53) 76.04%, rgba(243, 247, 250, 0) 100%)',
    'gradient-right-hover':
      'linear-gradient(270deg, #f9fbfc 1.56%, #f9fbfc 52.4%, rgba(249, 251, 252, 0.53) 76.04%, rgba(249, 251, 252, 0) 100%)',
  },
}

export const darkImageEditorTheme = {
  palette: {
    // Surfaces
    'bg-secondary': '#0f172a',
    'bg-primary': '#1e293b',
    'bg-primary-light': '#1e293b',
    'bg-primary-hover': '#334155',
    'bg-primary-active': '#475569',
    'bg-primary-stateless': '#334155',
    'bg-primary-0-5-opacity': 'rgba(15, 23, 42, 0.5)',
    'bg-stateless': '#0f172a',
    'bg-active': '#334155',
    'bg-hover': '#1e293b',
    'bg-grey': '#334155',
    'bg-base-light': '#1e293b',
    'bg-base-medium': '#334155',
    'bg-tooltip': '#334155',
    'active-secondary': '#1e293b',
    'active-secondary-hover': '#334155',

    // Text
    'txt-primary': '#e2e8f0',
    'txt-secondary': '#94a3b8',
    'txt-secondary-invert': '#0f172a',
    'txt-placeholder': '#64748b',
    'txt-warning': '#fb923c',
    'txt-error': '#f87171',
    'txt-info': '#60a5fa',

    // Icons
    'icon-primary': '#94a3b8',
    'icons-primary-opacity-0-6': 'rgba(148, 163, 184, 0.6)',
    'icons-secondary': '#64748b',
    'icons-placeholder': '#475569',
    'icons-invert': '#f8fafc',
    'icons-muted': '#64748b',
    'icons-primary-hover': '#e2e8f0',
    'icons-secondary-hover': '#94a3b8',

    // Borders
    'borders-primary': '#334155',
    'borders-primary-hover': '#475569',
    'borders-secondary': '#1e293b',
    'borders-strong': '#475569',
    'borders-invert': '#64748b',
    'border-primary-stateless': '#334155',
    'borders-item': '#334155',
    'borders-base-light': '#1e293b',
    'borders-base-medium': '#334155',
    'borders-button': '#475569',

    // Links and buttons
    'link-primary': '#fb923c',
    'link-stateless': '#fb923c',
    'link-hover': '#fdba74',
    'link-active': '#fb923c',
    'link-pressed': '#f97316',
    'link-muted': '#ea580c',
    'btn-secondary-text': '#e2e8f0',

    // Accent (orange)
    'accent-primary': '#f97316',
    'accent-primary-hover': '#fb923c',
    'accent-primary-active': '#fb923c',
    'accent-primary-disabled': '#475569',
    'accent-secondary-disabled': '#1e293b',
    'accent-stateless': '#fb923c',
    'accent-stateless_0_4_opacity': 'rgba(251, 146, 60, 0.4)',
    'accent_0_5_5_opacity': 'rgba(249, 115, 22, 0.55)',
    'accent_0_7_opacity': 'rgba(249, 115, 22, 0.7)',
    'accent_0_5_opacity': 'rgba(251, 146, 60, 0.05)',
    'accent_1_2_opacity': 'rgba(251, 146, 60, 0.12)',
    'accent_1_8_opacity': 'rgba(251, 146, 60, 0.18)',
    'accent_2_8_opacity': 'rgba(251, 146, 60, 0.28)',
    'accent_4_0_opacity': 'rgba(251, 146, 60, 0.4)',
    modified: '#fb923c',

    // Tinted status surfaces
    'bg-red': 'rgba(127, 29, 29, 0.35)',
    'bg-red-light': 'rgba(127, 29, 29, 0.25)',
    'background-red-medium': 'rgba(127, 29, 29, 0.5)',
    'bg-green': 'rgba(6, 78, 59, 0.35)',
    'bg-green-medium': 'rgba(6, 78, 59, 0.5)',
    'bg-blue': 'rgba(30, 58, 138, 0.35)',
    'bg-orange': 'rgba(120, 53, 15, 0.35)',

    // Edge gradients used by scrollable rows
    'gradient-right':
      'linear-gradient(270deg, #0f172a 1.56%, rgba(15, 23, 42, 0.89) 52.4%, rgba(15, 23, 42, 0.53) 76.04%, rgba(15, 23, 42, 0) 100%)',
    'gradient-right-active':
      'linear-gradient(270deg, #1e293b 1.56%, #1e293b 52.4%, rgba(30, 41, 59, 0.53) 76.04%, rgba(30, 41, 59, 0) 100%)',
    'gradient-right-hover':
      'linear-gradient(270deg, #1e293b 1.56%, #1e293b 52.4%, rgba(30, 41, 59, 0.53) 76.04%, rgba(30, 41, 59, 0) 100%)',
  },
}
