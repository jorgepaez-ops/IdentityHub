#!/usr/bin/env python3
"""Stdlib-only tests for the traceability scenario-to-E2E correspondence."""
from __future__ import annotations

import importlib.util
import pathlib
import sys
import tempfile
import unittest


# Loading the script as a module would otherwise leave scripts/__pycache__/ behind on
# Linux runners, and the spec-drift job requires a clean working tree.
sys.dont_write_bytecode = True

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
                        "test(`RNF-009 template title`, () => {})",
                    ]
                ),
                encoding="utf-8",
            )
            scenarios = [
                traceability.Scenario("RF-001", "Covered scenario", "sample.feature"),
                traceability.Scenario("RF-002", "Duplicate scenario", "sample.feature"),
                traceability.Scenario("RF-003", "Missing scenario", "sample.feature"),
                traceability.Scenario("RF-001", "Covered scenario", "other.feature", line=7),
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
        # A template-string title is reported, not silently ignored.
        self.assertIn("Títulos RF/RNF no literales (template o calculados), invisibles para este chequeo:", errors)
        self.assertIn("  - scenarios.spec.ts:5", errors)
        # Two scenarios with the same RF tag and name are an error, not one collapsed dict key.
        self.assertIn("Escenarios Gherkin repetidos (misma etiqueta RF y mismo nombre):", errors)
        self.assertIn("  - RF-001 Covered scenario: sample.feature:0, other.feature:7", errors)


class GherkinParserTests(unittest.TestCase):
    def parse(self, feature: str) -> list:
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            acceptance = root / "specs" / "06-acceptance"
            acceptance.mkdir(parents=True)
            (acceptance / "sample.feature").write_text(feature, encoding="utf-8")
            previous_root = traceability.ROOT
            self.addCleanup(setattr, traceability, "ROOT", previous_root)
            traceability.ROOT = root
            return traceability.load_gherkin_scenarios()

    def test_tags_do_not_leak_across_scopes(self) -> None:
        scenarios = self.parse(
            "@RF-100\n"
            "Característica: Sample\n"
            "  Antecedentes:\n"
            "    Dado algo\n"
            "  Escenario: Untagged\n"
            "    Cuando algo\n"
            "  @RF-101 @RNF-002\n"
            "  Escenario: Tagged\n"
            "    Cuando algo\n"
            "  @RF-102\n"
            "  Regla: Nueva\n"
            "  Escenario: After rule\n"
        )

        self.assertEqual([(s.name, s.tags) for s in scenarios], [("Tagged", ("RF-101", "RNF-002"))])
        self.assertEqual(scenarios[0].requirement, "RF-101")

    def test_matrix_and_one_to_one_check_share_the_parse(self) -> None:
        scenarios = self.parse("@RF-101 @RF-102\nEscenario: Shared\n")

        self.assertEqual(traceability.load_scenarios(scenarios)["RF-102"], ["sample.feature — Shared"])
        self.assertEqual(scenarios[0].requirement, "RF-101")


if __name__ == "__main__":
    unittest.main()
