#!/usr/bin/env python3
"""Stdlib-only test: los extractos de código del informe caen en límites de bloque.

docs/informe/informe-tecnico.md incrusta archivos reales con {{code:LANG:RUTA:A-B}} por rango fijo de
líneas. Cualquier edición de un workflow desplaza esas líneas sin avisar y el informe muestra un
extracto cortado. Para cada directiva se comprueba que el archivo exista, que el rango sea válido y
que el extracto empiece y termine en un límite de bloque (no en mitad de uno).
"""
from __future__ import annotations

import pathlib
import re
import sys
import unittest

# Avoid leaving scripts/__pycache__/ behind (spec-drift requires a clean tree).
sys.dont_write_bytecode = True

ROOT = pathlib.Path(__file__).resolve().parent.parent
REPORT = ROOT / "docs" / "informe" / "informe-tecnico.md"
DIRECTIVE = re.compile(r"^\{\{code:([^:}]+):([^:}]+):(\d+)-(\d+)\}\}$", re.M)


def indent(line: str) -> int:
    return len(line) - len(line.lstrip(" \t"))


def blank(line: str) -> bool:
    return not line.strip()


STARTER = re.compile(r"^\s*(- |#|[\w.-]+:)")  # a YAML key, a list item or a comment


def neighbour(lines: list[str], i: int, step: int) -> str | None:
    """First line from index i moving by step that is neither blank nor a comment."""
    while 0 <= i < len(lines):
        if not blank(lines[i]) and not lines[i].lstrip().startswith("#"):
            return lines[i]
        i += step
    return None


def check_excerpt(lines: list[str], a: int, b: int) -> list[str]:
    """Devuelve un mensaje por cada regla que el rango A-B (1-based, inclusivo) incumple.

    Inicio: la línea A abre algo (clave YAML, ítem de lista o comentario) y la línea significativa
    anterior es su padre (menos indentada y terminada en «:») o el final de un bloque hermano.
    Fin: la siguiente línea significativa tras B no está más indentada que A (los blancos de un
    bloque `run: |` no cuentan como límite).
    """
    n = len(lines)
    if not 1 <= a <= b <= n:
        return [f"rango {a}-{b} fuera de 1..{n}"]
    first, last = lines[a - 1], lines[b - 1]
    problems: list[str] = []
    if blank(first):
        problems.append(f"la línea {a} (inicio) está en blanco")
    if blank(last):
        problems.append(f"la línea {b} (fin) está en blanco")
    after = neighbour(lines, b, 1)
    if after is not None and indent(after) > indent(first):
        problems.append(f"termina a mitad de bloque: sigue {after.strip()!r}, más indentada que la línea {a}")
    before = neighbour(lines, a - 2, -1)
    if before is not None:
        parent = indent(before) < indent(first) and before.rstrip().endswith(":")
        sibling = indent(before) >= indent(first) and STARTER.match(first) is not None
        if not (parent or sibling):
            problems.append(f"empieza a mitad de bloque: la línea {a} sigue a {before.strip()!r}")
    return problems


def directives(text: str) -> list[tuple[str, str, int, int]]:
    return [(m[0], m[1], int(m[2]), int(m[3])) for m in DIRECTIVE.findall(text)]


class CheckerTest(unittest.TestCase):
    SRC = [
        "jobs:",
        "  a:",
        "    steps:",
        "      - run: x",
        "        env:",
        "          K: v",
        "",
        "  b:",
        "    steps:",
        "      - run: y",
    ]

    def test_block_aligned_ranges_pass(self):
        self.assertEqual(check_excerpt(self.SRC, 2, 6), [])
        self.assertEqual(check_excerpt(self.SRC, 8, 10), [])

    def test_shifted_range_fails(self):
        self.assertTrue(check_excerpt(self.SRC, 3, 7))  # termina en blanco
        self.assertTrue(check_excerpt(self.SRC, 2, 5))  # corta el bloque env
        self.assertTrue(check_excerpt(self.SRC, 4, 5))  # corta el bloque env

    def test_out_of_range_fails(self):
        self.assertTrue(check_excerpt(self.SRC, 8, 11))
        self.assertTrue(check_excerpt(self.SRC, 0, 3))
        self.assertTrue(check_excerpt(self.SRC, 5, 4))

    def test_comment_before_start_is_boundary(self):
        src = ["a:", "  b:", "      # nota", "    c: 1"]
        self.assertEqual(check_excerpt(src, 4, 4), [])

    def test_start_dropping_the_step_name_fails(self):
        src = ["steps:", "  - name: Paso", "    uses: x/y@1", "    with:", "      k: v"]
        self.assertEqual(check_excerpt(src, 2, 5), [])
        self.assertTrue(check_excerpt(src, 3, 5))  # empieza en uses: y deja fuera el - name:

    def test_cut_inside_a_block_scalar_after_a_blank_fails(self):
        src = ["- name: Script", "  run: |", "    echo uno", "", "    echo dos", "- name: Otro", "  run: true"]
        self.assertEqual(check_excerpt(src, 1, 5), [])
        self.assertTrue(check_excerpt(src, 1, 3))  # corta el run: | antes del blanco
        self.assertTrue(check_excerpt(src, 5, 7))  # empieza a mitad del run: |


class ReportExcerptsTest(unittest.TestCase):
    def test_report_has_directives(self):
        text = REPORT.read_text(encoding="utf-8")
        self.assertTrue(directives(text), "el informe ya no incrusta código")
        self.assertEqual(text.count("{{code:"), len(directives(text)), "hay directivas {{code:...}} mal formadas o no aisladas en su línea")

    def test_every_excerpt_is_block_aligned(self):
        failures: list[str] = []
        for _lang, rel, a, b in directives(REPORT.read_text(encoding="utf-8")):
            path = ROOT / rel
            if not path.is_file():
                failures.append(f"{rel}:{a}-{b}: el archivo no existe")
                continue
            lines = path.read_text(encoding="utf-8").splitlines()
            failures += [f"{rel}:{a}-{b}: {p}" for p in check_excerpt(lines, a, b)]
        self.assertEqual(failures, [], "\n" + "\n".join(failures))


if __name__ == "__main__":
    unittest.main()
