# 0009 — SSO entre dominios con authorization code y PKCE, en versión mínima

Estado: aceptada · 2026-09-27

## Contexto

Identity Hub existe para que otras aplicaciones no gestionen usuarios. La demostración que pide
el proyecto es la de un directorio corporativo (el papel de Entra ID) y una aplicación de negocio
(Contabilidad) que confía en él: el empleado inicia sesión una vez en el Hub y entra a la
aplicación sin volver a escribir la contraseña, y la aplicación decide qué le muestra según sus
roles.

Para que esa demostración sea creíble, las dos piezas viven en **sitios distintos**
(`identityhub.localhost` y `contabilidad.localhost`, D10 de `odd/tasks/idp-semana-3.md`). Servir
ambas bajo un mismo origen haría el SSO trivial —la cookie de sesión se compartiría sin más— y no
demostraría nada.

La sesión actual no sirve entre sitios: el refresh token viaja en una cookie `HttpOnly`,
`Secure`, `SameSite=Strict` con `Path=/api/v1/auth` (enmienda C1 de la semana 2), que el
navegador no envía a otro sitio ni en una navegación.

Las alternativas eran:

1. **OIDC completo** (discovery, registro dinámico de clientes, consentimiento, `id_token`,
   `userinfo`): lo que haría un IdP de producción, pero multiplica la superficie a implementar y
   a asegurar, y no cabe en una semana junto al resto del alcance.
2. **Llamar al login del Hub desde la aplicación** (lo que describía el mockup): la aplicación
   recibiría la contraseña del usuario, que es justo lo que un IdP debe evitar.
3. **OAuth 2.0 authorization code con PKCE, reducido a lo esencial.**

## Decisión

Implementar la opción 3 (RFC 6749 §4.1 con PKCE de RFC 7636), con estas reglas:

- **Un solo cliente, fijado en configuración**: Contabilidad, con su `client_id` y su
  `redirect_uri`. No hay registro dinámico ni pantalla de consentimiento (la aplicación es de la
  propia empresa).
- **Cliente público**: Contabilidad es un SPA sin backend, así que no tiene secreto de cliente.
  PKCE sustituye al secreto: solo quien generó el `code_verifier` puede canjear el código.
- `GET /oauth/authorize`: valida `client_id`, `redirect_uri` con **coincidencia exacta**,
  `response_type=code`, `state` obligatorio y `code_challenge` con `code_challenge_method=S256`
  obligatorio (`plain` se rechaza). Si el usuario no tiene sesión en el Hub, pasa por el login del
  Hub, MFA incluido (ADR 0010); si la tiene, redirige de vuelta sin pedir nada.
- **Sesión del Hub**: una cookie propia del dominio del Hub, `HttpOnly`, `Secure`,
  `SameSite=Lax`. Tiene que ser `Lax` y no `Strict` porque la llegada a `/oauth/authorize` es una
  navegación de primer nivel desde otro sitio, y `Strict` no la acompañaría. `Lax` sigue sin
  enviarse en peticiones `POST` entre sitios, que es lo que importa frente a CSRF. Cómo se
  representa en base de datos (tabla propia o reutilizar las familias de refresh de la ADR 0005)
  se fija en T3; el refresh token de la consola del Hub conserva su cookie `Strict`.
- **Código de autorización**: aleatorio de `crypto/rand`, guardado solo como hash, de **un solo
  uso** y con vencimiento corto (del orden de un minuto), atado al cliente, al `redirect_uri` y al
  `code_challenge`. Presentarlo dos veces falla y queda auditado.
- `POST /oauth/token`: canjea código + `code_verifier` por un **access token** (el mismo JWT
  Ed25519 de la ADR existente, verificable con el JWKS), con `aud` igual al cliente y con **solo
  los roles de esa aplicación** (D8). No se emite refresh token para la aplicación: cuando el
  access token vence, la aplicación vuelve a `/oauth/authorize`, y si la sesión del Hub sigue viva
  el usuario no nota nada.
- **CORS** abierto solo donde el navegador de la aplicación lo necesita: `POST /oauth/token` y el
  JWKS (`/.well-known/jwks.json`), ambos **sin credenciales** y solo para el origen del cliente
  registrado.
- Cada control anterior tiene una prueba que falla si se quita.

## Consecuencias

- La demostración es real: dos sitios, ninguna contraseña pasa por la aplicación, y el segundo
  acceso no pide credenciales. Es el mismo flujo que usan Entra ID o Google con una SPA.
- No es OIDC: no hay `id_token`, ni discovery (`/.well-known/openid-configuration`), ni
  `userinfo`, ni cierre de sesión único (cerrar sesión en el Hub no cierra la de la aplicación
  hasta que vence su access token, 15 minutos). Se documenta como límite; un cliente OIDC
  estándar no podría integrarse sin más.
- El access token vive en la memoria de un SPA, expuesto a cualquier XSS de Contabilidad. Se
  mitiga con vida corta, CSP estricta en Contabilidad (RNF-009) y sin refresh token en el
  navegador de la aplicación.
- Agregar un segundo cliente exige tocar configuración y desplegar; aceptable con un solo cliente.
- Superficie nueva que el pipeline debe cubrir: redirecciones abiertas, reuso de códigos,
  sustitución del `code_verifier` y CSRF en el flujo. ZAP (T15) y las pruebas de T9 la ejercitan.
