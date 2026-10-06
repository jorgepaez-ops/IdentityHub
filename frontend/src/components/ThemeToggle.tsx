import { useEffect, useState } from 'react'
import { MoonIcon, SunIcon } from './icons'
import { applyTheme, resolveTheme, themeStorageKey } from './theme'

export function ThemeToggle() {
  const [theme, setTheme] = useState(resolveTheme)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  const next = theme === 'dark' ? 'light' : 'dark'
  const change = () => {
    setTheme(next)
    window.localStorage.setItem(themeStorageKey, next)
  }

  return <button className="icon-button ghost-button theme-toggle" type="button" onClick={change} aria-label={`Cambiar a tema ${next === 'dark' ? 'oscuro' : 'claro'}`} title={`Cambiar a tema ${next === 'dark' ? 'oscuro' : 'claro'}`}>{theme === 'dark' ? <SunIcon /> : <MoonIcon />}</button>
}
