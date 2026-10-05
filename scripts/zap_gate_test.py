#!/usr/bin/env python3
"""Tests for zap-gate.py (stdlib unittest). Run: python3 scripts/zap_gate_test.py"""
import importlib.util
import json
import os
import sys
import tempfile
import unittest

# Linux runners would otherwise leave scripts/__pycache__ in the working tree.
sys.dont_write_bytecode = True

HERE = os.path.dirname(os.path.abspath(__file__))
_spec = importlib.util.spec_from_file_location("zap_gate", os.path.join(HERE, "zap-gate.py"))
zap_gate = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(zap_gate)


def alert(plugin, name, riskcode, uri="http://x/", count="1"):
    return {
        "pluginid": plugin,
        "alert": name,
        "riskcode": str(riskcode),
        "count": count,
        "instances": [{"uri": uri}],
    }


def report(*alerts):
    return {"site": [{"@name": "http://x", "alerts": list(alerts)}]}


class GateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def write(self, name, content):
        path = os.path.join(self.tmp.name, name)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(content if isinstance(content, str) else json.dumps(content))
        return path

    def run_gate(self, reports, rules=None):
        args = list(reports)
        if rules is not None:
            args = ["--rules", rules] + args
        return zap_gate.main(args)

    def test_medium_breaks_the_gate(self):
        rep = self.write("r.json", report(alert("10055", "CSP", 2)))
        self.assertEqual(self.run_gate([rep]), 1)

    def test_high_breaks_the_gate(self):
        rep = self.write("r.json", report(alert("40012", "XSS", 3)))
        self.assertEqual(self.run_gate([rep]), 1)

    def test_low_and_info_do_not_break(self):
        rep = self.write("r.json", report(alert("90004", "COOP", 1), alert("10027", "Info", 0)))
        self.assertEqual(self.run_gate([rep]), 0)

    def test_ignore_with_justification_suppresses(self):
        rep = self.write("r.json", report(alert("10055", "CSP", 2)))
        rules = self.write("rules.tsv", "10055\tIGNORE\tFalse positive, see T99\n")
        self.assertEqual(self.run_gate([rep], rules), 0)

    def test_ignore_without_justification_is_rejected(self):
        rep = self.write("r.json", report(alert("10055", "CSP", 2)))
        rules = self.write("rules.tsv", "10055\tIGNORE\t\n")
        self.assertEqual(self.run_gate([rep], rules), 2)

    def test_warn_does_not_suppress_a_medium(self):
        rep = self.write("r.json", report(alert("10055", "CSP", 2)))
        rules = self.write("rules.tsv", "10055\tWARN\tjustified\n")
        self.assertEqual(self.run_gate([rep], rules), 1)

    def test_any_report_breaking_fails(self):
        ok = self.write("a.json", report(alert("90004", "COOP", 1)))
        bad = self.write("b.json", report(alert("10055", "CSP", 2)))
        self.assertEqual(self.run_gate([ok, bad]), 1)

    def test_missing_report_is_an_error(self):
        self.assertEqual(self.run_gate([os.path.join(self.tmp.name, "nope.json")]), 2)

    def test_report_without_scanned_sites_is_an_error(self):
        # ZAP that never reached the target can still write a report; it must not read as "clean".
        for content in ({}, {"site": []}):
            rep = self.write("empty.json", content)
            self.assertEqual(self.run_gate([rep]), 2)

    def test_malformed_report_is_an_input_error_not_a_finding(self):
        for content in ({"site": ["not-a-dict"]}, {"site": [{"alerts": [{"riskcode": "high"}]}]}):
            rep = self.write("bad.json", content)
            self.assertEqual(self.run_gate([rep]), 2)

    def test_single_site_object_is_accepted(self):
        rep = self.write("one.json", {"site": {"alerts": [{"pluginid": "10055", "alert": "CSP", "riskcode": "2", "count": "1", "instances": []}]}})
        self.assertEqual(self.run_gate([rep]), 1)


if __name__ == "__main__":
    unittest.main()
