# VULN-029 — `google.golang.org/protobuf` desactualizado

| | |
|---|---|
| **Severidad** | HIGH (CVSS 3.1: 7.5; no alcanzable según análisis de llamadas) |
| **Estado** | remediado (T34b) |
| **Detectado por** | osv-scanner v2.6.0 (`ci.yml`, job `5 · Dependencias vulnerables`, paso `osv-scanner`) |
| **Componente** | `backend/go.mod`: `google.golang.org/protobuf@v1.31.0` → `v1.36.12` (T34b, indirecta) |
| **Avisos** | GO-2024-2611 / GHSA-8r3f-844c-mc37 (CVE-2024-24786), corregido en 1.33.0 |
| **Amenaza** | AM-009 (dependencia comprometida en la cadena de suministro) |
| **Sembrada** | no — hallazgo nuevo detectado al actualizar el escáner |
| **Evidencia antes** | `docs/evidencia/VULN-029/evidencia.json` |
| **Commit de remediación** | pendiente (T34b) |
| **Evidencia después** | pendiente (T34b) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36281691237 / — |

## Evidencia

```
GO-2024-2611 / GHSA-8r3f-844c-mc37 (CVE-2024-24786)
  google.golang.org/protobuf@v1.31.0
  CVSS 3.1 7.5: AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H
  fixed in 1.33.0
```

`osv-scanner` v2.6.0 clasificó el aviso como no llamado y `govulncheck` lo reportó como no
alcanzable. Esa condición reduce la exposición inmediata, pero no elimina el aviso HIGH ni la
necesidad de sustituir la versión indirecta por una corregida.

## Por qué importa en esta aplicación

La disponibilidad del proveedor de identidad es un activo de seguridad. Una dependencia indirecta
con un fallo de denegación de servicio puede quedar al alcance al cambiar las dependencias o sus
usos; mantenerla corregida cumple AM-009 y evita arrastrar esa deuda a futuras versiones.

## Remediación

T34b actualizó `google.golang.org/protobuf` de 1.31.0 a 1.36.12 (última versión estable
disponible al cerrar la tarea, muy por encima del mínimo 1.33.0) mediante `go get` +
`go mod tidy`, y confirmó con osv-scanner v2.6.0 (`--recursive --all-vulns`) y `govulncheck` que
el aviso ya no aparece.

La dependencia que arrastra `protobuf` como indirecta es `github.com/prometheus/client_golang`
(confirmado con `go mod why -m google.golang.org/protobuf`: la cadena es
`internal/observability` → `prometheus/client_golang` → `google.golang.org/protobuf/proto`;
`prometheus/client_model` y `prometheus/common` también la requieren en la misma versión). Como
`google.golang.org/protobuf` ya aparecía fijada explícitamente en `backend/go.mod` (indirecta),
se subió directamente con `go get google.golang.org/protobuf@v1.36.12` sin tocar
`prometheus/client_golang`.
