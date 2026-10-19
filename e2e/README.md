# e2e

Pruebas de extremo a extremo con Playwright: abren un navegador real contra el stack levantado y comprueban el sistema como lo usaría una persona. Cada escenario de [`specs/06-acceptance/`](../specs/06-acceptance/README.md) tiene aquí una prueba con el mismo nombre.

- `tests/`: las pruebas, un archivo por tema (autenticación, roles y permisos, claves de servicio, observabilidad...).
- `support/`: utilidades compartidas (cuentas, API, correo en Mailpit, base de datos).
- `manual/`: el script que toma las capturas del manual de usuario.

```bash
make up-app      # el stack debe estar levantado
make e2e         # corre la suite completa
make capturas    # regenera las capturas del manual de usuario
```

Cómo ejecutarlas a mano y cómo se relacionan con los requisitos: [guía de desarrollo](../docs/guia-desarrollo.md) y [matriz de trazabilidad](../specs/07-traceability.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
