# 06 — Criterios de aceptación

Cada archivo `.feature` describe el comportamiento observable del sistema en
Gherkin. **No se ejecutan directamente**: son la fuente de la que se derivan los
specs de Playwright en `e2e/tests/`, uno por escenario, con el mismo nombre.

La correspondencia se verifica en CI: el job `spec-drift` comprueba que todo
escenario `@RF-NNN` tiene un `test(...)` de Playwright cuyo título empieza por
ese mismo identificador. Un escenario sin prueba rompe el build.

Los escenarios de `requisitos-no-funcionales.feature` solo llevan etiquetas `@RNF-NNN`: no se derivan
en un spec de Playwright y cada uno nombra, en un comentario `# Verificación:`, la prueba o el
control que lo ejecuta de verdad (por ejemplo `make load-test`).

Convención de etiquetas:

- `@RF-NNN` — requisito que verifica
- `@AM-NNN` — amenaza del modelo STRIDE que ejercita
- `@p0` / `@p1` — prioridad, para poder correr solo el núcleo cuando urge
