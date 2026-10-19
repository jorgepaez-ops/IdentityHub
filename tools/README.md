# tools

Fija las versiones de las herramientas de desarrollo y de CI para que todos usen las mismas: el módulo Go con las herramientas declaradas (`go.mod`), la dependencia de Python para validar el contrato OpenAPI (`requirements-openapi.in` y `.txt`) y la configuración de `osv-scanner.toml`. No tiene código propio.

Los generadores se instalan desde aquí, sin indicar versión:

```bash
go -C tools install <paquete>
```

El Makefile exige la versión de `sqlc` que fija este módulo (`make gen`). Más contexto en la [guía de desarrollo](../docs/guia-desarrollo.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
