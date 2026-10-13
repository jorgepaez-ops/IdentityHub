# language: es
# Escenarios de requisitos no funcionales. A diferencia de los de RF-NNN no se derivan en un spec de
# Playwright (no llevan etiqueta @RF): cada uno nombra, en «Verificación», la prueba que lo ejecuta
# de verdad. Un escenario sin esa prueba no debe existir.
Característica: Propiedades no funcionales del sistema
  Para confiar en cómo se construye, se entrega y se opera
  Como equipo responsable del servicio
  Quiero que cada propiedad se compruebe contra los archivos y el stack reales

  # Verificación: tests/load/auth-latency.js vía "make load-test", paso de carga del job e2e de CI
  @RNF-010 @p1
  Escenario: El p95 de los endpoints de autenticación se mantiene por debajo de 300 ms
    Dado que el stack está levantado y hay una sesión iniciada (el Argon2id del login queda fuera de la medición)
    Cuando se renueva la sesión, se consulta "GET /api/v1/me" y se descargan las claves JWKS bajo carga
    Entonces el p95 de cada endpoint es menor que 300 ms
    Y todas las respuestas son correctas
    Y si algún umbral se supera la ejecución termina con código distinto de cero
