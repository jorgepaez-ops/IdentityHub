import { act, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoadingSkeleton } from './LoadingSkeleton'
import { ThemeToggle } from './ThemeToggle'
import { Toast } from './Toast'
import { initializeTheme } from './theme'

function ToastHarness({ onClose }: { onClose: () => void }) {
  const [message, setMessage] = useState('Invitación reenviada a beto@example.test.')
  return message ? <Toast message={message} onClose={() => { onClose(); setMessage('') }} /> : null
}

describe('visual system feedback', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    window.localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  describe('system theme changes', () => {
    function fakeSystemTheme(dark: boolean) {
      let matches = dark
      const listeners = new Set<(event: { matches: boolean }) => void>()
      vi.stubGlobal('matchMedia', vi.fn(() => ({
        get matches() { return matches },
        addEventListener: (_: string, listener: (event: { matches: boolean }) => void) => listeners.add(listener),
        removeEventListener: (_: string, listener: (event: { matches: boolean }) => void) => listeners.delete(listener),
      })))
      return (next: boolean) => {
        matches = next
        act(() => listeners.forEach((listener) => listener({ matches: next })))
      }
    }

    it('TestRF010_ElTemaSigueAlSistemaEnVivoSinEleccionGuardada', () => {
      const setSystemDark = fakeSystemTheme(false)
      initializeTheme()
      render(<ThemeToggle />)
      expect(document.documentElement).toHaveAttribute('data-theme', 'light')

      setSystemDark(true)
      expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
      expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
    })

    it('TestRF010_TrasUnClicElCambioDelSistemaSeIgnora', () => {
      const setSystemDark = fakeSystemTheme(false)
      initializeTheme()
      render(<ThemeToggle />)
      fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))

      setSystemDark(false)
      setSystemDark(true)
      expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
      setSystemDark(false)
      expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
      expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
    })

    it('TestRF010_UnTemaGuardadoAlArrancarIgnoraAlSistema', () => {
      window.localStorage.setItem('identityhub.theme', 'light')
      const setSystemDark = fakeSystemTheme(false)
      initializeTheme()
      render(<ThemeToggle />)

      setSystemDark(true)
      expect(document.documentElement).toHaveAttribute('data-theme', 'light')
      expect(screen.getByRole('button', { name: 'Cambiar a tema oscuro' })).toBeInTheDocument()
    })

    it('TestRF010_SiGuardarFallaElClicDejaDeSeguirAlSistema', () => {
      const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
      const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
      try {
        const setSystemDark = fakeSystemTheme(false)
        initializeTheme()
        render(<ThemeToggle />)
        setSystemDark(true)
        expect(document.documentElement).toHaveAttribute('data-theme', 'dark')

        fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema claro' }))
        setSystemDark(false)
        setSystemDark(true)
        expect(document.documentElement).toHaveAttribute('data-theme', 'light')
        expect(screen.getByRole('button', { name: 'Cambiar a tema oscuro' })).toBeInTheDocument()
      } finally {
        getItem.mockRestore()
        setItem.mockRestore()
      }
    })
  })

  it('TestRF010_CierraElAvisoManualSinDejarUnTemporizadorActivo', () => {
    vi.useFakeTimers()
    const onClose = vi.fn()
    render(<ToastHarness onClose={onClose} />)

    expect(screen.getByRole('status')).toHaveTextContent('Invitación reenviada a beto@example.test.')
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar aviso' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()

    vi.advanceTimersByTime(5000)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('TestRF010_CierraElAvisoAutomaticamente', () => {
    vi.useFakeTimers()
    const onClose = vi.fn()
    render(<ToastHarness onClose={onClose} />)

    act(() => { vi.advanceTimersByTime(5000) })
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF010_ElTemaUsaElDelSistemaSiElAlmacenamientoFalla', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
    try {
      expect(() => initializeTheme()).not.toThrow()
      expect(document.documentElement.dataset.theme).toBe('dark')
      render(<ThemeToggle />)
      expect(() => fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema claro' }))).not.toThrow()
      expect(document.documentElement.dataset.theme).toBe('light')
      fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))
      expect(document.documentElement.dataset.theme).toBe('dark')
    } finally {
      getItem.mockRestore()
      setItem.mockRestore()
    }
  })

  it('TestRF010_ElAvisoSeCierraAunqueElPadreCambieElCallback', () => {
    vi.useFakeTimers()
    const calls: number[] = []
    const { rerender } = render(<Toast message="Cambios guardados" onClose={() => calls.push(1)} />)

    vi.advanceTimersByTime(3000)
    rerender(<Toast message="Cambios guardados" onClose={() => calls.push(2)} />)
    vi.advanceTimersByTime(2000)
    expect(calls).toEqual([2])
  })

  it('TestRF010_ReiniciaElTiempoDelAvisoAlCambiarElMensaje', () => {
    vi.useFakeTimers()
    const onClose = vi.fn()
    const { rerender } = render(<Toast message="Primer aviso" onClose={onClose} />)

    vi.advanceTimersByTime(3000)
    rerender(<Toast message="Segundo aviso" onClose={onClose} />)
    vi.advanceTimersByTime(3000)
    expect(onClose).not.toHaveBeenCalled()
    vi.advanceTimersByTime(2000)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('TestRF010_PersisteElTemaSeleccionadoEnElDocumento', () => {
    render(<ThemeToggle />)

    fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
    expect(window.localStorage.getItem('identityhub.theme')).toBe('dark')
    expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
  })

  it('TestRF010_HidrataElTemaGuardadoAntesDeMostrarLaAplicacion', () => {
    window.localStorage.setItem('identityhub.theme', 'dark')

    initializeTheme()

    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  it('TestRF010_UsaLaPreferenciaOscuraDelSistemaSinTemaGuardado', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))

    initializeTheme()

    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  it('TestRF010_AnunciaLaCargaConEstadoOcupado', () => {
    render(<LoadingSkeleton rows={3} label="Cargando usuarios…" />)

    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByText('Cargando usuarios…')).toHaveClass('sr-only')
  })
})
