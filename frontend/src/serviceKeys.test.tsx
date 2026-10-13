import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, problem, profile, serviceKeyFixture, signedIn, stubApi, type } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const keysPath = '/api/v1/admin/service-keys'
const issuedBearer = 'ihk_0123456789abcdef_0123456789012345678901234567890123456789012'
const second = serviceKeyFixture({ id: '44444444-4444-4444-8444-444444444442', name: 'Reportes', prefix: 'ihk_fedcba9876543210', revokedAt: '2026-10-05T10:00:00Z', lastUsedAt: '2026-10-04T10:00:00Z' })

const openKeys = async (extra: Record<string, Handler> = {}, keys: unknown[] = [serviceKeyFixture(), second]) => {
  goTo('/claves')
  const api = stubApi({ ...signedIn(adminUser), [`GET ${keysPath}`]: () => json(200, keys), ...extra })
  render(<App />)
  await screen.findByRole('heading', { name: 'Claves de servicio' })
  await waitFor(() => expect(screen.queryByText('Cargando claves…')).not.toBeInTheDocument())
  return api
}
const sent = (api: ReturnType<typeof stubApi>, key: string) => api.calls.find((call) => call.key === key)?.body

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('service keys page', () => {
  it('TestRF018_ListaLasClavesSoloConMetadatos', async () => {
    await openKeys()
    const table = screen.getByRole('table', { name: 'Claves de servicio' })
    expect(within(table).getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual(['Nombre', 'Prefijo', 'Alcances', 'Vence', 'Último uso', 'Estado', 'Acciones'])
    expect(within(table).getByText('SIEM de seguridad')).toBeInTheDocument()
    expect(within(table).getByText('ihk_0123456789abcdef')).toBeInTheDocument()
    expect(within(table).getAllByText('audit-log:read')).toHaveLength(2)
    expect(within(table).getByText('Vigente')).toBeInTheDocument()
    expect(within(table).getByText('Revocada')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Claves de servicio/ })).toBeInTheDocument()
    expect(within(table).queryByRole('button', { name: 'Revocar Reportes' })).not.toBeInTheDocument()
  })

  it('TestRF018_EmitirEnviaElPostYMuestraLaClaveUnaSolaVez', async () => {
    const created = { ...serviceKeyFixture({ id: '44444444-4444-4444-8444-444444444443', name: 'Nueva', prefix: 'ihk_0123456789abcdef' }), key: issuedBearer }
    const api = await openKeys({ [`POST ${keysPath}`]: () => json(201, created) }, [])
    expect(screen.getByText('Todavía no hay claves de servicio.')).toBeInTheDocument()

    type('Nombre de la clave', ' Nueva ')
    type('Vigencia en días', '30')
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))

    const reveal = await screen.findByRole('alert', { name: 'Clave emitida' })
    expect(within(reveal).getByText(/no se volverá a mostrar/i)).toBeInTheDocument()
    expect(within(reveal).getByLabelText('Clave de servicio completa')).toHaveValue(issuedBearer)
    expect(sent(api, `POST ${keysPath}`)).toEqual({ name: 'Nueva', scopes: ['audit-log:read'], expiresInDays: 30 })
    expect(screen.getByRole('table', { name: 'Claves de servicio' })).toBeInTheDocument()
    expect(within(screen.getByRole('table', { name: 'Claves de servicio' })).queryByText(issuedBearer)).not.toBeInTheDocument()

    fireEvent.click(within(reveal).getByRole('button', { name: 'Ya la copié' }))
    await waitFor(() => expect(screen.queryByText(issuedBearer)).not.toBeInTheDocument())
    expect(screen.queryByLabelText('Clave de servicio completa')).not.toBeInTheDocument()
  })

  it('TestRF018_ElBotonCopiarUsaElPortapapeles', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    await openKeys({ [`POST ${keysPath}`]: () => json(201, { ...serviceKeyFixture(), key: issuedBearer }) })
    type('Nombre de la clave', 'SIEM')
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))
    const reveal = await screen.findByRole('alert', { name: 'Clave emitida' })
    fireEvent.click(within(reveal).getByRole('button', { name: 'Copiar clave' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(issuedBearer))
    expect(await screen.findByText('Clave copiada.')).toBeInTheDocument()
  })

  it('TestRF018_EmitirValidaElFormularioAntesDeLlamarALaApi', async () => {
    const api = await openKeys()
    type('Nombre de la clave', '   ')
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))
    expect(await screen.findByText(/El nombre es obligatorio/)).toBeInTheDocument()
    type('Nombre de la clave', 'SIEM')
    type('Vigencia en días', '400')
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))
    expect(await screen.findByText(/entre 1 y 365 días/)).toBeInTheDocument()
    type('Vigencia en días', '30')
    fireEvent.click(screen.getByRole('checkbox', { name: 'audit-log:read' }))
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))
    expect(await screen.findByText(/Elige al menos un alcance/)).toBeInTheDocument()
    expect(api.calls.some((call) => call.key === `POST ${keysPath}`)).toBe(false)
  })

  it('TestRF018_UnErrorDeLaApiSeMuestraYNoRevelaUnaClave', async () => {
    await openKeys({ [`POST ${keysPath}`]: () => problem(400, { errors: [{ field: 'expiresInDays', message: 'must be between 1 and 365' }] }) })
    type('Nombre de la clave', 'SIEM')
    fireEvent.click(screen.getByRole('button', { name: 'Emitir clave' }))
    expect(await screen.findByText(/expiresInDays: must be between 1 and 365/)).toBeInTheDocument()
    expect(screen.queryByRole('alert', { name: 'Clave emitida' })).not.toBeInTheDocument()
  })

  it('TestRF018_RevocarPideConfirmacionYMarcaLaClave', async () => {
    const api = await openKeys({ [`DELETE ${keysPath}/${serviceKeyFixture().id}`]: () => json(204) })
    fireEvent.click(screen.getByRole('button', { name: 'Revocar SIEM de seguridad' }))
    expect(api.calls.some((call) => call.key.startsWith('DELETE'))).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar revocación de SIEM de seguridad' }))
    expect(screen.getByRole('button', { name: 'Revocar SIEM de seguridad' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Revocar SIEM de seguridad' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar revocación de SIEM de seguridad' }))
    expect(await screen.findByText('Clave SIEM de seguridad revocada.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Revocar SIEM de seguridad' })).not.toBeInTheDocument()
    expect(screen.getAllByText('Revocada')).toHaveLength(2)
  })

  it('TestRF018_UnNoAdminNoVeLaPagina', async () => {
    goTo('/claves')
    stubApi({ ...signedIn(profile({ roles: ['user'] })), 'GET /api/v1/me/sessions': () => json(200, []) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Mi cuenta' })
    expect(screen.queryByRole('link', { name: /Claves de servicio/ })).not.toBeInTheDocument()
  })
})
