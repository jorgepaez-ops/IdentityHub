# language: es
Característica: Roles y permisos configurables por aplicación
  Para que cada aplicación tenga roles a la medida de su negocio
  Como administrador de la plataforma
  Quiero armar roles de aplicación marcando permisos en una grilla

  @RF-021 @p1
  Escenario: Admin crea auditor de Contabilidad con capacidades acotadas
    Dado un administrador autenticado y la aplicación "contabilidad"
    Cuando crea el rol "contabilidad.auditor" con "reportes.ver" y "movimientos.ver_todos"
    Y asigna ese rol a un empleado
    Entonces el empleado ve todos los movimientos y el Resumen
    Pero no puede registrar, aprobar movimientos ni ejecutar el cierre
    Y el token de Contabilidad contiene solo "reportes.ver" y "movimientos.ver_todos"

  @RF-021 @p1
  Escenario: Se rechaza crear un rol de directorio desde la grilla
    Dado un administrador autenticado
    Cuando intenta crear el rol "admin"
    Entonces recibe 400 o 403 y el rol de directorio no cambia

  @RF-021 @p1
  Escenario: Un administrador no edita permisos de un rol que posee
    Dado un administrador que tiene el rol "contabilidad.auditor"
    Cuando intenta actualizar sus permisos
    Entonces recibe 403 y el rol no cambia

  @RF-021 @p1
  Escenario: Se rechaza un permiso de otra aplicación
    Dado un administrador autenticado y un rol de "contabilidad"
    Cuando intenta asignarle un permiso de otra aplicación
    Entonces recibe 400 y el rol no cambia

  @RF-021 @p1
  Escenario: No se elimina un rol asignado
    Dado un rol de aplicación asignado a un empleado
    Cuando un administrador intenta eliminarlo
    Entonces recibe 409 y el rol permanece asignado

  @RF-021 @p1
  Escenario: Cada cambio de rol queda auditado
    Dado un administrador autenticado
    Cuando crea, actualiza y elimina un rol de aplicación no asignado
    Entonces el audit log contiene "role_created", "role_updated" y "role_deleted"
