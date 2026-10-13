#!/usr/bin/env python3
"""Stdlib-only tests that check, against the real files, the properties of RNF-001, RNF-002, RNF-006 and RNF-008.

Each test class backs one acceptance scenario of specs/06-acceptance/requisitos-no-funcionales.feature.
Compose is rendered with `docker compose config --format json` (no containers are started); workflows
and Dockerfiles are read as text, like scripts/traceability.py (no PyYAML). Every check has a negative
unit test so a regression in the check itself is visible.
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
sys.path.insert(0, str(ROOT / "scripts"))
import traceability  # noqa: E402  (section_block reads a workflow job as text)

COMPOSE_FILE = ROOT / "deploy" / "docker-compose.yml"
CI = ROOT / ".github" / "workflows" / "ci.yml"
RELEASE = ROOT / ".github" / "workflows" / "release.yml"
OWN_SERVICES = ("api", "worker", "web")
CORE_SERVICES = ("db", "broker", "mailpit", "migrate", *OWN_SERVICES)


def render_config(*profiles: str) -> dict:
    """Rendered compose config. Required variables get dummy values; --env-file /dev/null ignores .env."""
    if shutil.which("docker") is None:
        if os.environ.get("CI"):
            raise AssertionError("docker is required in CI")
        raise unittest.SkipTest("docker not installed")
    required = sorted(set(re.findall(r"\$\{([A-Z0-9_]+):\?", COMPOSE_FILE.read_text())))
    env = {"PATH": os.environ["PATH"], "HOME": os.environ.get("HOME", "")}
    env.update({name: "dummy-value" for name in required})
    cmd = ["docker", "compose", "--env-file", "/dev/null", "-f", str(COMPOSE_FILE)]
    for profile in profiles:
        cmd += ["--profile", profile]
    res = subprocess.run([*cmd, "config", "--format", "json"], env=env, capture_output=True, text=True)
    if res.returncode != 0:
        raise AssertionError(f"docker compose config failed: {res.stderr.strip()}")
    return json.loads(res.stdout)


def job_block(workflow: pathlib.Path, job: str) -> list[str]:
    block = traceability.section_block(workflow.read_text(encoding="utf-8").splitlines(), job)
    if block is None:
        raise AssertionError(f"{workflow.name} has no job '{job}'")
    return [line for line in block if not line.lstrip().startswith("#")]


# ── RNF-001 ──────────────────────────────────────────────────────────────────────────────────────
def containerization_problems(config: dict) -> list[str]:
    """Every core service is a container of its own, started without profiles; the three own ones are distinct builds."""
    services = config.get("services", {})
    problems = [f"{name}: missing from the default `docker compose up`" for name in CORE_SERVICES if name not in services]
    for name in CORE_SERVICES:
        if name not in services:
            continue
        service = services[name]
        if not (service.get("image") or service.get("build")):
            problems.append(f"{name}: neither image nor build")
        if service.get("profiles"):
            problems.append(f"{name}: needs a profile, so a plain `docker compose up` would not start it")
    targets = [(services.get(n, {}).get("build") or {}).get("target") for n in OWN_SERVICES]
    if None in targets or len(set(targets)) != len(OWN_SERVICES):
        problems.append(f"api, worker and web must each build their own image target, got {targets}")
    return problems


def dockerfile_stages(path: pathlib.Path) -> dict[str, dict]:
    """stage name -> {"from": base image, "user": last USER or None}."""
    stages: dict[str, dict] = {}
    current = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if m := re.match(r"FROM\s+(\S+)(?:\s+AS\s+(\S+))?", line, re.I):
            current = m.group(2) or f"stage{len(stages)}"
            stages[current] = {"from": m.group(1), "user": None}
        elif current and (m := re.match(r"USER\s+(\S+)", line, re.I)):
            stages[current]["user"] = m.group(1)
    return stages


class Rnf001Containerization(unittest.TestCase):
    def test_flags_missing_profiled_or_shared_services(self) -> None:
        config = {"services": {"db": {"image": "x"}, "api": {"build": {"target": "api"}, "profiles": ["p"]},
                               "worker": {"build": {"target": "api"}}, "web": {}}}
        text = "\n".join(containerization_problems(config))
        for expected in ("broker: missing", "api: needs a profile", "web: neither image nor build", "must each build"):
            self.assertIn(expected, text)

    def test_default_compose_up_runs_every_service_in_its_own_container(self) -> None:
        self.assertEqual(containerization_problems(render_config()), [])

    def test_own_images_build_inside_containers_from_a_multi_stage_dockerfile(self) -> None:
        for dockerfile, target in (("backend/Dockerfile", "builder"), ("frontend/Dockerfile", "builder")):
            self.assertIn(target, dockerfile_stages(ROOT / dockerfile), f"{dockerfile} compiles outside a container")


# ── RNF-002 ──────────────────────────────────────────────────────────────────────────────────────
STAGE_JOBS = {
    "build/test": ("test-unit", "test-integration", "e2e"),
    "SAST": ("sast",),
    "SCA": ("sca",),
    "secrets": ("secrets",),
    "image scan": ("image",),
    "DAST": ("dast",),
}


def pipeline_problems(ci_text: str, job_texts: dict[str, list[str]]) -> list[str]:
    problems = []
    for stage, jobs in STAGE_JOBS.items():
        for job in jobs:
            if not re.search(rf"^  {re.escape(job)}:", ci_text, re.M):
                problems.append(f"stage {stage}: ci.yml has no job '{job}'")
            elif any("continue-on-error: true" in line for line in job_texts.get(job, [])):
                problems.append(f"stage {stage}: job '{job}' has continue-on-error, so it cannot break the build")
    return problems


class Rnf002Pipeline(unittest.TestCase):
    def test_flags_a_missing_stage_and_a_soft_gate(self) -> None:
        problems = pipeline_problems("  sast:\n  sca:\n", {"sast": ["    continue-on-error: true"]})
        text = "\n".join(problems)
        self.assertIn("job 'secrets'", text)
        self.assertIn("job 'sast' has continue-on-error", text)

    def test_every_stage_exists_and_every_gate_breaks_the_build(self) -> None:
        jobs = {job: job_block(CI, job) for names in STAGE_JOBS.values() for job in names}
        self.assertEqual(pipeline_problems(CI.read_text(encoding="utf-8"), jobs), [])

    def test_the_image_gate_fails_on_findings_and_the_release_builds_sbom_signs_and_publishes(self) -> None:
        self.assertTrue(any('exit-code: "1"' in line for line in job_block(CI, "image")))
        publish = "\n".join(job_block(RELEASE, "publish"))
        for needle in ("format: cyclonedx-json", "cosign sign --yes", "docker push", "cosign attest --yes --type cyclonedx"):
            self.assertIn(needle, publish)
        self.assertTrue(any("exit-code" in line and "1" in line for line in job_block(RELEASE, "gate")))


# ── RNF-006 ──────────────────────────────────────────────────────────────────────────────────────
def provenance_problems(publish: list[str]) -> list[str]:
    """sign -> attest -> verify -> verify-attestation -> the only push of latest, in that order."""
    wanted = ["cosign sign --yes", "cosign attest --yes --type cyclonedx", "cosign verify ", "cosign verify-attestation",
              '"${IMAGE}:latest"']
    positions = []
    for needle in wanted:
        hits = [i for i, line in enumerate(publish) if needle in line and "docker tag" not in line and "echo" not in line]
        if not hits:
            return [f"missing step: {needle}"]
        positions.append(hits[0])
    problems = [] if positions == sorted(positions) and len(set(positions)) == len(positions) else [
        f"wrong order {dict(zip(wanted, positions))}"]
    for i, line in enumerate(publish):
        if "cosign verify" in line and "--certificate-identity" not in "\n".join(publish[i : i + 2]):
            problems.append(f"verification does not pin the signer identity: {line.strip()}")
    return problems


class Rnf006Provenance(unittest.TestCase):
    def test_flags_latest_moved_before_verification(self) -> None:
        lines = ['cosign sign --yes x', 'cosign attest --yes --type cyclonedx x', 'docker push "${IMAGE}:latest"',
                 'cosign verify x --certificate-identity-regexp y', 'cosign verify-attestation x --certificate-identity-regexp y']
        self.assertTrue(provenance_problems(lines))
        self.assertTrue(provenance_problems(lines[:2]))

    def test_release_signs_attests_and_verifies_before_moving_latest(self) -> None:
        self.assertEqual(provenance_problems(job_block(RELEASE, "publish")), [])

    def test_the_sbom_is_cyclonedx_and_signing_is_keyless(self) -> None:
        publish = "\n".join(job_block(RELEASE, "publish"))
        self.assertIn("format: cyclonedx-json", publish)
        self.assertNotIn("--key", publish)  # keyless: no private key handled by the workflow
        self.assertIn("id-token: write", publish)  # OIDC identity for the Sigstore certificate


# ── RNF-008 ──────────────────────────────────────────────────────────────────────────────────────
def hardening_problems(config: dict, stages_by_target: dict[str, dict]) -> list[str]:
    problems = []
    for name in OWN_SERVICES:
        service = config.get("services", {}).get(name)
        if service is None:
            problems.append(f"{name}: not in the rendered config")
            continue
        if service.get("read_only") is not True:
            problems.append(f"{name}: root filesystem is not read-only")
        if [c.upper() for c in service.get("cap_drop") or []] != ["ALL"]:
            problems.append(f"{name}: cap_drop is {service.get('cap_drop')}, expected ['ALL']")
        if "no-new-privileges:true" not in (service.get("security_opt") or []):
            problems.append(f"{name}: no-new-privileges is not set")
        user = (stages_by_target.get(name) or {}).get("user")
        if user is None or user.split(":")[0] in ("0", "root"):
            problems.append(f"{name}: final image stage runs as {user!r}")
    return problems


def final_stage_users() -> dict[str, dict]:
    config = render_config()
    result = {}
    for name in OWN_SERVICES:
        build = config["services"][name]["build"]
        stages = dockerfile_stages(pathlib.Path(build["context"]) / build.get("dockerfile", "Dockerfile"))
        result[name] = stages.get(build["target"], {})
    return result


class Rnf008Hardening(unittest.TestCase):
    def test_flags_each_missing_control(self) -> None:
        config = {"services": {"api": {"read_only": True, "cap_drop": ["ALL"], "security_opt": ["no-new-privileges:true"]},
                               "worker": {"cap_drop": ["NET_RAW"]}}}
        problems = "\n".join(hardening_problems(config, {"api": {"user": "0"}, "worker": {"user": "65532:65532"}}))
        for expected in ("api: final image stage runs as '0'", "worker: root filesystem", "worker: cap_drop", "worker: no-new-privileges",
                         "web: not in the rendered config"):
            self.assertIn(expected, problems)

    def test_own_services_drop_privileges_in_compose_and_run_as_non_root(self) -> None:
        self.assertEqual(hardening_problems(render_config(), final_stage_users()), [])

    def test_every_base_image_is_pinned_by_digest(self) -> None:
        for dockerfile in ("backend/Dockerfile", "frontend/Dockerfile"):
            for stage, info in dockerfile_stages(ROOT / dockerfile).items():
                if info["from"] in dockerfile_stages(ROOT / dockerfile):
                    continue  # FROM <previous stage>
                self.assertRegex(info["from"], r"@sha256:[0-9a-f]{64}$", f"{dockerfile} stage {stage}")


if __name__ == "__main__":
    unittest.main()
