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

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = ROOT / "specs" / "07-traceability.md"

RF_HEADING = re.compile(r"^### (R[FN]-\d{3}) — (.+?) · (P\d)$", re.M)
RF_TAG = re.compile(r"\b((?:RNF|RF)-\d{3})\b")


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


def load_scenarios() -> dict[str, list[str]]:
    """@RF-NNN -> [nombre del escenario]."""
    scenarios: dict[str, list[str]] = defaultdict(list)
    for path in sorted((ROOT / "specs" / "06-acceptance").glob("*.feature")):
        pending: set[str] = set()
        for line in path.read_text(encoding="utf-8").splitlines():
            stripped = line.strip()
            if stripped.startswith("@"):
                pending = set(RF_TAG.findall(stripped))
            elif re.match(r"(Escenario|Esquema del escenario):", stripped):
                name = stripped.split(":", 1)[1].strip()
                for rf in pending:
                    scenarios[rf].append(f"{path.name} — {name}")
                pending = set()
    return scenarios


def render() -> str:
    reqs = load_requirements()
    ops = load_operations()
    scenarios = load_scenarios()

    # Los nombres Go usan TestRF003_ / TestRNF012_, sin guion; aquí se normalizan.
    go_by_req: dict[str, list[str]] = defaultdict(list)
    for root in ["backend"]:
        base = ROOT / root
        if base.exists():
            for path in sorted(base.rglob("*_test.go")):
                for m in re.finditer(r"func (Test(RNF|RF)(\d{3})_\w+)", path.read_text(encoding="utf-8")):
                    go_by_req[f"{m.group(2)}-{m.group(3)}"].append(m.group(1))

    e2e_by_req: dict[str, list[str]] = defaultdict(list)
    base = ROOT / "e2e"
    if base.exists():
        for path in sorted(base.rglob("*.spec.ts")):
            for m in re.finditer(r"""test\(\s*['"`]((?:RNF|RF)-\d{3})""", path.read_text(encoding="utf-8")):
                e2e_by_req[m.group(1)].append(path.name)

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

    counts = {"completo": 0, "parcial": 0, "sin cubrir": 0}
    for rid in sorted(reqs):
        title, prio = reqs[rid]
        o = ", ".join(f"`{x}`" for x in ops.get(rid, [])) or "—"
        s = str(len(scenarios.get(rid, []))) if scenarios.get(rid) else "—"
        g = str(len(go_by_req.get(rid, []))) if go_by_req.get(rid) else "—"
        e = str(len(e2e_by_req.get(rid, []))) if e2e_by_req.get(rid) else "—"
        covered = sum(x != "—" for x in (s, g, e))
        state = "✅ completo" if covered >= 2 else ("🟡 parcial" if covered == 1 else "🔴 sin cubrir")
        counts["completo" if covered >= 2 else ("parcial" if covered == 1 else "sin cubrir")] += 1
        lines.append(f"| **{rid}** | {title} | {prio} | {o} | {s} | {g} | {e} | {state} |")

    total = sum(counts.values())
    lines += [
        "",
        f"**Resumen:** {total} requisitos · "
        f"{counts['completo']} completos · {counts['parcial']} parciales · "
        f"{counts['sin cubrir']} sin cubrir.",
        "",
        "Leyenda de estado: *completo* = verificado por al menos dos de las tres",
        "columnas de prueba · *parcial* = una sola · *sin cubrir* = ninguna.",
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

    content = render()
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
