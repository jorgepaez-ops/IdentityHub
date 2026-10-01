# language: es
Característica: Autenticación y emisión de tokens
  Para acceder a mis recursos
  Como titular de una cuenta activa
  Quiero completar una contraseña y un segundo factor verificable

  Antecedentes:
    Dado que existe una cuenta activa "ana@example.com" con la contraseña "correcta-horse-battery"

  @RF-003 @RF-013 @RF-014 @p0
  Escenario: Todo inicio de sesión exige MFA por correo
    Cuando inicio sesión con "ana@example.com" y "correcta-horse-battery"
    Entonces recibo una respuesta 202 con un "mfaToken"
    Y recibo en Mailpit un código de 6 dígitos
    Y todavía no recibo un access token ni una cookie "refresh_token"
    Cuando canjeo el "mfaToken" con el código recibido
    Entonces recibo una respuesta 200 con un "accessToken"
    Y recibo una cookie "refresh_token" HttpOnly Secure y SameSite Strict
    Y el "accessToken" está firmado con el algoritmo "EdDSA"
    Y se registra un evento de auditoría "mfa_succeeded"

  @RF-014 @p1
  Escenario: Reenviar el código invalida el anterior
    Dado que inicié un desafío MFA
    Cuando solicito reenviar el código
    Entonces recibo una respuesta 202 y un código nuevo en Mailpit
    Y el código anterior ya no completa el desafío

  @RF-014 @p1
  Escenario: El reenvío respeta la ventana mínima
    Dado que inicié un desafío MFA hace menos de 60 segundos
    Cuando solicito reenviar el código
    Entonces recibo una respuesta 429

  @RF-014 @RF-017 @AM-001 @p1
  Escenario: Agotar los intentos MFA anula el desafío
    Dado que inicié un desafío MFA
    Cuando presento códigos incorrectos hasta agotar sus intentos
    Entonces el "mfaToken" deja de ser válido
    Y los fallos cuentan para el bloqueo de la cuenta
    Y se registra un evento de auditoría "mfa_challenge_exhausted"

  @RF-014 @RF-017 @AM-001 @p1
  Escenario: Adivinar códigos MFA en desafíos sucesivos bloquea la cuenta
    Dado que conozco la contraseña de "ana@example.com"
    Cuando inicio varios desafíos MFA y presento códigos incorrectos hasta alcanzar el umbral de RF-017
    Entonces la cuenta queda bloqueada y se registra "account_locked"
    Y un código correcto en un desafío posterior es rechazado

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
