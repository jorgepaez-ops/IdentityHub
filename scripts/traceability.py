#!/usr/bin/env python3
"""Genera specs/07-traceability.md a partir de los artefactos reales del repo.

La matriz de trazabilidad es un documento que se pudre en cuanto se escribe a
mano. Aquí se deriva de las cuatro fuentes que ya existen:

  · specs/01-requirements.md    identificadores y títulos de los requisitos
  · specs/03-api/openapi.yaml   x-requirement -> operationId
  · specs/06-acceptance/*.feature   etiquetas @RF-NNN
  · backend/**/*_test.go, e2e/tests/*.spec.ts   nombres TestRFNNN_ / 'RF-NNN ...'

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

RF_HEADING = re.compile(r"^### (R[FN]-\d{3}) — (.+?) · (P\d)$", re.M)
RF_TAG = re.compile(r"\b((?:RNF|RF)-\d{3})\b")
E2E_TEST = re.compile(r"\btest\(\s*'((?:RNF|RF)-\d{3}[^']*)'\s*,")

# Requisitos pendientes de implementación. T3 define sus contratos, pero la matriz los
# mantiene diferidos hasta que sus tareas de backend/frontend queden cerradas.
DEFERRED = {
    "RF-018": "Backlog de semana 3 por decisión Q14.",
    "RF-019": "Backlog de semana 3 por decisión Q14.",
}


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


GHERKIN_SCENARIO = re.compile(r"(Escenario|Esquema del escenario):")
# Keywords that open a new scope: tags collected so far must not leak past them.
GHERKIN_SCOPE = re.compile(r"(Característica|Regla|Antecedentes|Fondo):")


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
        pending: list[str] = []
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            if stripped.startswith("@"):
                pending.extend(RF_TAG.findall(stripped))
            elif GHERKIN_SCENARIO.match(stripped):
                name = stripped.split(":", 1)[1].strip()
                first_rf = next((tag for tag in pending if tag.startswith("RF-")), "")
                if pending:
                    scenarios.append(Scenario(first_rf, name, path.name, tuple(pending), number))
                pending = []
            else:
                # Scope keywords, steps, tables and docstrings all end the tag run.
                pending = []
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


def e2e_correspondence_errors(scenarios: list[Scenario]) -> list[str]:
    """Return actionable errors for the scenario-to-E2E one-to-one mapping."""
    titles = load_e2e_test_titles()
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
    computed = computed_requirement_titles()

    missing = [title for title in expected if title not in titles]
    orphan = [title for title in titles if title.startswith("RF-") and title not in expected]
    duplicates = [title for title in expected if len(titles.get(title, [])) > 1]
    if not (missing or orphan or duplicates or computed or duplicate_scenarios):
        return []

    errors = ["La correspondencia escenario↔prueba E2E no es uno a uno."]
    if missing:
        errors.append("Faltan pruebas E2E para escenarios:")
        for title in missing:
            scenario = expected[title]
            errors.append(f"  - {title} ({scenario.source})")
    if orphan:
        errors.append("Títulos E2E con prefijo RF sin escenario:")
        for title in orphan:
            for location in titles[title]:
                errors.append(f"  - {location}: {title}")
    if duplicates:
        errors.append("Escenarios con más de una prueba E2E:")
        for title in duplicates:
            scenario = expected[title]
            errors.append(f"  - {title} ({scenario.source}): {', '.join(titles[title])}")
    if duplicate_scenarios:
        errors.append("Escenarios Gherkin repetidos (misma etiqueta RF y mismo nombre):")
        for title, first, second in duplicate_scenarios:
            errors.append(f"  - {title}: {first.source}:{first.line}, {second.source}:{second.line}")
    if computed:
        errors.append("Títulos RF/RNF no literales (template o calculados), invisibles para este chequeo:")
        errors.extend(f"  - {location}" for location in computed)
    errors.append("Usa títulos literales: 'RF-NNN <nombre exacto del escenario>'.")
    return errors


def render(gherkin: list[Scenario]) -> str:
    reqs = load_requirements()
    ops = load_operations()
    scenarios = load_scenarios(gherkin)

    # Los nombres Go usan TestRF003_ / TestRNF012_, sin guion; aquí se normalizan.
    go_by_req: dict[str, list[str]] = defaultdict(list)
    for root in ["backend"]:
        base = ROOT / root
        if base.exists():
            for path in sorted(base.rglob("*_test.go")):
                for m in re.finditer(r"func (Test(RNF|RF)(\d{3})_\w+)", path.read_text(encoding="utf-8")):
                    go_by_req[f"{m.group(2)}-{m.group(3)}"].append(m.group(1))

    e2e_by_req: dict[str, list[str]] = defaultdict(list)
    for title, locations in load_e2e_test_titles().items():
        requirement = title.split(" ", 1)[0]
        e2e_by_req[requirement].extend(locations)

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
        "| Requisito | Título | Pri | Operaciones de la API | Escenarios | Pruebas Go | E2E | Estado |",
        "|---|---|---|---|---|---|---|---|",
    ]

    counts = {"completo": 0, "parcial": 0, "sin cubrir": 0, "diferido": 0}
    for rid in sorted(reqs):
        title, prio = reqs[rid]
        o = ", ".join(f"`{x}`" for x in ops.get(rid, [])) or "—"
        s = str(len(scenarios.get(rid, []))) if scenarios.get(rid) else "—"
        g = str(len(go_by_req.get(rid, []))) if go_by_req.get(rid) else "—"
        e = str(len(e2e_by_req.get(rid, []))) if e2e_by_req.get(rid) else "—"
        covered = sum(x != "—" for x in (s, g, e))
        if rid in DEFERRED:
            state = "⏳ diferido (semana 3)"
            counts["diferido"] += 1
        else:
            state = "✅ completo" if covered >= 2 else ("🟡 parcial" if covered == 1 else "🔴 sin cubrir")
            counts["completo" if covered >= 2 else ("parcial" if covered == 1 else "sin cubrir")] += 1
        lines.append(f"| **{rid}** | {title} | {prio} | {o} | {s} | {g} | {e} | {state} |")

    total = sum(counts.values())
    lines += [
        "",
        f"**Resumen:** {total} requisitos · "
        f"{counts['completo']} completos · {counts['parcial']} parciales · "
        f"{counts['sin cubrir']} sin cubrir · {counts['diferido']} diferidos (semana 3).",
        "",
        "Leyenda de estado: *completo* = verificado por al menos dos de las tres",
        "columnas de prueba · *parcial* = una sola · *sin cubrir* = ninguna · "
        "*diferido (semana 3)* = backlog de semana 3 por decisión Q14.",
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
