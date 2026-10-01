# language: es
Característica: Control de acceso por roles
  Para proteger las operaciones sensibles
  Como plataforma
  Quiero separar los roles de directorio de los roles de aplicación

  @RF-009 @AM-021 @p0
  Escenario: Un usuario corriente no accede a la administración
    Dado que inicié sesión con una cuenta de rol "user"
    Cuando consulto el listado de cuentas de administración
    Entonces recibo una respuesta 403

  @RF-009 @AM-007 @p0
  Escenario: Añadir el rol admin al token no concede privilegios
    Dado que inicié sesión con una cuenta de rol "user"
    Cuando presento un token cuyo claim "roles" fue alterado para incluir "admin"
    Entonces recibo una respuesta 401

  @RF-009 @RF-020 @p1
  Escenario: Un empleado sin rol de aplicación no accede a Contabilidad
    Dado que existe un empleado con únicamente el rol base "user"
    Cuando completa el flujo OAuth de Contabilidad
    Entonces el token no contiene roles de Contabilidad
    Y Contabilidad muestra "sin acceso"

  @RF-010 @p0
  Escenario: Un administrador deshabilita una cuenta
    Dado que inicié sesión con una cuenta de rol "admin"
    Y que existe una cuenta activa "bruno@example.com"
    Cuando deshabilito la cuenta "bruno@example.com"
    Entonces recibo una respuesta 200
    Y "bruno@example.com" no puede iniciar sesión
    Y se registra un evento de auditoría "user_disabled" con mi identificador como actor

  @RF-010 @p0
  Escenario: Un administrador no puede deshabilitarse a sí mismo
    Dado que inicié sesión con una cuenta de rol "admin"
    Cuando intento deshabilitar mi propia cuenta
    Entonces recibo una respuesta 400

  @RF-010 @p0
  Escenario: Un administrador no puede asignarse roles a sí mismo
    Dado que inicié sesión con una cuenta de rol "admin"
    Cuando intento asignarme el rol "contabilidad.senior"
    Entonces recibo una respuesta 400
    Y se registra un evento de auditoría "role_assignment_rejected"

  @RF-009 @p1
  Escenario: El rol base "user" no se puede quitar
    Dado que inicié sesión con una cuenta de rol "admin"
    Y existe otra cuenta con los roles "user" y "contabilidad.analista"
    Cuando le asigno solo el rol "contabilidad.senior", sin "user"
    Entonces recibo una respuesta 400
    Y la cuenta conserva sus roles anteriores

  @RF-011 @AM-010 @p0
  Escenario: El registro de auditoría no se puede alterar
    Dado que existen eventos en el registro de auditoría
    Cuando la aplicación intenta modificar o borrar una entrada
    Entonces la base de datos rechaza la operación por falta de permisos
