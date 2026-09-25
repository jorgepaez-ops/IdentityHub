# VULN-005 — Consulta SQL construida por concatenación de cadenas

| | |
|---|---|
| **Severidad** | CRITICAL |
| **Estado** | remediado |
| **Detectado por** | gosec G201 · CodeQL `go/sql-injection` · Semgrep |
| **Componente** | `backend/internal/api/legacy_auth.go:66-68` |
| **Amenaza** | AM-006 (manipulación) |
| **Sembrada** | sí |
| **Evidencia antes** | |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | |

## Evidencia

```
[backend/internal/api/legacy_auth.go:67] - G201 (CWE-89): SQL string formatting
(Confidence: HIGH, Severity: MEDIUM)
  > 67:     return fmt.Sprintf("SELECT id, email, password_hash FROM users WHERE email = '%s'", email)
```

## Por qué importa en esta aplicación

El parámetro `email` llega directamente de la cadena de consulta, sin validación.
Una entrada como:

```
' OR '1'='1' --
```

convierte la condición en universalmente verdadera y devuelve la primera fila de
`users`, que en un sistema con datos semilla suele ser la cuenta de
administración. En este endpoint concreto eso significa **saltarse la
autenticación por completo**.

Agrava el fallo que el handler además registre la consulta ya construida:

```go
s.logger.Info("consulta legacy construida", "query", query)
```

Eso deja el intento de inyección —y cualquier dato que devuelva— escrito en los
logs, que se envían a Loki. Es una segunda vulnerabilidad encima de la primera
(RNF-012).

## Remediación

Consultas parametrizadas generadas por `sqlc` desde `db/queries/`. El motor
recibe la consulta y los valores por canales separados, así que ningún contenido
del parámetro puede cambiar la estructura de la sentencia:

```sql
-- name: GetUserByEmail :one
SELECT id, email, password_hash, status FROM users WHERE email = $1;
```

Con sqlc esto no es disciplina, es tipado: la función generada acepta un
`string` como valor, y no hay forma de que ese `string` acabe siendo SQL.
