export type Theme = 'light' | 'dark'

export const themeStorageKey = 'identityhub.theme'

export function preferredTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function resolveTheme(): Theme {
  const saved = window.localStorage.getItem(themeStorageKey)
  return saved === 'light' || saved === 'dark' ? saved : preferredTheme()
}

export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}

export function initializeTheme() {
  applyTheme(resolveTheme())
}
