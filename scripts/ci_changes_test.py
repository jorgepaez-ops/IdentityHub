#!/usr/bin/env python3
"""Stdlib-only tests for scripts/ci_changes.sh (job 0 of ci.yml) and a lint of the image job guards."""
from __future__ import annotations

import os
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest

# Avoid leaving scripts/__pycache__/ behind (spec-drift requires a clean tree).
sys.dont_write_bytecode = True

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "ci_changes.sh"
CI_WORKFLOW = ROOT / ".github" / "workflows" / "ci.yml"
GUARD = "needs.changes.outputs.code != 'false'"
ZERO_SHA = "0" * 40


def git(cwd: pathlib.Path, *args: str) -> str:
    env = {
        **os.environ,
        "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
        "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
    }
    out = subprocess.run(["git", *args], cwd=cwd, env=env, check=True, capture_output=True, text=True)
    return out.stdout.strip()


class ClassifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.repo = pathlib.Path(self._tmp.name) / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-q")
        self.write("README.md", "base\n")
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", "base")
        self.base = git(self.repo, "rev-parse", "HEAD")

    def write(self, rel: str, content: str = "x\n") -> None:
        path = self.repo / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def commit(self, *rels: str) -> str:
        for rel in rels:
            self.write(rel, f"change {rel}\n")
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", "change")
        return git(self.repo, "rev-parse", "HEAD")

    def run_script(self, **env: str) -> tuple[str, str]:
        """Runs the classifier and returns (code value, step summary)."""
        out = pathlib.Path(self._tmp.name) / "output"
        summary = pathlib.Path(self._tmp.name) / "summary"
        out.write_text("")
        summary.write_text("")
        full_env = {
            "PATH": os.environ["PATH"], "HOME": os.environ.get("HOME", ""),
            "GITHUB_OUTPUT": str(out), "GITHUB_STEP_SUMMARY": str(summary),
            "EVENT_NAME": "", "PR_BASE_SHA": "", "PR_HEAD_SHA": "", "PUSH_BEFORE": "", "PUSH_SHA": "",
            **env,
        }
        res = subprocess.run(["bash", str(SCRIPT)], cwd=self.repo, env=full_env, capture_output=True, text=True)
        self.assertEqual(res.returncode, 0, res.stderr)
        lines = [line for line in out.read_text().splitlines() if line.startswith("code=")]
        self.assertEqual(len(lines), 1, out.read_text())
        return lines[0].removeprefix("code="), summary.read_text()

    def pr(self, head: str) -> str:
        code, _ = self.run_script(EVENT_NAME="pull_request", PR_BASE_SHA=self.base, PR_HEAD_SHA=head)
        return code

    def test_docs_only_is_not_code(self) -> None:
        head = self.commit("docs/guide.md", "odd/tasks/x.md", "notes.md")
        self.assertEqual(self.pr(head), "false")

    def test_docs_and_backend_is_code(self) -> None:
        head = self.commit("docs/guide.md", "backend/main.go")
        self.assertEqual(self.pr(head), "true")

    def test_markdown_under_governing_paths_is_code(self) -> None:
        for rel in ("specs/spec.md", ".github/PULL_REQUEST_TEMPLATE.md", "scripts/README.md"):
            with self.subTest(path=rel):
                self.setUp()  # fresh repo per path
                head = self.commit(rel)
                self.assertEqual(self.pr(head), "true")

    def test_rename_from_code_to_docs_is_code(self) -> None:
        self.write("backend/a.go")
        git(self.repo, "add", "-A")
        git(self.repo, "commit", "-q", "-m", "add code")
        self.base = git(self.repo, "rev-parse", "HEAD")
        (self.repo / "docs").mkdir()
        git(self.repo, "mv", "backend/a.go", "docs/a.md")
        git(self.repo, "commit", "-q", "-m", "rename")
        # Both source and destination must be listed (--no-renames), so the source counts as code.
        self.assertEqual(self.pr(git(self.repo, "rev-parse", "HEAD")), "true")

    def test_empty_diff_is_code(self) -> None:
        self.assertEqual(self.pr(self.base), "true")

    def test_git_error_is_code(self) -> None:
        code, summary = self.run_script(EVENT_NAME="pull_request", PR_BASE_SHA="deadbeef", PR_HEAD_SHA="cafebabe")
        self.assertEqual(code, "true")
        self.assertIn("git diff falló", summary)

    def test_push_with_zero_before_is_code(self) -> None:
        head = self.commit("docs/guide.md")
        code, _ = self.run_script(EVENT_NAME="push", PUSH_BEFORE=ZERO_SHA, PUSH_SHA=head)
        self.assertEqual(code, "true")

    def test_push_docs_only_is_not_code(self) -> None:
        head = self.commit("docs/guide.md")
        code, _ = self.run_script(EVENT_NAME="push", PUSH_BEFORE=self.base, PUSH_SHA=head)
        self.assertEqual(code, "false")

    def test_workflow_dispatch_is_code(self) -> None:
        self.commit("docs/guide.md")
        code, _ = self.run_script(EVENT_NAME="workflow_dispatch")
        self.assertEqual(code, "true")

    def test_works_without_summary_file(self) -> None:
        head = self.commit("docs/guide.md")
        out = pathlib.Path(self._tmp.name) / "output2"
        out.write_text("")
        env = {"PATH": os.environ["PATH"], "GITHUB_OUTPUT": str(out), "EVENT_NAME": "pull_request",
               "PR_BASE_SHA": self.base, "PR_HEAD_SHA": head, "PUSH_BEFORE": "", "PUSH_SHA": ""}
        res = subprocess.run(["bash", str(SCRIPT)], cwd=self.repo, env=env, capture_output=True, text=True)
        self.assertEqual(res.returncode, 0, res.stderr)
        self.assertIn("code=false", out.read_text())


def image_job_steps(workflow: str) -> list[str]:
    """Returns the raw text of each step of the `image` job (text-based; no YAML dependency)."""
    match = re.search(r"^  image:\n(.*?)(?=^  [a-z0-9-]+:\n|\Z)", workflow, re.S | re.M)
    if match is None:
        raise AssertionError("job image not found in ci.yml")
    body = match.group(1)
    steps_part = body.split("\n    steps:\n", 1)
    if len(steps_part) != 2:
        raise AssertionError("steps of job image not found")
    return [s for s in re.split(r"^      - ", steps_part[1], flags=re.M)[1:]]


class ImageJobGuardTest(unittest.TestCase):
    """The image job always starts (its expanded matrix names are required checks); on
    docs-only changes every step must be skipped by its own `if`, so each step needs the guard."""

    def test_every_image_step_has_the_guard(self) -> None:
        steps = image_job_steps(CI_WORKFLOW.read_text())
        self.assertGreaterEqual(len(steps), 4)
        missing = []
        for step in steps:
            ifs = [line for line in step.splitlines() if re.match(r"^\s*(?:- )?if:", line)]
            if not any(GUARD in line for line in ifs):
                missing.append(step.splitlines()[0])
        self.assertEqual(missing, [], f"steps of job image without the code guard: {missing}")

    def test_lint_detects_missing_guard(self) -> None:
        sample = (
            "  image:\n    runs-on: x\n    steps:\n"
            "      - uses: a/b@1\n        if: needs.changes.outputs.code != 'false'\n\n"
            "      - uses: c/d@1\n\n"
            "  dast:\n    steps:\n      - uses: e/f@1\n"
        )
        steps = image_job_steps(sample)
        self.assertEqual(len(steps), 2)
        self.assertIn(GUARD, steps[0])
        self.assertNotIn(GUARD, steps[1])


if __name__ == "__main__":
    unittest.main()
