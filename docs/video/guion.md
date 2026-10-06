# Guion de demostración: ciclo DevSecOps de Identity Hub

Este guion orienta una demostración de aproximadamente 13 minutos. Muestra una historia de autorización distinta de la autenticación, el pipeline con su evidencia de seguridad, los despliegues local y de referencia, y la operación observable. La grabación sigue pendiente: este documento solo prepara la demostración.

## Propósito, duración y equipo

| Elemento | Definición |
|---|---|
| Propósito | Demostrar el ciclo completo de Identity Hub: aplicación, seguridad integrada, pipeline, despliegue y observabilidad. |
| Duración objetivo | 13:00 minutos; el rango aceptable del entregable es de 10 a 15 minutos. |
| Formato | Grabación de pantalla con narración, sin mostrar secretos, tokens ni códigos de correo. |
| Equipo completo | Miguel Ángel Díaz Díaz; María Fernanda Giraldo Osorio; William Ricardo Niño Pico; Jorge Iván Páez Rincón; Milena Rocío Ramírez Espinosa. |

### Reparto sugerido, modificable

El equipo puede intercambiar estos papeles sin alterar el contenido ni los tiempos.

| Persona | Papel sugerido | Escenas |
|---|---|---|
| Miguel Ángel Díaz Díaz | Apertura y arquitectura | 1 y 2 |
| María Fernanda Giraldo Osorio | Historia funcional RF-021 | 3 |
| William Ricardo Niño Pico | Pipeline, línea base y despliegue local | 4 y 5 |
| Jorge Iván Páez Rincón | Producción simulada e infraestructura | 6 y 7 |
| Milena Rocío Ramírez Espinosa | Observabilidad, amenazas y cierre | 8, 9 y 10 |

## Checklist de preparación

Complete esta lista antes de grabar. El orden de arranque no es intercambiable: primero **make setup** y después **make up** o **make up-obs**.

- [ ] Decidir si se necesita un reinicio limpio. Solo si se acepta perder los datos locales, ejecutar **make clean** antes del arranque; este objetivo borra los volúmenes.
- [ ] Desde la raíz, ejecutar **make setup**. No mostrar el archivo .env generado ni sus valores; si ya existe, el objetivo no lo sobrescribe.
- [ ] Después de setup, ejecutar **make up-obs** para la demostración completa. Si no se mostrará observabilidad, usar **make up**. Confirmar el estado con **make ps**.
- [ ] Antes del primer arranque, identificar en .env la dirección de bootstrap mediante el nombre de variable **BOOTSTRAP_ADMIN_EMAIL**. No mostrar su valor ni una contraseña: la invitación crea una cuenta pendiente y la persona elige su contraseña.
- [ ] Preparar, sin revelar contraseñas ni códigos, una cuenta administradora y una cuenta de empleado. La persona empleada debe tener una invitación disponible, poder definir su contraseña y después participar en RF-021. Verificar que el rol de demostración no exista antes de crearlo.
- [ ] Abrir Mailpit en http://localhost:8025 y confirmar que los mensajes de prueba pueden leerse allí. Mantener ocultos el cuerpo de los correos, enlaces de invitación y códigos MFA.
- [ ] Abrir Grafana en http://localhost:3000 y Prometheus en http://localhost:9090 cuando se use el perfil de observabilidad. Iniciar la sesión antes de grabar y no mostrar las credenciales, que viven en .env.
- [ ] Antes de la escena de observabilidad, generar tráfico visible con una ejecución autorizada de **make e2e** contra el stack levantado; este comando eleva y restaura el límite de fallos por IP. Confirmar que los paneles contienen datos antes de iniciar la grabación.
- [ ] Dejar abiertas las pestañas del Hub, Contabilidad, Mailpit, Grafana, Prometheus, el workflow de CI y los documentos referenciados por cada escena.
- [ ] Configurar navegador y aplicación de videollamada para compartir solo la ventana necesaria. Como recomendación de grabación, usar 1920 × 1080, un tamaño de letra legible y zoom consistente; ocultar marcadores personales, notificaciones, historial y pestañas ajenas.
- [ ] No abrir .env, terminales con variables exportadas, cabeceras HTTP, herramientas de desarrollo, JWT, refresh tokens, cuerpos de correo ni salidas que contengan secretos. La narración explica los controles sin exponer el tráfico sensible.

## Escenas

### 1. Apertura y objetivo — 00:00 a 00:35

| Campo | Indicación |
|---|---|
| Presentador | Miguel Ángel Díaz Díaz |
| Pantalla exacta | Portada de [informe técnico](../informe/informe-tecnico.md) y, después, [índice de la entrega](../README.md). |
| Narración | “Somos el equipo de Identity Hub. Construimos un proveedor de identidad y una aplicación cliente de Contabilidad para demostrar un ciclo DevSecOps completo. En trece minutos mostraremos una historia funcional diferente de iniciar sesión, cómo el pipeline protege los cambios, cómo se prepara el despliegue y cómo se observan los resultados. El código, los diagramas y la evidencia están versionados en el repositorio.” |

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
| Pantalla exacta | Hub: Usuarios y Roles; después Contabilidad: Resumen y Transacciones; al final Hub: Auditoría filtrada por role_created. Seguir la [historia de sustentación](../manuales/usuario.md#5-historia-de-usuario-para-la-sustentación). |
| Narración | “La historia es deliberadamente distinta de autenticación. Como administradora, creo el rol contabilidad.auditor y marco únicamente reportes.ver y movimientos.ver_todos. Creo el rol y lo asigno a un empleado. La grilla impide crear roles del sistema, mezclar permisos de otra aplicación, editar los permisos de un rol propio o eliminar un rol asignado. Cada cambio deja auditoría. Ahora el empleado entra a Contabilidad. Puede ver el resumen organizacional y todas las transacciones, pero no aparecen acciones para registrar, aprobar o rechazar, y el cierre queda bloqueado. El Hub emite solo los permisos seleccionados y Contabilidad decide su interfaz a partir de ellos. No se cambió código ni se redesplegó la aplicación. Las seis pruebas E2E de RF-021 en e2e/tests/roles-y-permisos.spec.ts comprueban este recorrido y sus restricciones, incluida la auditoría. Si el empleado tenía una sesión previa, debe obtener un token nuevo; el cambio rige desde el siguiente token, hasta quince minutos.” |

### 5. Pipeline, línea base y estado endurecido — 04:50 a 07:10

| Campo | Indicación |
|---|---|
| Presentador | William Ricardo Niño Pico |
| Pantalla exacta | [Workflow principal de CI](../../.github/workflows/ci.yml), [manual de seguridad](../manuales/seguridad.md), [guía de hooks](../guia-desarrollo.md#6-hooks-de-pre-commit-rnf-003), [ADR 0007](../../specs/adr/0007-linea-base-vulnerable-deliberada.md), [ficha VULN-019](../../security/findings/VULN-019-imagenes-base-antiguas-compose.md) y [evidencia posterior VULN-019](../../security/evidence/actions-37473616659/README.md). |
| Narración | “La seguridad se integra desde el cambio, no al final. El pipeline ejecuta secretos con Gitleaks; SAST con CodeQL y Semgrep; dependencias con govulncheck, npm audit y osv-scanner; pruebas unitarias e integración; E2E con Playwright; configuración de contenedores con Trivy; construcción de imágenes y Trivy image; DAST con OWASP ZAP; e IaC con Checkov. Semgrep, Trivy y Checkov publican sus resultados SARIF en Code Scanning. Antes del commit, el hook local incluye Gitleaks: para demostrarlo sin dejar una cadena sensible en este guion, mostramos un clip pregrabado de un archivo desechable con una clave falsa de forma AWS, el rechazo Detect hardcoded secrets y su eliminación posterior. La línea base vulnerable fue deliberada: sirve para mostrar que los controles detectan problemas reales. No se despliega ni se demuestra ejecutando el tag v0.0.0-vuln-baseline. El ejemplo visual antes y después es VULN-019: Trivy pasó de 710 a 108 hallazgos HIGH o CRITICAL en las ocho imágenes del Compose. Mostramos la ficha y la evidencia; el estado endurecido está identificado por v0.1.0-hardened. Los workflows de línea base y reescaneo semanal complementan el pipeline principal.” |

### 6. Despliegue local reproducible — 07:10 a 08:15

| Campo | Indicación |
|---|---|
| Presentador | William Ricardo Niño Pico |
| Pantalla exacta | Terminal en la raíz con la salida ya preparada de **make setup**, **make up-obs** y **make ps**; después Hub, Mailpit y la [guía de despliegue](../manuales/despliegue-y-operacion.md). |
| Narración | “El arranque local es reproducible. Primero ejecutamos make setup, que genera .env local sin sobrescribirlo; después make up-obs construye y levanta el stack con observabilidad. make ps confirma los contenedores. Mailpit recibe el correo local sin enviar mensajes a Internet. Evitamos mostrar .env, contraseñas, códigos y enlaces porque son datos sensibles incluso en un entorno de demostración. La guía documenta también cuándo usar make clean: solo para borrar voluntariamente los volúmenes y reiniciar desde cero.” |

### 7. Producción simulada y escena condicional de T17 — 08:15 a 09:05

| Campo | Indicación |
|---|---|
| Presentador | Jorge Iván Páez Rincón |
| Pantalla exacta | [Sección de producción del manual](../manuales/despliegue-y-operacion.md#producción) y [ADR 0011](../../specs/adr/0011-docker-hub-como-registry.md). |
| Narración | “La producción simulada usa make up-prod con imágenes por digest, sin build, y solo publica el puerto del servicio web. **[si T17 está publicada]** mostramos el comando y su resultado únicamente cuando existen las imágenes publicadas y están configurados sus digests y el relay SMTP. **[si T17 no está publicada]** no ejecutamos ni simulamos la publicación: mostramos la configuración, explicamos que falta decidir el namespace y el token de Docker Hub, y declaramos que la validación completa queda pendiente. T17 es la tarea que agrega la publicación versionada, el SBOM y la firma.” |

### 8. Infraestructura de referencia, sin apply — 09:05 a 09:55

| Campo | Indicación |
|---|---|
| Presentador | Jorge Iván Páez Rincón |
| Pantalla exacta | [Diagrama AWS](../diagramas/uml/despliegue-aws.md), [ADR 0012](../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md) y terminal con evidencia preparada de terraform validate y Checkov. |
| Narración | “La infraestructura describe una referencia de AWS con Terraform. No ejecutamos terraform apply en la demostración: la decisión documentada es validar la configuración con terraform validate y Checkov. El diagrama permite revisar los componentes sin afirmar que exista un despliegue cloud activo. Esta separación evita convertir una demostración académica en una operación con costo o credenciales no autorizadas.” |

### 9. Observabilidad: métricas y logs — 09:55 a 11:50

| Campo | Indicación |
|---|---|
| Presentador | Milena Rocío Ramírez Espinosa |
| Pantalla exacta | Grafana: dashboard Identity Hub — Seguridad; Grafana Explore con Loki; Grafana Explore con Prometheus; página Targets de Prometheus. Como respaldo, [sección de observabilidad del informe](../informe/informe-tecnico.md#monitoreo-y-observabilidad). |
| Narración | “Con make up-obs, Prometheus recoge métricas y Loki recibe los logs estructurados mediante Grafana Alloy. En el dashboard de seguridad, los intentos de inicio de sesión por resultado se alinean con AM-001, los reusos de refresh token con AM-002 y la profundidad de la DLQ con AM-019. También aparecen latencia p95 y eventos publicados frente a consumidos. En Explore, Loki permite seguir los logs del worker sin exponer secretos. La fuente Prometheus permite consultar las métricas por tipo de evento. Finalmente, Targets confirma que API, worker, RabbitMQ y Prometheus están disponibles. AM-001 y AM-002 figuran como mitigadas; AM-019 sigue abierta de forma parcial: existe el panel, no una alerta versionada ni un runbook de purga. Estas pantallas deben contener tráfico generado por una ejecución autorizada de la suite E2E o por la operación local, nunca tokens o cuerpos de mensajes.” |

### 10. Amenazas, resultados y cierre — 11:50 a 13:00

| Campo | Indicación |
|---|---|
| Presentador | Milena Rocío Ramírez Espinosa |
| Pantalla exacta | [Modelo de amenazas](../threat-model/README.md), [conclusiones del informe](../informe/informe-tecnico.md#conclusiones), [manual de usuario](../manuales/usuario.md) y [índice de documentación](../README.md). |
| Narración | “El modelo de amenazas usa STRIDE y evidencia de mitigaciones por componente. El resultado es una historia de permisos verificable, un pipeline que produce evidencia y una operación observable. La principal lección es que un gate debe fallar de forma controlada para demostrar que protege; por eso conservamos la línea base y su comparación con el estado endurecido. Declaramos los límites: Falco no está incluido; AM-019 conserva el panel pero no una alerta ni runbook; T17 sigue pendiente si no hay publicación; y el correo del worker tiene un defecto de charset documentado, que puede mostrar caracteres mal codificados en Mailpit. El trabajo futuro incluye publicar imágenes con SBOM y firma, corregir el charset del correo, completar alerta y runbook de la DLQ y evaluar Falco. La ubicación de cada resultado está en docs/README.md, el informe técnico y los manuales. Gracias.” |

## Tabla de tiempos

| Escena | Inicio | Fin | Duración |
|---|---:|---:|---:|
| 1. Apertura y objetivo | 00:00 | 00:35 | 00:35 |
| 2. Arquitectura y límites | 00:35 | 01:20 | 00:45 |
| 3. Flujo de la persona empleada | 01:20 | 02:45 | 01:25 |
| 4. Historia RF-021 | 02:45 | 04:50 | 02:05 |
| 5. Pipeline y evidencia | 04:50 | 07:10 | 02:20 |
| 6. Despliegue local | 07:10 | 08:15 | 01:05 |
| 7. Producción simulada | 08:15 | 09:05 | 00:50 |
| 8. IaC de referencia | 09:05 | 09:55 | 00:50 |
| 9. Observabilidad | 09:55 | 11:50 | 01:55 |
| 10. Amenazas, resultados y cierre | 11:50 | 13:00 | 01:10 |
| **Total** | **00:00** | **13:00** | **13:00** |

La duración total es 13 minutos, dentro del rango exigido de 10 a 15 minutos.

## Consejos de grabación y contingencia

- Grabar a 1920 × 1080 como recomendación y conservar un clip pregrabado y revisado por escena para reemplazar errores sin reiniciar toda la demostración.
- Ensayar con cronómetro; reducir explicación repetida, no la escena RF-021 ni la declaración de límites.
- Tener capturas aprobadas del manual y del informe como respaldo si una interfaz local no responde. Identificarlas como evidencia capturada, no como estado en vivo.
- Ocultar marcadores y notificaciones; nunca mostrar .env, contraseña de Grafana, tokens, códigos MFA, enlaces de invitación ni cuerpos de correo. Enmascarar la vista previa de Mailpit si contiene datos de una cuenta.
- Si Mailpit no contiene el correo esperado, no revelar otro mensaje ni repetir credenciales: pausar la escena, mostrar la captura correspondiente del manual y explicar que el buzón es local.
- Si Grafana no tiene datos, verificar que el perfil de observabilidad está levantado y usar las capturas del informe; no inventar series ni afirmar que un panel contiene tráfico que no se ve.
- Si el arranque requiere limpieza, explicar que make clean borra volúmenes y repetir el orden seguro: make setup y después make up-obs.
- Si T17 no está completo, mantener la escena 7 como declaración transparente de pendiente; no ejecutar ni simular la publicación, la firma o un despliegue de producción.
- Si Terraform validate o Checkov no están disponibles en el equipo de grabación, mostrar la evidencia ya verificada y el ADR 0012; no ejecutar apply ni usar credenciales cloud.
