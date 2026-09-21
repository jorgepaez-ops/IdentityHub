module github.com/jorgepaez/identity-hub

// ⚠️  LÍNEA BASE VULNERABLE — ver specs/adr/0007-linea-base-vulnerable-deliberada.md
//
// La directiva `go` se elevó a 1.25 en T6 (decisión Q11 enmendada), y
// golang-jwt está fijado en la rama v4 con un CVE publicado. Es DELIBERADO:
// existe para que `govulncheck` tenga algo real que informar en el job `sca`.
// No actualizar sin leer el ADR y sin registrar la remediación en
// security/findings/.
go 1.25

require (
	github.com/go-chi/chi/v5 v5.3.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.1
	github.com/oapi-codegen/runtime v1.1.2
	github.com/prometheus/client_golang v1.18.0
	github.com/rabbitmq/amqp091-go v1.9.0
)

require github.com/golang-jwt/jwt/v5 v5.3.1

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	golang.org/x/crypto v0.17.0
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
