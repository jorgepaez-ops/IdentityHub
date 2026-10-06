import { useEffect, useState } from 'react'
import { MoonIcon, SunIcon } from './icons'
import { applyTheme, resolveTheme, saveTheme } from './theme'

export function ThemeToggle() {
  const [theme, setTheme] = useState(resolveTheme)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  const next = theme === 'dark' ? 'light' : 'dark'
  const label = `Cambiar a tema ${next === 'dark' ? 'oscuro' : 'claro'}`
  const change = () => {
    setTheme(next)
    saveTheme(next)
  }

  return <button className="icon-button" type="button" onClick={change} aria-label={label} title={label}>{theme === 'dark' ? <SunIcon /> : <MoonIcon />}</button>
}
