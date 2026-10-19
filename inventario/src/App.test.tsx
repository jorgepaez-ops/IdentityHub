import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { beginLogin } from './auth/flow'
import { AppearanceProvider } from './AppearanceProvider'
import { HUB_ORIGIN } from './config'
import * as navigation from './navigation'
import { ThemeToggle } from './ThemeToggle'
import { makeKey, signToken, validClaims, type TestKey } from './testing'

const HUB = HUB_ORIGIN
let key: TestKey

const CONSULTA = ['productos.ver']
const BODEGUERO = ['productos.ver', 'productos.editar', 'movimientos.registrar']

beforeAll(async () => { key = await makeKey('hub-key') })

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
  vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined)
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  window.history.replaceState({}, '', '/')
  document.documentElement.removeAttribute('data-theme')
})

function stubHub(claims: Record<string, unknown>) {
  const nowSeconds = Math.floor(Date.now() / 1000)
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === `${HUB}/oauth/token`) {
      const access = await signToken(key, validClaims({ iat: nowSeconds, exp: nowSeconds + 900, ...claims }))
      return new Response(JSON.stringify({ access_token: access, token_type: 'Bearer', expires_in: 900 }), { status: 200 })
    }
    if (url === `${HUB}/.well-known/jwks.json`) return new Response(JSON.stringify({ keys: [key.jwk] }), { status: 200 })
    throw new Error(`unexpected request ${url}`)
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

/** Plays the round trip: start the flow, then land on the callback with the stored state. */
async function signInAs(roles: string[], permissions: string[], landing: 'shell' | 'denied' = 'shell') {
  await beginLogin()
  const state = sessionStorage.getItem('inventario.oauth_state')
  stubHub({ roles, permissions })
  window.history.replaceState({}, '', `/oauth/callback?code=the-code&state=${state}`)
  render(<App />)
  if (landing === 'shell') await screen.findByRole('navigation', { name: 'Secciones' })
  else await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })
}

const rail = () => screen.getByRole('navigation', { name: 'Secciones' })
const goTo = (name: string) => fireEvent.click(within(rail()).getByRole('button', { name }))

describe('login screen', () => {
  it('TestRF020_LoginOffersOnlyContinueWithIdentityHub', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Inventario' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
    expect(document.querySelector('input')).toBeNull()
  })

  it('TestRF020_ContinueStartsTheAuthorizationRedirect', async () => {
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'Continuar con Identity Hub' }))
    await waitFor(() => expect(navigation.redirectTo).toHaveBeenCalledTimes(1))
    const target = new URL(String(vi.mocked(navigation.redirectTo).mock.calls[0]?.[0]))
    expect(target.searchParams.get('client_id')).toBe('inventario')
    expect(target.searchParams.get('code_challenge_method')).toBe('S256')
  })
})

describe('no access', () => {
  it('TestRF022_AccessDeniedFromTheHubShowsNoAccessWithoutRequestingATokenOrJwks', async () => {
    await beginLogin()
    const state = sessionStorage.getItem('inventario.oauth_state')
    const fetchMock = stubHub({})
    window.history.replaceState({}, '', `/oauth/callback?error=access_denied&state=${state}`)
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })).toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(screen.getByRole('link', { name: 'Ir a Identity Hub' })).toHaveAttribute('href', HUB)
  })

  it('TestRF022_ATokenWithoutInventarioPermissionsShowsNoAccess', async () => {
    await signInAs(['inventario.consulta'], [], 'denied')
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).toBeNull()
  })

  it('TestRF022_ATokenWithRolesOfAnotherApplicationShowsNoAccess', async () => {
    await signInAs(['user', 'contabilidad.senior'], ['movimientos.registrar'], 'denied')
  })

  it('TestRF020_LoggingOutFromNoAccessReturnsToLogin', async () => {
    await signInAs([], [], 'denied')
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(await screen.findByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
  })
})

describe('screens gated by permissions', () => {
  it('TestRF021_ConsultaSeesTheCatalogWithoutEditOrMovements', async () => {
    await signInAs(['inventario.consulta'], CONSULTA)
    expect(screen.getByRole('heading', { name: 'Catálogo' })).toBeInTheDocument()
    expect(within(screen.getByRole('table', { name: 'Productos' })).getAllByRole('row').length).toBeGreaterThan(1)
    expect(screen.queryByRole('button', { name: /^Editar / })).toBeNull()
    expect(within(rail()).getByRole('button', { name: /Movimientos/ })).toBeDisabled()
    // The header names the user and the company.
    expect(screen.getAllByText(/Empresa B/).length).toBeGreaterThan(0)
    expect(screen.getByText('Consulta de inventario')).toBeInTheDocument()
  })

  it('TestRF021_BodegueroEditsAProductInTheCatalog', async () => {
    await signInAs(['inventario.bodeguero'], BODEGUERO)
    fireEvent.click(screen.getByRole('button', { name: 'Editar P-1001' }))
    const form = screen.getByRole('form', { name: 'Editar P-1001' })
    fireEvent.change(within(form).getByLabelText('Nombre'), { target: { value: 'Tornillo M8 reforzado' } })
    fireEvent.change(within(form).getByLabelText('Precio unitario (COP)'), { target: { value: '30000' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Guardar cambios' }))
    expect(screen.getByText('Tornillo M8 reforzado')).toBeInTheDocument()
    expect(screen.queryByRole('form', { name: 'Editar P-1001' })).toBeNull()
    expect(screen.getByText('Producto P-1001 actualizado (datos de ejemplo).')).toBeInTheDocument()
  })

  it('TestRF021_BodegueroRegistersAMovementThatChangesTheStock', async () => {
    await signInAs(['inventario.bodeguero'], BODEGUERO)
    goTo('Movimientos')
    const form = screen.getByRole('form', { name: 'Registrar movimiento' })
    fireEvent.change(within(form).getByLabelText('Producto'), { target: { value: 'P-1004' } })
    fireEvent.change(within(form).getByLabelText('Tipo'), { target: { value: 'out' } })
    fireEvent.change(within(form).getByLabelText('Cantidad'), { target: { value: '4' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Registrar movimiento' }))
    expect(screen.getByText('MV-004')).toBeInTheDocument()
    expect(screen.getByText('Salida de 4 unidades registrada.')).toBeInTheDocument()
    goTo('Catálogo')
    const row = screen.getByText('P-1004').closest('tr') as HTMLElement
    expect(within(row).getByText('10')).toBeInTheDocument()
  })

  it('TestRF021_AnExitLargerThanTheStockIsRefused', async () => {
    await signInAs(['inventario.bodeguero'], BODEGUERO)
    goTo('Movimientos')
    const form = screen.getByRole('form', { name: 'Registrar movimiento' })
    fireEvent.change(within(form).getByLabelText('Tipo'), { target: { value: 'out' } })
    fireEvent.change(within(form).getByLabelText('Cantidad'), { target: { value: '9999' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Registrar movimiento' }))
    expect(screen.getByText('No hay existencias suficientes para esa salida.')).toBeInTheDocument()
    expect(screen.queryByText('MV-004')).toBeNull()
  })

  it('TestRF021_AMovementsOnlyUserStartsOnMovementsAndHasTheCatalogLocked', async () => {
    await signInAs(['inventario.bodeguero'], ['movimientos.registrar'])
    expect(screen.getByRole('heading', { name: 'Movimientos' })).toBeInTheDocument()
    expect(within(rail()).getByRole('button', { name: /Catálogo/ })).toBeDisabled()
  })

  it('TestRF021_AnEditOnlyUserSeesLockedContentInsteadOfTheCatalog', async () => {
    await signInAs(['inventario.bodeguero'], ['productos.editar'])
    expect(screen.queryByRole('table', { name: 'Productos' })).toBeNull()
    expect(screen.getByText(/no incluye la consulta del catálogo/)).toBeInTheDocument()
  })

  it('TestRF020_DemoDataIsLabelledAsSuch', async () => {
    await signInAs(['inventario.consulta'], CONSULTA)
    expect(screen.getByText(/Datos de ejemplo/)).toBeInTheDocument()
  })
})

describe('appearance', () => {
  it('TestRF021_ToggleSwitchesThemeAndRemembersIt', () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined })))
    render(<AppearanceProvider><ThemeToggle /></AppearanceProvider>)
    expect(document.documentElement.dataset.theme).toBe('light')
    act(() => { fireEvent.click(screen.getByRole('button', { name: 'Cambiar a tema oscuro' })) })
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(localStorage.getItem('inventario.theme')).toBe('dark')
  })
})
