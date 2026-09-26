# CLAUDE.md — Identity Hub (específico para Claude)

Complementa `AGENTS.md`, que está dirigido a Codex. Esto es lo que Claude (orquestador y revisor
en este proyecto) debe recordar activamente porque no vive en un archivo que se cargue solo cada
sesión, o porque ya causó un problema real y vale la pena dejarlo anotado en vez de confiar en
volver a acordarse.

## Checkpoints al cerrar cada tarea

- **Corte de fase.** Revisar la tabla "Progreso" de `odd/tasks/idp-semana-2.md`. Si una fase
  completa queda en `[x]`, avisar proactivamente al usuario que es el punto de corte (estrategia
  de entrega `ask-on-risk`, PR por corte de fase — sección "Alcance autorizado" del archivo de
  tareas) en vez de esperar a que lo pida. No abrir el PR sin que el usuario lo confirme.
- **Composición real, no solo pruebas aisladas.** Si la tarea agrega o modifica un handler HTTP,
  confirmar que queda compuesto de verdad en `Server.Routes()` (implementado directo como método
  de `*Server`, como `Login`/`Register`/`VerifyEmail`/`GetCurrentUser`), no solo un `http.Handler`
  aislado con sus propias pruebas vía `httptest`. T14 y T15 quedaron sin componer (`501` en el
  router real) durante dos tareas completas porque sus pruebas aisladas pasaban igual.
- **Prompt de Desktop en cada run que sirva de evidencia.** Cada vez que se dispare un run de
  CI/baseline-scan que vaya a usarse como "antes" o "después" de un VULN (push, PR o
  `workflow_dispatch`), entregar de inmediato — sin que el usuario lo pida — el prompt para que
  tome las capturas con Claude Desktop (protocolo de dos capas, ver "Protocolo de evidencia" en
  `odd/tasks/idp-semana-2.md`). Ya tocó recordarlo varias veces; no esperar a que el usuario lo pida.
- **Hallazgos nuevos fuera de alcance: anotar, no arreglar de una vez ni ignorar.** Si al correr
  `govulncheck`/`gosec`/lint aparece un hallazgo que no es el que la tarea actual remedia (p. ej.
  una vulnerabilidad en una dependencia distinta), no lo arregles de paso ni lo dejes sin mención:
  anótalo en el archivo de tareas (sección de la tarea donde apareció) con el aviso y el componente,
  **sin inventar un id `VULN-NNN`** (esa asignación es del usuario/Codex al crear la ficha), para que
  se decida en qué tarea futura se remedia. Así se manejó GO-2026-6372 (`amqp091-go`) al cerrar T24.

## Entorno local

- `golangci-lint` no viene instalado en este entorno: `make lint` solo avisa y sigue. Antes de
  cerrar cualquier tarea que toque Go, instalarlo una vez (`go install
  github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2`, PATH incluye `~/go/bin`) y
  correrlo de verdad — CI sí lo tiene y es la única señal real hasta entonces.
- Generadores cacheados: usar `~/go/bin/oapi-codegen` (v2.5.1 real, no "(devel)") y `~/go/bin/sqlc`.
  Evitar binarios en `/private/tmp/idp-gen-bin/` sin el ldflag de versión correcto.

## Delegación a Codex (`codex:codex-rescue`)

- Nunca usar backticks simples en el prompt de instrucciones — ni para citar comandos, flags o
  rutas, ni para decir "no hagas X". El forwarder los interpreta como shell real y los ejecuta
  literalmente (se confirmó con un `git push` real que solo falló por falta de upstream). Usar
  comillas normales o prosa sin backticks.
- **Codex no tiene red.** Verificado con `codex doctor` y con el `sandbox_policy` real de sus
  sesiones (`.codex/sessions/**/*.jsonl`): sus tareas corren en sandbox `workspace-write` con
  `"network_access": false`, bajo un `permission_profile` de tipo `"managed"` (no es un flag que se
  cambie con una edición cualquiera de `config.toml`; parece una política de cuenta/organización).
  Por eso T24 falló con "no such host" al intentar `go get`/`govulncheck`: no fue un límite diario ni
  un bug puntual, es la política de red del sandbox. **No delegar a Codex ninguna tarea que necesite
  red saliente** (subir dependencias, `npm install` con paquetes nuevos, `curl` a un host externo,
  etc.); esas se toman directo con Claude (que sí tiene red en este entorno), como ya pasó en T23
  (401 de la API) y T24 (DNS del sandbox).
- **Monitoreo obligatorio de toda tarea delegada a Codex.** No basta con lanzar la tarea y esperar a
  que el usuario pregunte "¿cómo va?". Justo después de lanzarla (ID `task-...`), levantar en el acto
  un poll en segundo plano (`codex-companion.mjs status <job> --json` en un bucle con espera de ~15-20s)
  que avise solo: (a) cuando termine (éxito o bloqueo), trayendo el `result` completo; (b) si pasa un
  tiempo largo sin que cambie de fase (revisar `phase`/`updatedAt` en cada poll) — en ese caso avisar
  proactivamente que Codex podría estar atascado, en vez de dejar que el usuario se entere solo si
  pregunta. El usuario lo ha tenido que pedir varias veces: es la norma para toda delegación, no una
  excepción puntual.

## Evidencia de vulnerabilidades

- El "después" de cada VULN es de la tarea `Remedia:` correspondiente (T23 en adelante) y, salvo
  indicación explícita del usuario, se documenta cuando esa tarea cierra — no antes, aunque CI ya
  muestre un hallazgo puntual resuelto. VULN-020 es el ejemplo: remediado en T6, pero su captura
  "después" espera a T35 (CI verde), por decisión ya tomada en el archivo de tareas.
