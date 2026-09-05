# 00 — Visión

## Problema

Toda aplicación multiusuario necesita responder tres preguntas: *quién eres* (autenticación),
*qué puedes hacer* (autorización) y *qué hiciste* (auditoría). Resolverlas mal es la causa
raíz de las categorías A01 (Broken Access Control) y A07 (Identification and Authentication
Failures) del OWASP Top 10.

**Identity Hub** es una plataforma de gestión de identidad autocontenida: un proveedor de
identidad (IdP) que emite y valida tokens, gestiona el ciclo de vida de las cuentas y deja
rastro auditable de cada evento de seguridad.

## Propósito real de este proyecto

La aplicación **no es el entregable principal**. El entregable es el **pipeline de DevSecOps de
ciclo completo** que la construye, prueba, escanea, firma, publica, despliega y vigila. Se
eligió el dominio de identidad precisamente porque concentra riesgo real —criptografía,
secretos, control de acceso— y por tanto da superficie auténtica a los controles de seguridad
del pipeline. Un pipeline sobre un "hola mundo" no demuestra nada.

## Alcance

Dentro:

- Registro de cuentas con verificación por correo electrónico.
- Autenticación por contraseña con Argon2id.
- Emisión de JWT firmados con Ed25519 y publicación del JWKS.
- Refresh tokens rotativos con detección de reuso.
- Segundo factor con TOTP y códigos de recuperación.
- Control de acceso basado en roles (`admin`, `user`).
- Registro de auditoría inmutable de eventos de seguridad.
- Notificaciones asíncronas por correo mediante un worker y una cola de mensajes.

Fuera:

- Federación con proveedores externos (Google, GitHub). No hay callbacks públicos en un
  entorno local; se documenta como trabajo futuro.
- Multi-tenancy / organizaciones. La entidad existe en el modelo pero no se explota.
- Alta disponibilidad, réplicas y escalado horizontal. El despliegue objetivo es un host
  único con Docker Compose.
- WebAuthn / passkeys. Deseable, fuera del presupuesto de un mes.

## Criterio de éxito

1. `git push` a `main` ejecuta el pipeline entero sin intervención manual y termina en un
   despliegue cuya firma ha sido verificada.
2. Cada gate de seguridad tiene al menos un hallazgo real documentado que lo justifica
   (ver `05-security/` y `security/findings/`).
3. Todo requisito funcional es trazable hasta una operación de la API, una prueba unitaria y
   un escenario de aceptación (ver `07-traceability.md`).
4. La imagen de producción de la API no contiene vulnerabilidades HIGH/CRITICAL corregibles.

## Licencia

Apache-2.0. El requisito del curso pide una aplicación "de libre uso"; el código, los specs y
los workflows son abiertos y reutilizables.
