# VULN-023 — Las reglas por defecto de Gitleaks pasan por alto la mayoría de los secretos sembrados

| | |
|---|---|
| **Severidad** | HIGH (fallo de cobertura de un control, no del código) |
| **Estado** | remediado |
| **Detectado por** | comparación manual entre lo sembrado y lo detectado |
| **Componente** | job `secrets` del pipeline |
| **Amenaza** | AM-012 |
| **Sembrada** | **NO** — hallazgo sobre la herramienta, no sobre la aplicación |

## Qué pasó

Se sembraron doce secretos en la línea base. Gitleaks con su configuración por
defecto encontró **dos**:

```
  · aws-access-token   backend/internal/api/legacy_auth.go:44
  · slack-bot-token    backend/internal/api/legacy_auth.go:46
```

Los otros diez —incluidas todas las contraseñas de PostgreSQL y RabbitMQ del
`docker-compose.yml` y los `ENV` del Dockerfile— pasaron sin ser vistos.

Sin la línea base deliberada nunca lo habríamos sabido: el job habría estado en
verde y lo habríamos leído como "no hay secretos", cuando lo que decía en
realidad era "no hay secretos *con formato reconocible*".

## Por qué ocurre

Las reglas por defecto de Gitleaks identifican secretos por su **forma**: una
clave de AWS empieza por `AKIA` y tiene veinte caracteres, un token de Slack
empieza por `xoxb-`. Es detección por firma, y funciona muy bien para
credenciales de proveedores conocidos.

Una contraseña propia no tiene forma reconocible:

```yaml
POSTGRES_PASSWORD: postgres_admin_2024
```

es indistinguible de cualquier otra cadena. Para encontrarla hay que buscar por
**contexto** —el nombre de la variable— y eso exige reglas propias.

## El detalle que casi se nos escapa

La primera versión de la regla propia tampoco encontraba `POSTGRES_PASSWORD`:

```
\b(?:password|passwd|secret|...)\b
```

`\b` no coincide dentro de `POSTGRES_PASSWORD` porque el guion bajo **es**
carácter de palabra: no hay frontera entre `POSTGRES_` y `PASSWORD`. Una regla
escrita con la intención correcta y un fallo de una sola letra habría dejado el
gate igual de ciego, y en verde.

Se corrigió con un prefijo explícito de segmentos: `(?:[a-z0-9]+[_-])*`.

## Remediación

Se añade [`.gitleaks.toml`](../../.gitleaks.toml) con dos reglas propias:

| Regla | Qué detecta |
|---|---|
| `contrasena-en-variable-de-entorno` | `*_PASSWORD`, `*_SECRET`, `*_KEY`, `*_PASS` asignados a un valor |
| `url-de-conexion-con-credenciales` | DSN con usuario y contraseña incrustados (`postgres://u:p@…`) |

Resultado: **de 2 a 12 secretos detectados, sin falsos positivos.**

Las excepciones se enumeran valor por valor, nunca por patrón de ruta amplio.
Excluir `*_test.go` en bloque habría sido lo cómodo, y habría abierto justo el
agujero que este archivo existe para cerrar: los secretos reales también acaban
en los tests.

## Lo que queda fuera, y por qué está bien

La regla sigue sin ver `legacySigningKey = "…"`, un identificador en camelCase
sin separadores. Ese lo detecta **gosec G101**, que analiza el árbol sintáctico
de Go en lugar de texto plano.

No es un descuido: es el argumento a favor de tener varias herramientas con
enfoques distintos sobre el mismo código. Gitleaks lee texto y llega a
cualquier archivo del historial; gosec entiende Go pero solo ve Go. Ninguna de
las dos sobra, y ninguna basta sola.
