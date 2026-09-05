# language: es
Característica: Control de acceso por roles
  Para proteger las operaciones sensibles
  Como plataforma
  Quiero que solo los administradores accedan a la administración

  @RF-009 @AM-021 @p0
  Escenario: Un usuario corriente no accede a la administración
    Dado que inicié sesión con una cuenta de rol "user"
    Cuando consulto el listado de cuentas de administración
    Entonces recibo una respuesta 403

  @RF-009 @AM-007 @p0
  Escenario: Añadir el rol admin al token no concede privilegios
    Dado que inicié sesión con una cuenta de rol "user"
    Cuando presento un token cuyo claim "roles" fue alterado para incluir "admin"
    Entonces recibo una respuesta 401

  @RF-010 @p0
  Escenario: Un administrador deshabilita una cuenta
    Dado que inicié sesión con una cuenta de rol "admin"
    Y que existe una cuenta activa "bruno@example.com"
    Cuando deshabilito la cuenta "bruno@example.com"
    Entonces recibo una respuesta 200
    Y "bruno@example.com" no puede iniciar sesión
    Y se registra un evento de auditoría "user_disabled" con mi identificador como actor

  @RF-010 @p0
  Escenario: Un administrador no puede deshabilitarse a sí mismo
    Dado que inicié sesión con una cuenta de rol "admin"
    Cuando intento deshabilitar mi propia cuenta
    Entonces recibo una respuesta 400

  @RF-011 @AM-010 @p0
  Escenario: El registro de auditoría no se puede alterar
    Dado que existen eventos en el registro de auditoría
    Cuando la aplicación intenta modificar o borrar una entrada
    Entonces la base de datos rechaza la operación por falta de permisos
