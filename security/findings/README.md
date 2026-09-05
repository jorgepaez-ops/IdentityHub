# Fichas de hallazgos

Una ficha por vulnerabilidad detectada en la línea base
(`v0.0.0-vuln-baseline`). Formato fijo para que sean comparables y se puedan
resumir mecánicamente en `docs/security-report.md`.

## Numeración

- **VULN-001 a VULN-019** — vulnerabilidades **sembradas a propósito**
  (specs/adr/0007). Cada una existe para que un gate concreto tenga algo real
  que informar.
- **VULN-020 en adelante** — hallazgos **no previstos** que los escáneres
  encontraron por su cuenta. Estos son los más interesantes: demuestran que el
  pipeline aporta información que no teníamos, en vez de limitarse a confirmar
  lo que ya sabíamos.

## Estados

`abierto` · `en remediación` · `remediado` · `aceptado` (con entrada en
`security/.trivyignore`, justificación y fecha de expiración)

## Plantilla

```markdown
# VULN-NNN — <título en una línea>

| | |
|---|---|
| **Severidad** | CRITICAL / HIGH / MEDIUM / LOW |
| **Estado** | abierto |
| **Detectado por** | <herramienta> · job `<job>` |
| **Componente** | <ruta:línea o dependencia@versión> |
| **Amenaza** | AM-NNN del modelo STRIDE |
| **Sembrada** | sí / no |

## Evidencia
(salida literal del escáner)

## Por qué importa en esta aplicación
(explotabilidad concreta aquí, no la descripción genérica del CVE)

## Remediación
(qué se cambia, y el commit o PR cuando esté hecho)
```
