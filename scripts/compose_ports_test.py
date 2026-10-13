#!/usr/bin/env python3
"""Stdlib-only test: Prometheus, Loki and Grafana (profile observability) publish ports on loopback only.

Parses `docker compose config --format json`. Required variables get dummy values taken from the
compose file itself and --env-file /dev/null keeps Compose from reading a real .env.
"""
from __future__ import annotations

import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import unittest

# Avoid leaving scripts/__pycache__/ behind (spec-drift requires a clean tree).
sys.dont_write_bytecode = True

ROOT = pathlib.Path(__file__).resolve().parent.parent
COMPOSE_FILE = ROOT / "deploy" / "docker-compose.yml"
GUARDED = ("prometheus", "loki", "grafana")  # no authentication (Prometheus, Loki) or admin UI (Grafana)
LOOPBACK = "127.0.0.1"


def unsafe_ports(config: dict, services: tuple[str, ...] = GUARDED) -> list[str]:
    """Returns one message per guarded service that is missing or publishes a port outside loopback."""
    problems: list[str] = []
    for name in services:
        service = config.get("services", {}).get(name)
        if service is None:
            problems.append(f"{name}: service not found in the rendered config")
            continue
        ports = service.get("ports") or []
        if not ports:
            problems.append(f"{name}: no published port (the test would not protect anything)")
        for port in ports:
            if port.get("host_ip") != LOOPBACK:
                problems.append(f"{name}: port {port.get('published')} published with host_ip={port.get('host_ip')!r}")
    return problems


def render_config() -> dict:
    required = sorted(set(re.findall(r"\$\{([A-Z0-9_]+):\?", COMPOSE_FILE.read_text())))
    env = {"PATH": os.environ["PATH"], "HOME": os.environ.get("HOME", "")}
    env.update({name: "dummy-value" for name in required})
    cmd = ["docker", "compose", "--env-file", "/dev/null", "-f", str(COMPOSE_FILE),
           "--profile", "observability", "config", "--format", "json"]
    res = subprocess.run(cmd, env=env, capture_output=True, text=True)
    if res.returncode != 0:
        raise AssertionError(f"docker compose config failed: {res.stderr.strip()}")
    return json.loads(res.stdout)


class UnsafePortsTest(unittest.TestCase):
    def test_flags_missing_host_ip(self) -> None:
        config = {"services": {
            "prometheus": {"ports": [{"published": "9090", "target": 9090}]},
            "loki": {"ports": [{"published": "3100", "target": 3100, "host_ip": "0.0.0.0"}]},
            "grafana": {"ports": [{"published": "3000", "target": 3000, "host_ip": "127.0.0.1"}]},
        }}
        problems = unsafe_ports(config)
        self.assertEqual(len(problems), 2, problems)
        self.assertTrue(problems[0].startswith("prometheus"))
        self.assertTrue(problems[1].startswith("loki"))

    def test_flags_missing_service_or_ports(self) -> None:
        problems = unsafe_ports({"services": {"prometheus": {}, "loki": {"ports": []}}})
        self.assertEqual(len(problems), 3, problems)

    def test_accepts_loopback(self) -> None:
        config = {"services": {n: {"ports": [{"published": "1", "host_ip": LOOPBACK}]} for n in GUARDED}}
        self.assertEqual(unsafe_ports(config), [])


class ComposeConfigTest(unittest.TestCase):
    def test_observability_ports_are_loopback_only(self) -> None:
        if shutil.which("docker") is None:
            if os.environ.get("CI"):
                self.fail("docker is required in CI")
            self.skipTest("docker not installed")
        self.assertEqual(unsafe_ports(render_config()), [])


if __name__ == "__main__":
    unittest.main()
