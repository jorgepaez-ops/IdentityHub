# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Builder Node LTS vigente y nginx-unprivileged, ambos fijados por digest   ║
# ║  real, usuario no-root explícito (T28). Historia (ver specs/adr/0007):    ║
# ║  esta línea base sembraba a propósito VULN-016 (Node 18/bullseye sin      ║
# ║  soporte), VULN-017 (`nginx:latest` sin fijar) y VULN-018 (root).         ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# node:24-bookworm — Node 24, LTS activa (desde octubre de 2025).
FROM node@sha256:64af3819f9275802414d7cdc38c27e9d82bd564dec4d4da87d008255d36c63b4 AS builder

WORKDIR /app

# Copiar primero los manifiestos aprovecha la caché de capas: las dependencias
# solo se reinstalan cuando cambia package-lock.json, no en cada cambio de src.
COPY package.json package-lock.json* ./
RUN npm ci || npm install

COPY . .
RUN npm run build

# ── Imagen final ──────────────────────────────────────────────────────────
# nginxinc/nginx-unprivileged:stable — corre como uid 101 por defecto y ya
# escucha en el puerto 8080 que usa este proyecto, sin cambiar nada más.
FROM nginxinc/nginx-unprivileged@sha256:0918d093d6088225655ddf602fdf00679c1c0c9a89c01c6dcdce5ee5e6c2f3f3 AS web

COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080

# Explícito además del valor por defecto de la imagen: Trivy config (DS002)
# analiza el Dockerfile de forma estática y no resuelve el USER heredado de
# una base referenciada solo por digest (mismo caso que backend/Dockerfile, T27).
USER 101

CMD ["nginx", "-g", "daemon off;"]
