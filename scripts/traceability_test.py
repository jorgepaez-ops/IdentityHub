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
REPO_ROOT = SCRIPT.parent.parent
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
    def test_nothing_is_deferred_and_a_deferred_requirement_still_renders(self) -> None:
        # RF-018 and RF-019 are implemented: nothing is deferred any more.
        self.assertEqual(traceability.DEFERRED, {})
        # The generator must still render a deferred requirement if one is added again.
        expected = "Planificado para el 2026-10-13 (decisión del usuario del 2026-10-12)."
        self.addCleanup(setattr, traceability, "DEFERRED", traceability.DEFERRED)
        traceability.DEFERRED = {"RF-018": expected}

        requirements = {
            "RF-018": ("Claves de servicio", "P2"),
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
        self.assertIn("**Resumen:** 1 requisitos · 0 completos · 0 parciales · 0 sin cubrir · 1 diferidos (2026-10-13).", rendered)
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


class NonFunctionalVerificationTests(unittest.TestCase):
    """Every @RNF-only scenario links, via "# Verificación:", to a test that really exists."""

    FEATURE = (
        "Característica: NFR\n"
        "  # Verificación: scripts/x_test.py (RnfOne), job spec-drift de CI\n"
        "  @RNF-001\n"
        "  Escenario: Uno\n"
        '  # Verificación: tests/load/a.js vía "make load-test", paso "Pruebas X" del job e2e de CI\n'
        "  @RNF-002 @p1\n"
        "  Escenario: Dos\n"
    )
    FILES = {
        "scripts/x_test.py": "class RnfOne:\n    pass\n",
        "tests/load/a.js": "export default 1\n",
        ".github/workflows/ci.yml": "jobs:\n  spec-drift:\n    steps:\n  e2e:\n    steps:\n      - name: Pruebas X\n",
        "Makefile": "load-test: ## x\n\t@true\n",
        "e2e/tests/a.spec.ts": "test('RNF-010 Un título', async () => {});\n",
    }

    def errors(self, feature: str | None = None, files: dict[str, str] | None = None) -> list[str]:
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "specs" / "06-acceptance").mkdir(parents=True)
            (root / "specs" / "06-acceptance" / "nfr.feature").write_text(feature or self.FEATURE, encoding="utf-8")
            for name, text in (files or self.FILES).items():
                (root / name).parent.mkdir(parents=True, exist_ok=True)
                (root / name).write_text(text, encoding="utf-8")
            previous_root = traceability.ROOT
            self.addCleanup(setattr, traceability, "ROOT", previous_root)
            traceability.ROOT = root
            return traceability.verification_errors(traceability.load_gherkin_scenarios())

    def test_accepts_references_that_exist(self) -> None:
        self.assertEqual(self.errors(), [])

    def test_reports_a_missing_test_file(self) -> None:
        files = {k: v for k, v in self.FILES.items() if k != "scripts/x_test.py"}
        self.assertIn("scripts/x_test.py", "\n".join(self.errors(files=files)))

    def test_reports_a_missing_test_class_job_step_and_make_target(self) -> None:
        files = dict(self.FILES)
        files["scripts/x_test.py"] = "class Other:\n    pass\n"
        files[".github/workflows/ci.yml"] = "jobs:\n  e2e:\n    steps:\n      - name: Otro\n"
        files["Makefile"] = "other: ## x\n"
        text = "\n".join(self.errors(files=files))
        for expected in ("RnfOne", "job spec-drift", "paso Pruebas X", "make load-test"):
            self.assertIn(expected, text)

    def test_reports_a_scenario_without_a_verification_line_or_without_a_named_test(self) -> None:
        feature = "Característica: NFR\n  @RNF-001\n  Escenario: Sin enlace\n  # Verificación: scripts/x_test.py\n  @RNF-002\n  Escenario: Solo archivo\n"
        text = "\n".join(self.errors(feature))
        self.assertIn("RNF-001 Sin enlace", text)
        self.assertIn("RNF-002 Solo archivo", text)

    def test_a_verification_line_does_not_leak_to_the_next_scenario_and_rf_scenarios_are_exempt(self) -> None:
        feature = (
            "Característica: NFR\n"
            "  # Verificación: scripts/x_test.py (RnfOne)\n"
            "  @RNF-001\n  Escenario: Uno\n"
            "  @RNF-002\n  Escenario: Dos hereda?\n"
            "  @RF-001 @RNF-003\n  Escenario: Con RF\n"
        )
        text = "\n".join(self.errors(feature))
        self.assertIn("RNF-002 Dos hereda?", text)
        self.assertNotIn("Con RF", text)
        self.assertNotIn("RNF-001 Uno", text)

    def test_a_quoted_e2e_title_is_not_read_as_a_ci_step_name(self) -> None:
        feature = self.FEATURE.replace('paso "Pruebas X" del job e2e de CI', 'y e2e "RNF-010 Un título" del job e2e')
        self.assertEqual(self.errors(feature), [])
        missing = self.FEATURE.replace('paso "Pruebas X"', 'e2e "RNF-010 Otro"')
        self.assertIn("título E2E RNF-010 Otro", "\n".join(self.errors(missing)))

    def test_a_bare_quoted_string_is_an_error(self) -> None:
        feature = self.FEATURE.replace('paso "Pruebas X"', 'y "RNF-010 Un título"')
        text = "\n".join(self.errors(feature))
        self.assertIn('sin prefijo: "RNF-010 Un título"', text)
        self.assertIn('paso "…"', text)

    def with_make(self, replacement: str) -> str:
        return self.FEATURE.replace('vía "make load-test"', replacement)

    def test_a_make_target_is_checked_bare_in_backticks_or_in_double_quotes(self) -> None:
        for form in ("vía make {}", "vía `make {}`", 'vía "make {}"'):
            self.assertEqual(self.errors(self.with_make(form.format("load-test"))), [], form)
            text = "\n".join(self.errors(self.with_make(form.format("no-existe"))))
            self.assertIn("make no-existe", text, form)

    def test_prose_make_is_not_allowed_because_a_bare_make_is_a_reference(self) -> None:
        text = "\n".join(self.errors(self.with_make("make sure de que corra")))
        self.assertIn("make sure", text)

    def test_mismatched_make_delimiters_are_an_error(self) -> None:
        for form in ('vía "make load-test`', 'vía `make load-test"', 'vía "make load-test', "vía `make load-test"):
            text = "\n".join(self.errors(self.with_make(form)))
            self.assertIn("comilla sin cerrar o delimitadores distintos", text, form)

    def test_backticks_around_a_path_or_a_job_are_formatting(self) -> None:
        self.assertEqual(traceability.tokenize_verification("`scripts/load-test.sh` vía `job e2e`")[1], [])

    def test_every_real_verification_line_tokenizes_without_problems(self) -> None:
        lines = [
            (path.name, number, line.split("# Verificación:", 1)[1])
            for path in sorted((REPO_ROOT / "specs" / "06-acceptance").glob("*.feature"))
            for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
            if "# Verificación:" in line
        ]
        self.assertTrue(lines)
        for name, number, text in lines:
            self.assertEqual(traceability.tokenize_verification(text)[1], [], f"{name}:{number}")

    def test_the_grammar_copies_match_the_docstring(self) -> None:
        doc = traceability.verification_errors.__doc__
        if doc is None:
            self.skipTest("docstrings removidos (python -OO)")
        bullets = [line.strip() for line in doc.splitlines() if line.strip().startswith("- ")]
        self.assertTrue(bullets)
        for copy in (REPO_ROOT / "specs" / "06-acceptance" / "README.md", REPO_ROOT / "scripts" / "README.md"):
            text = copy.read_text(encoding="utf-8")
            for bullet in bullets:
                self.assertIn(bullet, text, f"{copy.relative_to(REPO_ROOT)}: falta «{bullet[:60]}…»")

    def test_an_e2e_title_resolves_with_any_quote_style_and_test_modifier(self) -> None:
        cases = {
            "It's ok": ('test("RNF-010 It\'s ok", async () => {});', "test(`RNF-010 It's ok`, async () => {});"),
            "Un título": ("test.only('RNF-010 Un título', async () => {});", "test.skip('RNF-010 Un título', async () => {});"),
        }
        for title, declarations in cases.items():
            feature = self.FEATURE.replace('paso "Pruebas X"', f'e2e "RNF-010 {title}"')
            for declaration in declarations:
                files = {**self.FILES, "e2e/tests/a.spec.ts": declaration + "\n"}
                self.assertEqual(self.errors(feature, files), [], declaration)

    def test_unknown_quoted_text_is_an_error_naming_the_accepted_prefixes(self) -> None:
        text = "\n".join(self.errors(self.FEATURE.replace('paso "Pruebas X"', 'y "algo suelto"')))
        for expected in ('sin prefijo: "algo suelto"', "paso", "e2e", "make"):
            self.assertIn(expected, text)

    def test_a_job_id_followed_by_punctuation_resolves(self) -> None:
        for suffix in (",", ".", ")", ";"):
            feature = self.FEATURE.replace("job spec-drift de CI", f"job spec-drift{suffix} de CI")
            self.assertEqual(self.errors(feature), [], suffix)

    def test_a_double_quoted_missing_make_target_is_reported(self) -> None:
        feature = self.FEATURE.replace('vía "make load-test"', "vía \"make no-existe\"")
        self.assertIn("make no-existe", "\n".join(self.errors(feature)))

    def test_a_partial_class_name_does_not_match(self) -> None:
        files = dict(self.FILES)
        files["scripts/x_test.py"] = "class RnfOneExtended:\n    pass\n"
        self.assertIn("RnfOne no está definido", "\n".join(self.errors(files=files)))
        files["scripts/x_test.py"] = "x = 'RnfOneExtended'\n"
        self.assertIn("RnfOne no está definido", "\n".join(self.errors(files=files)))
        files["scripts/x_test.py"] = "x = 'RnfOne'\n"
        self.assertEqual(self.errors(files=files), [])

    def test_a_scenario_with_no_named_reference_lists_every_accepted_form(self) -> None:
        feature = "Característica: NFR\n  # Verificación: scripts/x_test.py\n  @RNF-001\n  Escenario: Solo archivo\n"
        text = "\n".join(self.errors(feature))
        for form in ("clase/función", "job", "paso", "e2e", "make"):
            self.assertIn(form, text)

    def test_a_partial_step_name_does_not_match(self) -> None:
        files = dict(self.FILES)
        files[".github/workflows/ci.yml"] = "jobs:\n  spec-drift:\n  e2e:\n    steps:\n      - name: Pruebas X y más\n"
        self.assertIn("paso Pruebas X", "\n".join(self.errors(files=files)))

    def test_the_real_feature_is_machine_checked(self) -> None:
        self.assertEqual(traceability.verification_errors(traceability.load_gherkin_scenarios()), [])


if __name__ == "__main__":
    unittest.main()
