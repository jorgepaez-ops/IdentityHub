# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Builder Node LTS vigente y nginx-unprivileged, ambos fijados por digest   ║
# ║  real, usuario no-root explícito (T28). Historia (ver specs/adr/0007):    ║
# ║  esta línea base sembraba a propósito VULN-016 (Node 18/bullseye sin      ║
# ║  soporte), VULN-017 (`nginx:latest` sin fijar) y VULN-018 (root).         ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# T13 — el contexto de construcción es la raíz del repositorio (no `frontend/`):
# la imagen compila las dos SPA, la consola del Hub (frontend/) y Contabilidad
# (contabilidad/), y sirve cada una desde su propio `server` de Nginx.

# node:24-bookworm — Node 24, LTS activa (desde octubre de 2025).
FROM node@sha256:64af3819f9275802414d7cdc38c27e9d82bd564dec4d4da87d008255d36c63b4 AS builder

WORKDIR /app

# Copiar primero los manifiestos aprovecha la caché de capas: las dependencias
# solo se reinstalan cuando cambia package-lock.json, no en cada cambio de src.
COPY frontend/package.json frontend/package-lock.json* ./
# Sin scripts de ciclo de vida: la instalación no ejecuta código de terceros
# (Sonar S6505) y `npm ci` exige el lockfile, sin caer a resolver versiones nuevas.
RUN npm ci --ignore-scripts

COPY frontend/ ./
RUN npm run build

# ── Contabilidad (T13) ────────────────────────────────────────────────────
# Mismo Node fijado por digest que el builder del Hub.
FROM node@sha256:64af3819f9275802414d7cdc38c27e9d82bd564dec4d4da87d008255d36c63b4 AS contabilidad-builder

WORKDIR /app

COPY contabilidad/package.json contabilidad/package-lock.json* ./
RUN npm ci --ignore-scripts

COPY contabilidad/ ./
RUN npm run build

# ── Imagen final ──────────────────────────────────────────────────────────
# nginxinc/nginx-unprivileged:stable-alpine — corre como uid 101 por defecto y ya
# escucha en el puerto 8080 que usa este proyecto, sin cambiar nada más.
FROM nginxinc/nginx-unprivileged@sha256:ed04ec1ff34502c339ee5c3ae3f855442398edc1d05591e2b98981dcbbd20b1e AS web

COPY frontend/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY frontend/nginx/security-headers.conf /etc/nginx/security-headers.conf
COPY --from=builder /app/dist /usr/share/nginx/html
COPY --from=contabilidad-builder /app/dist /usr/share/nginx/contabilidad

# CVE-2026-103111 (HIGH): la base fijada trae pcre2 10.48-r0 y upstream todavía no
# publicó una imagen con 10.49. Se actualiza solo ese paquete (decisión del
# usuario, 2026-10-02); retirar esta capa cuando el digest de la base ya lo incluya.
USER 0
RUN apk upgrade --no-cache pcre2

EXPOSE 8080

# Explícito además del valor por defecto de la imagen: Trivy config (DS002)
# analiza el Dockerfile de forma estática y no resuelve el USER heredado de
# una base referenciada solo por digest (mismo caso que backend/Dockerfile, T27).
USER 101

CMD ["nginx", "-g", "daemon off;"]
