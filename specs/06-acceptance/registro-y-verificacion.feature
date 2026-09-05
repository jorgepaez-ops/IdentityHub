# language: es
Característica: Registro de cuenta y verificación por correo
  Para poder usar la plataforma
  Como visitante
  Quiero crear una cuenta y demostrar que controlo mi correo

  Antecedentes:
    Dado que la plataforma está disponible
    Y que no existe ninguna cuenta con el correo "ana@example.com"

  @RF-001 @p0
  Escenario: Registro con datos válidos
    Cuando me registro con el correo "ana@example.com" y la contraseña "correcta-horse-battery"
    Entonces recibo una respuesta 201
    Y la cuenta queda en estado "pending_verification"
    Y se encola un mensaje "user.registered" en el broker

  @RF-001 @p0
  Esquema del escenario: Contraseñas que no cumplen la política
    Cuando me registro con el correo "ana@example.com" y la contraseña "<password>"
    Entonces recibo una respuesta 400
    Y el error indica el campo "password"

    Ejemplos:
      | password     |
      | corta        |
      | 12345678901  |

  @RF-001 @AM-004 @p0
  Escenario: El registro no revela qué correos existen
    Dado que existe una cuenta activa con el correo "ana@example.com"
    Cuando me registro con el correo "ana@example.com" y la contraseña "correcta-horse-battery"
    Entonces recibo una respuesta 409
    Y el cuerpo de la respuesta no contiene la palabra "existe"
    Y el tiempo de respuesta no difiere en más de 50 ms del de un correo nuevo

  @RF-002 @p0
  Escenario: Verificación del correo con el enlace recibido
    Dado que me registré con el correo "ana@example.com"
    Y que el correo de verificación llegó a Mailpit
    Cuando abro el enlace de verificación
    Entonces recibo una respuesta 204
    Y la cuenta queda en estado "active"
    Y puedo iniciar sesión

  @RF-002 @p0
  Escenario: El enlace de verificación es de un solo uso
    Dado que ya verifiqué mi cuenta con el enlace recibido
    Cuando abro el mismo enlace por segunda vez
    Entonces recibo una respuesta 410

  @RF-003 @p0
  Escenario: Una cuenta sin verificar no puede iniciar sesión
    Dado que me registré con el correo "ana@example.com" pero no verifiqué el correo
    Cuando inicio sesión con "ana@example.com" y la contraseña correcta
    Entonces recibo una respuesta 401
