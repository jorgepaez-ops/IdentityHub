# 0013 — Roles y permisos configurables por aplicación

Estado: aceptada · 2026-10-05 · Enmienda RF-009, RF-020 y añade RF-021

## Contexto

El catálogo actual de roles es una convención fija en
`backend/internal/auth/roles/roles.go`: `admin`, `user`, `contabilidad.senior` y
`contabilidad.analista`. OpenAPI refleja esa lista como un `enum` cerrado y la validación de
administración la rechaza fuera de ese catálogo. Los permisos no son datos del Hub: solo existe
un mapeo implícito, de presentación, en `contabilidad/src/access.ts`.

La grilla aprobada en D9 requiere que cada aplicación declare sus permisos y que un administrador
arme roles sin desplegar código. La migración vigente `000002_identity_app_audit_permissions.up.sql`
solo concede `SELECT` sobre `roles` a `identity_app`; no le concede `INSERT`, `UPDATE` ni `DELETE`
sobre esa tabla. Por tanto, hoy no dispone de privilegios de escritura sobre un modelo de roles
configurable. Esta decisión especifica el contrato para T12-T15 y preserva los roles de directorio
como control del propio Hub.

## Decisión

### Modelo y migración inicial

- `permissions` pertenece a `applications`; una migración siembra las aplicaciones y sus permisos.
- `roles.application_id` es nullable. `NULL` identifica roles de directorio; una aplicación no nula
  identifica un rol de aplicación. El nombre de estos últimos es `<aplicacion>.<nombre>`.
- `role_permissions` relaciona roles de aplicación con permisos de su misma aplicación.
- Los roles de directorio `admin` y `user` llevan una marca de sistema y no pueden crearse,
  actualizarse ni eliminarse mediante la grilla. `user` continúa siendo obligatorio para cada
  cuenta.
- La migración convierte `contabilidad.senior` y `contabilidad.analista` en filas de Contabilidad
  con sus permisos, sin cambiar su comportamiento actual.

| Rol actual | Permisos sembrados |
|---|---|
| `contabilidad.senior` | `movimientos.registrar`, `movimientos.ver_todos`, `movimientos.aprobar`, `cierre.ejecutar`, `reportes.ver` |
| `contabilidad.analista` | `movimientos.registrar`, `reportes.ver` |

Las claves declaradas para Contabilidad son `movimientos.registrar`, `movimientos.ver_todos`,
`movimientos.aprobar`, `cierre.ejecutar` y `reportes.ver`. T14 sustituirá su mapeo por nombre de rol por estas claves.
Todo token de Contabilidad cuyo titular tenga al menos un rol de esa aplicación conserva la
visibilidad base de sus propios movimientos; no se introduce una quinta clave para esa condición de
acceso. `movimientos.ver_todos` eleva esa base a todos los movimientos. Las capacidades adicionales
se gobiernan por los permisos declarados, y `reportes.ver` conserva el Resumen para los roles
sembrados. La siguiente tabla explicita la conducta de presentación existente que debe conservarse;
no describe un control de servidor, porque Contabilidad todavía no tiene backend.

| Clave | Capacidad concreta actual | Consumidores actuales |
|---|---|---|
| `movimientos.registrar` | Muestra el botón «Registrar movimiento» y su formulario. Hoy no depende de ningún permiso: lo tiene cualquier titular de un rol de la aplicación; se agrega esta clave (sembrada en senior y analista) para que un rol de solo lectura, como `contabilidad.auditor`, no pueda registrar. Ajuste del 2026-10-05 sobre D9. | `views/Transactions.tsx` |
| `movimientos.ver_todos` | Eleva la visibilidad base de movimientos propios a todos los movimientos: el nivel `full` hace que `visibleMovements` devuelva todas las filas en Transacciones y Resumen. Hoy lo concede únicamente `contabilidad.senior`. | `contabilidad/src/access.ts`, `ledger.ts`, `views/Transactions.tsx` y `views/Summary.tsx` |
| `movimientos.aprobar` | `canApprove` muestra las acciones Aprobar y Rechazar para movimientos pendientes; sin él se muestra “esperando aprobación”. Hoy lo concede únicamente `contabilidad.senior`. | `contabilidad/src/access.ts` y `views/Transactions.tsx` |
| `cierre.ejecutar` | `canClose` desbloquea la navegación de Cierre contable y el botón que registra el cierre de ejemplo. Hoy lo concede únicamente `contabilidad.senior`. | `contabilidad/src/access.ts`, `Shell.tsx` y `views/Closing.tsx` |
| `reportes.ver` | El Resumen muestra totales y conteos del período. No existe un flag de permiso separado: hoy se renderiza para cualquier `level` distinto de `none`, incluidos senior y analista. Al sembrarlo en ambos roles se conserva esa visibilidad; T14 la hará depender de este permiso. | `contabilidad/src/access.ts` y `views/Summary.tsx` |

Estas comprobaciones son solo de presentación y datos de ejemplo: el cliente no puede proteger una
operación de negocio frente a un actor que evada la UI. Un futuro backend de Contabilidad debe
revalidar las mismas claves en servidor.

### Token y administración

El access token de una aplicación conserva `roles` con solo sus roles de aplicación y añade el
claim `permissions`, unión resuelta de los permisos de esos roles para `aud`. El cambio de una
grilla se observa al emitir el siguiente token; los tokens ya emitidos pueden conservar el estado
anterior durante un máximo de 15 minutos.

Todos los endpoints de la grilla requieren `admin`:

| Operación | Endpoint propuesto | Resultado |
|---|---|---|
| Listar aplicaciones y permisos | `GET /api/v1/admin/applications` | aplicaciones con su catálogo de permisos y roles configurables |
| Listar roles de una aplicación | `GET /api/v1/admin/applications/{applicationId}/roles` | página o arreglo de roles de esa aplicación |
| Crear rol | `POST /api/v1/admin/applications/{applicationId}/roles` | `201` con `name` y `permissionKeys` |
| Actualizar rol | `PATCH /api/v1/admin/applications/{applicationId}/roles/{roleId}` | `200` con el rol actualizado |
| Eliminar rol | `DELETE /api/v1/admin/applications/{applicationId}/roles/{roleId}` | `204` |

Los cuerpos de creación y actualización usan `name` y `permissionKeys`; las respuestas incluyen
`id`, `name`, `applicationId`, `permissionKeys`, `system` y marcas temporales cuando correspondan.
`400` comunica nombre inválido, permiso desconocido o permiso de otra aplicación; `401` falta de
sesión, `403` falta de `admin` o intento de modificar permisos de un rol del actor, `404` recurso
inexistente y `409` eliminación de un rol asignado.

`identity_app` recibe los `GRANT` mínimos para leer y escribir `permissions`, roles de aplicación y
`role_permissions`, y para consultar sus asignaciones al validar el borrado. No recibe capacidad
de modificar los roles del sistema fuera de las rutas y restricciones de la aplicación.

Crear, actualizar y eliminar un rol registra respectivamente `role_created`, `role_updated` y
`role_deleted` en el audit log, con actor y recurso. Las asignaciones de roles siguen usando el
evento existente `role_changed`.

### Controles obligatorios

1. Solo se crean roles de aplicación; `admin` y `user` no se editan ni borran.
2. Un administrador no modifica los permisos de un rol que posee.
3. Un rol solo referencia permisos de su propia aplicación; uno desconocido o de otra aplicación
   devuelve `400`.
4. Un rol asignado no se elimina.
5. Cada cambio de rol se audita.

El control 1 evita que la grilla altere la autoridad administrativa del directorio o quite el rol
base. El control 2 separa la administración del privilegio propio: sin él, un administrador con un
rol de aplicación podría ampliarlo y obtener capacidad no aprobada. Los controles 3 a 5 preservan
el aislamiento por audiencia, la integridad de asignaciones y la trazabilidad.

## Cambios de OpenAPI propuestos

T12 aplica estos fragmentos; esta ADR no modifica `specs/03-api/openapi.yaml`.

```yaml
/api/v1/admin/applications:
  get:
    operationId: listApplications
    tags: [admin]
    summary: Listar aplicaciones, permisos y roles configurables
    x-requirement: RF-021
    responses:
      '200':
        description: Aplicaciones con su catálogo declarativo
        content:
          application/json:
            schema:
              type: array
              items: { $ref: '#/components/schemas/Application' }
      '401': { $ref: '#/components/responses/Unauthorized' }
      '403': { $ref: '#/components/responses/Forbidden' }

/api/v1/admin/applications/{applicationId}/roles:
  get:
    operationId: listApplicationRoles
    tags: [admin]
    x-requirement: RF-021
    parameters:
      - { $ref: '#/components/parameters/ApplicationId' }
    responses:
      '200':
        description: Roles configurables de la aplicación
        content:
          application/json:
            schema:
              type: array
              items: { $ref: '#/components/schemas/ApplicationRole' }
      '401': { $ref: '#/components/responses/Unauthorized' }
      '403': { $ref: '#/components/responses/Forbidden' }
      '404': { $ref: '#/components/responses/NotFound' }
  post:
    operationId: createApplicationRole
    tags: [admin]
    x-requirement: RF-021
    parameters:
      - { $ref: '#/components/parameters/ApplicationId' }
    requestBody:
      required: true
      content:
        application/json:
          schema: { $ref: '#/components/schemas/CreateApplicationRoleRequest' }
    responses:
      '201':
        description: Rol de aplicación creado
        content:
          application/json:
            schema: { $ref: '#/components/schemas/ApplicationRole' }
      '400': { $ref: '#/components/responses/BadRequest' }
      '401': { $ref: '#/components/responses/Unauthorized' }
      '403': { $ref: '#/components/responses/Forbidden' }
      '404': { $ref: '#/components/responses/NotFound' }

/api/v1/admin/applications/{applicationId}/roles/{roleId}:
  patch:
    operationId: updateApplicationRole
    tags: [admin]
    x-requirement: RF-021
    parameters:
      - { $ref: '#/components/parameters/ApplicationId' }
      - { $ref: '#/components/parameters/RoleId' }
    requestBody:
      required: true
      content:
        application/json:
          schema: { $ref: '#/components/schemas/UpdateApplicationRoleRequest' }
    responses:
      '200':
        description: Rol de aplicación actualizado
        content:
          application/json:
            schema: { $ref: '#/components/schemas/ApplicationRole' }
      '400': { $ref: '#/components/responses/BadRequest' }
      '401': { $ref: '#/components/responses/Unauthorized' }
      '403': { $ref: '#/components/responses/Forbidden' }
      '404': { $ref: '#/components/responses/NotFound' }
  delete:
    operationId: deleteApplicationRole
    tags: [admin]
    x-requirement: RF-021
    parameters:
      - { $ref: '#/components/parameters/ApplicationId' }
      - { $ref: '#/components/parameters/RoleId' }
    responses:
      '204': { description: Rol de aplicación eliminado }
      '401': { $ref: '#/components/responses/Unauthorized' }
      '403': { $ref: '#/components/responses/Forbidden' }
      '404': { $ref: '#/components/responses/NotFound' }
      '409': { description: El rol tiene asignaciones activas }

components:
  parameters:
    ApplicationId:
      name: applicationId
      in: path
      required: true
      schema: { type: string, format: uuid }
    RoleId:
      name: roleId
      in: path
      required: true
      schema: { type: string, format: uuid }
  schemas:
    Role:
      type: string
      minLength: 3
      maxLength: 127
      pattern: '^(admin|user|[a-z][a-z0-9_-]{0,62}\.[a-z][a-z0-9_-]{0,62})$'
      description: Rol de directorio (`admin` o `user`) o rol configurable `<aplicacion>.<nombre>`.
    ApplicationRoleName:
      allOf:
        - $ref: '#/components/schemas/Role'
        - type: string
          pattern: '^[a-z][a-z0-9_-]{0,62}\.[a-z][a-z0-9_-]{0,62}$'
      description: Variante de Role exclusiva de una aplicación; no admite roles de directorio.
    Permission:
      type: object
      required: [key, applicationId]
      properties:
        key: { type: string, example: movimientos.ver_todos }
        applicationId: { type: string, format: uuid }
    Application:
      type: object
      required: [id, key, permissions]
      properties:
        id: { type: string, format: uuid }
        key: { type: string, example: contabilidad }
        permissions:
          type: array
          items: { $ref: '#/components/schemas/Permission' }
    ApplicationRole:
      type: object
      required: [id, name, applicationId, permissionKeys, system]
      properties:
        id: { type: string, format: uuid }
        name: { $ref: '#/components/schemas/ApplicationRoleName', example: contabilidad.auditor }
        applicationId: { type: string, format: uuid }
        permissionKeys:
          type: array
          items: { type: string }
        system: { type: boolean }
    CreateApplicationRoleRequest:
      type: object
      required: [name, permissionKeys]
      properties:
        name: { $ref: '#/components/schemas/ApplicationRoleName', example: contabilidad.auditor }
        permissionKeys:
          type: array
          uniqueItems: true
          items: { type: string }
    UpdateApplicationRoleRequest:
      type: object
      properties:
        name: { $ref: '#/components/schemas/ApplicationRoleName', example: contabilidad.auditor }
        permissionKeys:
          type: array
          uniqueItems: true
          items: { type: string }
```


## Escenarios de aceptación diferidos

La correspondencia uno a uno de `scripts/traceability.py` exige que cada escenario `@RF-021` tenga
su prueba E2E literal. Como T15 crea esas pruebas y T11 no puede crear archivos en `e2e/tests/`,
los escenarios quedan aquí hasta entonces; no se crea todavía un `.feature` que rompería ese gate.

```gherkin
@RF-021
Escenario: Admin crea auditor de Contabilidad con capacidades acotadas
  Dado un administrador autenticado y la aplicación "contabilidad"
  Cuando crea el rol "contabilidad.auditor" con "reportes.ver" y "movimientos.ver_todos"
  Y asigna ese rol a un empleado
  Entonces el empleado ve todos los movimientos y el Resumen
  Pero no puede registrar, aprobar movimientos ni ejecutar el cierre
  Y el token de Contabilidad contiene solo "reportes.ver" y "movimientos.ver_todos"

@RF-021
Escenario: Se rechaza crear un rol de directorio desde la grilla
  Dado un administrador autenticado
  Cuando intenta crear el rol "admin"
  Entonces recibe 400 o 403 y el rol de directorio no cambia

@RF-021
Escenario: Un administrador no edita permisos de un rol que posee
  Dado un administrador que tiene el rol "contabilidad.auditor"
  Cuando intenta actualizar sus permisos
  Entonces recibe 403 y el rol no cambia

@RF-021
Escenario: Se rechaza un permiso de otra aplicación
  Dado un administrador autenticado y un rol de "contabilidad"
  Cuando intenta asignarle un permiso de otra aplicación
  Entonces recibe 400 y el rol no cambia

@RF-021
Escenario: No se elimina un rol asignado
  Dado un rol de aplicación asignado a un empleado
  Cuando un administrador intenta eliminarlo
  Entonces recibe 409 y el rol permanece asignado

@RF-021
Escenario: Cada cambio de rol queda auditado
  Dado un administrador autenticado
  Cuando crea, actualiza y elimina un rol de aplicación no asignado
  Entonces el audit log contiene "role_created", "role_updated" y "role_deleted"
```

## Consecuencias

- `Role` deja de ser un `enum` cerrado en OpenAPI; los tipos generados y `roles.Catalog` cambian en
  T12. La validación pasa al almacenamiento y a la aplicación propietaria del rol.
- Durante un máximo de 15 minutos, los tokens ya emitidos pueden mostrar permisos anteriores.
- Contabilidad solo puede aplicar los permisos en presentación porque no tiene backend propio; un
  servicio de negocio futuro debe repetir la autorización en servidor.
- **Inconsistencia actual código↔D9:** `contabilidad/src/access.ts` acopla la visibilidad base y las
  capacidades adicionales a los nombres `contabilidad.senior` y `contabilidad.analista`. T14 debe
  separar la presencia de un rol de aplicación (movimientos propios) de las capacidades elevadas
  gobernadas por permisos, preservando los permisos sembrados de ambos roles.
- Las constantes y la configuración de cliente único de RF-020 se conservan: esta decisión no
  implementa registro dinámico de clientes ni modifica OAuth.
