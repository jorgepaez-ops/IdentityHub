# 0006 — Publicación directa al broker; outbox transaccional como trabajo futuro

Estado: aceptada · 2026-09-05

## Contexto

La API escribe el usuario en PostgreSQL y publica `user.registered` en RabbitMQ.
Son dos sistemas distintos y no hay transacción que los abarque: si el broker
falla después del `COMMIT`, queda un usuario sin correo de verificación.

## Decisión

Publicar directamente con **publisher confirms** y mensajes persistentes, y
responder `201` solo cuando el broker acusa recibo. Si la publicación falla, se
revierte la transacción y se devuelve `503`.

No se implementa outbox transaccional en esta entrega.

## Consecuencias

- **Se acepta un fallo conocido:** si el proceso muere entre el `COMMIT` y la
  confirmación del broker, queda una cuenta sin correo enviado. El usuario puede
  pedir el reenvío; el impacto es bajo y el caso es raro.
- La disponibilidad del registro queda acoplada a la del broker. Es visible en la
  sonda `/readyz` y en el panel de Grafana.
- **La alternativa correcta** es escribir el evento en una tabla `outbox` dentro
  de la misma transacción y publicarlo con un relay aparte. Se descarta por
  presupuesto, no por desconocimiento: con un mes, un outbox a medio hacer es
  peor que una publicación directa bien instrumentada. Queda como el primer
  candidato de trabajo futuro.
