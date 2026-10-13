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


class DeferredRequirementTests(unittest.TestCase):
    def test_rf018_and_rf019_are_rendered_as_deferred_with_planned_date(self) -> None:
        expected = "Planificado para el 2026-10-13 (decisión del usuario del 2026-10-12)."
        self.assertEqual(traceability.DEFERRED["RF-018"], expected)
        self.assertEqual(traceability.DEFERRED["RF-019"], expected)

        requirements = {
            "RF-018": ("Claves de servicio", "P2"),
            "RF-019": ("Exportación del audit log", "P2"),
        }
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            previous_root = traceability.ROOT
            previous_requirements = traceability.load_requirements
            previous_operations = traceability.load_operations
            previous_scenarios = traceability.load_scenarios
            previous_e2e_titles = traceability.load_e2e_test_titles
            previous_controls = traceability.load_controls
            self.addCleanup(setattr, traceability, "ROOT", previous_root)
            self.addCleanup(setattr, traceability, "load_requirements", previous_requirements)
            self.addCleanup(setattr, traceability, "load_operations", previous_operations)
            self.addCleanup(setattr, traceability, "load_scenarios", previous_scenarios)
            self.addCleanup(setattr, traceability, "load_e2e_test_titles", previous_e2e_titles)
            self.addCleanup(setattr, traceability, "load_controls", previous_controls)
            traceability.ROOT = root
            traceability.load_requirements = lambda: requirements
            traceability.load_operations = lambda: {}
            traceability.load_scenarios = lambda scenarios: {}
            traceability.load_e2e_test_titles = lambda: {}
            traceability.load_controls = lambda: {}

            rendered = traceability.render([])

        self.assertIn("| **RF-018** | Claves de servicio | P2 | — | — | — | — | — | ⏳ diferido (2026-10-13) |", rendered)
        self.assertIn("| **RF-019** | Exportación del audit log | P2 | — | — | — | — | — | ⏳ diferido (2026-10-13) |", rendered)
        self.assertIn("**Resumen:** 2 requisitos · 0 completos · 0 parciales · 0 sin cubrir · 2 diferidos (2026-10-13).", rendered)
        self.assertIn("*diferido (2026-10-13)* = planificado para el 2026-10-13 por decisión del usuario (2026-10-12).", rendered)


class ControlsTests(unittest.TestCase):
    """The fourth evidence column: CI/pipeline controls declared in specs/07-controls.yaml."""

    def in_tree(self, files: dict[str, str]) -> None:
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        root = pathlib.Path(directory.name)
        for name, content in files.items():
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8")
        previous_root = traceability.ROOT
        self.addCleanup(setattr, traceability, "ROOT", previous_root)
        traceability.ROOT = root

    WORKFLOW = (
        "jobs:\n"
        "  secrets:\n"
        "    steps:\n"
        "      - uses: gitleaks/gitleaks-action@v3\n"
        "  lint:\n"
        "    steps:\n"
        "      - run: golangci-lint\n"
    )

    def test_parses_the_declarative_mapping(self) -> None:
        self.in_tree(
            {
                "specs/07-controls.yaml": "# comment\n"
                "RNF-003:\n"
                "  - .github/workflows/ci.yml | secrets | gitleaks-action\n"
                "RNF-008:\n"
                "  - backend/Dockerfile | - | USER 65532\n"
            }
        )

        controls = traceability.load_controls()

        self.assertEqual(
            controls,
            {
                "RNF-003": [traceability.Control(".github/workflows/ci.yml", "secrets", "gitleaks-action")],
                "RNF-008": [traceability.Control("backend/Dockerfile", "", "USER 65532")],
            },
        )

    def test_verified_controls_produce_no_errors(self) -> None:
        self.in_tree({".github/workflows/ci.yml": self.WORKFLOW, "backend/Dockerfile": "USER 65532\n"})
        controls = {
            "RNF-003": [traceability.Control(".github/workflows/ci.yml", "secrets", "gitleaks-action")],
            "RNF-008": [traceability.Control("backend/Dockerfile", "", "USER 65532")],
        }

        self.assertEqual(traceability.control_errors(controls, {"RNF-003": ("x", "P0"), "RNF-008": ("y", "P0")}), [])

    def test_missing_job_text_file_or_requirement_fail_the_check(self) -> None:
        self.in_tree({".github/workflows/ci.yml": self.WORKFLOW})
        ci = ".github/workflows/ci.yml"
        controls = {
            "RNF-003": [
                traceability.Control(ci, "nonexistent-job", "gitleaks-action"),
                # The text exists in another job, not in the declared one: still unverifiable.
                traceability.Control(ci, "lint", "gitleaks-action"),
                traceability.Control("no/such/file.yml", "secrets", "x"),
            ],
            "RNF-777": [traceability.Control(ci, "secrets", "gitleaks-action")],
        }

        errors = "\n".join(traceability.control_errors(controls, {"RNF-003": ("x", "P0")}))

        self.assertIn("RNF-003", errors)
        self.assertIn("nonexistent-job", errors)
        self.assertIn("lint", errors)
        self.assertIn("no/such/file.yml", errors)
        self.assertIn("RNF-777", errors)

    def test_text_found_only_in_a_comment_does_not_verify_the_control(self) -> None:
        workflow = "jobs:\n  secrets:\n    steps:\n      # TODO: gitleaks-action\n      - run: echo none\n"
        self.in_tree({".github/workflows/ci.yml": workflow})
        controls = {"RNF-003": [traceability.Control(".github/workflows/ci.yml", "secrets", "gitleaks-action")]}

        errors = traceability.control_errors(controls, {"RNF-003": ("x", "P0")})

        self.assertEqual(len(errors), 1)
        self.assertIn("gitleaks-action", errors[0])

    def render_with(self, controls: dict, e2e: dict, scenarios: dict) -> str:
        requirements = {"RNF-001": ("Uno", "P0"), "RNF-002": ("Dos", "P0"), "RNF-003": ("Tres", "P0")}
        for name, value in {
            "load_requirements": lambda: requirements,
            "load_operations": lambda: {},
            "load_scenarios": lambda _: scenarios,
            "load_e2e_test_titles": lambda: e2e,
            "load_controls": lambda: controls,
        }.items():
            self.addCleanup(setattr, traceability, name, getattr(traceability, name))
            setattr(traceability, name, value)
        self.in_tree({})
        return traceability.render([])

    def test_controls_column_counts_as_one_evidence_source(self) -> None:
        ci = ".github/workflows/ci.yml"
        controls = {
            "RNF-001": [traceability.Control(ci, "e2e", "a")],
            "RNF-002": [traceability.Control(ci, "sast", "a"), traceability.Control(".github/workflows/release.yml", "publish", "b")],
            "RNF-003": [traceability.Control(ci, "secrets", "a")],
        }
        rendered = self.render_with(controls, {}, {"RNF-003": ["sample.feature — x"]})

        self.assertIn("Controles CI", rendered)
        # Controls alone: one source -> parcial; two refs in one column are still one source.
        self.assertIn("| **RNF-001** | Uno | P0 | — | — | — | — | `ci.yml#e2e` | 🟡 parcial |", rendered)
        self.assertIn("`ci.yml#sast`, `release.yml#publish`", rendered)
        self.assertIn("| 🟡 parcial |", rendered.split("**RNF-002**")[1].splitlines()[0])
        # A test column plus controls -> completo.
        self.assertIn("| 1 | — | — | `ci.yml#secrets` | ✅ completo |", rendered)

    def test_requirement_without_tests_or_controls_stays_uncovered(self) -> None:
        rendered = self.render_with({}, {}, {})

        self.assertIn("| **RNF-001** | Uno | P0 | — | — | — | — | — | 🔴 sin cubrir |", rendered)
        self.assertIn("**Resumen:** 3 requisitos · 0 completos · 0 parciales · 3 sin cubrir", rendered)
        self.assertIn("cuatro columnas de evidencia", rendered)


if __name__ == "__main__":
    unittest.main()
