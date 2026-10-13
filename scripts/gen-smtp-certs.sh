#!/bin/sh
# Genera una CA local y un certificado de servidor para Mailpit (SMTP con STARTTLS).
#
#   sh scripts/gen-smtp-certs.sh [--force] [--dir <directorio>] [--env-file <.env>]
#
# Salida (por defecto deploy/certs/, ignorado por git):
#   ca.pem, ca-key.pem          CA local (el worker confía en ca.pem vía SMTP_CA_FILE)
#   server.pem, server-key.pem  certificado de Mailpit, SAN: mailpit, localhost, 127.0.0.1
# Es idempotente: si los archivos existen no hace nada salvo con --force.
#
# Con --env-file <.env> escribe además smtp-auth.txt (usuario:contraseña para el
# --smtp-auth-file de Mailpit) a partir de SMTP_USERNAME (por defecto
# identity-hub) y SMTP_PASSWORD de ese archivo; se reescribe siempre, porque
# tiene que coincidir con el .env vigente.
set -eu

force=0
env_file=""
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
dir="$root/deploy/certs"

while [ $# -gt 0 ]; do
  case "$1" in
    --force) force=1 ;;
    --dir)
      [ $# -ge 2 ] || { echo "error: --dir requiere un valor" >&2; exit 2; }
      dir=$2
      shift
      ;;
    --env-file)
      [ $# -ge 2 ] || { echo "error: --env-file requiere un valor" >&2; exit 2; }
      env_file=$2
      shift
      ;;
    *) echo "uso: $0 [--force] [--dir <directorio>] [--env-file <.env>]" >&2; exit 2 ;;
  esac
  shift
done

command -v openssl >/dev/null 2>&1 || { echo "error: openssl no está instalado" >&2; exit 1; }

umask 077
mkdir -p "$dir"

# env_value NOMBRE: último valor de NOMBRE en el .env, sin comentario final ni las
# comillas que lo rodean (Compose las quita igual). Se lee con grep, no con ".".
env_value() {
  grep -E "^$1=" "$env_file" | tail -n 1 | cut -d= -f2- | sed -e 's/[[:space:]]*#.*$//' \
    -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/" || true
}

write_auth_file() {
  [ -n "$env_file" ] || return 0
  [ -f "$env_file" ] || { echo "error: no existe $env_file (corre make setup)" >&2; exit 1; }
  user=$(env_value SMTP_USERNAME)
  pass=$(env_value SMTP_PASSWORD)
  user=${user:-identity-hub}
  if [ -z "$pass" ]; then
    echo "error: falta SMTP_PASSWORD en $env_file (make setup lo genera; en un .env viejo agrégala con: openssl rand -hex 24)" >&2
    exit 1
  fi
  printf '%s:%s\n' "$user" "$pass" > "$dir/smtp-auth.txt"
  chmod 600 "$dir/smtp-auth.txt"
  echo "Credenciales SMTP de Mailpit escritas en $dir/smtp-auth.txt"
}

if [ "$force" -eq 0 ] && [ -f "$dir/ca.pem" ] && [ -f "$dir/server.pem" ] && [ -f "$dir/server-key.pem" ]; then
  echo "Los certificados ya existen en $dir (usa --force para regenerarlos)."
  write_auth_file
  exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat > "$work/server.cnf" <<'CNF'
[req]
distinguished_name = dn
prompt = no
[dn]
CN = mailpit
[ext]
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = DNS:mailpit,DNS:localhost,IP:127.0.0.1
CNF

# CA: clave EC P-256, válida 825 días (suficiente para un entorno local).
openssl ecparam -name prime256v1 -genkey -noout -out "$dir/ca-key.pem"
openssl req -x509 -new -key "$dir/ca-key.pem" -sha256 -days 825 \
  -subj "/CN=Identity Hub local SMTP CA" \
  -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -out "$dir/ca.pem"

# Servidor: CSR firmada por la CA con los SAN requeridos.
openssl ecparam -name prime256v1 -genkey -noout -out "$dir/server-key.pem"
openssl req -new -key "$dir/server-key.pem" -config "$work/server.cnf" -out "$work/server.csr"
openssl x509 -req -in "$work/server.csr" -CA "$dir/ca.pem" -CAkey "$dir/ca-key.pem" -CAcreateserial \
  -CAserial "$work/ca.srl" -days 825 -sha256 -extfile "$work/server.cnf" -extensions ext \
  -out "$dir/server.pem" 2>/dev/null

# Las claves quedan 0600 (Mailpit corre como root en su imagen, así que lee
# la suya montada en solo lectura). Los certificados no son secretos: 0644,
# para que el worker (usuario no-root) lea ca.pem.
chmod 600 "$dir/ca-key.pem"
chmod 644 "$dir/ca.pem" "$dir/server.pem"
chmod 600 "$dir/server-key.pem"

echo "Certificados generados en $dir:"
echo "  ca.pem / server.pem / server-key.pem (y ca-key.pem, solo para regenerar)"
write_auth_file
