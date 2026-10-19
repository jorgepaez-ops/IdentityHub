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

- [ ] **E1. README raíz y bitácora.** Ruta: delegada (writer único para E1–E3: varios archivos no triviales).
- [ ] **E2. Índice de `docs/` por preguntas y glosario.** Ruta: delegada (mismo writer).
- [ ] **E3. README por carpeta.** Ruta: delegada (mismo writer).
- [ ] **E4. Revisión y verificación.** Ruta: sesión principal. Checks: comprobación de enlaces relativos,
  comandos citados existen en el `Makefile`, lectura completa como lector externo.

## Criterios de aceptación

- Desde el README raíz se llega en uno o dos clics a cualquier documento, sabiendo de antemano qué contiene.
- Cada carpeta principal explica su contenido al abrirla en GitHub.
- Cero enlaces relativos rotos en los archivos tocados.

## Progreso

- 2026-10-19: rama `docs/lector-externo` creada desde `main`; inventario hecho (ver Problema).

## Próximo paso

E1–E3 (writer).
