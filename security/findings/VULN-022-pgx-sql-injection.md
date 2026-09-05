# VULN-022 — Inyección SQL en `jackc/pgx` v5.5.1

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | abierto |
| **Detectado por** | govulncheck · job `sca` |
| **Componente** | `github.com/jackc/pgx/v5@v5.5.1` → corregido en `v5.5.4` |
| **Aviso** | GO-2024-2606 |
| **Amenaza** | AM-006 (manipulación) |
| **Sembrada** | parcialmente — la versión antigua sí; este aviso concreto, no |

## Evidencia

```
Vulnerability #6: GO-2024-2606
    SQL injection in github.com/jackc/pgproto3 and github.com/jackc/pgx
    Found in: github.com/jackc/pgx/v5@v5.5.1
    Fixed in: github.com/jackc/pgx/v5@v5.5.4
    Example traces found:
      #1: internal/store/store.go:36:36: store.New calls pgxpool.NewWithConfig,
          which eventually calls pgconn.ConnectConfig
      … 11 trazas en total
```

## Por qué este hallazgo es incómodo, y por eso instructivo

VULN-005 documenta una inyección SQL **nuestra**: la escribimos, la vemos, la
arreglamos. Esta es distinta: está **en el driver**, por debajo de nuestro
código. Y es la que desmonta la respuesta cómoda de "uso consultas
parametrizadas, luego estoy a salvo".

El fallo permite que un mensaje del protocolo manipulado altere la sentencia
enviada al servidor. Es decir: **la parametrización correcta no protege si la
capa que la implementa está rota**. Ninguna revisión de código nuestro lo habría
encontrado; solo un escáner que conoce el árbol completo de dependencias.

Es el argumento entero a favor de tener SCA en el pipeline, en una sola
vulnerabilidad.

## Remediación

Subir a `github.com/jackc/pgx/v5 >= v5.5.4` — en la práctica, a la última
estable, junto con el resto de dependencias fijadas en versiones antiguas
durante la línea base.

Como control continuo, `scheduled-scan.yml` reejecuta `govulncheck` cada lunes:
esta clase de aviso aparece en código que no ha cambiado, así que un escaneo que
solo corra en los push no lo detectaría nunca.
