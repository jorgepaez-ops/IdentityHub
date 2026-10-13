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


def check_excerpt(lines: list[str], a: int, b: int) -> list[str]:
    """Devuelve un mensaje por cada regla que el rango A-B (1-based, inclusivo) incumple."""
    n = len(lines)
    if not 1 <= a <= b <= n:
        return [f"rango {a}-{b} fuera de 1..{n}"]
    first, last = lines[a - 1], lines[b - 1]
    problems: list[str] = []
    if blank(first):
        problems.append(f"la línea {a} (inicio) está en blanco")
    if blank(last):
        problems.append(f"la línea {b} (fin) está en blanco")
    if problems:
        return problems
    if b < n:
        after = lines[b]
        if not blank(after) and indent(after) > indent(first):
            problems.append(f"termina a mitad de bloque: la línea {b + 1} sigue más indentada que la {a}: {after.strip()!r}")
    if a > 1:
        before = lines[a - 2]
        if not (blank(before) or before.lstrip().startswith("#") or indent(before) <= indent(first)):
            problems.append(f"empieza a mitad de bloque: la línea {a - 1} está más indentada que la {a}: {before.strip()!r}")
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


class ReportExcerptsTest(unittest.TestCase):
    def test_report_has_directives(self):
        self.assertTrue(directives(REPORT.read_text(encoding="utf-8")), "el informe ya no incrusta código")

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
