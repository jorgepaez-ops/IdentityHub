import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { App } from './App'
import * as browser from './browser'
import { HUB_ORIGIN } from './hub'
import { startAuthorization } from './oauth/hubLogin'
import { MIN_REMAINING_MS } from './session'
import { installFakeHubEnvironment } from './testkit/hubEnvironment'

const VIEW_ONLY = ['productos.ver']
const FULL = ['productos.ver', 'productos.editar', 'movimientos.registrar']

const fakeHub = installFakeHubEnvironment()

const tabs = () => screen.getByRole('navigation', { name: 'Secciones' })
const openSection = (name: string) => fireEvent.click(within(tabs()).getByRole('button', { name }))

/** Plays the round trip: leave for the Hub, then come back on the callback URL with what the Hub would send. */
async function comeBackWith(returned: (state: string) => string, token: Record<string, unknown> = {}) {
  await startAuthorization()
  const state = sessionStorage.getItem('inventario.login.state') ?? ''
  const seen = fakeHub().serveToken(token)
  window.history.replaceState(null, '', `/oauth/callback?${returned(state)}`)
  render(<App />)
  return seen
}

/** The sign-in screen after a failure: the alert carries `text`, the spinner is gone and retrying is offered. */
async function expectFailureScreen(text: string, retry = true) {
  expect(await screen.findByRole('alert')).toHaveTextContent(text)
  expect(screen.queryByText(/Validando/)).toBeNull()
  if (retry) expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
}

const signInWith = (roles: string[], permissions: string[]) =>
  comeBackWith((state) => `code=the-code&state=${state}`, { roles, permissions })

async function signedInAs(roles: string[], permissions: string[]) {
  await signInWith(roles, permissions)
  await screen.findByRole('navigation', { name: 'Secciones' })
}

describe('sign-in screen', () => {
  it('TestRF020_OffersOnlyContinueWithIdentityHub', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Inventario' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
    expect(document.querySelector('input')).toBeNull()
  })

  it('TestRF020_ContinuingSendsThePersonToTheHubWithPkce', async () => {
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'Continuar con Identity Hub' }))
    await waitFor(() => expect(browser.goTo).toHaveBeenCalledTimes(1))
    const target = new URL(String(vi.mocked(browser.goTo).mock.calls[0]?.[0]))
    expect(target.searchParams.get('client_id')).toBe('inventario')
    expect(target.searchParams.get('code_challenge_method')).toBe('S256')
  })

  it('TestRF020_ARedirectThatCannotStartEndsInAnErrorScreenNotASpinner', async () => {
    vi.mocked(browser.goTo).mockImplementation(() => { throw new Error('navigation blocked') })
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'Continuar con Identity Hub' }))
    await expectFailureScreen('No se pudo completar el inicio de sesión')
  })
})

describe('returning from the Hub', () => {
  it('TestRF020_WhileTheTokenIsChecked_TheScreenSaysSo', async () => {
    vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>(() => undefined)))
    window.history.replaceState(null, '', '/oauth/callback?code=c&state=s')
    sessionStorage.setItem('inventario.login.state', 's')
    sessionStorage.setItem('inventario.login.verifier', 'v')
    render(<App />)
    expect(screen.getByText(/Validando con Identity Hub/)).toBeInTheDocument()
  })

  it('TestRF020_ANetworkFailureEndsInAnErrorScreenWithRetry', async () => {
    await startAuthorization()
    const state = sessionStorage.getItem('inventario.login.state')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    window.history.replaceState(null, '', `/oauth/callback?code=c&state=${state}`)
    render(<App />)
    await expectFailureScreen('No se pudo completar el inicio de sesión')
  })

  it('TestRF020_AForgedStateEndsInAnErrorScreen', async () => {
    await comeBackWith(() => 'code=c&state=forged')
    await expectFailureScreen('no corresponde a esta solicitud', false)
  })

  it('TestRF020_ATokenWithLessThanTheLeewayLeftIsTreatedAsAClockProblem', async () => {
    const exp = Math.floor(Date.now() / 1000) + MIN_REMAINING_MS / 1000 - 30
    await comeBackWith((state) => `code=c&state=${state}`, { exp })
    expect(await screen.findByRole('alert')).toHaveTextContent('reloj')
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).toBeNull()
  })

  it('TestRF020_ATokenWithMoreThanTheLeewayLeftIsAccepted', async () => {
    const exp = Math.floor(Date.now() / 1000) + MIN_REMAINING_MS / 1000 + 60
    await comeBackWith((state) => `code=c&state=${state}`, { exp })
    expect(await screen.findByRole('navigation', { name: 'Secciones' })).toBeInTheDocument()
  })
})

describe('no access', () => {
  it('TestRF022_AccessDeniedFromTheHubShowsNoAccessWithoutRequestingATokenOrKeys', async () => {
    const seen = await comeBackWith((state) => `error=access_denied&state=${state}`)
    expect(await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })).toBeInTheDocument()
    expect(seen).toHaveLength(0)
    expect(screen.getByRole('link', { name: 'Ir a Identity Hub' })).toHaveAttribute('href', HUB_ORIGIN)
  })

  it('TestRF022_ATokenWithoutInventarioPermissionsShowsNoAccess', async () => {
    await signInWith(['inventario.consulta'], [])
    expect(await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).toBeNull()
  })

  it('TestRF022_RolesOfAnotherApplicationShowNoAccess', async () => {
    await signInWith(['user', 'contabilidad.senior'], ['movimientos.registrar'])
    expect(await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })).toBeInTheDocument()
  })

  it('TestRF020_SigningOutFromNoAccessReturnsToTheSignInScreen', async () => {
    await signInWith([], [])
    await screen.findByRole('heading', { name: 'Sin acceso a Inventario' })
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(await screen.findByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
  })
})

describe('sections gated by permissions', () => {
  it('TestRF021_ViewOnlyUsersSeeTheCatalogWithoutEditingOrMovements', async () => {
    await signedInAs(['inventario.consulta'], VIEW_ONLY)
    expect(screen.getByRole('heading', { name: 'Catálogo' })).toBeInTheDocument()
    expect(within(screen.getByRole('table', { name: 'Productos' })).getAllByRole('row').length).toBeGreaterThan(1)
    expect(screen.queryByRole('button', { name: /^Editar / })).toBeNull()
    expect(within(tabs()).getByRole('button', { name: /Movimientos/ })).toBeDisabled()
    expect(screen.getAllByText(/Empresa B/).length).toBeGreaterThan(0)
    expect(screen.getByText('Consulta de inventario')).toBeInTheDocument()
  })

  it('TestRF021_AWarehouseKeeperEditsAProduct', async () => {
    await signedInAs(['inventario.bodeguero'], FULL)
    fireEvent.click(screen.getByRole('button', { name: 'Editar P-1001' }))
    const form = screen.getByRole('form', { name: 'Editar P-1001' })
    fireEvent.change(within(form).getByLabelText('Nombre'), { target: { value: 'Tornillo M8 reforzado' } })
    fireEvent.change(within(form).getByLabelText('Precio unitario (COP)'), { target: { value: '30000' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Guardar cambios' }))
    expect(screen.getByText('Tornillo M8 reforzado')).toBeInTheDocument()
    expect(screen.queryByRole('form', { name: 'Editar P-1001' })).toBeNull()
    expect(screen.getByRole('status')).toHaveTextContent('Producto P-1001 actualizado (datos de ejemplo).')
  })

  it('TestRF021_CancellingAnEditLeavesTheProductAlone', async () => {
    await signedInAs(['inventario.bodeguero'], FULL)
    fireEvent.click(screen.getByRole('button', { name: 'Editar P-1002' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    expect(screen.queryByRole('form', { name: 'Editar P-1002' })).toBeNull()
    expect(screen.getByText('Cinta de embalaje 48 mm')).toBeInTheDocument()
  })

  it('TestRF021_SearchNarrowsTheCatalogAndSaysWhenNothingMatches', async () => {
    await signedInAs(['inventario.consulta'], VIEW_ONLY)
    const box = screen.getByLabelText('Buscar producto')
    fireEvent.change(box, { target: { value: 'casco' } })
    expect(within(screen.getByRole('table', { name: 'Productos' })).getAllByRole('row')).toHaveLength(2)
    fireEvent.change(box, { target: { value: 'zzz' } })
    expect(screen.getByText('Ningún producto coincide con la búsqueda.')).toBeInTheDocument()
  })

  it('TestRF021_ARegisteredExitLowersTheStock', async () => {
    await signedInAs(['inventario.bodeguero'], FULL)
    openSection('Movimientos')
    const form = screen.getByRole('form', { name: 'Registrar movimiento' })
    fireEvent.change(within(form).getByLabelText('Producto'), { target: { value: 'P-1004' } })
    fireEvent.change(within(form).getByLabelText('Tipo'), { target: { value: 'out' } })
    fireEvent.change(within(form).getByLabelText('Cantidad'), { target: { value: '4' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Registrar movimiento' }))
    expect(screen.getByText('MV-004')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Salida de 4 unidades registrada.')
    openSection('Catálogo')
    const row = screen.getByText('P-1004').closest('tr') as HTMLElement
    expect(within(row).getByText('10')).toBeInTheDocument()
  })

  it('TestRF021_ALateEveningMovementIsDatedWithTheLocalDay', async () => {
    await signedInAs(['inventario.bodeguero'], FULL)
    // 23:30 local time (Bogota, UTC-5, see vite.config.ts) is already the next day in UTC.
    vi.useFakeTimers({ toFake: ['Date'], now: new Date(2026, 9, 19, 23, 30) })
    try {
      openSection('Movimientos')
      const form = screen.getByRole('form', { name: 'Registrar movimiento' })
      fireEvent.change(within(form).getByLabelText('Cantidad'), { target: { value: '1' } })
      fireEvent.click(within(form).getByRole('button', { name: 'Registrar movimiento' }))
      const row = screen.getByText('MV-004').closest('tr') as HTMLElement
      expect(within(row).getByText('2026-10-19')).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('TestRF021_AnExitLargerThanTheStockIsRefused', async () => {
    await signedInAs(['inventario.bodeguero'], FULL)
    openSection('Movimientos')
    const form = screen.getByRole('form', { name: 'Registrar movimiento' })
    fireEvent.change(within(form).getByLabelText('Tipo'), { target: { value: 'out' } })
    fireEvent.change(within(form).getByLabelText('Cantidad'), { target: { value: '9999' } })
    fireEvent.click(within(form).getByRole('button', { name: 'Registrar movimiento' }))
    expect(screen.getByRole('status')).toHaveTextContent('No hay existencias suficientes para esa salida.')
    expect(screen.queryByText('MV-004')).toBeNull()
  })

  it('TestRF021_AMovementsOnlyUserStartsOnMovementsWithTheCatalogLocked', async () => {
    await signedInAs(['inventario.bodeguero'], ['movimientos.registrar'])
    expect(screen.getByRole('heading', { name: 'Movimientos' })).toBeInTheDocument()
    expect(within(tabs()).getByRole('button', { name: /Catálogo/ })).toBeDisabled()
  })

  it('TestRF021_AnEditOnlyUserSeesLockedContentInsteadOfTheCatalog', async () => {
    await signedInAs(['inventario.bodeguero'], ['productos.editar'])
    expect(screen.queryByRole('table', { name: 'Productos' })).toBeNull()
    expect(screen.getByText(/no incluye la consulta del catálogo/)).toBeInTheDocument()
  })

  it('TestRF020_DemoDataIsLabelledAsSuch', async () => {
    await signedInAs(['inventario.consulta'], VIEW_ONLY)
    expect(screen.getByText(/Datos de ejemplo/)).toBeInTheDocument()
  })

  it('TestRF020_SigningOutReturnsToTheSignInScreen', async () => {
    await signedInAs(['inventario.consulta'], VIEW_ONLY)
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(await screen.findByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
  })
})
