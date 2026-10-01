# language: es
Característica: Rotación de sesión y detección de credenciales robadas
  Para limitar el daño si me roban un token
  Como titular de una cuenta
  Quiero que el sistema detecte el uso simultáneo de la misma credencial

  Antecedentes:
    Dado que inicié sesión y recibí una cookie "refresh_token"

  @RF-005 @p0
  Escenario: La renovación rota la cookie refresh
    Cuando renuevo la sesión con mi cookie "refresh_token"
    Entonces recibo un "accessToken" nuevo y una cookie "refresh_token" nueva
    Y la cookie "refresh_token" nueva es distinta de la anterior
    Y la cookie "refresh_token" anterior deja de ser válida

  @RF-006 @AM-002 @p0
  Escenario: Reutilizar un refresh token rotado revoca toda la familia
    Dado que renové la sesión una vez
    Cuando presento la cookie "refresh_token" anterior, ya rotada
    Entonces recibo una respuesta 401
    Y la cookie "refresh_token" vigente también queda revocada
    Y se registra un evento de auditoría "refresh_reuse_detected"
    Y recibo un aviso de seguridad en Mailpit

  @RF-007 @p0
  Escenario: Cierre de sesión
    Cuando cierro la sesión
    Entonces recibo una respuesta 204
    Y la cookie "refresh_token" queda vacía y expirada
    Y renovar con la cookie anterior devuelve 401

  @RF-016 @p1
  Escenario: Revocar una sesión concreta desde otro dispositivo
    Dado que tengo dos sesiones abiertas
    Cuando revoco la primera desde la segunda
    Entonces la primera deja de poder renovarse
    Y la segunda sigue funcionando
