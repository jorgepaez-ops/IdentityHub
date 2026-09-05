# language: es
Característica: Autenticación y emisión de tokens
  Para acceder a mis recursos
  Como titular de una cuenta activa
  Quiero obtener credenciales de sesión verificables

  Antecedentes:
    Dado que existe una cuenta activa "ana@example.com" con la contraseña "correcta-horse-battery"

  @RF-003 @RF-004 @p0
  Escenario: Inicio de sesión correcto
    Cuando inicio sesión con "ana@example.com" y "correcta-horse-battery"
    Entonces recibo una respuesta 200
    Y la respuesta contiene un "accessToken" y un "refreshToken"
    Y el "accessToken" está firmado con el algoritmo "EdDSA"
    Y el "kid" del token coincide con una clave publicada en el JWKS
    Y se registra un evento de auditoría "login_succeeded"

  @RF-003 @AM-004 @p0
  Escenario: Contraseña incorrecta
    Cuando inicio sesión con "ana@example.com" y "una-contraseña-cualquiera"
    Entonces recibo una respuesta 401
    Y el mensaje de error es genérico y no menciona si el correo existe
    Y se registra un evento de auditoría "login_failed"

  @RF-004 @AM-003 @p0
  Escenario: Un token con el algoritmo alterado se rechaza
    Dado que tengo un "accessToken" válido
    Cuando modifico su cabecera para que el algoritmo sea "none" y lo presento
    Entonces recibo una respuesta 401

  @RF-017 @AM-001 @p1
  Escenario: Bloqueo tras intentos fallidos repetidos
    Cuando fallo el inicio de sesión 5 veces seguidas
    Y vuelvo a intentarlo con la contraseña correcta
    Entonces recibo una respuesta 423
    Y recibo un correo de aviso en Mailpit

  @RF-013 @RF-014 @p1
  Escenario: Inicio de sesión con segundo factor activo
    Dado que tengo el segundo factor TOTP activado
    Cuando inicio sesión con "ana@example.com" y "correcta-horse-battery"
    Entonces recibo una respuesta 202 con un "mfaToken"
    Cuando canjeo el "mfaToken" con un código TOTP válido
    Entonces recibo una respuesta 200 con el par de tokens
