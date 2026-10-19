# security

La evidencia de seguridad del proyecto.

- `findings/`: una ficha por vulnerabilidad (VULN-001 a VULN-031): qué se encontró, dónde, cómo se remedió y su estado.
- `evidence/`: salidas crudas de los escáneres, conservadas por ejecución como evidencia «antes».
- `zap-reports/`: informes del escáner dinámico OWASP ZAP (los genera `make scan-dast`).

```bash
make scan        # todos los escaneos de seguridad en local
make scan-dast   # solo el escaneo dinámico (el stack debe estar levantado)
```

Para interpretarla: [manual de seguridad](../docs/manuales/seguridad.md), [informe de seguridad](../docs/security-report.html) y la [evidencia por vulnerabilidad](../docs/evidencia/README.md). Volver al [mapa del repositorio](../README.md#mapa-del-repositorio).
