# 06 — Criterios de aceptación

Cada archivo `.feature` describe el comportamiento observable del sistema en
Gherkin. **No se ejecutan directamente**: son la fuente de la que se derivan los
specs de Playwright en `e2e/tests/`, uno por escenario, con el mismo nombre.

La correspondencia se verifica en CI: el job `spec-drift` comprueba que todo
escenario `@RF-NNN` tiene un `test(...)` de Playwright cuyo título empieza por
ese mismo identificador. Un escenario sin prueba rompe el build.

Los escenarios de `requisitos-no-funcionales.feature` solo llevan etiquetas `@RNF-NNN`: no se derivan
en un spec de Playwright y cada uno nombra, en un comentario `# Verificación:`, la prueba o el
control que lo ejecuta de verdad (por ejemplo `scripts/pipeline_controls_test.py` o `make load-test`).

Ese enlace se verifica por máquina (`verification_errors` en `scripts/traceability.py`, parte de
`make spec-drift` y de `traceability.py --check`): la línea `# Verificación:` va justo encima de las
etiquetas y debe nombrar (1) un archivo existente y (2) algo concreto que exista por nombre: una clase o
función entre paréntesis en ese archivo (`scripts/x_test.py (ClaseDePrueba)`), un `job <id>` de `ci.yml`,
un paso entre comillas (`paso "Nombre"`, línea `- name:` de `ci.yml`) o un `"make <objetivo>"` del
Makefile. Cualquier referencia que no resuelva, o un escenario `@RNF` sin la línea, rompe el build. Los
escenarios con alguna etiqueta `@RF` quedan fuera (los cubre la correspondencia con el título E2E) y la
matriz sigue contando igual.

Para añadir un escenario `@RNF`: escribe la línea `# Verificación:` justo encima de sus etiquetas con la
prueba que lo ejecuta (archivo existente más clase/función, job, paso o `make`), y esa prueba debe existir
antes de dejar el escenario; `make spec-drift` lo comprueba.

Convención de etiquetas:

- `@RF-NNN` — requisito que verifica
- `@AM-NNN` — amenaza del modelo STRIDE que ejercita
- `@p0` / `@p1` — prioridad, para poder correr solo el núcleo cuando urge
