# Descripciones de Docker Hub

Estos archivos son la fuente de verdad de lo que Docker Hub muestra para `jorgeivp/identity-hub-api`, `-worker` y `-web`. No se editan en Docker Hub: el workflow [`dockerhub-description.yml`](../../.github/workflows/dockerhub-description.yml) los sincroniza al cambiar en `main` (o a mano con `workflow_dispatch`). Docker Hub limita el overview a 25 000 bytes y la descripción corta a 100 caracteres.

- `api.md`, `worker.md`, `web.md`: overview de cada imagen (usan URL absolutas, porque Docker Hub no resuelve enlaces relativos).
- `short-description-*.txt`: descripción corta de cada imagen, en una línea.
- Al publicar una versión nueva, actualice los digests y los comandos `cosign` de los tres overview.
