# Inventario

Aplicación mínima de ejemplo de la empresa «Empresa B». Demuestra que Identity Hub atiende a una segunda empresa sin mezclar sus roles con los de Contabilidad ([ADR 0015](../specs/adr/0015-empresas-y-aislamiento-de-roles.md)).

## Qué hace

- Inicia sesión con OAuth 2.0 Authorization Code y PKCE contra el Hub (`client_id` `inventario`); no tiene usuarios propios.
- Verifica el token con el JWKS del Hub y decide cada pantalla por sus permisos: `productos.ver`, `productos.editar` y `movimientos.registrar`.
- Muestra «Sin acceso a Inventario» si el Hub responde `error=access_denied` (otra empresa o sin rol) o si el token no trae permisos de Inventario.
- Sus datos (productos, movimientos) son de ejemplo y viven en memoria.

## Dónde está

| Qué | Dónde |
|---|---|
| Dirección local | `http://inventario.localhost:8080` (el mismo Nginx del Hub, bloque en [`frontend/nginx/default.conf`](../frontend/nginx/default.conf)) |
| Cliente, empresa, permisos y roles en el Hub | [`db/migrations/000011_organizations.up.sql`](../db/migrations/000011_organizations.up.sql) |
| Configuración de compilación | [`src/config.ts`](src/config.ts) |
| Guía de integración | [`docs/manuales/integracion-terceros.md`](../docs/manuales/integracion-terceros.md) |

## Comandos

```bash
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```
