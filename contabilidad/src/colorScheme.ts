// Pure helpers behind the appearance (light/dark) preference of Contabilidad.

export type Appearance = 'light' | 'dark'

export const APPEARANCE_KEY = 'contabilidad.theme'

const DARK_QUERY = '(prefers-color-scheme: dark)'

export const systemQuery = (): MediaQueryList | undefined => window.matchMedia?.(DARK_QUERY)

export const systemAppearance = (): Appearance => (systemQuery()?.matches ? 'dark' : 'light')

const isAppearance = (value: unknown): value is Appearance => value === 'light' || value === 'dark'

/** The stored choice, or undefined when there is none or storage is unavailable. */
export function storedAppearance(): Appearance | undefined {
  try {
    const value = window.localStorage.getItem(APPEARANCE_KEY)
    return isAppearance(value) ? value : undefined
  } catch {
    return undefined
  }
}

export function rememberAppearance(choice: Appearance): void {
  try {
    window.localStorage.setItem(APPEARANCE_KEY, choice)
  } catch {
    // Private mode or blocked storage: the choice simply lasts until the page closes.
  }
}

export const paint = (appearance: Appearance): void => {
  document.documentElement.dataset.theme = appearance
}

/** Runs before React renders so the first frame already has the right colors. */
export function paintFirstFrame(): void {
  paint(storedAppearance() ?? systemAppearance())
}
