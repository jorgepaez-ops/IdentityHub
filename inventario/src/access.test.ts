import { describe, expect, it } from 'vitest'
import { accessFor } from './access'

describe('accessFor: Inventario membership and permissions', () => {
  it('TestRF020_BodegueroGetsEverythingThePermissionsGrant', () => {
    const access = accessFor(['productos.ver', 'productos.editar', 'movimientos.registrar'], ['inventario.bodeguero'])
    expect(access).toMatchObject({ granted: true, canView: true, canEdit: true, canMove: true, label: 'Bodeguero' })
  })

  it('TestRF020_ConsultaOnlyViews', () => {
    const access = accessFor(['productos.ver'], ['inventario.consulta'])
    expect(access).toMatchObject({ granted: true, canView: true, canEdit: false, canMove: false })
  })

  it('TestRF020_NoInventarioRoleMeansNoAccessEvenWithPermissions', () => {
    expect(accessFor(['productos.ver'], ['user', 'contabilidad.senior']).granted).toBe(false)
  })

  it('TestRF020_InventarioRoleWithoutInventarioPermissionsMeansNoAccess', () => {
    expect(accessFor([], ['inventario.consulta']).granted).toBe(false)
    expect(accessFor(['movimientos.aprobar', 'reportes.ver'], ['inventario.consulta']).granted).toBe(false)
  })
})
