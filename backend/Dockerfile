# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  Imágenes finales: distroless (sin shell, sin gestor de paquetes), usuario ║
# ║  no-root fijo (65532) e imagen base fijada por digest real (T27).         ║
# ║  Historia (ver specs/adr/0007): esta línea base sembraba a propósito      ║
# ║  VULN-008 (Debian 11 en vez de distroless), VULN-009 (USER root),         ║
# ║  VULN-010 (apt-get sin fijar versión) y VULN-011 (ADD remoto).            ║
# ╚══════════════════════════════════════════════════════════════════════════╝

# golang:1.26-bookworm (go1.26.8) — satisface el "go 1.26.0" de go.mod (T34b:
# golang.org/x/crypto >= 0.56.0, VULN-028, declara go 1.26.0 en su propio
# go.mod). GOTOOLCHAIN=local en la imagen oficial de Go, así que la base debe
# traer el toolchain exacto: descargarlo en build time rompería el
# endurecimiento de imagen fijada por digest de T27.
FROM golang@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d AS builder

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

# La imagen distroless no tiene shell ni curl: el binario se autosondea con su
# propio subcomando "healthcheck" (mismo comando que usa el compose).
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
  CMD ["/usr/local/bin/api", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/api"]

# ── Imagen final del worker ───────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS worker

COPY --from=builder /out/worker /usr/local/bin/worker

EXPOSE 9091

# Sin HEALTHCHECK a proposito: el worker no tiene un subcomando de sondeo y la
# imagen distroless no trae shell, wget ni curl. Checkov (CKV_DOCKER_2) se
# satisface a nivel de archivo con el de la API; la salud del worker la vigila el
# orquestador (compose / ECS) con su propia comprobacion.

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/worker"]
