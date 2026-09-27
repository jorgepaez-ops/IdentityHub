# language: es
Característica: Autorización de Contabilidad con Authorization Code y PKCE
  Para entrar a una aplicación en otro dominio sin compartir mi contraseña
  Como empleado autenticado en Identity Hub
  Quiero autorizar a Contabilidad con un código seguro y de un solo uso

  @RF-020 @p1
  Escenario: Flujo OAuth correcto con PKCE S256
    Dado que Contabilidad inició una solicitud con estado y desafío PKCE S256
    Y que completé el login y MFA en Identity Hub
    Cuando autorizo al cliente configurado con su URI de retorno exacta
    Entonces Identity Hub redirige con un código y el mismo estado
    Y al canjear el código y el verificador recibo un access token sin refresh token
    Y el claim "aud" identifica a Contabilidad

  @RF-020 @p1
  Escenario: Un segundo acceso usa la sesión SSO del Hub
    Dado que ya completé login y MFA y conservo la sesión SSO del Hub
    Cuando Contabilidad inicia una nueva autorización
    Entonces Identity Hub emite un código sin volver a pedir la contraseña

  @RF-020 @p1
  Escenario: Una URI de retorno distinta se rechaza
    Cuando solicito autorización con una "redirect_uri" que no coincide exactamente
    Entonces recibo una respuesta 400 sin redirección a esa URI

  @RF-020 @AM-019 @p1
  Escenario: El estado es obligatorio
    Cuando solicito autorización sin "state"
    Entonces la autorización se rechaza

  @RF-020 @p1
  Escenario: PKCE plain se rechaza
    Cuando solicito autorización con "code_challenge_method" igual a "plain"
    Entonces la autorización se rechaza

  @RF-020 @p1
  Escenario: Un código de autorización no puede reutilizarse
    Dado que ya canjeé un código de autorización válido
    Cuando intento canjear el mismo código de nuevo
    Entonces recibo una respuesta 400
    Y se registra un evento de auditoría "authorization_code_reused"

  @RF-009 @RF-020 @p1
  Escenario: El token contiene solo los roles de Contabilidad
    Dado que mi cuenta tiene los roles "admin", "user" y "contabilidad.senior"
    Cuando completo el flujo OAuth de Contabilidad
    Entonces el claim "roles" contiene únicamente "contabilidad.senior"

  @RF-020 @p1
  Escenario: Sin sesión en el Hub se pasa por el login y se vuelve a la autorización
    Dado que no tengo sesión SSO en Identity Hub
    Cuando Contabilidad inicia una autorización válida
    Entonces Identity Hub redirige a su login con "continue" igual a la ruta relativa de esa autorización
    Y tras completar login y MFA vuelvo a la autorización y recibo un código

  @RF-020 @p1
  Escenario: Un "continue" absoluto o externo no provoca una redirección abierta
    Cuando abro el login del Hub con "continue" igual a "https://sitio-malicioso.example/robo"
    Y completo login y MFA
    Entonces Identity Hub no redirige a ese sitio y me deja en la consola del Hub
