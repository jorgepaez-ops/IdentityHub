import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { useState, type ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { accessFor } from './access'
import { Shell } from './Shell'
import { ThemeToggle } from './ThemeToggle'
import { Toast } from './Toast'
import { AppearanceProvider } from './AppearanceProvider'
import { paintFirstFrame } from './colorScheme'

function ToastHarness({ onClose }: { onClose: () => void }) {
  const [message, setMessage] = useState('Movimiento M-2043 aprobado.')
  return message ? <Toast message={message} onClose={() => { onClose(); setMessage('') }} /> : null
}

const withTheme = (ui: ReactElement) => render(<AppearanceProvider>{ui}</AppearanceProvider>)
const mountToggle = () => withTheme(<ThemeToggle />)
const blockStorage = () => [
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') }),
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') }),
]

const auditor = accessFor(['movimientos.ver_todos', 'reportes.ver'], ['contabilidad.auditor'])
const senior = accessFor(['movimientos.registrar', 'movimientos.ver_todos', 'movimientos.aprobar', 'cierre.ejecutar', 'reportes.ver'], ['contabilidad.senior'])

describe('visual system of Contabilidad', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    window.localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  describe('system theme changes', () => {
    // A stand-in for the operating system: flip() fires the same event a real change would.
    function operatingSystem(startsDark: boolean) {
      let dark = startsDark
      const subscribers = new Set<(event: { matches: boolean }) => void>()
      vi.stubGlobal('matchMedia', vi.fn(() => ({
        get matches() { return dark },
        addEventListener: (_: string, fn: (event: { matches: boolean }) => void) => subscribers.add(fn),
        removeEventListener: (_: string, fn: (event: { matches: boolean }) => void) => subscribers.delete(fn),
      })))
      return {
        flip(to: boolean) {
          dark = to
          act(() => subscribers.forEach((fn) => fn({ matches: to })))
        },
        listeners: () => subscribers.size,
      }
    }
    const themeNow = () => document.documentElement.dataset.theme
    const toggle = () => fireEvent.click(screen.getByRole('button', { name: /Cambiar a tema/ }))

    it('TestRF021_ElProveedorSigueAlSistemaEnVivoSinEleccionGuardada', () => {
      const os = operatingSystem(false)
      mountToggle()
      expect(themeNow()).toBe('light')
      os.flip(true)
      expect(themeNow()).toBe('dark')
      expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
    })

    it.each([
      ['a click in this page', () => undefined],
      ['a saved choice', () => window.localStorage.setItem('contabilidad.theme', 'light')],
    ])('TestRF021_DejaDeSeguirAlSistemaTras_%s', (_name, setup) => {
      setup()
      const os = operatingSystem(false)
      mountToggle()
      if (window.localStorage.getItem('contabilidad.theme') === null) toggle()
      const chosen = themeNow()
      os.flip(true)
      os.flip(false)
      os.flip(true)
      expect(themeNow()).toBe(chosen)
      expect(os.listeners()).toBe(0)
    })

    it('TestRF021_SiGuardarFallaElClicDejaDeSeguirAlSistema', () => {
      const spies = blockStorage()
      try {
        const os = operatingSystem(false)
        mountToggle()
        os.flip(true)
        expect(themeNow()).toBe('dark')
        toggle()
        os.flip(false)
        os.flip(true)
        expect(themeNow()).toBe('light')
        expect(screen.getByRole('button', { name: 'Cambiar a tema oscuro' })).toBeInTheDocument()
      } finally {
        spies.forEach((spy) => spy.mockRestore())
      }
    })

    it('TestRF021_AlDesmontarseElProveedorDejaDeEscucharAlSistema', () => {
      const os = operatingSystem(false)
      const view = mountToggle()
      expect(os.listeners()).toBe(1)
      view.unmount()
      expect(os.listeners()).toBe(0)
    })
  })

  it('TestRF021_PersisteElTemaEnLaLlaveDeContabilidad', () => {
    mountToggle()
    fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
    expect(window.localStorage.getItem('contabilidad.theme')).toBe('dark')
    expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
  })

  it('TestRF021_HidrataElTemaGuardadoAntesDeQueReactPinte', () => {
    window.localStorage.setItem('contabilidad.theme', 'dark')
    paintFirstFrame()
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  it('TestRF021_ElPrimerCuadroUsaElSistemaSinEleccionGuardada', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
    paintFirstFrame()
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  it('TestRF021_ElTemaSobreviveAlAlmacenamientoBloqueado', () => {
    const spies = blockStorage()
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
    try {
      expect(() => paintFirstFrame()).not.toThrow()
      expect(document.documentElement.dataset.theme).toBe('dark')
      mountToggle()
      expect(() => fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema claro' }))).not.toThrow()
      expect(document.documentElement.dataset.theme).toBe('light')
      fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))
      expect(document.documentElement.dataset.theme).toBe('dark')
    } finally {
      spies.forEach((spy) => spy.mockRestore())
    }
  })

  it('TestRF021_UsarElBotonSinProveedorFallaConUnMensajeClaro', () => {
    const quiet = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    expect(() => render(<ThemeToggle />)).toThrow(/AppearanceProvider/)
    quiet.mockRestore()
  })

  it('TestRF021_ElAvisoSeCierraSoloEnCincoSegundos', () => {
    vi.useFakeTimers()
    const onClose = vi.fn()
    render(<ToastHarness onClose={onClose} />)
    expect(screen.getAllByRole('status')).toHaveLength(1)
    act(() => { vi.advanceTimersByTime(5000) })
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF021_ElAvisoSeCierraConElBoton', () => {
    vi.useFakeTimers()
    const onClose = vi.fn()
    render(<ToastHarness onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar aviso' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    act(() => { vi.advanceTimersByTime(5000) })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('TestRF021_UnMensajeRepetidoReiniciaElTemporizadorDelShell', () => {
    vi.useFakeTimers()
    withTheme(<Shell access={senior} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Transacciones' }))
    fireEvent.click(screen.getByRole('button', { name: 'Aprobar M-2043' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar aprobación' }))
    expect(screen.getByText('Movimiento M-2043 aprobado.')).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(3000) })
    fireEvent.click(screen.getByRole('button', { name: 'Aprobar M-2044' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar aprobación' }))
    act(() => { vi.advanceTimersByTime(3000) })
    expect(screen.getByRole('status')).toHaveTextContent('Movimiento M-2044 aprobado.')
    act(() => { vi.advanceTimersByTime(2000) })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF021_UnMismoTextoRepetidoReiniciaElTemporizadorDelShell', () => {
    vi.useFakeTimers()
    withTheme(<Shell access={senior} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Transacciones' }))
    const register = (description: string) => {
      fireEvent.click(screen.getByRole('button', { name: '+ Registrar movimiento' }))
      fireEvent.change(screen.getByLabelText('Descripción'), { target: { value: description } })
      fireEvent.change(screen.getByLabelText('Monto'), { target: { value: '100' } })
      fireEvent.change(screen.getByLabelText('Categoría'), { target: { value: 'Servicios' } })
      fireEvent.click(screen.getByRole('button', { name: 'Registrar' }))
    }
    const notice = 'Movimiento registrado, queda pendiente de aprobación.'
    register('Primero')
    expect(screen.getByRole('status')).toHaveTextContent(notice)
    act(() => { vi.advanceTimersByTime(3000) })
    register('Segundo')
    expect(screen.getByRole('status')).toHaveTextContent(notice)
    act(() => { vi.advanceTimersByTime(3000) })
    expect(screen.getByRole('status')).toHaveTextContent(notice)
    act(() => { vi.advanceTimersByTime(2000) })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF021_ElCandadoConservaElNombreAccesibleBloqueado', () => {
    withTheme(<Shell access={auditor} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    const closing = screen.getByRole('button', { name: /Cierre contable/ })
    expect(closing).toBeDisabled()
    expect(within(closing).getByRole('img', { name: 'Bloqueado' })).toBeInTheDocument()
  })

  it('TestRF021_OfreceUnEnlaceParaSaltarAlContenidoYUnaTablaConNombre', () => {
    withTheme(<Shell access={auditor} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    expect(screen.getByRole('link', { name: 'Saltar al contenido' })).toHaveAttribute('href', '#main-content')
    fireEvent.click(screen.getByRole('button', { name: 'Transacciones' }))
    expect(screen.getByRole('table', { name: /Movimientos/ })).toBeInTheDocument()
    expect(screen.getAllByRole('columnheader').every((header) => header.getAttribute('scope') === 'col')).toBe(true)
  })
})
