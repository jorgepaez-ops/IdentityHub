import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { accessFor } from './access'
import { Shell } from './Shell'
import { ThemeToggle } from './ThemeToggle'
import { Toast } from './Toast'
import { initializeTheme } from './theme'

function ToastHarness({ onClose }: { onClose: () => void }) {
  const [message, setMessage] = useState('Movimiento M-2043 aprobado.')
  return message ? <Toast message={message} onClose={() => { onClose(); setMessage('') }} /> : null
}

const auditor = accessFor(['movimientos.ver_todos', 'reportes.ver'], ['contabilidad.auditor'])
const senior = accessFor(['movimientos.registrar', 'movimientos.ver_todos', 'movimientos.aprobar', 'cierre.ejecutar', 'reportes.ver'], ['contabilidad.senior'])

describe('visual system of Contabilidad', () => {
  afterEach(() => {
    vi.useRealTimers()
    window.localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('TestRF021_PersisteElTemaEnLaLlaveDeContabilidad', () => {
    render(<ThemeToggle />)
    fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' }))
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
    expect(window.localStorage.getItem('contabilidad.theme')).toBe('dark')
    expect(screen.getByRole('button', { name: 'Cambiar a tema claro' })).toBeInTheDocument()
  })

  it('TestRF021_HidrataElTemaGuardado', () => {
    window.localStorage.setItem('contabilidad.theme', 'dark')
    initializeTheme()
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  })

  it('TestRF021_ElTemaSobreviveAlAlmacenamientoBloqueado', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('blocked', 'SecurityError') })
    try {
      expect(() => initializeTheme()).not.toThrow()
      render(<ThemeToggle />)
      expect(() => fireEvent.click(screen.getByRole('button', { name: /Cambiar a tema/ }))).not.toThrow()
    } finally {
      getItem.mockRestore()
      setItem.mockRestore()
    }
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
    render(<Shell access={senior} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Transacciones' }))
    fireEvent.click(screen.getByRole('button', { name: 'Aprobar M-2043' }))
    expect(screen.getByText('Movimiento M-2043 aprobado.')).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(3000) })
    fireEvent.click(screen.getByRole('button', { name: 'Aprobar M-2044' }))
    act(() => { vi.advanceTimersByTime(3000) })
    expect(screen.getByRole('status')).toHaveTextContent('Movimiento M-2044 aprobado.')
    act(() => { vi.advanceTimersByTime(2000) })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF021_ElCandadoConservaElNombreAccesibleBloqueado', () => {
    render(<Shell access={auditor} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    const closing = screen.getByRole('button', { name: /Cierre contable/ })
    expect(closing).toBeDisabled()
    expect(within(closing).getByRole('img', { name: 'Bloqueado' })).toBeInTheDocument()
  })

  it('TestRF021_OfreceUnEnlaceParaSaltarAlContenidoYUnaTablaConNombre', () => {
    render(<Shell access={auditor} subject="3f2c1a9e-0000" onLogout={() => undefined} />)
    expect(screen.getByRole('link', { name: 'Saltar al contenido' })).toHaveAttribute('href', '#main-content')
    fireEvent.click(screen.getByRole('button', { name: 'Transacciones' }))
    expect(screen.getByRole('table', { name: /Movimientos/ })).toBeInTheDocument()
    expect(screen.getAllByRole('columnheader').every((header) => header.getAttribute('scope') === 'col')).toBe(true)
  })
})
