# 05 — Modelo de amenazas (STRIDE)

Metodología STRIDE aplicada por componente sobre el diagrama de flujo de datos de
`02-domain-model.md` y la arquitectura de la sección 2 del plan.

Cada amenaza tiene: identificador estable, categoría STRIDE, mitigación en el
producto y —esto es lo que hace de esto un proyecto de DevSecOps y no solo de
seguridad— **el gate del pipeline que impide que la mitigación se pierda**. Una
mitigación sin gate se degrada en cuanto alguien la borre sin darse cuenta.

## Diagrama de flujo de datos y fronteras de confianza

```
   ╔═══════════════ Internet / no confiable ═══════════════╗
   ║   navegador                                            ║
   ╚════════════════════════│═══════════════════════════════╝
                            │ HTTPS
   ╔════════════════════════▼═══════ DMZ (contenedor web) ══╗
   ║   Nginx  ── estáticos ── proxy /api                    ║
   ╚════════════════════════│═══════════════════════════════╝
                            │ HTTP, red interna de Docker
   ╔════════════════════════▼═══ Red de aplicación ═════════╗
   ║   api ──── db (5432)                                   ║
   ║    │                                                    ║
   ║    └────── broker (5672) ──── worker ──── mailpit(1025)║
   ╚════════════════════════════════════════════════════════╝

   Fronteras de confianza:
     T1  navegador → Nginx        (entrada no confiable)
     T2  Nginx → api              (cabeceras reenviadas, XFF falsificable)
     T3  api → db                 (credenciales de base de datos)
     T4  api → broker → worker    (el mensaje lleva tokens en claro)
     T5  CI → registry → despliegue (cadena de suministro)
```

## Amenazas

### Spoofing — suplantación de identidad

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-001 | Fuerza bruta sobre contraseñas | Argon2id (coste alto), bloqueo tras 5 fallos, `limit_req` en Nginx sobre `/auth/*` | `test-unit` verifica el bloqueo; ZAP comprueba el rate limit | RF-017 |
| AM-002 | Robo de refresh token y uso paralelo | Rotación obligatoria + detección de reuso que revoca la familia | `test-integration` `TestRF006_...` | RF-005, RF-006 |
| AM-003 | Falsificación de JWT con `alg: none` o cambio a HS256 usando la clave pública como secreto | Validación explícita: solo se acepta `EdDSA`; se rechaza cualquier otro `alg` antes de verificar la firma | `test-unit` con tokens hostiles; Semgrep regla `jwt-none-algorithm` | RF-004 |
| AM-004 | Enumeración de cuentas por mensajes o tiempos de respuesta distintos | Respuestas y códigos idénticos para correo existente e inexistente; comparación de contraseña en tiempo constante y hash señuelo cuando el usuario no existe | `test-unit` compara respuestas; escenario Gherkin dedicado | RF-001, RF-015 |
| AM-005 | Fijación de sesión tras cambiar la contraseña | El restablecimiento revoca todas las sesiones activas | `test-integration` | RF-015 |

### Tampering — manipulación

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-006 | Inyección SQL en la búsqueda de usuarios | Consultas parametrizadas generadas por `sqlc`; nunca concatenación de cadenas | `gosec` G201/G202, CodeQL, Semgrep | RF-010 |
| AM-007 | Escalada de privilegios alterando el claim `roles` del cliente | Los roles se leen de la base de datos en cada petición sensible, no del token; el token solo se cree tras verificar la firma | `test-integration` de RBAC; ZAP con token manipulado | RF-009 |
| AM-008 | Imagen manipulada entre la construcción y el despliegue | Firma Cosign *keyless* y despliegue por digest, no por etiqueta mutable | `cd.yml` no despliega si `cosign verify` falla | RNF-006 |
| AM-009 | Dependencia comprometida en la cadena de suministro | SBOM en CycloneDX por imagen, `osv-scanner`, Dependabot, lockfiles versionados | `sca`, `image-scan`, escaneo semanal | RNF-004 |

### Repudiation — repudio

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-010 | Un administrador niega haber deshabilitado una cuenta | Audit log append-only con actor, IP y user-agent; permiso `UPDATE`/`DELETE` denegado a nivel de base de datos | `test-integration` intenta modificar el log y espera fallo por permisos | RF-011 |
| AM-011 | Borrado de rastro por quien controla la aplicación | El rol de base de datos de la aplicación no puede borrar; los logs se envían a Loki fuera del contenedor | Revisión de la migración en el escaneo IaC | RF-011, RNF-007 |

### Information disclosure — divulgación

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-012 | Secretos en el repositorio o en la imagen | Sin valores por defecto en la configuración; secretos por entorno; `.env` fuera de git | **Gitleaks** sobre el historial completo; Trivy busca secretos en la imagen | RNF-003 |
| AM-013 | Contraseñas o tokens en los logs | Tipo `Secret` con `String()` redactado; identificadores de token truncados a 8 caracteres | `test-unit` sobre el logger; Semgrep contra `log.*password` | RNF-012 |
| AM-014 | Volcado de la base de datos por un puerto expuesto | En producción no se publica ningún puerto de `db` ni de `broker` al host | Trivy config sobre el compose de producción | RNF-001 |
| AM-015 | XSS que roba el token del `localStorage` | CSP estricta sin `unsafe-inline`; el refresh token viaja en cookie `HttpOnly`; React escapa por defecto | ZAP baseline verifica las cabeceras; ESLint prohíbe `dangerouslySetInnerHTML` | RNF-009 |
| AM-016 | El correo de verificación lleva el token en claro por el broker | Vigencia de 24 h, un solo uso; el broker no es accesible desde fuera de la red interna; el worker no registra el cuerpo del mensaje | Revisión de código y `test-unit` del logger | RF-002 |

### Denial of service — denegación de servicio

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-017 | Argon2id como amplificador: peticiones masivas de login agotan la CPU | `limit_req` en Nginx antes de llegar a la API; parámetros de Argon2 calibrados a ~250 ms | Prueba de carga documentada en el runbook | RNF-010 |
| AM-018 | Cuerpos de petición enormes | `client_max_body_size 1m` en Nginx y `http.MaxBytesReader` en la API | `test-unit` con cuerpo sobredimensionado | — |
| AM-019 | Acumulación en la DLQ que llena el disco | Alerta en Grafana cuando la profundidad de la DLQ supera 10; runbook de purga | Regla de alerta versionada en `deploy/observability/` | RNF-007 |

### Elevation of privilege — elevación de privilegios

| ID | Amenaza | Mitigación | Gate del pipeline | Requisito |
|---|---|---|---|---|
| AM-020 | Escape de contenedor desde un proceso comprometido | Usuario no-root, `cap_drop: ALL`, `no-new-privileges`, raíz en solo lectura, base distroless sin shell | **Hadolint** + **Trivy config**; una regla falla ante `USER root` | RNF-008 |
| AM-021 | Un `user` accede a endpoints de administración | Middleware de autorización por ruta, verificado del lado del servidor | `test-integration` recorre todas las rutas `/admin/*` con un token de `user` y exige `403` | RF-009 |
| AM-022 | Token de CI con permisos excesivos publica en el registry | `permissions:` mínimas por job en cada workflow; OIDC *keyless* en vez de credenciales de larga vida | Revisión del workflow; `actionlint` en el job de lint | RNF-006 |

## Trazabilidad inversa

Cada ficha de `security/findings/VULN-NNN-*.md` referencia la amenaza que
materializa. La línea base vulnerable (`v0.0.0-vuln-baseline`) existe justamente
para demostrar que estas amenazas no son teóricas: AM-006, AM-012 y AM-020 se
sembraron a propósito y los gates los encontraron.
