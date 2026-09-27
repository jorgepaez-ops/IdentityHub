# language: es
Característica: Alta de empleados y aceptación de invitación
  Para incorporar empleados sin exponer sus contraseñas
  Como administrador
  Quiero crear cuentas que el titular active desde su correo

  Antecedentes:
    Dado que la plataforma está disponible
    Y que inicié sesión con una cuenta de rol "admin"
    Y que no existe ninguna cuenta con el correo "ana@example.com"

  @RF-001 @RF-012 @p0
  Escenario: Alta administrativa con datos válidos
    Cuando doy de alta a "ana@example.com" con nombre "Ana" y rol "contabilidad.analista"
    Entonces recibo una respuesta 201
    Y la cuenta queda en estado "pending_verification" con los roles "user" y "contabilidad.analista"
    Y se encola una invitación para "ana@example.com"

  @RF-001 @p0
  Escenario: El autorregistro público no está disponible
    Cuando intento crear una cuenta en "/api/v1/auth/register"
    Entonces recibo una respuesta 404

  @RF-001 @AM-004 @p0
  Escenario: Un correo no puede darse de alta dos veces
    Dado que existe una cuenta activa con el correo "ana@example.com"
    Cuando doy de alta a "ana@example.com" con nombre "Ana" y rol "user"
    Entonces recibo una respuesta 409
    Y el cuerpo de la respuesta no revela datos de la cuenta existente

  @RF-002 @p0
  Escenario: Aceptar la invitación verifica el correo y fija la contraseña
    Dado que el correo de invitación para "ana@example.com" llegó a Mailpit
    Cuando acepto el enlace con la contraseña "correcta-horse-battery"
    Entonces recibo una respuesta 204
    Y la cuenta queda en estado "active"
    Y puedo iniciar el desafío MFA con esa contraseña

  @RF-002 @p0
  Escenario: La invitación es de un solo uso
    Dado que ya acepté la invitación recibida
    Cuando vuelvo a aceptar el mismo enlace
    Entonces recibo una respuesta 410

  @RF-002 @p0
  Escenario: Una cuenta que no aceptó la invitación no puede iniciar sesión
    Dado que la invitación para "ana@example.com" sigue pendiente
    Cuando inicio sesión con "ana@example.com" y cualquier contraseña
    Entonces recibo una respuesta 401
