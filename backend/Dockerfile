# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  ⚠️  DOCKERFILE DELIBERADAMENTE VULNERABLE — NO CORREGIR SIN LEER ESTO    ║
# ║                                                                          ║
# ║  Ver specs/adr/0007-linea-base-vulnerable-deliberada.md                   ║
# ║                                                                          ║
# ║    VULN-008  base Debian 11 en vez de distroless  → Trivy image           ║
# ║    VULN-009  USER root en la imagen final         → Hadolint DL3002, Trivy║
# ║    VULN-010  apt-get sin fijar versión ni limpiar → Hadolint DL3008/DL3009║
# ║    VULN-011  ADD desde una URL remota             → Hadolint DL3020       ║
# ║                                                                          ║
# ║  La versión remediada (semana 3) usa distroless, usuario no-root, imagen  ║
# ║  base fijada por digest y ningún secreto en tiempo de construcción.       ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# VULN-008 — Go 1.22 sobre Debian bullseye (11), ya fuera de soporte estándar.
FROM golang:1.22-bullseye AS builder

WORKDIR /src

# VULN-010 — Instalación sin fijar versiones y sin limpiar la caché de apt:
# la construcción no es reproducible y la imagen carga listas de paquetes que
# no aporta nada llevar.
RUN apt-get update && apt-get install -y curl ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Sin CGO para obtener un binario estático; -trimpath evita filtrar rutas de la
# máquina de construcción en los símbolos.
RUN CGO_ENABLED=0 go build -trimpath -o /out/api    ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker

# ── Imagen final de la API ────────────────────────────────────────────────
# VULN-008 — debian:11-slim arrastra decenas de CVE de paquetes del sistema.
# Una imagen distroless no llevaría ninguno de ellos porque no lleva ni shell
# ni gestor de paquetes.
FROM debian:11-slim AS api

# VULN-011 — ADD desde una URL remota: descarga sin verificar suma ni firma, y
# el contenido puede cambiar entre construcciones.
ADD https://raw.githubusercontent.com/golang/go/master/LICENSE /licenses/go-LICENSE

RUN apt-get update && apt-get install -y ca-certificates curl

COPY --from=builder /out/api /usr/local/bin/api

EXPOSE 8081

# VULN-009 — No se declara USER: el proceso corre como root. Si alguien logra
# ejecución de código, la logra como root dentro del contenedor.
ENTRYPOINT ["/usr/local/bin/api"]

# ── Imagen final del worker ───────────────────────────────────────────────
FROM debian:11-slim AS worker

RUN apt-get update && apt-get install -y ca-certificates

COPY --from=builder /out/worker /usr/local/bin/worker

EXPOSE 9091

ENTRYPOINT ["/usr/local/bin/worker"]
