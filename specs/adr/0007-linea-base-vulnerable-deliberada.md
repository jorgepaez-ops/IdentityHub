# 0007 — Línea base deliberadamente vulnerable

Estado: aceptada · 2026-09-05

## Contexto

Un pipeline cuyos gates nunca han fallado no demuestra nada: es indistinguible de
uno mal configurado que deja pasar todo. Hace falta evidencia de que cada control
detecta lo que dice detectar.

## Decisión

Sembrar vulnerabilidades **reales y conocidas** en el commit inicial —imágenes
base antiguas, dependencias con CVE publicado, secretos en el código, SQL
concatenado, `USER root`, cabeceras de seguridad ausentes— y congelarlo en el tag
`v0.0.0-vuln-baseline`.

Sobre ese tag corre `baseline-scan.yml` con todos los gates en modo reporte
(`continue-on-error: true`): su trabajo es *encontrar*, no *impedir*. Cada
hallazgo se documenta en `security/findings/VULN-NNN-*.md` y se remedia en `main`
en un PR propio que referencia su ficha.

Es la misma lógica de OWASP Juice Shop o DVWA, aplicada a nuestra propia cadena
de suministro en vez de a una aplicación de terceros.

## Consecuencias

- El historial de git **es** la evidencia: hay un antes y un después medible por
  herramienta (`docs/security-report.md` compara el conteo de CVE entre el tag y
  `main`).
- Riesgo real de confusión: alguien —persona o asistente— puede "arreglar" por su
  cuenta una vulnerabilidad sembrada y destruir la evidencia. Se mitiga con este
  ADR, con avisos en el encabezado de cada archivo afectado y con la
  inmutabilidad del tag.
- El repositorio contiene código inseguro a propósito. **El tag nunca se
  despliega** y así se documenta en el README, para que nadie lo tome por una
  versión utilizable.
