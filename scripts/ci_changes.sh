#!/usr/bin/env bash
# Job 0 de ci.yml: decide si el cambio toca código (code=true) o solo documentación (code=false).
# Falla hacia "correr todo": lista vacía, error de git, evento distinto de pull_request/push o push
# sin commit previo producen code=true. Se prueba en scripts/ci_changes_test.py.
#
# Entrada (variables de entorno): EVENT_NAME, PR_BASE_SHA, PR_HEAD_SHA, PUSH_BEFORE, PUSH_SHA.
# Salida: code=... en $GITHUB_OUTPUT y un resumen en $GITHUB_STEP_SUMMARY (cada uno solo si está definido).
set -u

EVENT_NAME="${EVENT_NAME:-}"
PR_BASE_SHA="${PR_BASE_SHA:-}"
PR_HEAD_SHA="${PR_HEAD_SHA:-}"
PUSH_BEFORE="${PUSH_BEFORE:-}"
PUSH_SHA="${PUSH_SHA:-}"

summary() {
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    cat >> "$GITHUB_STEP_SUMMARY"
  else
    cat > /dev/null
  fi
}

decide() {
  echo "Decisión: code=$1 ($2)"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "code=$1" >> "$GITHUB_OUTPUT"
  fi
  {
    echo "### Detección de cambios"
    echo "- Evento: $EVENT_NAME"
    echo "- code=$1 ($2)"
  } | summary
  exit 0
}

files=""
case "$EVENT_NAME" in
  pull_request)
    files=$(git diff --no-renames --name-only "$PR_BASE_SHA...$PR_HEAD_SHA") || decide true "git diff falló"
    ;;
  push)
    case "$PUSH_BEFORE" in
      ""|*[!0]*) ;;
      *) decide true "push sin commit previo" ;;
    esac
    files=$(git diff --no-renames --name-only "$PUSH_BEFORE..$PUSH_SHA") || decide true "git diff falló"
    ;;
  *)
    decide true "evento $EVENT_NAME: se corre todo"
    ;;
esac
if [ -z "$files" ]; then
  decide true "lista de archivos vacía"
fi
echo "Archivos modificados:"
echo "$files"
{
  echo "### Archivos modificados"
  echo '```'
  echo "$files"
  echo '```'
} | summary
while IFS= read -r f; do
  case "$f" in
    # Rutas que gobiernan comportamiento: siempre código, aunque sean .md.
    .github/*|specs/*|scripts/*) decide true "código: $f" ;;
    *.md|odd/*|docs/*) ;;
    *) decide true "código: $f" ;;
  esac
done <<< "$files"
decide false "solo documentación"
