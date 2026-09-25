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

## Evidencia de vulnerabilidades

- El "después" de cada VULN es de la tarea `Remedia:` correspondiente (T23 en adelante) y, salvo
  indicación explícita del usuario, se documenta cuando esa tarea cierra — no antes, aunque CI ya
  muestre un hallazgo puntual resuelto. VULN-020 es el ejemplo: remediado en T6, pero su captura
  "después" espera a T35 (CI verde), por decisión ya tomada en el archivo de tareas.
