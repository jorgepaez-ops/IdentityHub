# specs

Las especificaciones: la fuente de verdad de lo que el sistema debe hacer. El código y las pruebas se derivan de ellas, y CI comprueba que no se separen.

| Archivo o carpeta | Qué contiene |
|---|---|
| [`00-vision.md`](00-vision.md) | El problema, el alcance y lo que queda fuera |
| [`01-requirements.md`](01-requirements.md) | Requisitos funcionales (RF) y no funcionales (RNF) |
| [`02-domain-model.md`](02-domain-model.md) | Entidades y reglas del dominio |
| [`03-api/`](03-api/openapi.yaml) | Contrato OpenAPI de la API |
| [`04-events/`](04-events/topology.md) | Contrato de eventos y topología de RabbitMQ |
| [`05-security/`](05-security/threat-model.md) | Modelo de amenazas (STRIDE) |
| [`06-acceptance/`](06-acceptance/README.md) | Escenarios de aceptación en Gherkin |
| [`07-traceability.md`](07-traceability.md) | Matriz requisito, escenario, prueba y control (generada) |
| [`adr/`](adr/README.md) | Decisiones de arquitectura, con contexto y consecuencias |

```bash
make gen         # regenera lo derivado de los specs, incluida la matriz
make spec-drift  # comprueba sin red que matriz, escenarios y pruebas coinciden
```

Volver al [mapa del repositorio](../README.md#mapa-del-repositorio) o al [índice de docs](../docs/README.md).
