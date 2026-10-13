import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, profile, signedIn, stubApi, applicationsRoute } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const ANA = 'aaaaaaaa-1111-4222-8333-444444444444'
const BOM = '﻿'

// jsdom has no object URLs: capture the Blob handed to the anchor and the file name it downloads.
const downloads: { name: string; text: string }[] = []
function captureDownloads() {
  downloads.length = 0
  const blobs = new Map<string, Blob>()
  let next = 0
  Object.assign(URL, {
    createObjectURL: (blob: Blob) => {
      const url = `blob:test-${next++}`
      blobs.set(url, blob)
      return url
    },
    revokeObjectURL: vi.fn(),
  })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    const blob = blobs.get(this.getAttribute('href') ?? '')
    if (!blob) return
    const reader = new FileReader()
    reader.onload = () => downloads.push({ name: this.download, text: new TextDecoder('utf-8', { ignoreBOM: true }).decode(reader.result as ArrayBuffer) })
    reader.readAsArrayBuffer(blob)
  })
}
const exported = () => waitFor(() => expect(downloads).toHaveLength(1)).then(() => downloads[0]!)
const exportButton = () => screen.getByRole('button', { name: 'Exportar CSV' })
const today = () => {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}

afterEach(() => {
  resetSessionForTests()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  Reflect.deleteProperty(URL, 'createObjectURL')
  Reflect.deleteProperty(URL, 'revokeObjectURL')
  goTo('/')
})

const person = (overrides: Record<string, unknown>) => profile({ mfaEnabled: true, lastLoginAt: '2026-10-01T09:30:00Z', ...overrides })
const directory = [
  person({ id: 'ana-id', email: 'ana@example.test', displayName: 'Ana "La Jefa", Pérez', roles: ['user', 'admin'] }),
  person({ id: 'carla-id', email: 'carla@example.test', displayName: '=HYPERLINK("x")', status: 'locked', mfaEnabled: false, lastLoginAt: null, createdAt: '2026-02-03T04:05:06Z' }),
]
const usersApi = (nextCursor: string | null = null): Handler => (_body, url) => {
  const status = new URL(url, 'http://localhost').searchParams.get('status')
  return json(200, { items: directory.filter((item) => !status || item.status === status), nextCursor })
}
const openUsers = async (path = '/usuarios', handler: Handler = usersApi()) => {
  goTo(path)
  const api = stubApi({ ...signedIn(adminUser), ...applicationsRoute, 'GET /api/v1/admin/users': handler })
  render(<App />)
  await screen.findByRole('heading', { name: 'Usuarios' })
  return api
}

describe('users CSV export', () => {
  it('TestRF010_ExportsTheLoadedUsersWithTheAgreedColumns', async () => {
    captureDownloads()
    await openUsers()
    await screen.findByText('carla@example.test')
    fireEvent.click(exportButton())
    const file = await exported()
    expect(file.name).toBe(`usuarios-${today()}.csv`)
    expect(file.text.startsWith(BOM)).toBe(true)
    expect(file.text.slice(1).split('\r\n')).toEqual([
      'Correo,Nombre,Estado,Roles,MFA,Último acceso,Creado',
      `ana@example.test,"Ana ""La Jefa"", Pérez",Activo,user; admin,Sí,2026-10-01T09:30:00Z,2026-01-01T00:00:00Z`,
      `carla@example.test,"'=HYPERLINK(""x"")",Bloqueado,user; admin,No,,2026-02-03T04:05:06Z`.replace('user; admin', 'user; contabilidad.senior'),
      '',
    ])
  })

  it('TestRF010_ExportsOnlyTheUsersMatchingTheStatusFilter', async () => {
    captureDownloads()
    await openUsers('/usuarios?estado=locked')
    await screen.findByText('carla@example.test')
    fireEvent.click(exportButton())
    const rows = (await exported()).text.slice(1).split('\r\n').filter(Boolean)
    expect(rows).toHaveLength(2)
    expect(rows[1]).toContain('carla@example.test')
  })

  it('TestRF010_AnnouncesTheExportInTheSingleStatusRegion', async () => {
    captureDownloads()
    await openUsers()
    await screen.findByText('carla@example.test')
    fireEvent.click(exportButton())
    await exported()
    expect(within(screen.getByRole('status')).getByText('Exportados 2 registros')).toBeInTheDocument()
    expect(screen.getAllByRole('status')).toHaveLength(1)
  })

  it('TestRF010_WarnsThatOnlyTheLoadedUsersAreExportedWhenThereAreMore', async () => {
    await openUsers('/usuarios', usersApi('next'))
    await screen.findByText('carla@example.test')
    expect(screen.getByText('Exporta los 2 cargados; carga más para incluir el resto.')).toBeInTheDocument()
  })

  it('TestRF010_DisablesTheExportWhenThereAreNoUsers', async () => {
    await openUsers('/usuarios?estado=disabled')
    await screen.findByText('No hay usuarios con este estado.')
    expect(exportButton()).toBeDisabled()
    expect(exportButton().querySelector('svg.icon')).not.toBeNull()
  })
})

const event = (id: number, overrides: Record<string, unknown> = {}) =>
  ({ id, actorUserId: ANA, action: 'login_failed', resourceType: null, resourceId: null, ip: null, userAgent: null, metadata: {}, createdAt: '2026-10-01T10:00:00Z', ...overrides })
const exportFile = (body: string, type: string, name: string, truncated = false) =>
  new Response(body, { status: 200, headers: { 'Content-Type': type, 'Content-Disposition': `attachment; filename="${name}"`, 'X-Export-Truncated': String(truncated) } })
const exportRoute = (handler: Handler) => ({ 'GET /api/v1/admin/audit-log/export': handler })
const openAuditWithExport = async (handler: Handler, path = '/auditoria') => {
  goTo(path)
  const api = stubApi({
    ...signedIn(adminUser),
    ...applicationsRoute,
    'GET /api/v1/admin/audit-log': () => json(200, { items: [event(1)], nextCursor: null }),
    [`GET /api/v1/admin/users/${ANA}`]: () => json(200, profile({ id: ANA, email: 'ana@example.test' })),
    ...exportRoute(handler),
  })
  render(<App />)
  await screen.findByRole('table', { name: 'Registro de auditoría' })
  return api
}
const exportCalls = (api: ReturnType<typeof stubApi>) => api.calls.filter((call) => call.key === 'GET /api/v1/admin/audit-log/export')

describe('audit log server-side export', () => {
  it('TestRF019_DownloadsTheCsvTheApiBuildsUsingTheServerFileName', async () => {
    captureDownloads()
    const api = await openAuditWithExport(() => exportFile('id,action\r\n1,login_failed\r\n', 'text/csv; charset=utf-8', 'audit-log-20261012T100000Z.csv'))
    fireEvent.click(screen.getByRole('button', { name: 'Exportar CSV' }))
    const file = await exported()
    expect(file).toEqual({ name: 'audit-log-20261012T100000Z.csv', text: 'id,action\r\n1,login_failed\r\n' })
    expect(exportCalls(api).map((call) => call.url)).toEqual(['/api/v1/admin/audit-log/export?format=csv'])
    expect(await screen.findByText('Exportado CSV con los filtros actuales')).toBeInTheDocument()
  })

  it('TestRF019_ExportsJsonWithTheCurrentFiltersAndTheBearerToken', async () => {
    captureDownloads()
    const api = await openAuditWithExport(() => exportFile('[]', 'application/json; charset=utf-8', 'audit-log-20261012T100000Z.json'), `/auditoria?accion=login_failed&actor=${ANA}`)
    fireEvent.click(screen.getByRole('button', { name: 'Exportar JSON' }))
    expect((await exported()).name).toBe('audit-log-20261012T100000Z.json')
    const url = new URL(exportCalls(api)[0]!.url, 'http://localhost')
    expect(url.searchParams.get('format')).toBe('json')
    expect(url.searchParams.get('action')).toBe('login_failed')
    expect(url.searchParams.get('actorId')).toBe(ANA)
    const init = api.fetch.mock.calls.find(([input]) => String(input).startsWith('/api/v1/admin/audit-log/export'))![1]
    expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer access')
  })

  it('TestRF019_WarnsWhenTheExportWasTruncated', async () => {
    captureDownloads()
    await openAuditWithExport(() => exportFile('[]', 'application/json', 'audit-log-x.json', true))
    fireEvent.click(screen.getByRole('button', { name: 'Exportar JSON' }))
    expect(await screen.findByText('Exportación recortada a 10 000 eventos; acota los filtros para el resto')).toBeInTheDocument()
  })

  it('TestRF019_ShowsAnErrorAndDownloadsNothingWhenTheExportFails', async () => {
    captureDownloads()
    await openAuditWithExport(() => json(500, { type: 'about:blank', title: 'Internal Server Error', status: 500 }, 'application/problem+json'))
    fireEvent.click(screen.getByRole('button', { name: 'Exportar CSV' }))
    expect(await screen.findByText('No fue posible exportar el registro de auditoría. Inténtalo de nuevo.')).toBeInTheDocument()
    expect(downloads).toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Exportar CSV' })).toBeEnabled()
  })

  it('TestRF019_DisablesBothButtonsWhileAnExportIsRunning', async () => {
    captureDownloads()
    let release: (response: Response) => void = () => undefined
    await openAuditWithExport(() => new Promise<Response>((resolve) => { release = resolve }))
    fireEvent.click(screen.getByRole('button', { name: 'Exportar CSV' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Exportar JSON' })).toBeDisabled())
    expect(screen.getByRole('button', { name: 'Exportar CSV' })).toBeDisabled()
    release(exportFile('x', 'text/csv', 'audit-log-x.csv'))
    await exported()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Exportar JSON' })).toBeEnabled())
  })

  it('TestRF019_FallsBackToAGenericFileNameWithoutContentDisposition', async () => {
    captureDownloads()
    await openAuditWithExport(() => new Response('[]', { status: 200, headers: { 'Content-Type': 'application/json' } }))
    fireEvent.click(screen.getByRole('button', { name: 'Exportar JSON' }))
    expect((await exported()).name).toBe('auditoria.json')
  })
})
