# language: es
Característica: Rotación de sesión y detección de credenciales robadas
  Para limitar el daño si me roban un token
  Como titular de una cuenta
  Quiero que el sistema detecte el uso simultáneo de la misma credencial

  Antecedentes:
    Dado que inicié sesión y tengo un par de tokens válido

  @RF-005 @p0
  Escenario: La renovación rota el refresh token
    Cuando renuevo la sesión con mi "refreshToken"
    Entonces recibo un par de tokens nuevo
    Y el "refreshToken" nuevo es distinto del anterior
    Y el "refreshToken" anterior deja de ser válido

  @RF-006 @AM-002 @p0
  Escenario: Reutilizar un refresh token rotado revoca toda la familia
    Dado que renové la sesión una vez
    Cuando presento el "refreshToken" anterior, ya rotado
    Entonces recibo una respuesta 401
    Y el "refreshToken" vigente también queda revocado
    Y se registra un evento de auditoría "refresh_reuse_detected"
    Y recibo un aviso de seguridad en Mailpit

  @RF-007 @p0
  Escenario: Cierre de sesión
    Cuando cierro la sesión
    Entonces recibo una respuesta 204
    Y renovar con ese "refreshToken" devuelve 401

  @RF-016 @p1
  Escenario: Revocar una sesión concreta desde otro dispositivo
    Dado que tengo dos sesiones abiertas
    Cuando revoco la primera desde la segunda
    Entonces la primera deja de poder renovarse
    Y la segunda sigue funcionando
