# 0010 — Segundo factor por código de un solo uso enviado por correo

Estado: aceptada · 2026-09-27 · Enmienda RF-013 y RF-014

## Contexto

RF-013 y RF-014 pedían TOTP: alta del segundo factor con un código QR, diez códigos de
recuperación y validación en el login. Con el alcance de la semana 3 (SSO entre dominios, alta de
empleados, dos interfaces, DAST y E2E) no hay tiempo para TOTP con la calidad que exige, y la
demostración en clase necesita que el segundo factor se vea sin teléfonos ni aplicaciones de
terceros.

El entorno ya tiene Mailpit: un servidor SMTP de captura con buzón web (`:8025`). El alta de
empleados (D6 y D9 de `odd/tasks/idp-semana-3.md`) verifica el correo de cada cuenta al aceptar la
invitación, así que todas las cuentas tienen un correo comprobado.

## Decisión

- **Segundo factor obligatorio para todas las cuentas** (D11), sin paso de activación: el correo
  verificado es el canal.
- Tras una contraseña correcta, el login responde `202` con un `mfa_token` temporal y el worker
  envía un código de **6 dígitos** generado con `crypto/rand`.
- El código se guarda solo como hash, vence en pocos minutos, es de **un solo uso**, admite un
  número limitado de intentos por desafío y está atado al `mfa_token`. Agotar los intentos anula
  el desafío y cuenta para el bloqueo de RF-017.
- Reenviar un código anula el anterior y tiene límite de frecuencia, para que el endpoint no
  sirva para inundar un buzón.
- Cada desafío, acierto, fallo y bloqueo queda en el audit log.
- RF-013 pasa a describir la política (segundo factor obligatorio por correo) y RF-014 el desafío
  en el login. TOTP y los códigos de recuperación quedan fuera de alcance.

## Consecuencias

- **Es más débil que TOTP.** Quien controle el buzón del empleado controla el segundo factor, y
  el correo es un canal que se reenvía, se sincroniza y se lee en muchos dispositivos. NIST
  SP 800-63B no admite el correo electrónico como autenticador fuera de banda, precisamente porque
  no prueba la posesión de un dispositivo concreto. Se acepta a sabiendas por el alcance académico
  y se dice así en el informe; la evolución natural es TOTP o WebAuthn.
- Igual que TOTP, no protege contra el phishing en tiempo real (un proxy que reenvía contraseña y
  código). Solo WebAuthn lo haría, y está fuera de alcance desde la visión del proyecto.
- El login depende del worker y de RabbitMQ: si el correo no sale, nadie entra. Es coherente con
  RF-012 (notificaciones asíncronas) pero convierte al worker en parte del camino crítico; su
  salud tiene que vigilarse igual que la de la API.
- La demostración queda completa con el buzón web de Mailpit: el profesor ve llegar el código.
- En producción Mailpit se sustituye por un servidor SMTP real; el código de la aplicación no
  cambia.
