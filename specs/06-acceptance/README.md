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
etiquetas, se lee una sola vez de izquierda a derecha y admite exactamente esta gramática (el resto del texto es libre):

- una ruta de archivo (.py, .js, .ts, .sh, .yml, .yaml o .go) que debe existir;
- `(Nombre)`: clase o función que debe existir como palabra completa en ese archivo;
- `job <id>`: id de un job de ci.yml, con id = letras, dígitos, `_` y `-` (la puntuación final no es parte del id);
- `paso "<nombre>"`: paso de ci.yml con `- name:` exactamente igual;
- `e2e "<título>"`: prueba de Playwright con ese título exacto (`test(`, `test.only(` o `test.skip(`, con comillas simples, dobles o invertidas) en `e2e/tests/**/*.spec.ts`;
- `make <objetivo>`: objetivo del Makefile, suelto, entre comillas invertidas o entre comillas dobles del mismo tipo al abrir y cerrar. Un `make <palabra>` suelto siempre es referencia y debe existir, así que los `.feature` no usan «make» como prosa;
- cualquier otro texto entre comillas dobles es un error que nombra los prefijos aceptados (paso, e2e, make);
- un escenario sin ninguna referencia nombrada falla y el error lista todas las formas aceptadas.

Cualquier referencia que no resuelva, o un escenario `@RNF` sin la línea, rompe el build. Los
escenarios con alguna etiqueta `@RF` quedan fuera (los cubre la correspondencia con el título E2E) y la
matriz sigue contando igual.

Para añadir un escenario `@RNF`: escribe la línea `# Verificación:` justo encima de sus etiquetas con la
prueba que lo ejecuta (archivo existente más clase/función, job, paso o `make`), y esa prueba debe existir
antes de dejar el escenario; `make spec-drift` lo comprueba.

Convención de etiquetas:

- `@RF-NNN` — requisito que verifica
- `@AM-NNN` — amenaza del modelo STRIDE que ejercita
- `@p0` / `@p1` — prioridad, para poder correr solo el núcleo cuando urge
