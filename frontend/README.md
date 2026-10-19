# frontend

La consola web del Hub, hecha con React, TypeScript y Vite, y la configuración de Nginx que la sirve. Es lo que ven las personas empleadas y administradoras: inicio de sesión con MFA, perfil, usuarios, roles y permisos, registro de auditoría y claves de servicio.

- `src/`: el código de la aplicación y sus pruebas (`*.test.tsx`).
- `nginx/`: configuración del servidor web.
- `Dockerfile`: construye la imagen `web`, que sirve también la aplicación de [`contabilidad/`](../contabilidad/README.md).

```bash
make test-front   # pruebas del frontend y de Contabilidad
make lint         # lint y tipos de todo el proyecto
```

Los tipos de la API se regeneran desde el contrato con `make gen`. Para ver cómo se usa: [manual de usuario](../docs/manuales/usuario.md). Para el entorno de desarrollo: [guía de desarrollo](../docs/guia-desarrollo.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
