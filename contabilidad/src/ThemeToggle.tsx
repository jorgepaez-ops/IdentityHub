import { useAppearance } from './AppearanceProvider'
import { MoonIcon, SunIcon } from './icons'

export function ThemeToggle() {
  const { appearance, toggle } = useAppearance()
  const label = `Cambiar a tema ${appearance === 'dark' ? 'claro' : 'oscuro'}`
  return <button className="icon-button" type="button" onClick={toggle} aria-label={label} title={label}>{appearance === 'dark' ? <SunIcon /> : <MoonIcon />}</button>
}
