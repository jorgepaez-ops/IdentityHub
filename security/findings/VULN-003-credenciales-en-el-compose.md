# VULN-003 — Credenciales en el compose

| | |
|---|---|
| **Severidad** | no informada por el escáner |
| **Estado** | remediado |
| **Detectado por** | Gitleaks (baseline-scan) |
| **Componente** | `deploy/docker-compose.yml:24,42,82,102-103,133-134` |
| **Amenaza** | AM-012 (secretos en el repositorio o en la imagen) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-003/evidencia.json` |
| **Commit de remediación** | `afab4e9` (T26) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / — |

## Evidencia

```
7 hallazgos en deploy/docker-compose.yml: contrasena-en-variable-de-entorno en líneas 24 y 42; url-de-conexion-con-credenciales en líneas 82, 102, 103, 133 y 134
```

## Por qué importa en esta aplicación

Las credenciales sembradas permiten que quien lea la configuración acceda a servicios internos.

## Remediación

Eliminar los valores sembrados del compose y usar configuración segura en T26.
