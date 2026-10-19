# tests

Pruebas que no pertenecen a un solo servicio. Hoy contiene la prueba de carga de latencia de autenticación (RNF-010), escrita para k6.

- `load/auth-latency.js`: mide la latencia de renovación de sesión, `GET /me` y JWKS contra el stack levantado; falla si el p95 supera 300 ms.

```bash
make up-app      # el stack debe estar levantado
make load-test   # corre k6 en un contenedor
```

Las pruebas unitarias viven junto al código ([`backend/`](../backend/README.md), [`frontend/`](../frontend/README.md)) y las de extremo a extremo en [`e2e/`](../e2e/README.md). Detalle de la prueba de carga en la [guía de desarrollo](../docs/guia-desarrollo.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
