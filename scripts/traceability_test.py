#!/usr/bin/env python3
"""Stdlib-only tests for the traceability scenario-to-E2E correspondence."""
from __future__ import annotations

import importlib.util
import pathlib
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("traceability.py")
SPEC = importlib.util.spec_from_file_location("traceability", SCRIPT)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError("could not load traceability.py")
traceability = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(traceability)


class E2EScenarioCorrespondenceTests(unittest.TestCase):
    def test_reports_missing_orphan_and_duplicate_titles(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            tests = root / "e2e" / "tests"
            tests.mkdir(parents=True)
            (tests / "scenarios.spec.ts").write_text(
                "\n".join(
                    [
                        "test('RF-001 Covered scenario', () => {})",
                        "test('RF-002 Duplicate scenario', () => {})",
                        "test('RF-002 Duplicate scenario', () => {})",
                        "test('RF-999 Orphan scenario', () => {})",
                        "test(`RNF-009 ignored template title`, () => {})",
                    ]
                ),
                encoding="utf-8",
            )
            scenarios = [
                traceability.Scenario("RF-001", "Covered scenario", "sample.feature"),
                traceability.Scenario("RF-002", "Duplicate scenario", "sample.feature"),
                traceability.Scenario("RF-003", "Missing scenario", "sample.feature"),
            ]
            previous_root = traceability.ROOT
            self.addCleanup(setattr, traceability, "ROOT", previous_root)
            traceability.ROOT = root

            errors = traceability.e2e_correspondence_errors(scenarios)

        self.assertIn("Faltan pruebas E2E para escenarios:", errors)
        self.assertIn("  - RF-003 Missing scenario (sample.feature)", errors)
        self.assertIn("Títulos E2E con prefijo RF sin escenario:", errors)
        self.assertIn("  - scenarios.spec.ts:4: RF-999 Orphan scenario", errors)
        self.assertIn("Escenarios con más de una prueba E2E:", errors)
        self.assertIn("  - RF-002 Duplicate scenario (sample.feature): scenarios.spec.ts:2, scenarios.spec.ts:3", errors)


if __name__ == "__main__":
    unittest.main()
