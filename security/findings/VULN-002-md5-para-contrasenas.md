# VULN-002 — MD5 como función de derivación de contraseñas

| | |
|---|---|
| **Severidad** | CRITICAL |
| **Estado** | remediado |
| **Detectado por** | gosec G401/G501 · job `lint` · CodeQL · Semgrep |
| **Componente** | `backend/internal/api/legacy_auth.go:52-55` |
| **Amenaza** | AM-001 (suplantación por fuerza bruta) |
| **Sembrada** | sí |
| **Evidencia antes** | `docs/evidencia/VULN-002/evidencia.json` |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | `docs/evidencia/VULN-002/evidencia.json` (golangci-lint: "0 issues."; Code scanning #43 sigue "Open" en `main`, ver Discrepancia #6 del informe) |
| **Run de Actions (antes/después)** | https://github.com/jorgepaez-ops/IdentityHub/actions/runs/35476102444 / https://github.com/jorgepaez-ops/IdentityHub/actions/runs/36208104969 |

## Evidencia

```
[backend/internal/api/legacy_auth.go:53] - G401 (CWE-327): Use of weak cryptographic
primitive (Confidence: HIGH, Severity: HIGH)
    52: func legacyHashPassword(password string) string {
  > 53:     sum := md5.Sum([]byte(password))
    54:     return hex.EncodeToString(sum[:])
```

## Por qué importa en esta aplicación

El problema no son las colisiones de MD5, que aquí no vienen al caso: es la
**velocidad**. MD5 se diseñó para ser rápido, y eso es exactamente lo contrario
de lo que se quiere en una contraseña. Una GPU de consumo prueba del orden de
10¹⁰ candidatos MD5 por segundo; con Argon2id a 64 MiB por intento, la misma
tarjeta no pasa de unos miles. La diferencia entre "toda la base de datos
descifrada en una tarde" y "computacionalmente inviable".

Además no hay sal: dos personas con la misma contraseña comparten hash, y las
tablas arcoíris de MD5 para contraseñas comunes están publicadas desde hace más
de una década.

## Remediación

Argon2id con los parámetros de [`adr/0004`](../../specs/adr/0004-argon2id-para-contrasenas.md):
64 MiB, 3 iteraciones, paralelismo 2, sal de 16 bytes de `crypto/rand`.

La migración `000001` ya lo hace estructuralmente imposible de revertir:

```sql
CONSTRAINT users_password_hash_is_argon2id
    CHECK (password_hash LIKE '$argon2id$%')
```

Un `INSERT` con un hash MD5 es rechazado por la base de datos. Es la clase de
control que prefiere fallar a confiar en que nadie se equivoque.
