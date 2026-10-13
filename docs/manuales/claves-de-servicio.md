# Claves de servicio

Una clave de servicio es la credencial de una integración máquina a máquina (RF-018). El caso de uso previsto: un SIEM o un sistema de reportes que consulta el registro de auditoría del Hub sin una sesión humana. Su diseño está en la [ADR 0014](../../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md); cómo emitirlas desde la consola, en la [sección 3.7 del manual de usuario](usuario.md#37-claves-de-servicio).

## Formato y propiedades

`ihk_<publicId>_<secreto>`: el `publicId` son 16 caracteres hexadecimales (no es secreto y identifica la clave en el listado y en la auditoría) y el secreto son 32 bytes aleatorios en base64url.

- **Se muestra una sola vez.** Al emitirla, la respuesta de la API (`201`) y la consola devuelven la clave completa; después solo se puede ver el prefijo `ihk_<publicId>`. El Hub guarda únicamente el SHA-256 del secreto.
- **Siempre vence.** Vigencia obligatoria de 1 a 365 días; no hay claves permanentes. Para rotar, emita una nueva y revoque la anterior.
- **Alcance mínimo.** Lista cerrada; hoy solo `audit-log:read`.
- **Revocable.** La revocación es inmediata, lógica e idempotente.

## Cómo usarla

Se envía como `Authorization: Bearer`, igual que un access token, pero solo la aceptan las operaciones que declaran su alcance en el contrato (esquema de seguridad `serviceKeyAuth` de [`openapi.yaml`](../../specs/03-api/openapi.yaml)). Hoy:

| Operación | Alcance |
|---|---|
| `GET /api/v1/admin/audit-log` | `audit-log:read` |

Ejemplo (la variable `IH_SERVICE_KEY` contiene la clave y viene del gestor de secretos de la integración):

```sh
curl -s -H "Authorization: Bearer $IH_SERVICE_KEY" \
  "http://identityhub.localhost:8080/api/v1/admin/audit-log?limit=100"
```

La paginación y los filtros son los de siempre (`cursor`, `limit`, `action`, `actorId`, `since`).

## Qué responde la API

- Una clave desconocida, con secreto incorrecto, expirada o revocada recibe el **mismo `401`**: la respuesta no dice cuál fue la causa. El motivo real no se devuelve: si la clave existe (secreto incorrecto, expirada, revocada o sin alcance) queda en la auditoría (`service_key_rejected`, con el `publicId`, nunca el secreto); si es malformada o desconocida solo queda en el log estructurado, porque auditarla dejaría que cualquiera sin autenticarse llenara la tabla de auditoría (Alloy/Loki ya recogen esos logs).
- En cualquier otra operación (usuarios, roles, gestión de claves, `/me`) la clave recibe `401`: una clave de servicio no puede emitir ni revocar claves.
- Emitir, listar y revocar claves exige un access token de administrador (`403` para un usuario sin el rol `admin`).

## Buenas prácticas

- Una clave por integración y con el nombre de la integración, para poder revocarla sin afectar a otras.
- Guárdela en un gestor de secretos, nunca en el repositorio ni en un archivo de configuración versionado.
- Revise `lastUsedAt` en el listado: una clave que ya no se usa debe revocarse.
- Si sospecha que se filtró, revóquela primero y revise en la auditoría los eventos `service_key_rejected` y el uso de esa clave.
