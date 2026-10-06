# Guion de la demo en vivo

Recorrido de punta a punta del Identity Hub: alta de un empleado, invitación, MFA, SSO hacia
Contabilidad y vistas según el rol. Dura unos 5 minutos.

## Requisitos previos

- Stack levantado con `make up` (esperar a que todos los servicios estén sanos).
- Un usuario admin del Hub (el de la instalación) y su contraseña.
- Tres pestañas del navegador, idealmente en dos ventanas (admin y empleado):
  - Consola del Hub: <http://identityhub.localhost:8080>
  - Contabilidad: <http://contabilidad.localhost:8080>
  - Mailpit (buzón de la demo): <http://localhost:8025>
- Una ventana privada para el empleado, para que no comparta sesión con el admin.
- La demo se comprueba en Chrome y Firefox: ambos dominios deben responder sin tocar `/etc/hosts` ni instalar certificados. Contabilidad no puede leer la sesión del Hub; recibe el token solo por el flujo de autorización. También se verificó a mano en Safari. Con dos usuarios a la vez, cada uno necesita su propia sesión (ventana normal e incógnito, o dos navegadores): si comparten la cookie del Hub, el SSO entra directo con la sesión que ya existe.
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

## Ejecución automática

`make e2e` corre este mismo guion como `e2e/tests/demo.spec.ts` (cada paso es un `test.step` con la
misma numeración), junto con el resto de las pruebas de extremo a extremo. Si la demo se rompe, esa
prueba falla antes de llegar a clase.
