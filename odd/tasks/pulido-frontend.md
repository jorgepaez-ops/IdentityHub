# pulido-frontend — Apariencia y funciones del Hub y de Contabilidad, más cierre de deudas menores

## Objetivo

Que la consola del Hub y Contabilidad se vean y se sientan como productos terminados para la demo, el
video y la sustentación, sin tocar el backend, y cerrar las deudas menores anotadas en
`odd/tasks/idp-semana-4.md`.

## Problema y motivo

La base es sólida (tokens CSS, modo oscuro por sistema, tablas accesibles, estados vacíos en el Hub),
pero las dos aplicaciones se ven básicas: las fuentes declaradas (IBM Plex, Space Grotesk, Fraunces)
nunca se cargan, no hay íconos ni favicon, el Hub no tiene página de inicio, la auditoría muestra UUID y
JSON crudos, y Contabilidad son seis filas fijas sin gráficos, filtros ni exportación (su aviso nunca se
cierra). Exploración del 2026-10-06 (agente de solo lectura) con evidencia `ruta:línea`.

## Decisiones (2026-10-06, con el usuario)

- **D1 · Dirección visual:** refinar la identidad de cada aplicación. Hub sobrio y corporativo (azul,
  IBM Plex Sans/Mono, Space Grotesk en títulos); Contabilidad cálida (tonos tierra, Fraunces en títulos).
  Dos estilos distintos refuerzan en la demo que son aplicaciones separadas unidas por SSO.
- **D2 · Alcance:** apariencia y funciones en ambas aplicaciones, más las deudas menores.
- **D3 · Sin backend:** todo con la API actual (OpenAPI vigente). Los indicadores que exigen totales
  exactos se muestran como «100+» cuando hay más páginas; un endpoint de estadísticas queda como trabajo
  futuro.

## Restricciones

- CSP (`frontend/nginx/default.conf`): `style-src 'self'`, `font-src 'self'`, `img-src 'self' data:`.
  Fuentes incluidas en el bundle (woff2, licencia OFL), sin CDN; gráficos en SVG hecho a mano con
  clases, sin librerías que inyecten `<style>`.
- AGENTS.md: tipos de API desde `schema.d.ts`, sin `dangerouslySetInnerHTML`, sin tokens en
  `localStorage`, `lint` sin avisos, `typecheck` y `test` en verde, cada cambio de comportamiento con su
  prueba `TestRF0xx_`.
- E2E: conservar IDs (`#email`, `#password`, `#mfa-code`, `#new-password`, `#password-confirmation`),
  nombres accesibles y encabezados que usan las pruebas, y una sola región `role="status"` visible a la
  vez. `make e2e` debe seguir en 52/52.
- Codex no tiene red ni Docker: descargas de fuentes, E2E, capturas e informe los cierra Claude.
- Repo en iCloud: binarios generados en una carpeta temporal y copiados de una vez.
- Tamaño orientativo por tarea: ~400 líneas (heurística).

## Criterios de aceptación

1. Fuentes, íconos, favicon y selector de tema visibles en ambas aplicaciones.
2. La consola abre en una página de inicio con indicadores reales de la API.
3. Contabilidad muestra gráficos, filtros, búsqueda, orden y exportación, con avisos que se cierran.
4. Accesibilidad: foco visible global, enlace para saltar al contenido, tablas con nombre y `scope`.
5. Deudas menores cerradas o justificadas una por una.
6. `make e2e` 52/52 (o más), vitest, lint y typecheck en verde; capturas, manual e informe regenerados.

## Tareas

- [x] **P1 — Sistema visual del Hub.** Fuentes woff2 incluidas, íconos SVG en línea, favicon, selector de
  tema (`data-theme`), foco visible global, enlace para saltar al contenido, componente de aviso con
  cierre y esqueletos de carga.
  Ruta: delegada (Codex; se quedó sin cuota al final de su verificación), cierre de Claude. Evidencia
  (2026-10-06): fuentes `@fontsource` (IBM Plex Sans/Mono, Space Grotesk; también Fraunces y Plex para
  Contabilidad, instaladas por Claude con red; `npm audit` 0), `fonts.css`, íconos SVG, favicon,
  `ThemeToggle` con `data-theme`, foco visible, enlace para saltar al contenido, `Toast` con cierre
  automático y `LoadingSkeleton`. Claude corrigió dos defectos: la prueba del cierre automático avanzaba
  el reloj fuera de `act()`, y el `Toast` reiniciaba el temporizador cada vez que el padre pasaba un
  callback nuevo (no se cerraba mientras se escribía en la búsqueda): ahora el callback vive en un ref;
  prueba nueva que falla con la versión anterior. Verificación: typecheck y lint limpios, vitest 171/171,
  build con los woff2 emitidos, `make e2e` 52/52, vista previa de capturas revisada (`CAPTURAS_OUT`,
  variable nueva del script para no tocar las imágenes del manual). iCloud creó 224 copias « N» durante
  la edición (43 idénticas, 181 versiones intermedias): movidas fuera del repo, sin borrar.
  Commit `6ad0e55`. Revisión nativa (medio, 548 líneas, 1 lente): **aprobada**. Corregido después: el tema
  usa el del sistema si `localStorage` lanza excepción (prueba nueva, RED con el código anterior); un
  error en la grilla de roles retira el aviso de éxito y «Nuevo rol» también lo retira; los avisos
  llevan un id, así que el mismo texto dos veces reinicia el temporizador. vitest 172/172, lint y
  typecheck limpios, `make e2e` 52/52.
- [x] **P2 — Sistema visual de Contabilidad.** Lo mismo con su identidad cálida; el aviso se cierra solo.
  Ruta: delegada (Sonnet; Codex sin cuota), revisión de Claude. Evidencia (2026-10-06): fuentes
  `@fontsource` (Plex Sans/Mono, Fraunces), íconos SVG (candado SVG con nombre «Bloqueado»), favicon,
  tema con `data-theme` y almacenamiento protegido, foco visible, enlace para saltar al contenido, tabla
  con nombre y `scope`, `Toast` con cierre automático y reinicio por id (el aviso anterior nunca se
  cerraba), estilos cálidos (tarjetas con barra de acento, estados con punto de color, botones con
  ícono). RED: 8 pruebas TestRF021_ nuevas fallaban sin los módulos; GREEN: vitest 71/71 (63 previas),
  typecheck y lint limpios, build con 4 woff2, `make e2e` 52/52. Revisión de Claude: capturas de resumen
  y transacciones revisadas; un «+ +» en el botón de registrar era de una versión intermedia (el código
  final no tiene ícono ahí). Sin verificar a ojo: modo oscuro y móvil (P10).
  Commit `3cc9604`. Revisión nativa (medio, 432 líneas, P1 y P2, 1 lente): **aprobada** con avisos de
  cobertura; cerrados con pruebas (Sonnet): mismo texto dos veces reinicia el aviso en Contabilidad,
  usuarios y roles; un error y «Nuevo rol» retiran el aviso de éxito; almacenamiento bloqueado usa el
  tema del sistema y el botón sigue alternando. Cada prueba falla al revertir su comportamiento. vitest
  Hub 176/176 y Contabilidad 72/72, lint y typecheck limpios.
- [x] **P3 — Inicio de la consola.** Ruta `/inicio` con indicadores (usuarios por estado, inicios
  fallidos en 24 h, actividad reciente) y accesos directos; los administradores aterrizan ahí.
  Ruta: delegada (Sonnet; Codex sin cuota hasta las 15:13), revisión y corrección de Claude. Evidencia
  (2026-10-06): `HomePage.tsx` con cuatro tarjetas por estado, fallidos en 24 h, últimos 10 eventos y
  accesos directos; conteos «N+» con `limit` 100 (D3); aterrizaje de admin en `/inicio` (comodín,
  `/login` y tras MFA); no administradores a `/me`. RED: 11 de 13 pruebas nuevas fallaban sin la página.
  Revisión de Claude: los fallidos solo contaban `login_failed`, pero el backend registra el código MFA
  rechazado como `mfa_code_rejected` y el requisito lo cuenta como fallo; se suman ambas acciones
  (prueba `TestRF011_CountsRejectedMfaCodesAsFailedSignIns`, RED observado antes del cambio) y el
  ternario anidado del tono pasó a un mapa. vitest 190/190, typecheck y lint limpios, `make e2e` 52/52
  (corrido por Sonnet antes de la corrección, que solo añade una consulta). E2E ajustadas:
  `roles-y-permisos` y `demo` esperan «Inicio» tras el login. Límite conocido: siete consultas por
  visita y la ventana de 24 h usa el reloj del navegador.
  Commit `13cbcc4`. Revisión nativa (medio, 471 líneas, P3 más `47e88aa`, 1 lente de fiabilidad):
  **aprobada** y acusada. Dos sugerencias no bloqueantes quedan como trabajo posterior (P9): el
  manejador de auditoría de la prueba devuelve la lista reciente a la consulta `mfa_code_rejected` (enrutar
  esa acción a una lista vacía por defecto), y el aviso de error pide reintentar sin ofrecer un botón;
  además, un fallo en una de las siete consultas oculta todos los indicadores.
  Revisión nativa de la rama completa (desde `6c8bc11`, 45 archivos, 1421 líneas, 1 lente): **aprobada**
  y acusada. Aviso nuevo para P9: `initializeTheme` escribe `data-theme` al arrancar aunque el usuario no
  haya elegido tema, así que el Hub y Contabilidad ya no siguen en vivo el cambio de modo oscuro del
  sistema (solo al recargar). **Corregido** (con el usuario, antes de P4; delegado a Sonnet, revisado por
  Claude): `subscribeToSystemTheme` escucha `matchMedia` mientras no haya elección guardada ni clic en la
  página; `initializeTheme` y `ThemeToggle` se suscriben. RED: 2 de 4 pruebas nuevas por app fallaban;
  GREEN: vitest Hub 194/194 y Contabilidad 76/76, typecheck y lint limpios. Tercera revisión nativa de
  la rama (aprobada y acusada) dejó otra sugerencia para P9: en login, páginas públicas y pantalla de
  restauración, `main#main-content` no tiene `tabIndex=-1`, así que el enlace para saltar puede no mover
  el foco.
- [x] **P4 — Usuarios.** Filtro por estado con el parámetro `status` existente y tarjetas clicables.
  Ruta: delegada (Sonnet; Codex sin cuota), revisión y corrección de Claude. Evidencia (2026-10-06):
  grupo «Filtrar por estado» con botones `aria-pressed` (Todos, Activos, Pendientes, Bloqueados,
  Deshabilitados) que envía `status` a la API también en «Cargar más», reinicia el cursor, se combina con
  la búsqueda y vive en la URL como `?estado=` (valores desconocidos se ignoran); las tarjetas de estado
  del inicio enlazan a `/usuarios?estado=…` (la de fallidos queda sin enlace hasta P5, porque la
  auditoría aún no lee filtros de la URL). La prueba de inicio ya no cuenta la actividad reciente como
  `mfa_code_rejected` (sugerencia de la revisión de P3). RED: 11 pruebas fallaban sin el cambio.
  Revisión de Claude: bajo un filtro, las tarjetas de Usuarios (que cuentan la lista cargada) mostraban
  ceros falsos para los demás estados; ahora se ocultan mientras hay filtro (prueba nueva, RED observado),
  y el ternario anidado del estado vacío pasó a una función. vitest 207/207, typecheck y lint limpios,
  `make e2e` 52/52 (corrido por Sonnet antes de la corrección, que no toca flujos de E2E).
  Commit `c8059d7`. Revisión nativa del tramo tema + P4 (medio, 14 archivos, 518 líneas, 1 lente):
  **aprobada** y acusada. Su sugerencia se atendió: prueba de que la página sigue al sistema sin el botón
  de tema montado (pantallas públicas), en ambas apps; falla al quitar la suscripción de
  `initializeTheme`. vitest Hub 208/208 y Contabilidad 77/77, lint y typecheck limpios.
  Cuarta revisión nativa de la rama (aprobada y acusada): la regla global `svg` del Hub achicaba todo SVG en
  línea al tamaño de ícono (afectaría los gráficos); ahora se limita a `svg.icon`. Para P9 quedan dos
  sugerencias: `tabIndex=-1` en `main#main-content` de las páginas públicas (ya anotada) y reiniciar el
  estado de módulo de `theme.ts` en el `afterEach` de las pruebas para que no dependan del orden.
- [x] **P5 — Auditoría.** Atajos de filtro, actor resuelto a correo (`getUser`), metadatos legibles y
  «filtrar por este actor».
  Ruta: delegada (Sonnet; Codex sin cuota), revisión de Claude. Evidencia (2026-10-06): filtros en la URL
  (`?accion=`, `?actor=`, `?desde=`, con `desde=24h` relativo para que los enlaces no caduquen; valores
  inválidos se ignoran; cada cambio reinicia el cursor); atajos `aria-pressed` «Inicios fallidos»,
  «Códigos MFA rechazados», «Cambios de roles» y «Últimas 24 h»; actor resuelto a correo con `getUser`
  una vez por id y sin bloquear la tabla (id corto si falla o el usuario no existe, «Sistema» si es nulo);
  metadatos como `dl` con claves anidadas en notación de puntos; «Filtrar por este actor» por fila; la
  tarjeta de fallidos del inicio enlaza a `/auditoria?accion=login_failed&desde=24h`. RED: 13 pruebas
  fallaban sin el cambio. vitest 221/221, typecheck y lint limpios, `make e2e` 52/52. Límites conocidos:
  la API acepta una sola acción por consulta, así que la tarjeta del inicio suma `login_failed` y
  `mfa_code_rejected` pero su enlace filtra solo `login_failed`; «Cambios de roles» cubre `role_changed`
  (no `role_created`/`role_updated`/`role_deleted` de roles de aplicación); un objeto anidado vacío en
  los metadatos no muestra clave.
  Commit `41fef8c`. La revisión de la rama completa ya no cabe en el contexto del revisor
  (`lens_context_budget_exceeded`, 2325 líneas): desde aquí se revisa por tramos. Revisión nativa de P5
  (medio, 9 archivos, 401 líneas, 1 lente): **aprobada** y acusada, con dos avisos y una sugerencia
  corregidos por Claude con pruebas (RED observado en las tres primeras): claves de metadatos que
  colisionan al aplanarse (`a.b` literal y `a: { b }`) ya no repiten la clave de React; los atajos y
  «Filtrar por este actor» limpian el error del actor; volver a pulsar «Filtrar» sin cambios recarga
  (recalcula la ventana de 24 h). Al probar este último se encontró una doble petición cuando el filtro sí
  cambiaba; ahora solo se fuerza la recarga si la URL no cambia, y en ese caso también se limpia el error
  (prueba añadida después del arreglo, sin RED observado). vitest 225/225, typecheck y lint limpios.
- [x] **P6 — Contabilidad: resumen.** Más datos de ejemplo, gráficos SVG (por estado, por categoría,
  línea de tiempo) y actividad reciente.
  Ruta: delegada (Sonnet; Codex sin cuota), revisión y corrección de Claude. Evidencia (2026-10-06):
  47 movimientos deterministas (abril a septiembre de 2026, 7 categorías; folios `M-1xxx` por debajo del
  contador, así que los nuevos siguen en `M-2047` y las E2E con `M-2041`…`M-2046` no cambian);
  `aggregate.ts` con totales por estado, categoría y mes (meses vacíos en cero) y actividad reciente;
  tres gráficos SVG hechos a mano (dona por estado, barras por categoría, columnas por mes), cada uno
  `figure` con `figcaption`, `svg` con `role="img"` y `aria-label` resumido, y tabla alternativa en
  `details`; sin estilos en línea ni clase `icon`. Las tarjetas «del mes» cuentan solo el periodo abierto
  (`inOpenPeriod`, desde `2026-09-01`); los gráficos, todo el historial. RED: los módulos nuevos no
  existían. Revisión de Claude: el cierre de septiembre contaba todo el historial (38 aprobados en vez de
  5); ahora usa `inOpenPeriod` (prueba `TestRF021_ClosingCountsOnlyTheOpenPeriod`, RED observado). vitest
  96/96, typecheck, lint y build limpios, `make e2e` 52/52 (corrido por Sonnet antes de la corrección del
  cierre, que ninguna E2E afirma). Sin verificar a ojo: modo oscuro y móvil (P10).
  Commit `d5f7b19`. Revisión nativa de P6 (medio, 14 archivos, 571 líneas, 1 lente): **aprobada** y
  acusada. Aviso corregido por Claude: `totalsByMonth` entraba en un ciclo infinito con una fecha mal
  formada (RED observado: el proceso de pruebas murió por memoria agotada); ahora solo recorre claves
  `YYYY-MM` válidas. Sugerencia aceptada como límite de la demo: el periodo abierto es una constante
  (`2026-09-01`) sin límite superior, así que un movimiento nuevo (fechado con el reloj real, octubre)
  cuenta en el cierre de septiembre y no cambia tras cerrar. vitest 97/97, typecheck y lint limpios.
- [ ] **P7 — Contabilidad: movimientos.** Búsqueda, filtros, orden, categoría al registrar,
  confirmación antes de aprobar o rechazar, estado vacío, tabla accesible.
- [ ] **P8 — Exportar CSV** en usuarios, auditoría y movimientos.
- [ ] **P9 — Deudas menores.** Avisos de SonarCloud de las fases 3 y 4, sugerencias pendientes de las
  revisiones nativas (correos, refresh, informe, capturas) y alerta de Grafana más runbook para la DLQ
  (AM-019).
  Ampliada (2026-10-06, con el usuario) con las 120 alertas abiertas de code scanning en `main`:
  - 50 de Semgrep (`github-actions-mutable-action-tag`): fijar las acciones por SHA con el tag como
    comentario.
  - 33 de CodeQL (`js/remote-property-injection`), todas en los HTML de `docs/diagramas/` (biblioteca
    minificada embebida). Decisión del usuario (2026-10-06): **regenerar los tres diagramas actualizados**
    (grilla de roles, observabilidad, producción simulada, referencia AWS) con el mismo estilo visual,
    como HTML estático con SVG en línea y sin bibliotecas JavaScript de terceros; además, agregar los UML
    formales de secuencia del SSO con PKCE y de estados del ciclo de vida del usuario en
    `docs/diagramas/uml/` para el informe técnico.
  - 6 de Trivy (libpng y nghttp2 en `web`, tzdata en `api`/`worker`, x/crypto en `api` ligado a
    VULN-028, AWS-0089 en Terraform): actualizar imágenes base si hay versión corregida.
  - 27 de Checkov (omisiones ya justificadas en el código) y 4 de Semgrep (`request-host-used` en nginx).
  - Lo que no se pueda corregir se documenta en `README.md`, sección «Alertas abiertas conocidas», con
    motivo y justificación, para la exposición.
- [ ] **P10 — Documentación y evidencia.** Ajustes de E2E si hacen falta, capturas regeneradas,
  manual de usuario e informe técnico actualizados.

## Progreso

| Tareas | Hechas |
|---|---|
| P1 a P10 (10) | 6 (P1 a P6) |

## Entrega

Rama `feat/pulido-frontend` (sale de `feat/idp-semana-4`, que ya está en `main` salvo dos commits de
notas). Commits por unidad de trabajo; un PR al final con confirmación del usuario. Pronóstico: más de
400 líneas en total, una tarea por commit.

## Siguiente paso

P7 (Contabilidad: movimientos con búsqueda, filtros, orden y confirmación).
