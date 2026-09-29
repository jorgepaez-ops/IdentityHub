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

## Enmienda D16 · 2026-09-28 · restablecimiento y MFA

Dos flujos de la semana 3 publican **después** de confirmar la transacción, no antes, y no
revierten nada si el broker falla:

- **Aviso de restablecimiento completado (D14).** El restablecimiento ya está confirmado
  (contraseña cambiada, sesiones revocadas, desbloqueo de D13 si aplica). Si falla la publicación
  del aviso, `POST /auth/password-reset/confirm` sigue respondiendo `204` y el fallo queda en el log
  estructurado con el id del usuario (sin secretos). Se acepta que un aviso pueda perderse.
- **Código MFA (RF-014), al emitir el desafío.** Si falla la publicación tras el `COMMIT`,
  `POST /auth/login` responde `503` con `application/problem+json` y anula el desafío recién
  creado. Así no queda un desafío imposible de recibir ni consume uno de los cinco cupos de
  emisión de la ventana de 15 minutos; el audit conserva el intento para operación.
- **Código MFA (RF-014), al reenviarlo.** Si falla la publicación tras el `COMMIT`, el endpoint
  responde `503`, restaura el hash del código y `last_sent_at` anteriores, y permite reintentar
  de inmediato en vez de imponer los 60 segundos. La restauración es condicional para no pisar
  un reenvío posterior que sí haya tenido éxito.

Motivo: publicar dentro de la transacción acopla la recuperación de cuentas (incluido el
desbloqueo) a la disponibilidad del broker, retiene una conexión de la base mientras el broker es
lento y puede enviar un aviso de un cambio que luego se revierte. El resto de los flujos (registro,
invitación) conserva la decisión original de publicar con confirmación y revertir. El outbox
transaccional sigue descartado por tamaño y sigue siendo la mejora posible.
