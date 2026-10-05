#!/usr/bin/env python3
"""Stdlib-only tests for scripts/setup_env.py (random local secrets for .env)."""
from __future__ import annotations

import base64
import importlib.util
import os
import pathlib
import stat
import sys
import tempfile
import unittest


# Avoid leaving scripts/__pycache__/ behind (spec-drift requires a clean tree).
sys.dont_write_bytecode = True

SCRIPT = pathlib.Path(__file__).with_name("setup_env.py")
SPEC = importlib.util.spec_from_file_location("setup_env", SCRIPT)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError("could not load setup_env.py")
setup_env = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(setup_env)

# Mirrors the real template: empty secrets, inline comments, ${VAR} references and an
# empty TRUSTED_PROXIES (fake values only).
FIXTURE = """# Fixture template (fake placeholders only)
POSTGRES_USER=identity
POSTGRES_PASSWORD=          # obligatorio, sin valor por defecto
POSTGRES_DB=identity

# app role
IDENTITY_APP_PASSWORD=      # obligatorio
MIGRATE_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable
DATABASE_URL=postgres://identity_app:${IDENTITY_APP_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable
RABBITMQ_DEFAULT_USER=identity
RABBITMQ_DEFAULT_PASS=      # obligatorio
RABBITMQ_URL=amqp://${RABBITMQ_DEFAULT_USER}:${RABBITMQ_DEFAULT_PASS}@broker:5672/
JWT_SIGNING_KEY=
# proxies (a comment here also keeps the gitleaks rule from reading the next line as a value)
TRUSTED_PROXIES=
LOG_LEVEL=info
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=
"""


def parse(path: pathlib.Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in path.read_text().splitlines():
        if line and not line.startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            values[key] = value
    return values


class SetupEnvTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = pathlib.Path(self._tmp.name)
        self.template = self.root / ".env.example"
        self.output = self.root / ".env"
        self.template.write_text(FIXTURE)

    def run_setup(self, *extra: str, template: pathlib.Path | None = None) -> None:
        argv = ["--template", str(template or self.template), "--output", str(self.output), *extra]
        return setup_env.main(argv, root=self.root)

    def test_secrets_are_filled(self) -> None:
        self.run_setup()
        out = parse(self.output)
        for key in ("POSTGRES_PASSWORD", "IDENTITY_APP_PASSWORD", "RABBITMQ_DEFAULT_PASS", "GRAFANA_ADMIN_PASSWORD"):
            self.assertRegex(out[key], r"^[0-9a-f]{48}$")
        self.assertNotIn("obligatorio", self.output.read_text())

    def test_references_are_left_for_compose_to_interpolate(self) -> None:
        self.run_setup()
        out = parse(self.output)
        self.assertIn("${POSTGRES_PASSWORD}", out["MIGRATE_DATABASE_URL"])
        self.assertIn("${IDENTITY_APP_PASSWORD}", out["DATABASE_URL"])
        self.assertIn("${RABBITMQ_DEFAULT_PASS}", out["RABBITMQ_URL"])

    def test_empty_trusted_proxies_gets_the_compose_subnet(self) -> None:
        self.run_setup()
        self.assertEqual(parse(self.output)["TRUSTED_PROXIES"], "172.28.0.0/16")

    def test_explicit_trusted_proxies_is_kept(self) -> None:
        custom = self.root / "custom.example"
        custom.write_text(FIXTURE.replace("TRUSTED_PROXIES=\n", "TRUSTED_PROXIES=10.0.0.0/8\n"))
        self.run_setup(template=custom)
        self.assertEqual(parse(self.output)["TRUSTED_PROXIES"], "10.0.0.0/8")

    def test_jwt_key_is_base64_32_bytes(self) -> None:
        self.run_setup()
        key = parse(self.output)["JWT_SIGNING_KEY"]
        self.assertEqual(len(base64.b64decode(key, validate=True)), 32)

    def test_comments_blank_lines_and_order_preserved(self) -> None:
        self.run_setup()
        template_lines = FIXTURE.splitlines()
        output_lines = self.output.read_text().splitlines()
        self.assertEqual(len(template_lines), len(output_lines))
        for before, after in zip(template_lines, output_lines):
            if not before or before.startswith("#"):
                self.assertEqual(before, after)
            else:
                self.assertEqual(before.split("=", 1)[0], after.split("=", 1)[0])
        self.assertEqual(parse(self.output)["LOG_LEVEL"], "info")

    def test_existing_output_not_overwritten_without_force(self) -> None:
        self.output.write_text("KEEP=me\n")
        self.run_setup()
        self.assertEqual(self.output.read_text(), "KEEP=me\n")

    def test_force_overwrites(self) -> None:
        self.output.write_text("KEEP=me\n")
        self.run_setup("--force")
        self.assertIn("POSTGRES_PASSWORD=", self.output.read_text())
        self.assertNotIn("KEEP=me", self.output.read_text())

    def test_missing_secret_key_fails(self) -> None:
        broken = self.root / "broken.example"
        broken.write_text(FIXTURE.replace("GRAFANA_ADMIN_PASSWORD=\n", ""))
        with self.assertRaises(SystemExit) as ctx:
            self.run_setup(template=broken)
        self.assertIn("GRAFANA_ADMIN_PASSWORD", str(ctx.exception))
        self.assertNotEqual(ctx.exception.code, 0)
        self.assertFalse(self.output.exists())

    def test_output_outside_the_root_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as other:
            outside = pathlib.Path(other) / ".env"
            argv = ["--template", str(self.template), "--output", str(outside)]
            with self.assertRaises(SystemExit) as ctx:
                setup_env.main(argv, root=self.root)
            self.assertNotEqual(ctx.exception.code, 0)
            self.assertFalse(outside.exists())

    def test_template_outside_the_root_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as other:
            outside = pathlib.Path(other) / ".env.example"
            outside.write_text(FIXTURE)
            argv = ["--template", str(outside), "--output", str(self.output)]
            with self.assertRaises(SystemExit) as ctx:
                setup_env.main(argv, root=self.root)
            self.assertNotEqual(ctx.exception.code, 0)
            self.assertFalse(self.output.exists())

    def test_force_tightens_a_permissive_existing_file_before_writing(self) -> None:
        self.output.write_text("OLD=1\n")
        self.output.chmod(0o644)
        self.run_setup("--force")
        self.assertEqual(stat.S_IMODE(self.output.stat().st_mode), 0o600)

    def test_failed_replace_keeps_the_existing_file_and_leaves_no_temp(self) -> None:
        self.output.write_text("KEEP=me\n")
        original = setup_env.os.replace
        def broken_replace(*_args: object) -> None:
            raise OSError("simulated failure")
        setup_env.os.replace = broken_replace
        self.addCleanup(setattr, setup_env.os, "replace", original)
        with self.assertRaises(OSError):
            self.run_setup("--force")
        self.assertEqual(self.output.read_text(), "KEEP=me\n")
        self.assertEqual(sorted(p.name for p in self.root.iterdir()), [".env", ".env.example"])

    def test_relative_paths_resolve_from_root_not_cwd(self) -> None:
        elsewhere = tempfile.TemporaryDirectory()
        self.addCleanup(elsewhere.cleanup)
        previous = os.getcwd()
        os.chdir(elsewhere.name)
        self.addCleanup(os.chdir, previous)
        setup_env.main(["--template", ".env.example", "--output", ".env"], root=self.root)
        self.assertTrue(self.output.exists())

    def test_dotdot_escaping_root_is_rejected(self) -> None:
        with self.assertRaises(SystemExit):
            setup_env.main(["--output", "../escape.env"], root=self.root)

    def test_output_mode_is_0600(self) -> None:
        self.run_setup()
        self.assertEqual(stat.S_IMODE(self.output.stat().st_mode), 0o600)

    def test_two_runs_differ(self) -> None:
        self.run_setup()
        first = parse(self.output)
        self.run_setup("--force")
        second = parse(self.output)
        for key in setup_env.SECRET_KEYS:
            self.assertNotEqual(first[key], second[key])


if __name__ == "__main__":
    unittest.main()
