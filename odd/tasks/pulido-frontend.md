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

- [ ] **P1 — Sistema visual del Hub.** Fuentes woff2 incluidas, íconos SVG en línea, favicon, selector de
  tema (`data-theme`), foco visible global, enlace para saltar al contenido, componente de aviso con
  cierre y esqueletos de carga.
- [ ] **P2 — Sistema visual de Contabilidad.** Lo mismo con su identidad cálida; el aviso se cierra solo.
- [ ] **P3 — Inicio de la consola.** Ruta `/inicio` con indicadores (usuarios por estado, inicios
  fallidos en 24 h, actividad reciente) y accesos directos; los administradores aterrizan ahí.
- [ ] **P4 — Usuarios.** Filtro por estado con el parámetro `status` existente y tarjetas clicables.
- [ ] **P5 — Auditoría.** Atajos de filtro, actor resuelto a correo (`getUser`), metadatos legibles y
  «filtrar por este actor».
- [ ] **P6 — Contabilidad: resumen.** Más datos de ejemplo, gráficos SVG (por estado, por categoría,
  línea de tiempo) y actividad reciente.
- [ ] **P7 — Contabilidad: movimientos.** Búsqueda, filtros, orden, categoría al registrar,
  confirmación antes de aprobar o rechazar, estado vacío, tabla accesible.
- [ ] **P8 — Exportar CSV** en usuarios, auditoría y movimientos.
- [ ] **P9 — Deudas menores.** Avisos de SonarCloud de las fases 3 y 4, sugerencias pendientes de las
  revisiones nativas (correos, refresh, informe, capturas) y alerta de Grafana más runbook para la DLQ
  (AM-019).
- [ ] **P10 — Documentación y evidencia.** Ajustes de E2E si hacen falta, capturas regeneradas,
  manual de usuario e informe técnico actualizados.

## Progreso

| Tareas | Hechas |
|---|---|
| P1 a P10 (10) | 0 |

## Entrega

Rama `feat/pulido-frontend` (sale de `feat/idp-semana-4`, que ya está en `main` salvo dos commits de
notas). Commits por unidad de trabajo; un PR al final con confirmación del usuario. Pronóstico: más de
400 líneas en total, una tarea por commit.

## Siguiente paso

P1 (sistema visual del Hub): Claude descarga las fuentes (red) y Codex implementa.
