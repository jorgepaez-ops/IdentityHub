#!/usr/bin/env python3
"""Stdlib-only test of scripts/load-test.sh: the production guard and the exit status of the cleanup trap.

docker is replaced by a stub placed first on PATH, so no stack, k6 image or database is needed. The stub
appends every call to a log, records the SQL it receives on stdin, and its "run" (k6) behavior is chosen
with STUB_RUN: "ok", "fail" (exit 7) or "slow". The "slow" stub touches STUB_STARTED and then blocks, polling every
50 ms for up to 30 s, until the test creates STUB_RELEASE, and then exits 0. The interrupt tests create STUB_RELEASE
only after sending the signal, so the signal always arrives while the k6 stub is running, with no timing window.
"""
from __future__ import annotations

import os
import pathlib
import signal
import subprocess
import sys
import tempfile
import time
import unittest

# Avoid leaving scripts/__pycache__/ behind (spec-drift requires a clean tree).
sys.dont_write_bytecode = True

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "load-test.sh"
POLL_INTERVAL_SECONDS = 0.05  # how often the "slow" k6 stub (and the test) poll for a file
MAX_POLLS = 600  # 600 polls x 0.05 s = 30 s: ceiling for the stub waiting on STUB_RELEASE and for the test waiting on STUB_STARTED
# Exit status of the "slow" stub when STUB_RELEASE never appeared within MAX_POLLS: the test never released it.
STUB_NEVER_RELEASED = 99
STUB = rf"""#!/usr/bin/env bash
echo "docker $*" >> "$STUB_LOG"
case " $* " in
  *" exec "*) cat >> "$STUB_LOG"; exit 0 ;;
esac
if [ "$1" = "run" ]; then
  case "$STUB_RUN" in
    fail) exit 7 ;;
    slow)
      touch "$STUB_STARTED"
      for _ in $(seq {MAX_POLLS}); do [ -e "$STUB_RELEASE" ] && exit 0; sleep {POLL_INTERVAL_SECONDS}; done
      exit {STUB_NEVER_RELEASED} ;;
  esac
fi
exit 0
"""


class LoadTestScriptTests(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.tmp = pathlib.Path(tmp.name)
        bin_dir = self.tmp / "bin"
        bin_dir.mkdir()
        (bin_dir / "docker").write_text(STUB, encoding="utf-8")
        (bin_dir / "docker").chmod(0o755)
        self.log = self.tmp / "calls.log"
        self.started = self.tmp / "started"
        self.release = self.tmp / "release"
        self.env = {
            **os.environ,
            "PATH": f"{bin_dir}{os.pathsep}{os.environ['PATH']}",
            "STUB_LOG": str(self.log),
            "STUB_STARTED": str(self.started),
            "STUB_RELEASE": str(self.release),
            "STUB_RUN": "ok",
            "COMPOSE": "docker compose --env-file .env -f deploy/docker-compose.yml",
        }
        self.env.pop("COMPOSE_FILE", None)

    def calls(self) -> str:
        return self.log.read_text(encoding="utf-8") if self.log.exists() else ""

    def run_script(self, **overrides: str) -> subprocess.CompletedProcess[str]:
        env = {**self.env, **overrides}
        return subprocess.run(["bash", str(SCRIPT)], env=env, capture_output=True, text=True, timeout=60)

    def test_refuses_the_simulated_production_compose_command(self) -> None:
        result = self.run_script(COMPOSE="docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml")
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(self.calls(), "")

    def test_refuses_production_selected_through_compose_file(self) -> None:
        result = self.run_script(COMPOSE_FILE="deploy/docker-compose.yml:deploy/docker-compose.prod.yml")
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(self.calls(), "")

    def test_passes_and_disables_the_seeded_account(self) -> None:
        result = self.run_script()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("SET status = 'disabled'", self.calls())

    def test_a_failing_k6_run_keeps_its_status_and_still_cleans_up(self) -> None:
        result = self.run_script(STUB_RUN="fail")
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertIn("SET status = 'disabled'", self.calls())

    def _interrupt(self, sig: signal.Signals) -> tuple[int, str]:
        process = subprocess.Popen(
            ["bash", str(SCRIPT)], env={**self.env, "STUB_RUN": "slow"},
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
        deadline = time.time() + MAX_POLLS * POLL_INTERVAL_SECONDS
        while not self.started.exists() and time.time() < deadline:
            time.sleep(POLL_INTERVAL_SECONDS)
        self.assertTrue(self.started.exists(), "the k6 stub never started")
        process.send_signal(sig)  # only the script: bash runs the trap once the child ends (with status 0)
        self.release.touch()  # let the blocked k6 stub finish, strictly after the signal was delivered
        return process.wait(timeout=30), self.calls()

    def test_an_interrupted_run_exits_non_zero_and_cleans_up(self) -> None:
        status, calls = self._interrupt(signal.SIGINT)
        self.assertEqual(status, 130)
        self.assertIn("SET status = 'disabled'", calls)

    def test_a_terminated_run_exits_non_zero_and_cleans_up(self) -> None:
        status, calls = self._interrupt(signal.SIGTERM)
        self.assertEqual(status, 143)
        self.assertIn("SET status = 'disabled'", calls)


if __name__ == "__main__":
    unittest.main()
