# Guion de demostración: ciclo DevSecOps de Identity Hub

Este guion orienta una demostración de aproximadamente 15 minutos. Muestra una historia de autorización distinta de la autenticación, el pipeline con su evidencia de seguridad, los despliegues local y de referencia, y la operación observable. La grabación la realiza el equipo: este documento solo prepara la demostración.

## Propósito, duración y equipo

| Elemento | Definición |
|---|---|
| Propósito | Demostrar el ciclo completo de Identity Hub: aplicación, seguridad integrada, pipeline, despliegue y observabilidad. |
| Duración objetivo | 15:00 minutos; el rango aceptable del entregable es de 10 a 15 minutos. |
| Formato | Grabación de pantalla con narración, sin mostrar secretos, tokens ni códigos de correo. |
| Equipo completo | Miguel Ángel Díaz Díaz; María Fernanda Giraldo Osorio; William Ricardo Niño Pico; Jorge Iván Páez Rincón; Milena Rocío Ramírez Espinosa. |

### Reparto sugerido, modificable

El equipo puede intercambiar estos papeles sin alterar el contenido ni los tiempos.

| Persona | Papel sugerido | Escenas |
|---|---|---|
| Miguel Ángel Díaz Díaz | Apertura y arquitectura | 1 y 2 |
| María Fernanda Giraldo Osorio | Historia funcional RF-021, exportación de auditoría (RF-019) y claves de servicio (RF-018) | 3, 4, 5 y 6 |
| William Ricardo Niño Pico | Pipeline, línea base y despliegue local | 7 y 8 |
| Jorge Iván Páez Rincón | Producción simulada e infraestructura | 9 y 10 |
| Milena Rocío Ramírez Espinosa | Observabilidad, Falco, amenazas y cierre | 11, 12 y 13 |

## Checklist de preparación

Complete esta lista antes de grabar. El orden de arranque no es intercambiable: primero **make setup** y después **make up** o **make up-obs**.

- [ ] Decidir si se necesita un reinicio limpio. Solo si se acepta perder los datos locales, ejecutar **make clean** antes del arranque; este objetivo borra los volúmenes.
- [ ] Desde la raíz, ejecutar **make setup**. No mostrar el archivo .env generado ni sus valores; si ya existe, el objetivo no lo sobrescribe.
- [ ] Después de setup, ejecutar **make up-obs** para la demostración completa. Si no se mostrará observabilidad, usar **make up**. Confirmar el estado con **make ps**.
- [ ] Antes del primer arranque, identificar en .env la dirección de bootstrap mediante el nombre de variable **BOOTSTRAP_ADMIN_EMAIL**. No mostrar su valor ni una contraseña: la invitación crea una cuenta pendiente y la persona elige su contraseña.
- [ ] Preparar, sin revelar contraseñas ni códigos, una cuenta administradora y una cuenta de empleado. La persona empleada debe tener una invitación disponible, poder definir su contraseña y después participar en RF-021. Verificar que el rol de demostración no exista antes de crearlo.
- [ ] Abrir Mailpit en http://localhost:8025 y confirmar que los mensajes de prueba pueden leerse allí. Mantener ocultos el cuerpo de los correos, enlaces de invitación y códigos MFA.
- [ ] Abrir Grafana en http://localhost:3000 y Prometheus en http://localhost:9090 cuando se use el perfil de observabilidad. Iniciar la sesión antes de grabar y no mostrar las credenciales, que viven en .env.
- [ ] Antes de la escena de observabilidad, generar tráfico visible con una ejecución autorizada de **make e2e** contra el stack levantado; este comando eleva y restaura el límite de fallos por IP. Confirmar que los paneles contienen datos antes de iniciar la grabación. Para la escena 12, ejecutar **make falco-test** uno o dos minutos antes de mostrar el panel Alertas de Falco, y confirmar antes que Falco corre (**make ps**); requiere un kernel Linux con eBPF o Docker Desktop. Para la escena 6, la clave de servicio se copia a una variable de la terminal sin mostrarla y se revoca en la misma escena.
- [ ] Dejar abiertas las pestañas del Hub, Contabilidad, Mailpit, Grafana, Prometheus, el workflow de CI y los documentos referenciados por cada escena.
- [ ] Configurar navegador y aplicación de videollamada para compartir solo la ventana necesaria. Como recomendación de grabación, usar 1920 × 1080, un tamaño de letra legible y zoom consistente; ocultar marcadores personales, notificaciones, historial y pestañas ajenas.
- [ ] No abrir .env, terminales con variables exportadas, cabeceras HTTP, herramientas de desarrollo, JWT, refresh tokens, cuerpos de correo ni salidas que contengan secretos. La narración explica los controles sin exponer el tráfico sensible.

## Escenas

### 1. Apertura y objetivo — 00:00 a 00:35

| Campo | Indicación |
|---|---|
| Presentador | Miguel Ángel Díaz Díaz |
| Pantalla exacta | Portada de [informe técnico](../informe/informe-tecnico.md) y, después, [índice de la entrega](../README.md). |
| Narración | “Somos el equipo de Identity Hub. Construimos un proveedor de identidad y una aplicación cliente de Contabilidad para demostrar un ciclo DevSecOps completo. En quince minutos mostraremos una historia funcional diferente de iniciar sesión, la exportación y las claves de servicio, cómo el pipeline protege los cambios, cómo se prepara el despliegue y cómo se observan los resultados. El código, los diagramas y la evidencia están versionados en el repositorio.” |

### 2. Arquitectura y límites — 00:35 a 01:20

| Campo | Indicación |
|---|---|
| Presentador | Miguel Ángel Díaz Díaz |
| Pantalla exacta | [Diagrama de componentes](../diagramas/uml/componentes.md), seguido del [diagrama de despliegue](../diagramas/uml/despliegue.md). |
| Narración | “El navegador llega al Hub y a Contabilidad; el Hub concentra la API, PostgreSQL, RabbitMQ y un worker de notificaciones. Contabilidad delega el inicio de sesión mediante OAuth 2.0 Authorization Code con PKCE. La demostración no pretende ocultar los límites: el objetivo de despliegue es un host con Docker Compose y la arquitectura en AWS es una referencia, no una aplicación de infraestructura en la nube.” |

### 3. Flujo de la persona empleada — 01:20 a 02:45

| Campo | Indicación |
|---|---|
| Presentador | María Fernanda Giraldo Osorio |
| Pantalla exacta | Hub: formulario de invitación, Mailpit, pantalla Define tu contraseña, inicio de sesión, Código de verificación y Mi cuenta. Usar las capturas de respaldo de [la sección Empleado](../manuales/usuario.md#2-empleado) si el flujo en vivo no está disponible. |
| Narración | “Antes de la historia de permisos mostramos el flujo base de la persona empleada. Una administradora envía la invitación; el correo llega a Mailpit, que es un buzón local. Sin mostrar el enlace ni el cuerpo del correo, la persona abre la invitación, define una contraseña y activa su cuenta. Luego inicia sesión con correo y contraseña; el Hub solicita un código MFA de seis dígitos, que se lee en Mailpit y se escribe sin mostrarlo en la grabación. Al verificarlo llega a Mi cuenta, donde se ven perfil, roles y sesiones activas. Este recorrido establece el contexto de identidad; la siguiente escena demuestra autorización sin modificar código.” |

### 4. Historia RF-021: auditor sin redespliegue — 02:45 a 04:50

| Campo | Indicación |
|---|---|
| Presentador | María Fernanda Giraldo Osorio |
| Pantalla exacta | Hub: Inicio, Usuarios y Roles; después Contabilidad: Resumen y Transacciones; al final Hub: Auditoría filtrada por role_created. Seguir la [historia de sustentación](../manuales/usuario.md#5-historia-de-usuario-para-la-sustentación). |
| Narración | “La historia es deliberadamente distinta de autenticación. Como administradora, creo el rol contabilidad.auditor y marco únicamente reportes.ver y movimientos.ver_todos. Creo el rol y lo asigno a un empleado. La grilla impide crear roles del sistema, mezclar permisos de otra aplicación, editar los permisos de un rol propio o eliminar un rol asignado. Cada cambio deja auditoría. Ahora el empleado entra a Contabilidad. Puede ver el resumen organizacional y todas las transacciones, pero no aparecen acciones para registrar, aprobar o rechazar, y el cierre queda bloqueado. El Hub emite solo los permisos seleccionados y Contabilidad decide su interfaz a partir de ellos. No se cambió código ni se redesplegó la aplicación. Las seis pruebas E2E de RF-021 en e2e/tests/roles-y-permisos.spec.ts comprueban este recorrido y sus restricciones, incluida la auditoría. Si el empleado tenía una sesión previa, debe obtener un token nuevo; el cambio rige desde el siguiente token, hasta quince minutos.” |

### 5. Exportar la auditoría en CSV y JSON — 04:50 a 05:20

| Campo | Indicación |
|---|---|
| Presentador | María Fernanda Giraldo Osorio |
| Pantalla exacta | Hub: Auditoría con un filtro aplicado; botones Exportar CSV y Exportar JSON; después la misma pantalla filtrada por la acción audit_log_exported. Seguir la [sección 3.6 del manual](../manuales/usuario.md). |
| Narración | “La auditoría también se puede sacar del Hub. Con un filtro aplicado, Exportar CSV y Exportar JSON descargan todos los eventos que lo cumplen, hasta diez mil, y no solo los que están en pantalla. El CSV neutraliza las celdas que Excel ejecutaría como fórmula. Y cada exportación queda registrada en la propia auditoría como audit_log_exported: quien exporta deja rastro. No se muestra el contenido del archivo.” |

### 6. Claves de servicio para integraciones — 05:20 a 06:15

| Campo | Indicación |
|---|---|
| Presentador | María Fernanda Giraldo Osorio |
| Pantalla exacta | Hub: Claves de servicio (Emitir una clave, aviso Clave emitida, tabla Claves de servicio, Revocar) y una terminal con la variable IH_SERVICE_KEY ya cargada. Guía: [claves de servicio](../manuales/claves-de-servicio.md). |
| Narración | “Un SIEM necesita leer la auditoría sin una sesión humana. El administrador emite una clave con nombre, alcance audit-log:read y vigencia obligatoria. La clave se muestra una sola vez y no se enseña en pantalla: se copia directamente a una variable de la terminal. Con ella, una petición a la auditoría responde 200. Revocamos la clave y la misma petición responde 401; ese 401 es idéntico para una clave desconocida, caducada o revocada, para no dar pistas. De la clave solo se guarda un hash SHA-256, y su alcance es de solo lectura sobre la auditoría. Emisión, revocación y usos rechazados quedan auditados.” |

### 7. Pipeline, línea base y estado endurecido — 06:15 a 08:35

| Campo | Indicación |
|---|---|
| Presentador | William Ricardo Niño Pico |
| Pantalla exacta | [Workflow principal de CI](../../.github/workflows/ci.yml), [manual de seguridad](../manuales/seguridad.md), [guía de hooks](../guia-desarrollo.md#6-hooks-de-pre-commit-rnf-003), [ADR 0007](../../specs/adr/0007-linea-base-vulnerable-deliberada.md), [ficha VULN-019](../../security/findings/VULN-019-imagenes-base-antiguas-compose.md) y [evidencia posterior VULN-019](../../security/evidence/actions-37473616659/README.md). |
| Narración | “La seguridad se integra desde el cambio, no al final. El pipeline ejecuta secretos con Gitleaks; SAST con CodeQL y Semgrep; dependencias con govulncheck, npm audit y osv-scanner; pruebas unitarias e integración; E2E con Playwright; configuración de contenedores con Trivy; construcción de imágenes y Trivy image; DAST con OWASP ZAP; e IaC con Checkov. Semgrep, Trivy y Checkov publican sus resultados SARIF en Code Scanning. Antes del commit, el hook local incluye Gitleaks: para demostrarlo sin dejar una cadena sensible en este guion, mostramos un clip pregrabado de un archivo desechable con una clave falsa de forma AWS, el rechazo Detect hardcoded secrets y su eliminación posterior. La línea base vulnerable fue deliberada: sirve para mostrar que los controles detectan problemas reales. No se despliega ni se demuestra ejecutando el tag v0.0.0-vuln-baseline. El ejemplo visual antes y después es VULN-019: Trivy pasó de 710 a 108 hallazgos HIGH o CRITICAL en las ocho imágenes del Compose. Mostramos la ficha y la evidencia; el estado endurecido está identificado por v0.1.0-hardened. Los workflows de línea base y reescaneo semanal complementan el pipeline principal.” |

### 8. Despliegue local reproducible — 08:35 a 09:40

| Campo | Indicación |
|---|---|
| Presentador | William Ricardo Niño Pico |
| Pantalla exacta | Terminal en la raíz con la salida ya preparada de **make setup**, **make up-obs** y **make ps**; después Hub, Mailpit y la [guía de despliegue](../manuales/despliegue-y-operacion.md). |
| Narración | “El arranque local es reproducible. Primero ejecutamos make setup, que genera .env local sin sobrescribirlo; después make up-obs construye y levanta el stack con observabilidad. make ps confirma los contenedores. Mailpit recibe el correo local sin enviar mensajes a Internet. Evitamos mostrar .env, contraseñas, códigos y enlaces porque son datos sensibles incluso en un entorno de demostración. La guía documenta también cuándo usar make clean: solo para borrar voluntariamente los volúmenes y reiniciar desde cero.” |

### 9. Producción simulada con imágenes publicadas — 09:40 a 10:30

| Campo | Indicación |
|---|---|
| Presentador | Jorge Iván Páez Rincón |
| Pantalla exacta | [Sección de producción del manual](../manuales/despliegue-y-operacion.md#producción) y [ADR 0011](../../specs/adr/0011-docker-hub-como-registry.md). |
| Narración | “La producción simulada usa make up-prod con imágenes por digest, sin build, y solo publica el puerto del servicio web. Las imágenes ya están publicadas en Docker Hub, con etiquetas de versión y latest, firmadas con Cosign keyless y con un SBOM adjunto como atestación; mostramos el comando y su resultado únicamente cuando están configurados sus digests y el relay SMTP.” |

### 10. Infraestructura de referencia, sin apply — 10:30 a 11:20

| Campo | Indicación |
|---|---|
| Presentador | Jorge Iván Páez Rincón |
| Pantalla exacta | [Diagrama AWS](../diagramas/uml/despliegue-aws.md), [ADR 0012](../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md) y terminal con evidencia preparada de terraform validate y Checkov. |
| Narración | “La infraestructura describe una referencia de AWS con Terraform. No ejecutamos terraform apply en la demostración: la decisión documentada es validar la configuración con terraform validate y Checkov. El diagrama permite revisar los componentes sin afirmar que exista un despliegue cloud activo. Esta separación evita convertir una demostración académica en una operación con costo o credenciales no autorizadas.” |

### 11. Observabilidad: métricas y logs — 11:20 a 13:15

| Campo | Indicación |
|---|---|
| Presentador | Milena Rocío Ramírez Espinosa |
| Pantalla exacta | Grafana: dashboard Identity Hub — Seguridad; Grafana Explore con Loki; Grafana Explore con Prometheus; página Targets de Prometheus. Como respaldo, [sección de observabilidad del informe](../informe/informe-tecnico.md#monitoreo-y-observabilidad). |
| Narración | “Con make up-obs, Prometheus recoge métricas y Loki recibe los logs estructurados mediante Grafana Alloy. En el dashboard de seguridad, los intentos de inicio de sesión por resultado se alinean con AM-001, los reusos de refresh token con AM-002 y la profundidad de la DLQ con AM-019. También aparecen latencia p95 y eventos publicados frente a consumidos. En Explore, Loki permite seguir los logs del worker sin exponer secretos. La fuente Prometheus permite consultar las métricas por tipo de evento. Finalmente, Targets confirma que API, worker, RabbitMQ y Prometheus están disponibles. AM-001, AM-002 y AM-019 figuran como mitigadas; la de la DLQ cuenta además con una regla de alerta versionada de Grafana y un runbook de atención y purga. Estas pantallas deben contener tráfico generado por una ejecución autorizada de la suite E2E o por la operación local, nunca tokens o cuerpos de mensajes.” |

### 12. Detección en ejecución con Falco — 13:15 a 13:50

| Campo | Indicación |
|---|---|
| Presentador | Milena Rocío Ramírez Espinosa |
| Pantalla exacta | Terminal con make falco-test lanzado uno o dos minutos antes (por ejemplo, al empezar la escena 11); después Grafana: tablero Identity Hub — Seguridad, panel Alertas de Falco, y Explore con Loki y la consulta {service="falco"}. Respaldo: [runbook de Falco](../runbooks/falco-alerta.md) y [sección de observabilidad del informe](../informe/informe-tecnico.md#monitoreo-y-observabilidad). |
| Narración | “Falco vigila las llamadas al sistema de los contenedores de la aplicación. Esos contenedores son de solo lectura y no traen shell, así que un shell o una escritura en el sistema no ocurre en operación normal. make falco-test lanza un contenedor desechable con nombre de servicio de la aplicación, para que entre en el alcance de las reglas, y ese contenedor escribe en /etc, abre un shell y hace una conexión a una IP pública. Las reglas propias lo detectan, Alloy envía las alertas a Loki y aparecen en el panel de Grafana, con una regla de alerta y un runbook. Falco detecta y avisa; no bloquea, y solo corre en el perfil local de observabilidad, no en CI.” |

### 13. Amenazas, resultados y cierre — 13:50 a 15:00

| Campo | Indicación |
|---|---|
| Presentador | Milena Rocío Ramírez Espinosa |
| Pantalla exacta | [Modelo de amenazas](../threat-model/README.md), [conclusiones del informe](../informe/informe-tecnico.md#conclusiones), [manual de usuario](../manuales/usuario.md) y [índice de documentación](../README.md). |
| Narración | “El modelo de amenazas usa STRIDE y evidencia de mitigaciones por componente. El resultado es una historia de permisos verificable, un pipeline que produce evidencia y una operación observable. La principal lección es que un gate debe fallar de forma controlada para demostrar que protege; por eso conservamos la línea base y su comparación con el estado endurecido. Declaramos los límites: Falco corre solo en el perfil local de observabilidad y no en CI, y la integración OAuth y OpenID Connect de terceros y el outbox transaccional quedan como trabajo futuro. Las imágenes ya están publicadas en Docker Hub con SBOM y firma, y no queda ninguna amenaza en estado abierto. Las tres diferencias entre especificación y código que halló la revisión ya están resueltas: la especificación ahora dice que las cuentas sin verificar no se purgan (la invitación vence a las 24 h y el administrador la reenvía), que el bloqueo manual de un administrador no vence y que solo lo levanta un administrador o un restablecimiento de contraseña, y el worker ya no recibe DATABASE_URL porque no la usa. La ubicación de cada resultado está en docs/README.md, el informe técnico y los manuales. Gracias.” |

## Tabla de tiempos

| Escena | Inicio | Fin | Duración |
|---|---:|---:|---:|
| 1. Apertura y objetivo | 00:00 | 00:35 | 00:35 |
| 2. Arquitectura y límites | 00:35 | 01:20 | 00:45 |
| 3. Flujo de la persona empleada | 01:20 | 02:45 | 01:25 |
| 4. Historia RF-021 | 02:45 | 04:50 | 02:05 |
| 5. Exportar la auditoría | 04:50 | 05:20 | 00:30 |
| 6. Claves de servicio | 05:20 | 06:15 | 00:55 |
| 7. Pipeline y evidencia | 06:15 | 08:35 | 02:20 |
| 8. Despliegue local | 08:35 | 09:40 | 01:05 |
| 9. Producción simulada | 09:40 | 10:30 | 00:50 |
| 10. IaC de referencia | 10:30 | 11:20 | 00:50 |
| 11. Observabilidad | 11:20 | 13:15 | 01:55 |
| 12. Falco | 13:15 | 13:50 | 00:35 |
| 13. Amenazas, resultados y cierre | 13:50 | 15:00 | 01:10 |
| **Total** | **00:00** | **15:00** | **15:00** |

La duración total es 15 minutos, en el límite superior del rango exigido de 10 a 15 minutos; si el ensayo con cronómetro se pasa, recorte primero la repetición de explicaciones de las escenas 7 y 11 antes que la historia RF-021 o la declaración de límites.

## Consejos de grabación y contingencia

- Grabar a 1920 × 1080 como recomendación y conservar un clip pregrabado y revisado por escena para reemplazar errores sin reiniciar toda la demostración.
- Ensayar con cronómetro; reducir explicación repetida, no la escena RF-021 ni la declaración de límites.
- Tener capturas aprobadas del manual y del informe como respaldo si una interfaz local no responde. Identificarlas como evidencia capturada, no como estado en vivo.
- Ocultar marcadores y notificaciones; nunca mostrar .env, contraseña de Grafana, tokens, códigos MFA, enlaces de invitación ni cuerpos de correo. Enmascarar la vista previa de Mailpit si contiene datos de una cuenta.
- Si Mailpit no contiene el correo esperado, no revelar otro mensaje ni repetir credenciales: pausar la escena, mostrar la captura correspondiente del manual y explicar que el buzón es local.
- Si Grafana no tiene datos, verificar que el perfil de observabilidad está levantado y usar las capturas del informe; no inventar series ni afirmar que un panel contiene tráfico que no se ve.
- Si el arranque requiere limpieza, explicar que make clean borra volúmenes y repetir el orden seguro: make setup y después make up-obs.
- Si en el equipo de grabación no están configurados los digests o el relay SMTP, mantener la escena 9 como explicación con la configuración y la evidencia ya verificada; no ejecutar ni simular un despliegue de producción.
- Si Terraform validate o Checkov no están disponibles en el equipo de grabación, mostrar la evidencia ya verificada y el ADR 0012; no ejecutar apply ni usar credenciales cloud.
