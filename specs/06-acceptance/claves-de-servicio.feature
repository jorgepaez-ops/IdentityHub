# language: es
Característica: Claves de servicio para integraciones máquina a máquina
  Para que un SIEM o sistema de reportes consulte el audit log sin una sesión humana
  Como administrador de la plataforma
  Quiero emitir claves de servicio de alcance mínimo, vencimiento obligatorio y revocables

  @RF-018 @p2
  Escenario: Admin emite una clave, la usa para leer el audit log y la revoca
    Dado un administrador autenticado
    Cuando emite una clave de servicio con el alcance "audit-log:read" y vigencia de 30 días
    Entonces recibe la clave completa una sola vez
    Y con esa clave puede listar el audit log
    Pero al revocarla, el siguiente uso devuelve 401

  @RF-018 @p2
  Escenario: El listado de claves no expone el secreto
    Dado un administrador autenticado con una clave de servicio emitida
    Cuando lista las claves de servicio
    Entonces ve nombre, alcances, prefijo y fechas
    Pero nunca el secreto ni su hash

  @RF-018 @p2
  Escenario: Una clave de servicio solo abre las operaciones que declaran su alcance
    Dado una clave de servicio vigente con el alcance "audit-log:read"
    Cuando la usa en otra operación de administración
    Entonces recibe 401

  @RF-018 @p2
  Escenario: Una clave inválida, expirada o revocada recibe la misma respuesta
    Dado una clave de servicio con secreto incorrecto, otra desconocida, otra expirada y otra revocada
    Cuando cada una intenta listar el audit log
    Entonces todas reciben el mismo 401 y cada uso rechazado queda en el audit log

  @RF-018 @p2
  Escenario: Solo un administrador gestiona claves de servicio
    Dado un usuario sin el rol "admin"
    Cuando intenta emitir, listar o revocar una clave de servicio
    Entonces recibe 403
