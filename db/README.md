# db

El esquema de PostgreSQL y las consultas del proyecto.

- `migrations/`: migraciones numeradas con su par `.up.sql` y `.down.sql`. Son inmutables: un cambio nuevo se agrega como una migración nueva.
- `queries/`: consultas SQL (`audit`, `roles`, `service_keys`, `users`) de las que `sqlc` genera el código Go de acceso a datos.

```bash
make migrate   # aplica las migraciones pendientes (el stack debe estar levantado)
make psql      # abre una consola de PostgreSQL
make gen       # regenera el código derivado, incluido el de sqlc
```

El modelo de datos está en el [modelo de dominio](../specs/02-domain-model.md). Cómo se regenera el código: [guía de desarrollo](../docs/guia-desarrollo.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
