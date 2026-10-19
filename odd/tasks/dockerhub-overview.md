# Overview de las imágenes en Docker Hub

## Objetivo

Que cada repositorio de Docker Hub (`jorgeivp/identity-hub-api`, `-worker`, `-web`) explique qué es la
imagen, cómo se usa y cómo se verifica, y que ese texto no se desactualice.

## Problema y por qué

En la revisión, el profesor señaló que en Docker Hub no aparece ninguna información de las imágenes
publicadas. Verificado el 2026-10-19 con la API pública de Docker Hub: `description` vacía y
`full_description` de 0 bytes en los tres repositorios, con unas 200 descargas cada uno.

## Alcance autorizado

- Descripciones en el repositorio, en `deploy/dockerhub/` (una por imagen, más la descripción corta).
- Workflow `.github/workflows/dockerhub-description.yml` que las sincroniza con Docker Hub al cambiar
  en `main` y a mano con `workflow_dispatch`. Decisión del usuario (2026-10-19): sincronización
  automática, no pegado manual.
- Environment nuevo `dockerhub-description`, limitado a `main` y con su propio secreto, para no
  ampliar el environment `dockerhub` del release (restringido a tags `v*`).
- Enlace a las descripciones desde la documentación existente donde ya se mencionan las imágenes.

Fuera de alcance: cambiar `release.yml` o el environment `dockerhub`.

## Restricciones

- Mantenimiento posterior a la entrega: sin fichas VULN.
- Acciones fijadas por SHA, como en el resto de workflows.
- Docker Hub: descripción corta de 100 caracteres como máximo y overview de 25 000 bytes como máximo.
- Contenido en español, como el resto de la documentación del proyecto.
- Datos reales: digests de `v1.0.0` consultados en Docker Hub e identidad de firma tomada de `release.yml`.

## Tareas

- [x] **D1. Descripciones y workflow.** Ruta: delegada (writer: 3 documentos y 1 workflow, más de un
  archivo no trivial). Checks: validación de los workflows con las herramientas que use CI, límites de
  tamaño, comandos `cosign verify` probados contra los digests reales de `v1.0.0`.
- [ ] **D2. Configuración remota.** El usuario crea en Docker Hub un token con permiso Read, Write,
  Delete. Se crean el environment `dockerhub-description` (solo `main`) y su secreto. Requiere
  autorización explícita del usuario para cada operación remota.
- [ ] **D3. Publicación y verificación.** Merge, ejecución del workflow y comprobación con la API
  pública de Docker Hub de que los tres repositorios muestran la descripción.

## Criterios de aceptación

- Los tres repositorios muestran la descripción corta y el overview en Docker Hub.
- Un cambio en `deploy/dockerhub/` que llega a `main` vuelve a sincronizar sin intervención manual.
- El release y su environment quedan sin cambios.

## Progreso

- 2026-10-19: rama `docs/dockerhub-overview` creada; exploración hecha (ver Problema).
- 2026-10-19, D1 (writer, pendiente de revisión): creados `deploy/dockerhub/{api,worker,web}.md`
  (5,3 a 5,8 KB cada uno), `short-description-{api,worker,web}.txt` (86, 95 y 98 caracteres),
  `deploy/dockerhub/README.md` y `.github/workflows/dockerhub-description.yml` (acción
  `peter-evans/dockerhub-description` v5.0.0 fijada por SHA; la descripción corta se lee del archivo en un
  paso previo porque la acción la recibe como texto). Enlaces añadidos en `README.md` y en el manual de
  despliegue. Checks: `cosign verify` y `cosign verify-attestation --type cyclonedx` (Cosign v3.1.3)
  correctos contra los tres digests de `v1.0.0`; `python3 scripts/pipeline_controls_test.py` OK (13 pruebas);
  Checkov 3.3.23 (`github_actions`, el mismo que CI) sobre el workflow nuevo: 20 pasaron, 0 fallaron.
  actionlint y zizmor no se usan en CI ni están instalados.
- 2026-10-19, revisión de D1 (sesión principal): corregido `TRUSTED_PROXIES` en `api.md` (no es
  obligatoria: vacía, la API usa la dirección de la conexión y no acepta `X-Forwarded-For`) y el
  encabezado del workflow (`DOCKERHUB_NAMESPACE` es variable del repositorio, no del environment).
  Puertos, variables de imagen, `make setup`, upstream y `HEALTHCHECK` de `web` contrastados con el repo.
- 2026-10-19, D2 (parcial): environment `dockerhub-description` creado con autorización del usuario,
  con política de despliegue limitada a la rama `main` (leída de vuelta con la API de GitHub).
  Pendiente: el usuario crea el token Read, Write, Delete y carga `DOCKERHUB_DESCRIPTION_TOKEN`.

## Próximo paso

D2: secreto `DOCKERHUB_DESCRIPTION_TOKEN` (lo carga el usuario); luego D3.
