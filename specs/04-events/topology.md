# Topología de RabbitMQ

```
                    exchange: identity.events   (topic, durable)
                                 │
        ┌────────────────────────┼───────────────────────────┐
        │ user.*                 │ security.*                │
        ▼                        ▼                           │
  ┌───────────────────────────────────────────┐              │
  │ queue: notifications  (durable, quorum)   │              │
  │  x-dead-letter-exchange: identity.dlx     │              │
  │  x-delivery-limit: 3                      │              │
  └────────────────┬──────────────────────────┘              │
                   │ tras 3 entregas fallidas                │
                   ▼                                          │
        exchange: identity.dlx  (fanout, durable)             │
                   │                                          │
                   ▼                                          │
        ┌──────────────────────────┐                          │
        │ queue: notifications.dlq │◄─────────────────────────┘
        └──────────────────────────┘
             (inspección manual · métrica en Grafana)
```

## Decisiones

- **Exchange de tipo topic**, no direct: permite añadir consumidores nuevos
  (por ejemplo un worker de analítica que escuche `security.*`) sin tocar al
  productor.
- **Colas quorum**, no classic: replicación y semántica de entrega predecible.
  Es la recomendación por defecto en RabbitMQ 4.
- **`x-delivery-limit: 3`** deja el reintento en manos del broker en vez de
  reencolar a mano, que es la forma clásica de construir un bucle infinito.
- **Retroceso exponencial en el worker** (1 s, 4 s, 16 s) antes de rechazar, para
  no martillear a un servidor SMTP caído.
- **Idempotencia por `eventId`**: el worker guarda los identificadores ya
  procesados; un reintento del broker no reenvía el correo.
- **Publicación con confirmación** (`publisher confirms`) y mensajes persistentes:
  la API no responde `201` hasta que el broker acusa recibo.

## Contrapartida conocida

Si RabbitMQ está caído, el registro falla aunque la fila del usuario ya se haya
escrito. El patrón correcto es un *outbox transaccional* (escribir el evento en
la misma transacción que el usuario y publicarlo con un relay aparte). Se
documenta en `adr/0006` y queda como trabajo futuro: con un mes de presupuesto se
prefiere una publicación directa bien instrumentada a un outbox a medio hacer.
