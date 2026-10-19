# .github

El pipeline DevSecOps, que corre en GitHub Actions.

| Archivo | Para qué sirve |
|---|---|
| `workflows/ci.yml` | CI principal: deriva de especificaciones, lint, secretos, análisis estático, dependencias, pruebas, cobertura, imágenes, DAST e IaC |
| `workflows/release.yml` | Publica las imágenes en Docker Hub al crear un tag `vX.Y.Z`, las firma con Cosign y adjunta el SBOM |
| `workflows/baseline-scan.yml` | Escaneo de la línea base vulnerable, para conservar la evidencia «antes» |
| `workflows/scheduled-scan.yml` | Escaneo semanal que detecta avisos nuevos |
| `actions/stack-up/` | Acción reutilizable que levanta el stack en CI |
| `dependabot.yml` | Actualización semanal automática de dependencias (acciones y módulos Go) |

Para correr los mismos controles en local: `make scan` (todos los escaneos) y `make spec-drift`. Qué hace cada etapa y cómo leer sus resultados: [manual de seguridad](../docs/manuales/seguridad.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
