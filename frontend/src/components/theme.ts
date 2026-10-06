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

// Set once the user clicks the toggle in this page, so a failing write still stops following the system.
let chosenInPage = false

function hasSavedTheme(): boolean {
  try {
    const saved = window.localStorage.getItem(themeStorageKey)
    return saved === 'light' || saved === 'dark'
  } catch {
    return false
  }
}

export function followsSystemTheme(): boolean {
  return !chosenInPage && !hasSavedTheme()
}

// Calls back with the new system theme, but only while the user has not chosen one.
export function subscribeToSystemTheme(callback: (theme: Theme) => void): () => void {
  const query = window.matchMedia?.('(prefers-color-scheme: dark)')
  if (!query?.addEventListener) return () => {}
  const listener = (event: { matches: boolean }) => {
    if (followsSystemTheme()) callback(event.matches ? 'dark' : 'light')
  }
  query.addEventListener('change', listener)
  return () => query.removeEventListener('change', listener)
}

export function saveTheme(theme: Theme) {
  chosenInPage = true
  try { window.localStorage.setItem(themeStorageKey, theme) } catch { /* the choice lasts for this page */ }
}

export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}

let stopFollowingSystem = () => {}

export function initializeTheme() {
  chosenInPage = false
  applyTheme(resolveTheme())
  stopFollowingSystem()
  stopFollowingSystem = subscribeToSystemTheme(applyTheme)
}

// Test-only: drops the module-level state so a suite does not depend on the order of the tests before it.
export function resetThemeForTests() {
  chosenInPage = false
  stopFollowingSystem()
  stopFollowingSystem = () => {}
}
