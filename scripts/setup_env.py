#!/usr/bin/env python3
"""Genera un .env local con secretos aleatorios a partir de .env.example.

Los secretos nunca se versionan (AM-012): se generan en la maquina del desarrollador.
Solo usa la biblioteca estandar. Uso:

    python3 scripts/setup_env.py [--template .env.example] [--output .env] [--force]
"""
from __future__ import annotations

import argparse
import base64
import os
import pathlib
import secrets

PASSWORD_KEYS = (
    "POSTGRES_PASSWORD",
    "IDENTITY_APP_PASSWORD",
    "RABBITMQ_DEFAULT_PASS",
    "GRAFANA_ADMIN_PASSWORD",
)
JWT_KEY = "JWT_SIGNING_KEY"
SECRET_KEYS = (*PASSWORD_KEYS, JWT_KEY)
# Non-secret values the template leaves empty but deploy/docker-compose.yml requires (":?").
# Same value as the CI stack (.github/actions/stack-up): the compose network, where Nginx
# reaches the API from. It is the Docker network of deploy/docker-compose.yml, not a host
# address, hence the Sonar suppression (S1313).
CONFIG_DEFAULTS = {"TRUSTED_PROXIES": "172.28.0.0/16"}  # NOSONAR


def new_secret(key: str) -> str:
    if key == JWT_KEY:
        return base64.b64encode(secrets.token_bytes(32)).decode("ascii")
    return secrets.token_hex(24)


def split_assignment(line: str) -> tuple[str, str] | None:
    if not line.strip() or line.lstrip().startswith("#") or "=" not in line:
        return None
    key, value = line.split("=", 1)
    return key.strip(), value


def check_template(lines: list[str]) -> None:
    keys = {parsed[0] for parsed in map(split_assignment, lines) if parsed}
    missing = [key for key in SECRET_KEYS if key not in keys]
    if missing:
        raise SystemExit(f"error: la plantilla no define la clave secreta: {', '.join(missing)}")


def render(lines: list[str], new_values: dict[str, str]) -> list[str]:
    # The template references secrets as ${VAR} (URLs); Compose interpolates them from
    # this same file, so only the secret lines themselves get a value.
    rendered = []
    for line in lines:
        parsed = split_assignment(line)
        if parsed is None:
            rendered.append(line)
            continue
        key, value = parsed
        if key in new_values:
            rendered.append(f"{key}={new_values[key]}")
        elif key in CONFIG_DEFAULTS and not value.split("#", 1)[0].strip():
            rendered.append(f"{key}={CONFIG_DEFAULTS[key]}")
        else:
            rendered.append(line)
    return rendered


def write_private(path: pathlib.Path, content: str) -> None:
    # Write a fresh 0600 file next to the target and swap it in atomically: an existing .env is
    # never truncated, and the secrets never sit in a file with looser permissions.
    tmp = path.with_name(f".{path.name}.{secrets.token_hex(4)}.tmp")
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(content)
        os.replace(tmp, path)
    except BaseException:
        tmp.unlink(missing_ok=True)
        raise


def inside_root(raw: str, root: pathlib.Path, option: str) -> pathlib.Path:
    # Las rutas relativas se toman desde la raiz (no desde el directorio de trabajo), se
    # resuelven (symlinks y "..") y se rechaza cualquiera fuera de ella.
    path = pathlib.Path(raw)
    path = (path if path.is_absolute() else root / path).resolve()
    if not path.is_relative_to(root):
        raise SystemExit(f"error: {option} debe estar dentro de {root}; se rechaza {path}")
    return path


def main(argv: list[str] | None = None, *, root: pathlib.Path | None = None) -> None:
    # Los errores salen con SystemExit (código distinto de 0); terminar sin excepción es éxito.
    # root solo se cambia desde las pruebas; por defecto es la raiz del repositorio.
    root = (root or pathlib.Path(__file__).resolve().parent.parent).resolve()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--template", default=".env.example")
    parser.add_argument("--output", default=".env")
    parser.add_argument("--force", action="store_true", help="sobrescribe el archivo de salida")
    args = parser.parse_args(argv)

    template = inside_root(args.template, root, "--template")
    output = inside_root(args.output, root, "--output")
    if output.exists() and not args.force:
        print(
            f"{output} ya existe: no se modifica. --force lo regenera con secretos nuevos, pero"
            " Postgres y RabbitMQ conservan las contraseñas de sus volúmenes: tras --force hay que"
            " borrar los volúmenes (make clean, borra los datos) o el stack no podrá autenticarse."
        )
        return

    lines = template.read_text(encoding="utf-8").splitlines()
    check_template(lines)
    new_values = {key: new_secret(key) for key in SECRET_KEYS}
    write_private(output, "\n".join(render(lines, new_values)) + "\n")

    print(f"{output} generado (modo 0600) con secretos aleatorios para:")
    for key in SECRET_KEYS:
        print(f"  - {key}")


if __name__ == "__main__":
    main()
