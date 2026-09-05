import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'

/**
 * Pruebas del esqueleto. Las de los flujos de autenticación llegan en la
 * semana 2 y llevarán el identificador del requisito en el nombre
 * (`RF-003 …`), para que scripts/traceability.py las recoja.
 */
describe('App', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('muestra la versión de la API y el estado de cada dependencia', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        const body =
          url === '/healthz'
            ? { status: 'ok', version: '0.1.0-test' }
            : { status: 'ready', checks: { database: { status: 'up' }, broker: { status: 'up' } } }
        return { ok: true, json: async () => body } as Response
      }),
    )

    render(<App />)

    await waitFor(() => expect(screen.getByText('0.1.0-test')).toBeInTheDocument())
    expect(screen.getByText('database')).toBeInTheDocument()
    expect(screen.getAllByText('operativa')).toHaveLength(2)
  })

  it('avisa cuando la API no responde, en vez de quedarse en blanco', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 503, statusText: 'Service Unavailable' }) as Response))

    render(<App />)

    await waitFor(() =>
      expect(screen.getByText(/No se pudo contactar con la API/)).toBeInTheDocument(),
    )
  })
})
