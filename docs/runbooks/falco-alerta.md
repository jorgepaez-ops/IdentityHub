# Runbook: Falco detectó actividad anómala en un contenedor (AM-020)

Alerta de Grafana **Falco detectó actividad anómala en un contenedor**
(`deploy/observability/grafana/alerting/falco.yml`, carpeta «Identity Hub»). Se dispara cuando Loki
tiene al menos una alerta de Falco con prioridad `Warning` o superior en los últimos 5 minutos.
Severidad: `warning`. También se ve en el panel «Alertas de Falco» del tablero «Identity Hub — Seguridad».

## Qué significa

Falco observa las llamadas al sistema de todos los contenedores del host (driver modern eBPF) y evalúa
las reglas de `deploy/observability/falco/rules.d/identity-hub.yaml` solo sobre `api`, `worker` y `web`.
Esos contenedores son distroless (`api`, `worker`), de solo lectura, sin capacidades y con
`no-new-privileges` (AM-020), así que lo siguiente **no ocurre en operación normal**:

| Regla | Qué detecta | Por qué importa |
|---|---|---|
| `Identity Hub shell in application container` | Un `sh`, `bash`, etc. se ejecutó dentro del contenedor | Ejecución de comandos tras comprometer el proceso, o un `docker exec` humano |
| `Identity Hub write to system directory in application container` | Intento de abrir para escritura algo bajo `/etc`, `/bin`, `/sbin`, `/usr` o `/lib` | Persistencia o manipulación del sistema. La raíz es de solo lectura (verá `result=EROFS`): falló, pero alguien lo intentó |
| `Identity Hub package manager or downloader in application container` | `apk`, `apt`, `pip`, `npm`, `curl`, `wget`... | El atacante prepara herramientas |
| `Identity Hub outbound connection to public address from application container` | Conexión IPv4 hacia una dirección no privada ni loopback | Exfiltración o canal de mando; los servicios solo hablan con `db`, `broker` y `mailpit` |

Excepciones deliberadas: los shells de arranque de `web` (el entrypoint de nginx es un script `sh`) y la
sonda de salud de `web` (`wget` contra `127.0.0.1`).

## Diagnóstico

1. Identifique la regla, el contenedor y el comando. En Grafana (Explore, fuente Loki):

   ```logql
   {service="falco"} | json | line_format "{{.rule}} | {{.output_fields_container_name}} | {{.output_fields_proc_cmdline}}"
   ```

   O directamente en el log de Falco (cada alerta es una línea JSON):

   ```bash
   docker compose --env-file .env -f deploy/docker-compose.yml --profile observability logs falco --since 30m | grep '^{'
   ```

2. Decida si fue legítimo. Pregunte quién operó el host en esa ventana: un `docker exec` de depuración
   (`api` y `worker` no traen shell, así que un shell ahí es casi siempre un `exec` desde un contenedor
   efímero o un binario inyectado).
3. Si **no** hay explicación, trátelo como incidente: siga el paso de contención. Los campos `proc.cmdline`
   y `proc.pname` del JSON indican qué lo lanzó; `output_fields` incluye `container.id` e imagen.
4. Correlacione con los logs de la aplicación del mismo contenedor y minuto (`{service="api"}`) y con la
   auditoría de la API (`audit_log`) en busca de peticiones raras justo antes.

## Contención y remediación

1. Conserve evidencia antes de tocar nada: copie las líneas JSON de Falco y los logs del contenedor.
   No pegue cuerpos de peticiones ni datos de usuarios en tickets.
2. Aísle el contenedor sospechoso: `docker stop <contenedor>` (el filesystem de solo lectura y `tmpfs`
   dejan poco rastro persistente, pero el estado en memoria se pierde al parar).
3. Rote los secretos a los que ese contenedor tenía acceso (`JWT_SIGNING_KEY` para `api`, credenciales
   de base y de RabbitMQ) y revoque las
   familias de refresh token afectadas.
4. Reconstruya y vuelva a desplegar desde imágenes por digest; no reutilice el contenedor.
5. Si fue un falso positivo, ajuste la regla en `identity-hub.yaml` (excepción acotada, no un
   `enabled: false`) y deje constancia en el cambio.
6. La alerta vuelve a **Normal** unos 5 minutos después de la última detección (la ventana de consulta).

## Cómo probar la alerta

Con el stack de observabilidad levantado (`make up-obs`):

```bash
make falco-test
```

Lanza un contenedor desechable llamado `identity-hub-falco-test-api-1` (alpine, raíz de solo lectura)
que escribe en `/etc`, ejecuta un shell y un `wget` hacia una IP pública. Verá cuatro reglas en el log
de Falco al instante; Alloy descubre contenedores nuevos cada ~60 s, así que en Loki y en Grafana la
alerta aparece en 1-2 minutos. No hay nada que limpiar: el contenedor se borra solo.

## Limitaciones conocidas

- **Plataforma.** Requiere un kernel Linux con BTF y eBPF (Docker Desktop en macOS/Windows y Linux
  modernos). Se verificó en Docker Desktop con kernel 7.0.14 (aarch64). Falco corre sin `privileged`, con
  `BPF`, `PERFMON`, `SYS_RESOURCE` y `SYS_PTRACE`; en hosts con SELinux o políticas AppArmor
  restrictivas puede hacer falta ajustar `security_opt`.
- **Avisos de arranque.** Falco registra «failed to create tracepoint syscalls/sys_enter_*»: es la
  mitigación TOCTOU opcional, que necesita tracefs. La detección funciona igual; solo es menos robusta
  frente a una condición de carrera entre la comprobación y el uso del argumento de la llamada.
- **No está en CI.** Los runners de GitHub usan otro kernel y no se les da acceso a eBPF: es un control
  de ejecución del stack desplegado, no un gate de pipeline. Las reglas se validan con
  `falco -V deploy/observability/falco/rules.d/identity-hub.yaml` con la misma imagen fijada.
- **Falco o Alloy caídos no alertan.** Sin datos la regla queda en *Normal* (es el estado sano). Compruebe
  que `falco` esté `running` (`make ps` con el perfil) y que existan líneas recientes en Loki.
- **Solo observa, no bloquea.** Falco no detiene el proceso: detecta y avisa.
- **Sin punto de contacto.** Como la alerta del DLQ, usa la política por defecto de Grafana, que en local
  no envía correos; en producción hay que definir un punto de contacto para `severity=warning`.
- **Un solo host.** Falco ve los contenedores del host donde corre; en un despliegue multi-host hay uno
  por nodo.
