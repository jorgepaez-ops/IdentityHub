# Documentación para un lector externo

## Objetivo

Que alguien sin contexto del equipo (un profesor, otra empresa, un desarrollador nuevo) entienda qué es
Identity Hub, lo levante y encuentre cualquier documento sin perderse entre enlaces.

## Problema y por qué

En la revisión final, el profesor preguntó por todo y el usuario se perdía entre tantos enlaces. Inventario
del 2026-10-19:

- El README raíz mezcla orientación con una crónica semana a semana (F1–F10, PR, tags) y su mapa de
  carpetas es de una línea por carpeta, sin decir cuándo mirar cada una; omite `contabilidad/`, `e2e/`,
  `infraestructura/`, `tests/` y `tools/`.
- `docs/README.md` está ordenado por requisitos del enunciado (útil para calificar, no para leer) y su
  sección «Próximos artefactos» está desactualizada (aún dice que falta `v1.0.0`).
- Solo `docs/` y `scripts/` tienen README propio entre las carpetas principales.

## Alcance autorizado (decisión del usuario, 2026-10-19)

- README raíz como puerta de entrada: qué es (con analogía), levantarlo, mapa del repositorio
  (carpeta | qué hay | cuándo mirarla), rutas de lectura por perfil, seguridad y licencia breves.
- La crónica semanal del README pasa a `docs/BITACORA.md`.
- `docs/README.md` organizado por preguntas del lector; la tabla de cumplimiento del enunciado queda
  como anexo.
- `docs/glosario.md` con los términos técnicos explicados en simple.
- README corto en cada carpeta principal que no lo tenga.

Fuera de alcance: reescribir los manuales existentes, cambiar código o workflows.

## Restricciones

- Español neutro y profesional, como el resto de la documentación; explicaciones simples, sin jerga sin
  definir (o enlazada al glosario).
- Ningún dato inventado: todo comando, ruta y cifra sale del repositorio.
- Ningún enlace relativo roto.
- El PR #58 (overview de Docker Hub) toca la línea de estado de la semana 4 del README; al integrar,
  conservar su enlace a `deploy/dockerhub/`.

## Tareas

- [x] **E1. README raíz y bitácora.** Ruta: delegada (writer único para E1–E3: varios archivos no triviales).
- [x] **E2. Índice de `docs/` por preguntas y glosario.** Ruta: delegada (mismo writer).
- [x] **E3. README por carpeta.** Ruta: delegada (mismo writer).
- [x] **E4. Revisión y verificación.** Ruta: sesión principal. Checks: comprobación de enlaces relativos,
  comandos citados existen en el `Makefile`, lectura completa como lector externo.

## Criterios de aceptación

- Desde el README raíz se llega en uno o dos clics a cualquier documento, sabiendo de antemano qué contiene.
- Cada carpeta principal explica su contenido al abrirla en GitHub.
- Cero enlaces relativos rotos en los archivos tocados.

## Progreso

- 2026-10-19: rama `docs/lector-externo` creada desde `main`; inventario hecho (ver Problema).
- 2026-10-19 (writer; revisado y marcado en E4): E1 `README.md` reescrito (qué es con analogía, levantarlo, estado actual de 2 frases, mapa de 20 filas, rutas por perfil, seguridad, gestión, licencia); la crónica de estado y «Alertas abiertas conocidas» pasaron a `docs/BITACORA.md` (sección «Resumen del estado al cierre», con las rutas relativas de los enlaces ajustadas). E2 `docs/README.md` reorganizado por 7 preguntas con anexo de cumplimiento del enunciado (tablas movidas sin cambios) y sección «Estado final de la entrega» (`v1.0.0` publicado; video: solo el guion está versionado); `docs/glosario.md` creado (37 términos). E3 README nuevo en `backend`, `frontend`, `contabilidad`, `db`, `deploy`, `e2e`, `infraestructura`, `security`, `specs`, `tests`, `tools` y `.github`. Checks: enlaces relativos y anclas de los 16 archivos tocados, 0 rotos (script en el scratchpad); todos los `make <objetivo>` citados existen en el `Makefile`. No se enlazó `deploy/dockerhub/` (lo agrega el PR #58). El bloque «Si su `.env` es anterior a F2» salió del README; sigue en la guía de desarrollo («Primer arranque y `.env` anterior a F2»).

- 2026-10-19, E4 (sesión principal): revisión como lector externo. Repuestas en el README las
  tecnologías y los enlaces de arquitectura (el enunciado exige tecnologías en el README principal y el
  writer las había quitado). Corregida en `docs/README.md` la mención a «integración OIDC»: el Hub usa
  OAuth 2.0 con PKCE y OIDC es una de las mejoras futuras de la sección 9.1. Datos contrastados: run
  38460792159 (`Release`, `v1.0.0`, éxito), objetivos de `make` citados. Enlaces relativos de los 17
  archivos tocados: 0 rotos.
- 2026-10-19, revisión nativa (4 lentes, aprobada): corregidos dos avisos. En `infraestructura/README.md`,
  `make scan-iac` se corre desde la raíz, no desde `infraestructura/terraform`. En el README, las alertas
  de la bitácora se presentan como foto del cierre, con la pestaña *Security* como estado vigente.

## Próximo paso

PR y CI; al integrar con el PR #58, añadir `deploy/dockerhub/` a la fila de `deploy/` del mapa.
