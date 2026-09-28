# language: es
Característica: Restablecimiento de contraseña
  Para recuperar el acceso sin exponer qué correos están registrados
  Como titular de una cuenta
  Quiero recibir y usar un enlace de restablecimiento de un solo uso

  @RF-015 @AM-004 @p1
  Escenario: La solicitud no enumera cuentas
    Cuando solicito restablecer la contraseña de "ada@example.com"
    Entonces recibo una respuesta 202
    Cuando solicito restablecer la contraseña de "ausente@example.com"
    Entonces recibo una respuesta 202

  @RF-015 @AM-005 @p1
  Escenario: Restablecer revoca las sesiones anteriores
    Dado que recibí un enlace de restablecimiento vigente
    Y que tengo dos sesiones activas
    Cuando fijo una contraseña nueva con el enlace
    Entonces recibo una respuesta 204
    Y las sesiones anteriores ya no pueden renovar su token
    Y no puedo usar de nuevo el enlace

  @RF-015 @RF-017 @p1
  Escenario: Restablecer desbloquea la cuenta sin que un fallo la vuelva a bloquear
    Dado que mi cuenta está bloqueada por intentos fallidos
    Cuando fijo una contraseña nueva con el enlace de restablecimiento
    Entonces mi cuenta pasa a estar activa
    Y puedo iniciar sesión con la contraseña nueva
    Cuando fallo un inicio de sesión con la contraseña nueva
    Entonces mi cuenta no queda bloqueada de nuevo
