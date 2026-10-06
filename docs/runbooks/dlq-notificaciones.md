# Runbook: la DLQ de notificaciones tiene mensajes (AM-019)

Alerta de Grafana **DLQ de notificaciones con mensajes** (`deploy/observability/grafana/alerting/dlq.yml`,
carpeta «Identity Hub»). Se dispara cuando `rabbitmq_detailed_queue_messages{queue="notifications.dlq"}`
es mayor que 0 durante 2 minutos. Severidad: `warning`.

## Qué significa

El worker consume la cola `notifications` (quorum, `x-delivery-limit: 3`). Si falla al enviar el correo,
hace `Nack` con reencolado; tras el tercer intento RabbitMQ descarta el mensaje hacia el exchange
`identity.dlx` y este lo guarda en `notifications.dlq`. Los rechazos sin reencolado (cuerpo ilegible)
llegan por el mismo camino. **Un mensaje en la DLQ es un correo que no salió** (invitación, verificación,
restablecimiento, código MFA o aviso de bloqueo).

## Diagnóstico

1. Confirme el conteo y la cola (usuario y contraseña de `RABBITMQ_DEFAULT_USER/PASS` en `.env`):

   ```bash
   curl -s -u "$RABBITMQ_DEFAULT_USER:$RABBITMQ_DEFAULT_PASS" \
     http://localhost:15672/api/queues/%2F/notifications.dlq | python3 -m json.tool | grep -E '"(name|messages)"'
   ```

2. Mire por qué falló el worker (SMTP caído, destinatario inválido, cuerpo malformado):

   ```bash
   make logs S=worker
   ```

   En Grafana (Explore, fuente Loki) filtre por el servicio `worker` y busque errores de envío.
3. Lea sin consumir los mensajes (`ackmode: ack_requeue_true` los deja en la cola):

   ```bash
   curl -s -u "$RABBITMQ_DEFAULT_USER:$RABBITMQ_DEFAULT_PASS" -H 'content-type: application/json' \
     -X POST http://localhost:15672/api/queues/%2F/notifications.dlq/get \
     -d '{"count":10,"ackmode":"ack_requeue_true","encoding":"auto"}'
   ```

   El sobre trae `eventType`, `eventId` y `occurredAt`; no copie el cuerpo a tickets ni chats, puede
   incluir direcciones de correo.

## Remediación

1. Corrija la causa (restablecer el SMTP, revisar la dirección, desplegar el arreglo del worker).
2. Reinyecte los mensajes a la cola de trabajo, ya con la causa resuelta: con la interfaz de administración
   (`http://localhost:15672`, cola `notifications.dlq`, «Move messages» hacia `notifications`) o con la
   API de *shovel*. El worker es idempotente por `eventId`, así que reintentar no duplica correos ya enviados.
3. Si el mensaje no tiene arreglo (destinatario inexistente, evento obsoleto), regístrelo y descártelo:

   ```bash
   curl -s -u "$RABBITMQ_DEFAULT_USER:$RABBITMQ_DEFAULT_PASS" -X DELETE \
     http://localhost:15672/api/queues/%2F/notifications.dlq/contents
   ```

   Esto borra **todos** los mensajes de la DLQ: revíselos antes (paso 3 del diagnóstico).
4. Compruebe que la alerta vuelve a **Normal** (menos de dos minutos tras vaciar la cola; Grafana,
   «Alerting», «Alert rules») y que el panel «Profundidad de la DLQ» del tablero «Identity Hub — Seguridad»
   marca 0.

## Cómo probar la alerta

```bash
curl -s -u "$RABBITMQ_DEFAULT_USER:$RABBITMQ_DEFAULT_PASS" -H 'content-type: application/json' \
  -X POST http://localhost:15672/api/exchanges/%2F/identity.dlx/publish \
  -d '{"properties":{"delivery_mode":2},"routing_key":"","payload":"prueba-alerta","payload_encoding":"string"}'
```

La regla pasa a *Pending* en el siguiente minuto de evaluación y a *Firing* tras ~2 minutos. Luego vacíe la
cola con el comando del paso 3 de la remediación.

## Limitaciones conocidas

- La regla no tiene punto de contacto propio: usa la política por defecto de Grafana, que en el entorno
  local no envía correos. En producción hay que definir un punto de contacto y una política de
  notificación (correo, Slack o PagerDuty) para la etiqueta `severity=warning`.
- Con `noDataState: OK`, si Prometheus deja de raspar `broker:15692/metrics/detailed` la alerta no se
  dispara: revise el objetivo `rabbitmq_detailed` en Prometheus (`:9090/targets`).
