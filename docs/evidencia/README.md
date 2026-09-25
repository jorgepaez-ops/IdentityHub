# Evidencia de vulnerabilidades

Este directorio conserva la evidencia textual verificable de cada vulnerabilidad.
Las capturas se mantienen fuera del repositorio en el informe externo para no
versionar imágenes ni exponer datos sensibles.

## Flujo rápido

1. Claude Desktop navega GitHub en la sesión autenticada del usuario, toma las
   capturas y las reúne en `Evidencias-CI-LineaBase-IdentityHub.docx`, fuera del
   repositorio.
2. Desktop entrega como texto el workflow, run ID y URL, job, paso, SHA y alerta
   literal sin secretos.
3. Claude escribe o completa el `evidencia.json` del hallazgo; el campo
   `captura` referencia la sección correspondiente del informe externo.

## Dos capas de evidencia

| Capa | Contenido | Conservación |
|---|---|---|
| Repositorio | `docs/evidencia/VULN-XXX/evidencia.json` y los artefactos ya guardados en `security/evidence/` | Versionada; perdura tras la retención de logs de Actions. |
| Informe externo | Capturas de antes y después organizadas por hallazgo | No se versiona; el JSON apunta a su sección mediante `captura`. |

Cada hallazgo tiene su directorio `docs/evidencia/VULN-XXX/`. Solo contiene
texto estructurado: no se guardan imágenes en este directorio.

## Forma fija de `evidencia.json`

Los valores desconocidos permanecen en `null`; no se inventan SHA, IDs de run,
URLs ni referencias de captura.

```json
{
  "vuln": "VULN-002",
  "gate": ["gosec G401 (baseline-scan)", "CodeQL", "Semgrep"],
  "antes": {
    "commit_linea_base": "053e15f...",
    "commit_main": null,
    "workflow": null,
    "run_id": null,
    "run_url": null,
    "artefacto": "security/evidence/...",
    "captura": null
  },
  "remediacion": {
    "tarea": "T23",
    "commit": null,
    "ficha": "security/findings/VULN-002-md5-para-contrasenas.md"
  },
  "despues": {
    "commit": null,
    "workflow": null,
    "run_id": null,
    "run_url": null,
    "captura": null
  }
}
```

`captura` es una referencia de texto, por ejemplo, `"informe §VULN-002 antes"`.
Dentro de `antes` y `despues` se admiten dos campos opcionales de texto: `hallazgos` (lista con la
alerta tal como la muestra el gate, sin secretos) y `observacion` (matices, como "ningún gate lo
detecta" o "falta la parte de Trivy image"). Un `gate` vacío significa que ningún gate lo detecta hoy.
Los datos de antes reutilizan `security/evidence/*-baseline.*` y
`security/evidence/actions-<run_id>/`; los recortes SARIF o JSON nuevos también
se guardan allí.

## Responsabilidades y seguridad

| Actor | Responsabilidad |
|---|---|
| Usuario | Confirma que existe la captura y, para hallazgos sin gate, ejecuta localmente `curl -sI http://localhost:8080` sobre la línea base y tras la remediación. |
| Claude Desktop | Navega GitHub en modo solo lectura, realiza las capturas y prepara el informe externo. |
| Claude | Recibe los datos textuales y escribe o completa el `evidencia.json`. |
| Codex | Implementa la remediación y actualiza la ficha con el commit y el estado cuando corresponda; nunca toma capturas ni escribe bajo `docs/evidencia/VULN-*/`. |

No se incluyen secretos reales en capturas ni JSON. Antes de capturar la salida
de Gitleaks, se revisa como texto y se oculta la columna del secreto; los JSON
eliminan `Secret` y `Match`. El informe externo tampoco se sube al repositorio.

## Evidencia posterior a la remediación

`ci.yml` se ejecuta en `push` a `main`, `pull_request` y
`workflow_dispatch`; un push a una rama sin PR no lo dispara. Tras la
remediación, el usuario abre un PR por corte de fase o lanza CI manualmente
sobre la rama, y registra el run ID y la URL en el JSON y en el informe.
