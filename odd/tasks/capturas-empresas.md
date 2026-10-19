# Capturas del manual: empresas e Inventario

## Objetivo
Regenerar las capturas del manual de usuario para que muestren las pantallas actuales tras los roles por empresa (PR #62/#63) y quitar las notas de "captura pendiente".

## Problema
`docs/manuales/usuario.md` declara capturas desactualizadas (línea 39, sección 3 "3.0 a 3.5", nota de «Sin acceso» en 4.x) y la sección 4.9 (Inventario) no tiene capturas. `e2e/manual/capturas.ts` no recorre el selector **Empresa**, el bloque **Aplicaciones conectadas** ni Inventario.

## Alcance autorizado
- Ampliar `e2e/manual/capturas.ts` con las pantallas nuevas.
- Regenerar `docs/manuales/img/usuario/*.png` con `make capturas` contra el stack local.
- Actualizar `docs/manuales/usuario.md` (referencias nuevas, quitar notas de pendiente) y el conteo en `docs/README.md`.
- Sin cambios de producto. Inventario se captura tal cual (código propio, nada copiado).

## Tareas
- [x] T1 Ampliar el script de capturas (empresa en Inicio/Usuarios/Roles, campo Empresa al invitar, Aplicaciones conectadas, Sin acceso actual, Inventario consulta/bodeguero, roles de Empresa B). Ruta: delegada (escritor Sonnet; 2+ archivos no triviales + stack Docker).
- [x] T2 Regenerar imágenes y actualizar el manual y el conteo.

## Verificación
- `make capturas` termina sin fallos.
- Toda referencia `img/usuario/*.png` del manual existe y no quedan notas de captura pendiente.
- `npx tsc --noEmit` / lint de e2e sin errores.

## Entrega
Rama `docs/capturas-empresas`, un PR; mergear cuando pase CI (autorizado por el usuario para los PR de esta jornada).

## Progreso
- 2026-10-19: rama y documento creados.
- 2026-10-19: T1+T2 (escritor Sonnet). 10 capturas nuevas (administrador-23/24, empleado-17, inventario-01..07); 65 PNG en total. Notas de pendiente quitadas (1.3, 3, 4.6, 4.9).
  - `make capturas`: pasa (~48 s, segunda corrida; la primera falló por una aserción equivocada sobre Sin acceso en Inventario).
  - `npx tsc --noEmit -p .` en e2e: pasa.
  - Referencias del manual vs. archivos en img/usuario: coinciden en ambos sentidos.
  - Revisión del padre: capturas contabilidad-14, administrador-01 e inventario-03 abiertas y correctas.
