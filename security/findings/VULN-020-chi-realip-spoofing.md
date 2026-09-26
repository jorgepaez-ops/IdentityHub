# VULN-020 — Suplantación de IP en `middleware.RealIP` de chi

| | |
|---|---|
| **Severidad** | HIGH |
| **Estado** | remediado |
| **Detectado por** | govulncheck · job `sca` |
| **Componente** | `github.com/go-chi/chi/v5@v5.0.11` → corregido en `v5.3.0` |
| **Avisos** | GO-2026-5774, GO-2026-5775, GO-2026-5777 |
| **Amenaza** | AM-001, AM-010 (frontera de confianza T2) |
| **Sembrada** | **NO** — hallazgo no previsto |
| **Evidencia antes** | |
| **Commit de remediación** | `902a047` (T6) — hallazgo cerrado desde entonces; esta ficha quedó sin marcar hasta la revisión de T32 |
| **Evidencia después** | |
| **Run de Actions (antes/después)** | |

## Por qué esta ficha importa más que las demás

Las vulnerabilidades VULN-001 a VULN-019 se sembraron sabiendo lo que se
sembraba: confirman que los gates funcionan, pero no enseñan nada nuevo.
**Esta no.** Se eligió chi como router sin saber que su middleware `RealIP`
tenía tres avisos publicados, y el pipeline la encontró sola.

Eso es exactamente lo que un pipeline de DevSecOps tiene que aportar:
información que el equipo no tenía.

## Evidencia

```
Vulnerability #2: GO-2026-5777
    Chi's RealIP Middleware allows IP spoofing via unvalidated
    X-Forwarded-For header in github.com/go-chi/chi
  Module: github.com/go-chi/chi/v5
    Found in: github.com/go-chi/chi/v5@v5.0.11
    Fixed in: github.com/go-chi/chi/v5@v5.3.0
    Example traces found:
      #1: internal/api/server.go:45:7: api.Server.Routes calls chi.Mux.Get,
          which eventually calls middleware.RealIP
```

## Por qué importa en esta aplicación

`middleware.RealIP` toma la cabecera `X-Forwarded-For` sin validar de quién
viene. Como cualquier cliente puede enviar esa cabecera, el atacante controla
la IP que la aplicación cree estar viendo. En **esta** aplicación eso rompe tres
cosas a la vez:

1. **RF-017, bloqueo por fuerza bruta.** El contador de intentos fallidos se
   agrupa por IP. Rotando `X-Forwarded-For` en cada intento, el atacante nunca
   llega al umbral de 5 y el bloqueo deja de existir.
2. **RF-011, registro de auditoría.** La columna `ip` de `audit_log` queda bajo
   control del atacante: puede firmar sus intentos con la IP de otra persona.
   Un registro de auditoría falsificable no sirve como evidencia (AM-010).
3. **`limit_req` de Nginx**, cuando se añada en la remediación, se apoya en el
   mismo dato.

Es precisamente la frontera de confianza **T2** del modelo de amenazas
(`Nginx → api`, cabeceras reenviadas falsificables). El aviso confirma que esa
frontera estaba mal dibujada en el código.

## Remediación

Dos cambios, porque actualizar no basta:

1. Subir a `github.com/go-chi/chi/v5 >= v5.3.0`.
2. **Confiar en `X-Forwarded-For` solo si viene del proxy conocido.** La API no
   se publica al host en producción: el único origen legítimo es el contenedor
   `web`. La cabecera se acepta únicamente desde la red interna de Docker y se
   descarta en cualquier otro caso.

Se añade `TestRF017_BloqueoNoSeEvitaFalsificandoXForwardedFor` para que la
regresión no vuelva silenciosamente.
