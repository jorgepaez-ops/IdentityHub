# 0003 — OpenAPI 3.0.3 como fuente de verdad del contrato HTTP

Estado: aceptada · 2026-09-05

## Contexto

Trabajamos bajo SDD: el contrato manda y el código se genera. La duda era usar
OpenAPI 3.1, más moderno y alineado con JSON Schema, o 3.0.3.

## Decisión

Usar **3.0.3**. `oapi-codegen` se apoya en `kin-openapi`, cuyo soporte de 3.1 es
parcial; `openapi-typescript` sí lo soporta bien. Elegir 3.1 nos dejaría con
generación fiable en el frontend y frágil en el backend, que es justo el lado
donde el contrato tiene que ser vinculante.

## Consecuencias

- Se pierde `nullable` como tipo unión y algunas construcciones de JSON Schema
  2020-12. Se compensa con `nullable: true`, que 3.0.3 sí entiende.
- La migración a 3.1 será mecánica cuando la cadena de herramientas madure.
- El job `spec-drift` regenera `backend/internal/api/gen.go` y
  `frontend/src/api/schema.d.ts` y falla si difieren de lo commiteado. Ese gate
  es lo que convierte "los specs son la fuente de verdad" en algo verificable en
  vez de una norma que se respeta por costumbre.
