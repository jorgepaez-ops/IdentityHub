# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  ⚠️  DOCKERFILE DELIBERADAMENTE VULNERABLE — NO CORREGIR SIN LEER ESTO    ║
# ║                                                                          ║
# ║  Ver specs/adr/0007-linea-base-vulnerable-deliberada.md                   ║
# ║                                                                          ║
# ║    VULN-016  node:18-bullseye, rama sin soporte        → Trivy image      ║
# ║    VULN-017  nginx:latest, etiqueta móvil sin fijar    → Hadolint DL3007  ║
# ║    VULN-018  la imagen final corre como root           → Hadolint DL3002  ║
# ║                                                                          ║
# ║  La versión remediada usa una LTS de Node vigente, nginx-unprivileged     ║
# ║  fijado por digest y un usuario no-root.                                  ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# VULN-016 — Node 18 sobre Debian bullseye (11): ambas ramas acumulan CVE.
FROM node:18-bullseye AS builder

WORKDIR /app

# Copiar primero los manifiestos aprovecha la caché de capas: las dependencias
# solo se reinstalan cuando cambia package-lock.json, no en cada cambio de src.
COPY package.json package-lock.json* ./
RUN npm ci || npm install

COPY . .
RUN npm run build

# ── Imagen final ──────────────────────────────────────────────────────────
# VULN-017 — `latest` es una etiqueta móvil: la construcción de hoy y la de
# mañana pueden no ser la misma imagen, y eso rompe la reproducibilidad y la
# cadena de suministro. Lo correcto es fijar por digest.
FROM nginx:latest AS web

COPY nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080

# VULN-018 — nginx arranca su proceso maestro como root. La imagen
# `nginxinc/nginx-unprivileged` existe justamente para evitarlo.
CMD ["nginx", "-g", "daemon off;"]
