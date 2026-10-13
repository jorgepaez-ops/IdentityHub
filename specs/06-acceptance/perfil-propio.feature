# language: es
Característica: Perfil propio
  Para mantener mis datos al día
  Como titular de una cuenta activa
  Quiero consultar y actualizar mi nombre para mostrar

  Antecedentes:
    Dado que inicié sesión con una cuenta activa y tengo un access token

  @RF-008 @p0
  Escenario: Consultar el perfil propio devuelve al usuario autenticado
    Cuando consulto "GET /api/v1/me" con mi access token
    Entonces recibo una respuesta 200 con mi correo y mi nombre para mostrar
    Y la misma consulta sin access token recibe una respuesta 401

  @RF-008 @p0
  Escenario: Actualizar el nombre para mostrar
    Cuando envío "PATCH /api/v1/me" con un nombre nuevo
    Entonces recibo una respuesta 200 con el nombre nuevo
    Y una consulta posterior a "GET /api/v1/me" devuelve el nombre nuevo
    Y un nombre vacío recibe una respuesta 400 y no cambia el perfil
