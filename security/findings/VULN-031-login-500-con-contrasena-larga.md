# VULN-031 — Login devuelve 500 con contraseña larga

| | |
|---|---|
| **Severidad** | MEDIUM |
| **Confianza** | HIGH |
| **Estado** | en remediación |
| **Detectado por** | OWASP ZAP API scan (30002) · job 11 · DAST (OWASP ZAP) |
| **Componente** | Servicio de login (`backend/internal/auth/login`) y handler de API (`backend/internal/api/login.go`) |
| **Amenaza** | AM-001 (fuerza bruta sobre contraseñas; bloqueo tras 5 fallos) |
| **Sembrada** | no |
| **Evidencia antes** | `make scan-dast` local y reproducción manual documentados en `odd/tasks/idp-semana-3.md`, T15 |
| **Commit de remediación** | pendiente |
| **Evidencia después** | pendiente |
| **Run de Actions (antes/después)** | pendiente / pendiente |

## Evidencia

El escaneo de API de OWASP ZAP informó `Format String Error` (30002) de severidad Medium
sobre el parámetro `password` de `POST /api/v1/auth/login`. La reproducción manual registrada
en T15 confirma que una contraseña de 128 caracteres responde `401`, mientras que una de 129
respondía `500` con `Login could not be completed`; el error tampoco producía una línea de log
de la API.

## Por qué importa en esta aplicación

El límite de 128 caracteres se valida antes de ejecutar Argon2id. Al propagar ese error como un
fallo interno, los intentos inválidos no se registraban y no contribuían al bloqueo de cuenta ni
a la ventana de fallos por IP. Un cliente no autenticado podía repetir solicitudes de 129 caracteres
que producían errores 500 sin activar esos controles, debilitando la mitigación de AM-001 y
manteniendo el endpoint en un estado de error observable.

## Remediación

T15c trata `password.InvalidPasswordError` como credenciales inválidas tanto para cuentas existentes
como inexistentes. El camino de cuenta existente conserva el registro de fallo y la evaluación de
bloqueo. El handler devuelve el problema genérico `invalid-credentials` y registra únicamente el
error y el identificador de petición cuando existe un fallo interno inesperado. El commit de
remediación y la evidencia posterior quedan pendientes.
