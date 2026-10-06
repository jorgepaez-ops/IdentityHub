import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, applicationsFixture, applicationsRoute, goTo, json, problem, profile, signedIn, stubApi, type } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const auditorId = '22222222-2222-4222-8222-222222222223'
const appPath = '/api/v1/admin/applications/11111111-1111-4111-8111-111111111111/roles'

const openRoles = async (extra: Record<string, Handler> = {}, user = adminUser) => {
  goTo('/roles')
  const api = stubApi({ ...signedIn(user), ...applicationsRoute, ...extra })
  render(<App />)
  await screen.findByRole('table', { name: 'Permisos de roles de Contabilidad' })
  return api
}
const grid = () => screen.getByRole('table', { name: 'Permisos de roles de Contabilidad' })
const box = (permission: string, role: string) => within(grid()).getByRole('checkbox', { name: `${permission} para ${role}` }) as HTMLInputElement
const sent = (api: ReturnType<typeof stubApi>, key: string) => api.calls.find((call) => call.key === key)?.body

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('role grid', () => {
  it('TestRF021_GrillaMuestraRolesPorPermisosDeLaApi', async () => {
    await openRoles()
    expect(within(grid()).getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual([
      'Rol', 'Descripción', 'movimientos.ver_todos', 'movimientos.aprobar', 'cierre.ejecutar', 'reportes.ver', 'movimientos.registrar', 'Acciones',
    ])
    expect(within(grid()).getByRole('columnheader', { name: 'reportes.ver' })).toHaveAttribute('title', 'Ver reportes consolidados')
    expect(within(grid()).getAllByRole('rowheader').map((cell) => cell.textContent)).toEqual(['contabilidad.senior', 'contabilidad.analista', 'contabilidad.auditor'])
    expect(box('reportes.ver', 'contabilidad.auditor')).toBeChecked()
    expect(box('movimientos.ver_todos', 'contabilidad.auditor')).toBeChecked()
    expect(box('cierre.ejecutar', 'contabilidad.auditor')).not.toBeChecked()
    expect(box('movimientos.registrar', 'contabilidad.analista')).toBeChecked()
  })

  it('TestRF021_GrillaMarcaPermisosYGuardaElPatchExacto', async () => {
    const api = await openRoles({
      [`PATCH ${appPath}/${auditorId}`]: (payload) => json(200, { ...applicationsFixture()[0]!.roles[2], ...(payload as object) }),
    })
    const save = screen.getByRole('button', { name: 'Guardar contabilidad.auditor' })
    expect(save).toBeDisabled()
    fireEvent.click(box('cierre.ejecutar', 'contabilidad.auditor'))
    fireEvent.click(box('movimientos.ver_todos', 'contabilidad.auditor'))
    expect(save).toBeEnabled()
    fireEvent.click(save)
    expect(await screen.findByText('Rol contabilidad.auditor guardado.')).toBeInTheDocument()
    expect(sent(api, `PATCH ${appPath}/${auditorId}`)).toEqual({ permissionKeys: ['cierre.ejecutar', 'reportes.ver'] })
    expect(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' })).toBeDisabled()
  })

  it('TestRF021_GrillaEnviaLaDescripcionEditada', async () => {
    const api = await openRoles({
      [`PATCH ${appPath}/${auditorId}`]: (payload) => json(200, { ...applicationsFixture()[0]!.roles[2], ...(payload as object) }),
    })
    type('Descripción de contabilidad.auditor', 'Auditoría de solo lectura')
    fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
    await screen.findByText('Rol contabilidad.auditor guardado.')
    expect(sent(api, `PATCH ${appPath}/${auditorId}`)).toEqual({ description: 'Auditoría de solo lectura' })
  })

  it('TestRF021_GrillaMuestraSoloLecturaElRolQueElAdminTiene', async () => {
    const api = await openRoles({}, profile({ id: 'admin-id', roles: ['user', 'admin', 'contabilidad.auditor'] }))
    expect(box('reportes.ver', 'contabilidad.auditor')).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Guardar contabilidad.auditor' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Eliminar contabilidad.auditor' })).toBeDisabled()
    expect(within(grid()).getByText(/Tienes este rol/)).toBeInTheDocument()
    expect(box('reportes.ver', 'contabilidad.analista')).toBeEnabled()
    expect(api.calls.some((call) => call.key.startsWith('PATCH'))).toBe(false)
  })

  it('TestRF021_CrearRolValidaElNombreYEnviaElPost', async () => {
    const created = { ...applicationsFixture()[0]!.roles[2]!, id: '33333333-3333-4333-8333-333333333333', name: 'contabilidad.revisor', description: 'Revisa', permissionKeys: ['reportes.ver'], assignedCount: 0 }
    const api = await openRoles({ [`POST ${appPath}`]: () => json(201, created) })
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo rol de Contabilidad' }))
    expect(screen.getByLabelText('Nombre del rol')).toHaveValue('contabilidad.')
    type('Nombre del rol', 'contabilidad.Mal Nombre')
    fireEvent.click(screen.getByRole('button', { name: 'Crear rol' }))
    expect(await screen.findByText(/El nombre debe ser contabilidad\./)).toBeInTheDocument()
    expect(api.calls.some((call) => call.key.startsWith('POST /api/v1/admin/applications'))).toBe(false)
    type('Nombre del rol', 'otra.revisor')
    fireEvent.click(screen.getByRole('button', { name: 'Crear rol' }))
    expect(await screen.findByText(/El nombre debe ser contabilidad\./)).toBeInTheDocument()
    type('Nombre del rol', 'contabilidad.revisor')
    type('Descripción del nuevo rol', 'Revisa')
    fireEvent.click(screen.getByLabelText('reportes.ver'))
    fireEvent.click(screen.getByRole('button', { name: 'Crear rol' }))
    expect(await screen.findByText('Rol contabilidad.revisor creado.')).toBeInTheDocument()
    expect(sent(api, `POST ${appPath}`)).toEqual({ name: 'contabilidad.revisor', description: 'Revisa', permissionKeys: ['reportes.ver'] })
    expect(box('reportes.ver', 'contabilidad.revisor')).toBeChecked()
  })

  it('TestRF021_UnErrorPosteriorQuitaElAvisoDeExito', async () => {
    let calls = 0
    await openRoles({
      [`PATCH ${appPath}/${auditorId}`]: (payload) => (++calls === 1
        ? json(200, { ...applicationsFixture()[0]!.roles[2], ...(payload as object) })
        : problem(403, { detail: 'admins cannot change a role they hold' })),
    })
    fireEvent.click(box('cierre.ejecutar', 'contabilidad.auditor'))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
    expect(await screen.findByText('Rol contabilidad.auditor guardado.')).toBeInTheDocument()
    fireEvent.click(box('movimientos.registrar', 'contabilidad.auditor'))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('admins cannot change a role they hold')
    expect(screen.queryByText('Rol contabilidad.auditor guardado.')).not.toBeInTheDocument()
  })

  it('TestRF021_AbrirElFormularioDeNuevoRolQuitaElAvisoDeExito', async () => {
    await openRoles({
      [`PATCH ${appPath}/${auditorId}`]: (payload) => json(200, { ...applicationsFixture()[0]!.roles[2], ...(payload as object) }),
    })
    fireEvent.click(box('cierre.ejecutar', 'contabilidad.auditor'))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
    expect(await screen.findByText('Rol contabilidad.auditor guardado.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo rol de Contabilidad' }))
    expect(screen.queryByText('Rol contabilidad.auditor guardado.')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('TestRF021_UnMismoAvisoDeExitoRepetidoReiniciaElTemporizador', async () => {
    await openRoles({
      [`PATCH ${appPath}/${auditorId}`]: (payload) => json(200, { ...applicationsFixture()[0]!.roles[2], ...(payload as object) }),
    })
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      const saved = 'Rol contabilidad.auditor guardado.'
      // Testing Library's waitFor/findBy drain with setTimeout, which would hang under fake timers.
      const saveOnce = async (permission: string) => {
        fireEvent.click(box(permission, 'contabilidad.auditor'))
        fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
        await act(async () => { await vi.advanceTimersByTimeAsync(0) })
        expect(screen.getByRole('status')).toHaveTextContent(saved)
      }
      await saveOnce('cierre.ejecutar')
      act(() => { vi.advanceTimersByTime(3000) })
      await saveOnce('movimientos.registrar')
      act(() => { vi.advanceTimersByTime(3000) })
      expect(screen.getByRole('status')).toHaveTextContent(saved)
      act(() => { vi.advanceTimersByTime(2000) })
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('TestRF021_EliminarRolAsignadoQuedaDeshabilitadoConExplicacion', async () => {
    await openRoles()
    const remove = screen.getByRole('button', { name: 'Eliminar contabilidad.senior' })
    expect(remove).toBeDisabled()
    expect(within(grid()).getByText('Asignado a 1 usuario: quita la asignación antes de eliminarlo.')).toBeInTheDocument()
    expect(within(grid()).getByText('Asignado a 2 usuarios: quita la asignación antes de eliminarlo.')).toBeInTheDocument()
  })

  it('TestRF021_EliminarRolPideConfirmacionYLoQuitaDeLaGrilla', async () => {
    const api = await openRoles({ [`DELETE ${appPath}/${auditorId}`]: () => json(204) })
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar contabilidad.auditor' }))
    expect(api.calls.some((call) => call.key.startsWith('DELETE'))).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar eliminación de contabilidad.auditor' }))
    expect(api.calls.some((call) => call.key.startsWith('DELETE'))).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar contabilidad.auditor' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar eliminación de contabilidad.auditor' }))
    expect(await screen.findByText('Rol contabilidad.auditor eliminado.')).toBeInTheDocument()
    expect(within(grid()).queryByRole('rowheader', { name: 'contabilidad.auditor' })).not.toBeInTheDocument()
  })

  it('TestRF021_MuestraElDetalleDelProblemaDeLaApiEn409', async () => {
    await openRoles({ [`DELETE ${appPath}/${auditorId}`]: () => problem(409, { detail: 'role is still assigned to 1 user' }) })
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar contabilidad.auditor' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar eliminación de contabilidad.auditor' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('role is still assigned to 1 user')
    expect(within(grid()).getByRole('rowheader', { name: 'contabilidad.auditor' })).toBeInTheDocument()
  })

  it('TestRF021_MuestraElDetalleDelProblemaDeLaApiEn403AlGuardar', async () => {
    await openRoles({ [`PATCH ${appPath}/${auditorId}`]: () => problem(403, { detail: 'admins cannot change a role they hold' }) })
    fireEvent.click(box('cierre.ejecutar', 'contabilidad.auditor'))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar contabilidad.auditor' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('admins cannot change a role they hold')
    expect(box('cierre.ejecutar', 'contabilidad.auditor')).toBeChecked()
  })

  it('TestRF021_MuestraErrorSiNoCargaLaListaDeAplicaciones', async () => {
    goTo('/roles')
    stubApi({ ...signedIn(adminUser), 'GET /api/v1/admin/applications': () => problem(500) })
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('No fue posible cargar las aplicaciones')
  })

  it('TestRF021_UnNoAdminNoVeElEnlaceNiLaRutaDeRoles', async () => {
    goTo('/roles')
    const api = stubApi(signedIn(profile()))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Roles' })).not.toBeInTheDocument()
    expect(api.calls.some((call) => call.key.includes('/admin/'))).toBe(false)
  })

  it('TestRF021_AdminVeElEnlaceDeRolesEnElRail', async () => {
    await openRoles()
    expect(screen.getByRole('link', { name: 'Roles' })).toHaveAttribute('href', '/roles')
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Roles y permisos' })).toBeInTheDocument())
  })
})
