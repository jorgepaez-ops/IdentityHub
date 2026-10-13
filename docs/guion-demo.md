# Guion de la demo en vivo

Recorrido de punta a punta del Identity Hub: alta de un empleado, invitación, MFA, SSO hacia
Contabilidad y vistas según el rol, y cierra con la exportación de la auditoría, las claves de servicio y Falco. Dura unos 5 minutos los pasos 1 a 7 y unos 4 minutos más los pasos 8 a 10, que son opcionales.

## Requisitos previos

- Stack levantado con `make up` (`make setup` antes solo en un clon nuevo; incluye Grafana y Falco; esperar a que todos los servicios estén sanos).
- Un usuario admin del Hub (el de la instalación) y su contraseña.
- Tres pestañas del navegador, idealmente en dos ventanas (admin y empleado):
  - Consola del Hub: <http://identityhub.localhost:8080>
  - Contabilidad: <http://contabilidad.localhost:8080>
  - Mailpit (buzón de la demo): <http://localhost:8025>
- Una ventana privada para el empleado, para que no comparta sesión con el admin.
- La demo se comprueba en Chrome y Firefox: ambos dominios deben responder sin tocar `/etc/hosts` ni instalar certificados. Contabilidad no puede leer la sesión del Hub; recibe el token solo por el flujo de autorización. También se verificó a mano en Safari. Con dos usuarios a la vez, cada uno necesita su propia sesión (ventana normal e incógnito, o dos navegadores): si comparten la cookie del Hub, el SSO entra directo con la sesión que ya existe.
- Para los pasos 8 a 10: una terminal con `curl` abierta en la raíz del repositorio y, solo para el paso 10, el stack levantado con `make up` (Falco exige un kernel Linux con eBPF o Docker Desktop; si no corre, omitir ese paso) y Grafana en <http://localhost:3000> con la sesión ya iniciada (las credenciales viven en `.env`; no mostrarlas).
- Plan B si Mailpit web falla: leer el correo con `curl http://localhost:8025/api/v1/messages`.

## Guion

| # | Qué se hace | Qué mostrar en pantalla |
|---|-------------|-------------------------|
| 1 | El admin inicia sesión en el Hub y escribe el código MFA que llega a Mailpit. | Formulario de acceso, correo "sign-in code" en Mailpit, página Inicio de la consola. |
| 2 | Desde Inicio, abrir Usuarios, pulsar "Nuevo usuario", completar nombre y correo, marcar el rol `contabilidad.analista` y "Crear y enviar invitación". | El aviso "Invitación enviada" y el correo de invitación en Mailpit. |
| 3 | En la ventana del empleado, abrir el enlace del correo y definir la contraseña ("Definir contraseña"). | "Tu cuenta quedó activada". El admin nunca conoce la contraseña. |
| 4 | El empleado inicia sesión en el Hub y escribe el código MFA de Mailpit. | Su página "Mi cuenta" y el segundo correo de código. |
| 5 | El empleado abre Contabilidad y pulsa "Continuar con Identity Hub". | Redirección al Hub y regreso a Contabilidad sin formulario de contraseña ni nuevo correo: ya había sesión. |
| 6 | Mostrar la vista del analista. | Etiqueta "Analista contable" y "Cierre contable" bloqueado con candado. |
| 7 | El admin abre "Editar" sobre el empleado, cambia el rol a `contabilidad.senior` y guarda. El empleado vuelve a abrir Contabilidad y pulsa "Continuar con Identity Hub". | Etiqueta "Contador senior" y "Cierre contable" disponible. El cambio se ve en el siguiente acceso, no en la pantalla ya abierta. |
| 8 | El admin abre "Auditoría", deja los filtros por defecto (o elige una acción) y pulsa "Exportar CSV"; después "Exportar JSON". | Dos descargas con los eventos que cumplen el filtro (hasta 10 000, del más reciente al más antiguo). Filtrar luego por la acción `audit_log_exported` para mostrar que cada exportación quedó auditada. |
| 9 | El admin abre "Claves de servicio", escribe el nombre de la clave (por ejemplo "SIEM demo"), deja marcado `audit-log:read`, fija la vigencia en días y pulsa "Emitir clave". En el aviso "Clave emitida" pulsa "Copiar clave" y guarda el valor en la variable `IH_SERVICE_KEY` de la terminal, sin mostrarlo. Luego ejecuta `curl -s -H "Authorization: Bearer $IH_SERVICE_KEY" "http://identityhub.localhost:8080/api/v1/admin/audit-log?limit=5"`. Vuelve a la consola, pulsa "Ya la copié", "Revocar SIEM demo" y "Confirmar revocación de SIEM demo", y repite el `curl`. | El aviso "no se volverá a mostrar" (la clave nunca se vuelve a ver ni aparece en la tabla), la respuesta 200 con eventos de auditoría y, tras revocar, el mismo comando devolviendo 401. En Auditoría se ven `service_key_created`, `service_key_revoked` y `service_key_rejected`. |
| 10 | Con `make up` levantado, ejecutar `make falco-test` en la terminal. Esperar entre 1 y 2 minutos (Alloy descubre contenedores nuevos cada unos 60 s) y abrir en Grafana el tablero "Identity Hub - Seguridad", panel "Alertas de Falco", o Explore con la fuente Loki y la consulta `{service="falco"}`. | La salida de `make falco-test` (lanza un contenedor desechable llamado `identity-hub-falco-test-api-1`, que coincide con el patrón de nombres de `api`, `worker` y `web` al que se limitan las reglas, que escribe en `/etc`, abre un shell y hace un `wget` a una IP pública) y las alertas de las reglas propias de Falco en el panel y en Loki. Runbook: `docs/runbooks/falco-alerta.md`. |

## Ejecución automática

`make e2e` corre los pasos 1 a 7 como `e2e/tests/demo.spec.ts` (cada paso es un `test.step` con la
misma numeración), junto con el resto de las pruebas de extremo a extremo; los pasos 8 y 9 los ejercen `e2e/tests/control-de-acceso.spec.ts` (RF-019) y `e2e/tests/claves-de-servicio.spec.ts` (RF-018), y el paso 10 es manual (Falco no corre en CI). Si la demo se rompe, esa
prueba falla antes de llegar a clase.
