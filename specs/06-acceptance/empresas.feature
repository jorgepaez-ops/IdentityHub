# language: es
Característica: Empresas y aislamiento de roles
  Para que cada empresa use el Hub sin ver ni recibir roles de otra
  Como operador de la plataforma
  Quiero que la consola, los roles y el acceso a las aplicaciones se separen por empresa

  Antecedentes:
    Dado que existen la empresa "Contabilidad S.A." con la aplicación "contabilidad"
    Y la empresa "Empresa B" con la aplicación "inventario"

  @RF-022 @RF-021 @p1
  Escenario: La grilla de roles muestra solo las aplicaciones y roles de la empresa seleccionada
    Dado un administrador autenticado en la consola
    Cuando selecciona la empresa "Empresa B" en la página de Roles
    Entonces ve únicamente la aplicación "inventario" y sus roles
    Y no ve la aplicación "contabilidad" ni ninguno de sus roles
    Pero al cambiar a la empresa "Contabilidad S.A." ve solo "contabilidad" y sus roles

  @RF-022 @p1
  Escenario: El listado de usuarios muestra solo los usuarios de la empresa seleccionada
    Dado un administrador autenticado y un usuario en cada empresa
    Cuando selecciona la empresa "Empresa B" en el listado de usuarios
    Entonces ve únicamente los usuarios de "Empresa B"
    Y no ve ningún usuario de "Contabilidad S.A."

  @RF-022 @RF-021 @p1
  Escenario: Al asignar un rol solo se ofrecen roles de la empresa del usuario
    Dado un administrador autenticado y un usuario de "Empresa B"
    Cuando abre la asignación de roles de ese usuario
    Entonces se ofrecen los roles de "inventario"
    Y no se ofrece ningún rol de "contabilidad"

  @RF-022 @RF-021 @p1
  Escenario: El backend rechaza asignar un rol de otra empresa
    Dado un administrador autenticado y un usuario de "Empresa B"
    Cuando le asigna el rol "contabilidad.analista" por la API
    Entonces recibe una respuesta 400
    Y el usuario conserva sus roles anteriores
    Y se registra un evento de auditoría "role_assignment_rejected" con mi identificador como actor

  @RF-022 @RF-020 @p1
  Escenario: Un usuario de Contabilidad S.A. entra a Contabilidad
    Dado que existe un empleado de "Contabilidad S.A." con el rol "contabilidad.analista"
    Cuando completa el flujo OAuth de Contabilidad
    Entonces Identity Hub redirige con un código y el mismo estado
    Y el claim "aud" identifica a Contabilidad

  @RF-022 @RF-020 @p1
  Escenario: Un usuario de Empresa B que abre Contabilidad es rechazado
    Dado que existe un empleado de "Empresa B" con un rol de "inventario"
    Cuando completa el flujo OAuth de Contabilidad
    Entonces Identity Hub rechaza la autorización sin emitir código ni token
    Y se registra un evento de auditoría de autorización rechazada

  @RF-022 @RF-020 @p1
  Escenario: Un usuario sin rol en una aplicación de su empresa es rechazado
    Dado que existe un empleado de "Empresa B" con únicamente el rol base "user"
    Cuando completa el flujo OAuth de Inventario
    Entonces Identity Hub rechaza la autorización sin emitir código ni token

  @RF-022 @p1
  Escenario: Tras el login se listan solo las aplicaciones de la empresa del usuario
    Dado que existe un empleado de "Contabilidad S.A." con el rol "contabilidad.analista"
    Cuando inicia sesión en Identity Hub
    Entonces el menú de aplicaciones muestra "Contabilidad"
    Y no muestra "Inventario"
