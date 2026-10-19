# backend

La API y el worker de Identity Hub, escritos en Go. La API atiende las peticiones del Hub y de las aplicaciones conectadas; el worker consume eventos de RabbitMQ y envía los correos (invitaciones y códigos MFA).

- `cmd/api` y `cmd/worker`: los dos ejecutables.
- `internal/`: el código por responsabilidad (`api`, `auth`, `audit`, `domain`, `events`, `notify`, `store`, `config`, `observability`; `testdb` y `testsession` son apoyo de las pruebas).
- `Dockerfile`: construye las imágenes de la API y del worker.

```bash
make test-go            # pruebas unitarias con detector de carreras
make test-integration   # pruebas de integración con PostgreSQL temporal
```

Para entenderlo a fondo: [manual de arquitectura](../docs/manuales/arquitectura.md), [guía de desarrollo](../docs/guia-desarrollo.md) y el contrato [OpenAPI](../specs/03-api/openapi.yaml). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
