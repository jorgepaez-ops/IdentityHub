# 0008 — GitHub Actions como orquestador y GHCR como registry

Estado: aceptada · 2026-09-05

## Contexto

El pipeline es el entregable principal. Las alternativas eran GitLab CI
autoalojado en el propio compose, Jenkins en contenedor, o GitHub Actions.

## Decisión

**GitHub Actions** con publicación en **GHCR**.

- No consume recursos del portátil, que ya sostiene diez contenedores.
- El ecosistema de acciones de seguridad —CodeQL, Trivy, Gitleaks, Syft, Cosign,
  Dependabot— es el más maduro y no requiere configurar credenciales de larga
  vida.
- La integración con *Security → Code scanning* convierte los SARIF en hallazgos
  navegables en la interfaz del repositorio, en vez de líneas perdidas en un log.
- **Cosign keyless vía OIDC**: la firma se ancla en la identidad del workflow y
  no hay ninguna clave privada que guardar. Esto sería mucho más laborioso en un
  Jenkins autoalojado.

## Consecuencias

- Dependemos de un servicio externo; sin red no hay pipeline. Se mitiga con un
  objetivo `make scan` que corre los mismos gates en local con las mismas
  herramientas.
- El repositorio debe ser público para disponer de minutos y de CodeQL sin coste.
  Encaja con el requisito de "libre uso" y con la licencia Apache-2.0.
- Las `permissions:` se declaran al mínimo por job (AM-022); el token por defecto
  se restringe a solo lectura a nivel de repositorio.
