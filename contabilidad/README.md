# contabilidad

Aplicación de ejemplo (React y TypeScript) que representa a una empresa tercera. No tiene usuarios ni contraseñas propias: delega el inicio de sesión en Identity Hub con OAuth 2.0 y PKCE y recibe un token con los roles y permisos de la persona. Se sirve en <http://contabilidad.localhost:8080> cuando el stack está levantado.

- `src/`: la aplicación, con sus pruebas (`*.test.ts*`).
- `src/auth/`: el código del inicio de sesión contra el Hub.

```bash
make test-front   # pruebas del frontend y de Contabilidad
make lint         # lint y tipos de todo el proyecto
```

Si quiere integrar su propia aplicación, lea la [guía de integración de terceros](../docs/manuales/integracion-terceros.md), que usa esta como ejemplo completo. Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
