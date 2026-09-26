# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Imágenes finales: distroless (sin shell, sin gestor de paquetes), usuario ║
# ║  no-root fijo (65532) e imagen base fijada por digest real (T27).         ║
# ║  Historia (ver specs/adr/0007): esta línea base sembraba a propósito      ║
# ║  VULN-008 (Debian 11 en vez de distroless), VULN-009 (USER root),         ║
# ║  VULN-010 (apt-get sin fijar versión) y VULN-011 (ADD remoto).            ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# golang:1.25-bookworm — go1.25.14, satisface el "go 1.25.0" de go.mod (T6/T24).
FROM golang@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Sin CGO para obtener un binario estático; -trimpath evita filtrar rutas de la
# máquina de construcción en los símbolos.
RUN CGO_ENABLED=0 go build -trimpath -o /out/api    ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker

# ── Imagen final de la API ────────────────────────────────────────────────
# gcr.io/distroless/static-debian12:nonroot — sin shell, sin apt, corre como
# uid 65532 por defecto; trae /etc/ssl/certs/ca-certificates.crt para TLS
# saliente, así que el binario estático no necesita nada más.
FROM gcr.io/distroless/static-debian12@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS api

COPY --from=builder /out/api /usr/local/bin/api

EXPOSE 8081

# Explícito además del valor por defecto de la imagen: Trivy config (DS002)
# analiza el Dockerfile de forma estática y no resuelve el USER heredado de
# una base referenciada solo por digest.
USER 65532:65532

ENTRYPOINT ["/usr/local/bin/api"]

# ── Imagen final del worker ───────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS worker

COPY --from=builder /out/worker /usr/local/bin/worker

EXPOSE 9091

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/worker"]
