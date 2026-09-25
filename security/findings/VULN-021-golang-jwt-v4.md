# VULN-021 — Dos avisos en `golang-jwt/jwt/v4`

| | |
|---|---|
| **Severidad** | MEDIUM |
| **Estado** | remediado |
| **Detectado por** | govulncheck · job `sca` |
| **Componente** | `github.com/golang-jwt/jwt/v4@v4.5.0` |
| **Avisos** | GO-2024-3250 (corregido en v4.5.1) · GO-2025-3553 (corregido en v4.5.2) |
| **Amenaza** | AM-003 (suplantación), AM-017 (denegación de servicio) |
| **Sembrada** | sí |
| **Evidencia antes** | |
| **Commit de remediación** | `51a7a4f` (T23) |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | |

## Evidencia

```
Vulnerability #4: GO-2025-3553
    Excessive memory allocation during header parsing in github.com/golang-jwt/jwt
    Found in: github.com/golang-jwt/jwt/v4@v4.5.0
    Fixed in: github.com/golang-jwt/jwt/v4@v4.5.2

Vulnerability #5: GO-2024-3250
    Improper error handling in ParseWithClaims and bad documentation may cause
    dangerous situations in github.com/golang-jwt/jwt
    Found in: github.com/golang-jwt/jwt/v4@v4.5.0
    Fixed in: github.com/golang-jwt/jwt/v4@v4.5.1
```

## La lección sobre alcanzabilidad

En la primera pasada, `govulncheck` **no informó de nada de esto**, aunque la
dependencia vulnerable ya estaba en `go.mod`. La razón es que `legacyParseToken`
estaba definida pero nunca se llamaba, y govulncheck solo reporta
vulnerabilidades cuyo código es realmente invocable desde el nuestro.

Hubo que conectar la función a un handler para que el aviso apareciera.

Esa es la diferencia entre govulncheck y un escáner que se limita a leer
`go.mod`: **muchísimo menos ruido**, porque no alerta de código que nunca se
ejecuta. Y también su límite —si una dependencia vulnerable se invoca por
reflexión, no la ve—, por lo que `osv-scanner` corre en paralelo cubriendo el
inventario completo. Las dos herramientas no se solapan: se complementan.

## Por qué importa en esta aplicación

GO-2024-3250 es el más grave de los dos aquí. `ParseWithClaims` puede devolver
a la vez un error **y** unos claims parcialmente rellenados; quien comprueba el
error sin comprobar además `token.Valid` acaba tratando como buenos los claims
de un token que no lo era. Es exactamente la forma de la amenaza AM-003.

## Remediación

Migrar a `github.com/golang-jwt/jwt/v5`, que además obliga por API a declarar
los métodos de firma aceptados:

```go
jwt.ParseWithClaims(raw, &claims, keyFunc,
    jwt.WithValidMethods([]string{"EdDSA"}),  // se rechaza todo lo demás
    jwt.WithIssuer(cfg.JWTIssuer),
    jwt.WithExpirationRequired(),
)
```

`WithValidMethods` cierra por construcción el ataque de `alg: none` y el de
cambio a HS256 usando la clave pública como secreto. Se acompaña de
`TestRF004_RechazaTokenConAlgoritmoAlterado`, que presenta ambos tokens
hostiles y exige `401`.
