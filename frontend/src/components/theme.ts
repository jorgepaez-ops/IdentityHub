export type Theme = 'light' | 'dark'

export const themeStorageKey = 'identityhub.theme'

export function preferredTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

// Storage can throw (disabled storage, some privacy modes): the theme is a preference, so fall back to
// the system setting instead of failing to render.
export function resolveTheme(): Theme {
  let saved: string | null = null
  try { saved = window.localStorage.getItem(themeStorageKey) } catch { /* use the system preference */ }
  return saved === 'light' || saved === 'dark' ? saved : preferredTheme()
}

export function saveTheme(theme: Theme) {
  try { window.localStorage.setItem(themeStorageKey, theme) } catch { /* the choice lasts for this page */ }
}

export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}

export function initializeTheme() {
  applyTheme(resolveTheme())
}
