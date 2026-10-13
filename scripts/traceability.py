#!/usr/bin/env python3
"""Genera specs/07-traceability.md a partir de los artefactos reales del repo.

La matriz de trazabilidad es un documento que se pudre en cuanto se escribe a
mano. Aquí se deriva de las cuatro fuentes que ya existen:

  · specs/01-requirements.md    identificadores y títulos de los requisitos
  · specs/03-api/openapi.yaml   x-requirement -> operationId
  · specs/06-acceptance/*.feature   etiquetas @RF-NNN
  · backend/**/*_test.go, e2e/tests/*.spec.ts   nombres TestRFNNN_ / 'RF-NNN ...'
  · specs/07-controls.yaml      controles de CI/pipeline que hacen cumplir cada RNF; cada
                                referencia se verifica contra el archivo real o --check falla

Uso:
    python3 scripts/traceability.py            # escribe el archivo
    python3 scripts/traceability.py --check    # falla si está desactualizado
"""
from __future__ import annotations

import argparse
import pathlib
import re
import sys
from collections import defaultdict
from typing import NamedTuple

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = ROOT / "specs" / "07-traceability.md"
CONTROLS = "specs/07-controls.yaml"

RF_HEADING = re.compile(r"^### (R[FN]-\d{3}) — (.+?) · (P\d)$", re.M)
RF_TAG = re.compile(r"\b((?:RNF|RF)-\d{3})\b")
E2E_TEST = re.compile(r"\btest\(\s*'((?:RNF|RF)-\d{3}[^']*)'\s*,")

# Requisitos pendientes de implementación. T3 define sus contratos, pero la matriz los
# mantiene diferidos hasta que sus tareas de backend/frontend queden cerradas.
DEFERRED: dict[str, str] = {}


class Scenario(NamedTuple):
    """A tagged Gherkin scenario.

    ``requirement`` is the first RF tag (it owns the E2E title); ``tags`` keeps every RF/RNF tag
    in source order, which the traceability matrix needs.
    """

    requirement: str
    name: str
    source: str
    tags: tuple[str, ...] = ()
    line: int = 0
    verification: str = ""


GHERKIN_SCENARIO = re.compile(r"(Escenario|Esquema del escenario):")
VERIFICATION_COMMENT = "# Verificación:"
VERIFICATION_PREFIXES = 'paso "…" (paso de ci.yml), e2e "…" (título de Playwright) o make <objetivo>'
# One scan, left to right; the first alternative that matches at a position wins, so the quoted
# `make` forms must come before the bare one and the catch-all stray string comes last.
VERIFICATION_TOKEN = re.compile(
    r"""
    (?P<file>[\w./-]+\.(?:py|js|ts|sh|yml|yaml|go)\b)
  | \((?P<name>\w+)\)
  | \bjob\s+(?P<job>[\w-]+)
  | \bpaso\s+"(?P<step>[^"]+)"
  | \be2e\s+"(?P<e2e>[^"]+)"
  | (?P<quote>["`])make\s+(?P<quoted_make>[\w-]+)(?P=quote)
  | \bmake\s+(?P<make>[\w-]+)
  | "(?P<stray>[^"]*)"
    """,
    re.X,
)


class Control(NamedTuple):
    """A CI/pipeline control that enforces a requirement.

    ``section`` is a workflow job id, a compose service, a Makefile target or any other
    ``key:`` line; empty means the whole file. ``text`` must appear in that section.
    """

    file: str
    section: str
    text: str


def load_controls() -> dict[str, list[Control]]:
    """RNF-NNN -> controls, from specs/07-controls.yaml.

    Restricted YAML subset, read as text like the OpenAPI file (no PyYAML): ``RNF-NNN:``
    headers followed by ``- <file> | <section or -> | <text>`` entries.
    """
    controls: dict[str, list[Control]] = defaultdict(list)
    current = None
    for number, raw in enumerate((ROOT / CONTROLS).read_text(encoding="utf-8").splitlines(), start=1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if m := re.fullmatch(r"((?:RNF|RF)-\d{3}):", line):
            current = m.group(1)
        elif line.startswith("- ") and current:
            controls[current].append(parse_control_entry(line, number))
        else:
            raise ValueError(f"{CONTROLS}:{number}: línea no reconocida: {raw!r}")
    return dict(controls)


def parse_control_entry(line: str, number: int) -> Control:
    """One ``- <file> | <section or -> | <text>`` entry of specs/07-controls.yaml."""
    parts = [part.strip() for part in line[2:].split(" | ")]
    if len(parts) != 3 or not all(parts):
        raise ValueError(f"{CONTROLS}:{number}: se esperaba '- archivo | sección o - | texto'")
    file, section, text = parts
    return Control(file, "" if section == "-" else section, text)


def section_block(lines: list[str], section: str) -> list[str] | None:
    """Lines of the shallowest ``<section>:`` key and everything nested under it, or None.

    The shallowest match wins so a job id like ``image`` is not confused with an
    ``image:`` key nested in some other job's services.
    """
    pattern = re.compile(rf"^(\s*){re.escape(section)}:")
    matches = [(len(m.group(1)), i) for i, line in enumerate(lines) if (m := pattern.match(line))]
    if not matches:
        return None
    indent, start = min(matches)
    block = [lines[start]]
    for follower in lines[start + 1 :]:
        if follower.strip() and not follower.lstrip().startswith("#"):
            if len(follower) - len(follower.lstrip()) <= indent:
                break
        block.append(follower)
    return block


def control_errors(controls: dict[str, list[Control]], reqs: dict[str, tuple[str, str]]) -> list[str]:
    """Every declared control must point at something that really exists."""
    errors: list[str] = []
    for rid, entries in controls.items():
        if rid not in reqs:
            errors.append(f"{CONTROLS}: {rid} no es un requisito de specs/01-requirements.md")
        for control in entries:
            problem = control_problem(control)
            if problem:
                errors.append(f"{rid}: {problem}")
    return errors


def control_problem(control: Control) -> str | None:
    """Why one declared control does not resolve against the real file, or ``None``."""
    path = ROOT / control.file
    if not path.is_file():
        return f"el archivo {control.file} no existe"
    lines = path.read_text(encoding="utf-8").splitlines()
    if control.section:
        block = section_block(lines, control.section)
        if block is None:
            return f"{control.file} no tiene la sección '{control.section}'"
        lines = block
    # Comment lines do not count: a control must be backed by real configuration.
    if any(control.text in line for line in lines if not line.lstrip().startswith("#")):
        return None
    where = f"la sección '{control.section}' de " if control.section else ""
    return f"{where}{control.file} no contiene '{control.text}'"


def control_label(control: Control) -> str:
    name = pathlib.PurePosixPath(control.file).name
    return f"`{name}#{control.section}`" if control.section else f"`{control.file}`"


def load_requirements() -> dict[str, tuple[str, str]]:
    text = (ROOT / "specs" / "01-requirements.md").read_text(encoding="utf-8")
    reqs = {m.group(1): (m.group(2), m.group(3)) for m in RF_HEADING.finditer(text)}
    # Los RNF no llevan sufijo de prioridad en la misma forma; se recogen aparte.
    for m in re.finditer(r"^### (RNF-\d{3}) — (.+?) · (P\d)$", text, re.M):
        reqs[m.group(1)] = (m.group(2), m.group(3))
    return reqs


def load_operations() -> dict[str, list[str]]:
    """x-requirement -> [operationId]. Se lee como texto para no depender de PyYAML."""
    text = (ROOT / "specs" / "03-api" / "openapi.yaml").read_text(encoding="utf-8")
    ops: dict[str, list[str]] = defaultdict(list)
    current_op = None
    for line in text.splitlines():
        if m := re.match(r"\s+operationId:\s*(\S+)", line):
            current_op = m.group(1)
        elif m := re.match(r"\s+x-requirement:\s*((?:RNF|RF)-\d{3})", line):
            if current_op:
                ops[m.group(1)].append(current_op)
    return ops


def load_gherkin_scenarios() -> list[Scenario]:
    """The single Gherkin parser: every scenario that carries at least one RF/RNF tag.

    Tags only apply to the scenario that immediately follows them: any scope keyword
    (Característica, Regla, Antecedentes) or step line discards the tags collected so far.
    """
    scenarios: list[Scenario] = []
    for path in sorted((ROOT / "specs" / "06-acceptance").glob("*.feature")):
        scenarios.extend(parse_feature(path))
    return scenarios


def parse_feature(path: pathlib.Path) -> list[Scenario]:
    """The tagged scenarios of one ``.feature`` file."""
    scenarios: list[Scenario] = []
    pending: list[str] = []
    verification = ""
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        stripped = line.strip()
        if stripped.startswith(VERIFICATION_COMMENT):
            verification = stripped[len(VERIFICATION_COMMENT) :].strip()
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("@"):
            pending.extend(RF_TAG.findall(stripped))
            continue
        if pending and GHERKIN_SCENARIO.match(stripped):
            name = stripped.split(":", 1)[1].strip()
            first_rf = next((tag for tag in pending if tag.startswith("RF-")), "")
            scenarios.append(Scenario(first_rf, name, path.name, tuple(pending), number, verification))
        # Scenarios, scope keywords, steps, tables and docstrings all end the tag run.
        pending = []
        verification = ""
    return scenarios


def load_scenarios(scenarios: list[Scenario]) -> dict[str, list[str]]:
    """@RF-NNN -> [nombre del escenario], derived from the single Gherkin parse."""
    by_requirement: dict[str, list[str]] = defaultdict(list)
    for scenario in scenarios:
        for tag in scenario.tags:
            by_requirement[tag].append(f"{scenario.source} — {scenario.name}")
    return by_requirement


def load_e2e_test_titles() -> dict[str, list[str]]:
    """RF/RNF Playwright test titles -> ``file:line`` locations.

    Only a plain single-quoted literal directly passed to ``test(`` is a title.
    Template strings and computed titles are intentionally ignored so CI can
    enforce an auditable one-to-one mapping to Gherkin scenarios.
    """
    titles: dict[str, list[str]] = defaultdict(list)
    base = ROOT / "e2e" / "tests"
    if not base.exists():
        return titles
    for path in sorted(base.rglob("*.spec.ts")):
        text = path.read_text(encoding="utf-8")
        for match in E2E_TEST.finditer(text):
            line = text.count("\n", 0, match.start(1)) + 1
            titles[match.group(1)].append(f"{path.relative_to(base)}:{line}")
    return titles


def computed_requirement_titles() -> list[str]:
    """``file:line`` of RF/RNF test titles written as template or double-quoted strings.

    Those would escape the literal-title scan above, so a test could be missing
    from the mapping without anyone noticing; they are reported as errors.
    """
    locations: list[str] = []
    base = ROOT / "e2e" / "tests"
    if not base.exists():
        return locations
    for path in sorted(base.rglob("*.spec.ts")):
        text = path.read_text(encoding="utf-8")
        for match in re.finditer(r"""\btest\(\s*[`"](?:RNF|RF)-\d{3}""", text):
            line = text.count("\n", 0, match.start()) + 1
            locations.append(f"{path.relative_to(base)}:{line}")
    return locations


class VerificationSources(NamedTuple):
    """The repository texts a ``# Verificación:`` reference can resolve against, read once."""

    ci: str
    makefile: str
    e2e_specs: list[str]


def load_verification_sources() -> VerificationSources:
    def read(relative: str) -> str:
        path = ROOT / relative
        return path.read_text(encoding="utf-8") if path.is_file() else ""

    specs_dir = ROOT / "e2e" / "tests"
    specs = [p.read_text(encoding="utf-8") for p in sorted(specs_dir.rglob("*.spec.ts"))] if specs_dir.is_dir() else []
    return VerificationSources(read(".github/workflows/ci.yml"), read("Makefile"), specs)


def tokenize_verification(text: str) -> tuple[list[tuple[str, str]], list[str]]:
    """Scan one ``# Verificación:`` line once, left to right.

    Returns the ``(kind, value)`` references in order plus the problems that are not references: a
    double-quoted string without a prefix, a double quote left unconsumed, or an odd number of
    backticks left in the free text (backticks around a path or a name are plain formatting).
    """
    references: list[tuple[str, str]] = []
    problems: list[str] = []
    gaps: list[str] = []
    position = 0
    for match in VERIFICATION_TOKEN.finditer(text):
        gaps.append(text[position:match.start()])
        position = match.end()
        kind = match.lastgroup
        if kind == "stray":
            problems.append(f'texto entre comillas sin prefijo: "{match.group("stray")}"; use {VERIFICATION_PREFIXES}')
        else:
            references.append(("make" if kind == "quoted_make" else kind, match.group(kind)))
    gaps.append(text[position:])
    free_text = "".join(gaps)
    if '"' in free_text or free_text.count("`") % 2:
        problems.append('comilla sin cerrar o delimitadores distintos fuera de una referencia reconocida: cada "…" necesita un prefijo (paso, e2e, make) y cada `…` debe cerrarse con el mismo carácter')
    return references, problems


def reference_problem(kind: str, value: str, sources: VerificationSources, files: dict[str, str]) -> str | None:
    """Return why one named reference does not resolve, or ``None`` when it does."""
    if kind == "name":
        pattern = rf"(?:class|def|function)\s+{re.escape(value)}\b|(['\"`]){re.escape(value)}\1"
        if not any(re.search(pattern, body) for body in files.values()):
            return f"{value} no está definido en {', '.join(files) or 'ningún archivo'}"
    elif kind == "job" and not re.search(rf"^  {re.escape(value)}:", sources.ci, re.M):
        return f"el job {value} no existe en ci.yml"
    elif kind == "step" and not re.search(rf"^\s*- name: {re.escape(value)}\s*$", sources.ci, re.M):
        return f"el paso {value} no existe en ci.yml"
    elif kind == "make" and not re.search(rf"^{re.escape(value)}:", sources.makefile, re.M):
        return f"el objetivo make {value} no existe en el Makefile"
    elif kind == "e2e" and not any(re.search(rf"""\btest(?:\.only|\.skip)?\(\s*(['"`]){re.escape(value)}\1""", body) for body in sources.e2e_specs):
        return f"el título E2E {value} no existe en e2e/tests/*.spec.ts"
    return None


def verification_errors(scenarios: list[Scenario]) -> list[str]:
    """Machine-check the "# Verificación:" line of every scenario tagged only with @RNF-NNN.

    These scenarios are not derived into Playwright specs, so the free-text comment is their only link
    to the test that runs them. The line is tokenized once (``VERIFICATION_TOKEN``). This list is the
    canonical grammar; specs/06-acceptance/README.md and scripts/README.md copy it verbatim and
    ``test_the_grammar_copies_match_the_docstring`` keeps them in sync. Other words are free text:

    - una ruta de archivo (.py, .js, .ts, .sh, .yml, .yaml o .go) que debe existir;
    - `(Nombre)`: clase o función que debe existir como palabra completa en ese archivo;
    - `job <id>`: id de un job de ci.yml, con id = letras, dígitos, `_` y `-` (la puntuación final no es parte del id);
    - `paso "<nombre>"`: paso de ci.yml con `- name:` exactamente igual;
    - `e2e "<título>"`: prueba de Playwright con ese título exacto (`test(`, `test.only(` o `test.skip(`, con comillas simples, dobles o invertidas) en `e2e/tests/**/*.spec.ts`;
    - `make <objetivo>`: objetivo del Makefile, suelto, entre comillas invertidas o entre comillas dobles del mismo tipo al abrir y cerrar. Un `make <palabra>` suelto siempre es referencia y debe existir, así que los `.feature` no usan «make» como prosa;
    - cualquier otro texto entre comillas dobles es un error que nombra los prefijos aceptados (paso, e2e, make);
    - un escenario sin ninguna referencia nombrada falla y el error lista todas las formas aceptadas.

    Every named reference must resolve, and a scenario with no file or no named reference fails.
    Scenarios that carry any @RF tag are exempt (their link is the E2E title, checked elsewhere).
    """
    errors: list[str] = []
    sources = load_verification_sources()
    for scenario in scenarios:
        if not scenario.tags or any(tag.startswith("RF-") for tag in scenario.tags):
            continue
        label = f"{', '.join(scenario.tags)} {scenario.name} ({scenario.source}:{scenario.line})"
        errors += [f"{label}: {problem}" for problem in verification_problems(scenario, sources)]
    return errors


def verification_problems(scenario: Scenario, sources: VerificationSources) -> list[str]:
    """Everything wrong with the ``# Verificación:`` line of one @RNF-only scenario."""
    if not scenario.verification:
        return ["falta la línea «# Verificación:» justo encima de las etiquetas"]
    references, problems = tokenize_verification(scenario.verification)
    files = [value for kind, value in references if kind == "file"]
    file_texts = {f: (ROOT / f).read_text(encoding="utf-8") for f in files if (ROOT / f).is_file()}
    problems += [f"el archivo {f} no existe" for f in files if f not in file_texts]
    if not files:
        problems.append("no nombra ningún archivo de prueba")
    named = [(kind, value) for kind, value in references if kind != "file"]
    if not named:
        problems.append("no nombra ninguna prueba concreta (clase/función entre paréntesis, job, paso, e2e o make)")
    problems += [p for p in (reference_problem(k, v, sources, file_texts) for k, v in named) if p]
    return problems


def expected_e2e_titles(scenarios: list[Scenario]) -> tuple[dict[str, Scenario], list[tuple[str, Scenario, Scenario]]]:
    """Expected E2E title -> scenario, plus the scenarios that repeat an earlier title."""
    expected: dict[str, Scenario] = {}
    duplicate_scenarios: list[tuple[str, Scenario, Scenario]] = []
    for scenario in scenarios:
        if not scenario.requirement:
            continue
        title = f"{scenario.requirement} {scenario.name}"
        if title in expected:
            duplicate_scenarios.append((title, expected[title], scenario))
        else:
            expected[title] = scenario
    return expected, duplicate_scenarios


def e2e_correspondence_errors(scenarios: list[Scenario]) -> list[str]:
    """Return actionable errors for the scenario-to-E2E one-to-one mapping."""
    titles = load_e2e_test_titles()
    expected, duplicate_scenarios = expected_e2e_titles(scenarios)
    computed = computed_requirement_titles()

    missing = [title for title in expected if title not in titles]
    orphan = [title for title in titles if title.startswith("RF-") and title not in expected]
    duplicates = [title for title in expected if len(titles.get(title, [])) > 1]
    if not (missing or orphan or duplicates or computed or duplicate_scenarios):
        return []

    sections = [
        ("Faltan pruebas E2E para escenarios:", [f"  - {title} ({expected[title].source})" for title in missing]),
        (
            "Títulos E2E con prefijo RF sin escenario:",
            [f"  - {location}: {title}" for title in orphan for location in titles[title]],
        ),
        (
            "Escenarios con más de una prueba E2E:",
            [f"  - {title} ({expected[title].source}): {', '.join(titles[title])}" for title in duplicates],
        ),
        (
            "Escenarios Gherkin repetidos (misma etiqueta RF y mismo nombre):",
            [
                f"  - {title}: {first.source}:{first.line}, {second.source}:{second.line}"
                for title, first, second in duplicate_scenarios
            ],
        ),
        (
            "Títulos RF/RNF no literales (template o calculados), invisibles para este chequeo:",
            [f"  - {location}" for location in computed],
        ),
    ]
    return format_correspondence_report(sections)


def format_correspondence_report(sections: list[tuple[str, list[str]]]) -> list[str]:
    """Headline, every non-empty section (heading then items) and the closing hint."""
    errors = ["La correspondencia escenario↔prueba E2E no es uno a uno."]
    for heading, items in sections:
        if items:
            errors += [heading, *items]
    errors.append("Usa títulos literales: 'RF-NNN <nombre exacto del escenario>'.")
    return errors


def load_go_tests_by_requirement() -> dict[str, list[str]]:
    """RF/RNF -> Go test names. Go uses TestRF003_ / TestRNF012_ (no dash); normalized here."""
    go_by_req: dict[str, list[str]] = defaultdict(list)
    base = ROOT / "backend"
    if base.exists():
        for path in sorted(base.rglob("*_test.go")):
            for m in re.finditer(r"func (Test(RNF|RF)(\d{3})_\w+)", path.read_text(encoding="utf-8")):
                go_by_req[f"{m.group(2)}-{m.group(3)}"].append(m.group(1))
    return go_by_req


def load_e2e_by_requirement() -> dict[str, list[str]]:
    """RF/RNF -> ``file:line`` of its Playwright tests."""
    e2e_by_req: dict[str, list[str]] = defaultdict(list)
    for title, locations in load_e2e_test_titles().items():
        requirement = title.split(" ", 1)[0]
        e2e_by_req[requirement].extend(locations)
    return e2e_by_req


def count_or_dash(items: list[str] | None) -> str:
    return str(len(items)) if items else "—"


def requirement_state(rid: str, covered: int) -> str:
    """Matrix state key of one requirement; render() also uses it as the counts key."""
    if rid in DEFERRED:
        return "diferido"
    if covered >= 2:
        return "completo"
    return "parcial" if covered == 1 else "sin cubrir"


STATE_LABELS = {
    "diferido": "⏳ diferido (2026-10-13)",
    "completo": "✅ completo",
    "parcial": "🟡 parcial",
    "sin cubrir": "🔴 sin cubrir",
}


def render(gherkin: list[Scenario]) -> str:
    reqs = load_requirements()
    ops = load_operations()
    scenarios = load_scenarios(gherkin)
    controls = load_controls()
    go_by_req = load_go_tests_by_requirement()
    e2e_by_req = load_e2e_by_requirement()

    lines = [
        "# 07 — Matriz de trazabilidad",
        "",
        "> **Archivo generado.** No editar a mano.",
        "> `python3 scripts/traceability.py` lo regenera; el job `spec-drift` de CI",
        "> falla si el resultado difiere de lo commiteado.",
        "",
        "Un requisito sin prueba que lo verifique se considera no implementado. Esta",
        "tabla existe para que esa afirmación sea comprobable de un vistazo y no una",
        "declaración de buenas intenciones.",
        "",
        "| Requisito | Título | Pri | Operaciones de la API | Escenarios | Pruebas Go | E2E | Controles CI | Estado |",
        "|---|---|---|---|---|---|---|---|---|",
    ]

    counts = {"completo": 0, "parcial": 0, "sin cubrir": 0, "diferido": 0}
    for rid in sorted(reqs):
        title, prio = reqs[rid]
        o = ", ".join(f"`{x}`" for x in ops.get(rid, [])) or "—"
        s, g, e = (count_or_dash(source.get(rid)) for source in (scenarios, go_by_req, e2e_by_req))
        labels = list(dict.fromkeys(control_label(c) for c in controls.get(rid, [])))
        c = ", ".join(labels) or "—"
        covered = sum(x != "—" for x in (s, g, e, c))
        key = requirement_state(rid, covered)
        counts[key] += 1
        state = STATE_LABELS[key]
        lines.append(f"| **{rid}** | {title} | {prio} | {o} | {s} | {g} | {e} | {c} | {state} |")

    total = sum(counts.values())
    lines += [
        "",
        f"**Resumen:** {total} requisitos · "
        f"{counts['completo']} completos · {counts['parcial']} parciales · "
        f"{counts['sin cubrir']} sin cubrir · {counts['diferido']} diferidos (2026-10-13).",
        "",
        "Leyenda de estado: *completo* = verificado por al menos dos de las",
        "cuatro columnas de evidencia (Escenarios, Pruebas Go, E2E, Controles CI) · *parcial* = una sola ·",
        "*sin cubrir* = ninguna · "
        "*diferido (2026-10-13)* = planificado para el 2026-10-13 por decisión del usuario (2026-10-12).",
        "",
        "*Controles CI* lista los jobs, pasos o ajustes que hacen cumplir el requisito de forma",
        "automática (`archivo#sección`). Se declaran en `specs/07-controls.yaml` y `--check` falla",
        "si alguno apunta a algo que no existe; varios controles en la misma celda cuentan como",
        "una sola fuente de evidencia.",
        "",
        "## Escenarios de aceptación por requisito",
        "",
    ]
    for rid in sorted(scenarios):
        lines.append(f"- **{rid}**")
        for name in scenarios[rid]:
            lines.append(f"  - {name}")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="falla si el archivo está desactualizado")
    args = parser.parse_args()

    gherkin = load_gherkin_scenarios()
    errors = e2e_correspondence_errors(gherkin)
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1

    errors = verification_errors(gherkin)
    if errors:
        print("\n".join(["Escenarios @RNF sin enlace verificable a su prueba («# Verificación:»):", *(f"  - {e}" for e in errors)]), file=sys.stderr)
        return 1

    errors = control_errors(load_controls(), load_requirements())
    if errors:
        print("\n".join(["Controles de specs/07-controls.yaml no verificables:", *(f"  - {e}" for e in errors)]), file=sys.stderr)
        return 1

    content = render(gherkin)
    if args.check:
        current = OUT.read_text(encoding="utf-8") if OUT.exists() else ""
        if current != content:
            print("specs/07-traceability.md está desactualizado.", file=sys.stderr)
            print("Ejecuta: python3 scripts/traceability.py", file=sys.stderr)
            return 1
        print("Matriz de trazabilidad al día.")
        return 0

    OUT.write_text(content, encoding="utf-8")
    print(f"Escrito {OUT.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
