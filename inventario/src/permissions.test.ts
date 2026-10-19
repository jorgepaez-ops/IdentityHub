import { describe, expect, it } from 'vitest'
import { grantsFor } from './permissions'

describe('grantsFor', () => {
  it('TestRF021_PermissionsDecideWhatEachSectionAllows', () => {
    const everything = grantsFor(['inventario.bodeguero'], ['productos.ver', 'productos.editar', 'movimientos.registrar'])
    expect(everything).toMatchObject({ view: true, edit: true, move: true, roleTitle: 'Bodeguero' })
    expect(everything?.summary).toBe('Tu rol permite: consultar el catálogo, editar productos, registrar entradas y salidas de bodega.')
    expect(grantsFor(['inventario.consulta'], ['productos.ver'])).toMatchObject({ view: true, edit: false, move: false, roleTitle: 'Consulta de inventario' })
  })

  it('TestRF022_RolesOfOtherApplicationsGiveNoAccessEvenWithPermissions', () => {
    expect(grantsFor(['user', 'contabilidad.senior'], ['movimientos.registrar'])).toBeNull()
    expect(grantsFor([], ['productos.ver'])).toBeNull()
  })

  it('TestRF022_AnInventarioRoleWithoutAnyInventarioPermissionGivesNoAccess', () => {
    expect(grantsFor(['inventario.consulta'], [])).toBeNull()
    expect(grantsFor(['inventario.consulta'], ['facturas.ver'])).toBeNull()
  })

  it('TestRF021_AnUnknownInventarioRoleIsShownByItsOwnName', () => {
    expect(grantsFor(['inventario.auditor', 'user'], ['productos.ver'])?.roleTitle).toBe('inventario.auditor')
  })
})
