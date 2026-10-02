#!/usr/bin/env python3
"""DAST gate: fail when a ZAP JSON report holds an alert of severity Medium or higher.

Usage: zap-gate.py [--rules .zap/rules.tsv] report.json [report.json ...]

Exit codes: 0 = gate passed, 1 = gate broken, 2 = usage/input error.

Only an IGNORE entry in the rules file (with a non-empty justification) suppresses a
Medium/High alert. WARN entries document accepted Low alerts and never suppress anything.
"""
import json
import sys

sys.dont_write_bytecode = True

RISK_NAMES = {0: "Info", 1: "Low", 2: "Medium", 3: "High"}
THRESHOLD = 2  # riskcode >= 2 breaks the build
DEFAULT_RULES = ".zap/rules.tsv"


class InputError(Exception):
    pass


def load_rules(path):
    """Return {pluginid: (action, comment)} from a ZAP rules TSV."""
    rules = {}
    try:
        fh = open(path, encoding="utf-8")
    except FileNotFoundError:
        return rules
    with fh:
        for lineno, raw in enumerate(fh, 1):
            line = raw.rstrip("\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                raise InputError(f"{path}:{lineno}: expected pluginid<TAB>action<TAB>comment")
            plugin, action = parts[0].strip(), parts[1].strip().upper()
            comment = parts[2].strip() if len(parts) > 2 else ""
            if action == "IGNORE" and not comment:
                raise InputError(f"{path}:{lineno}: IGNORE for {plugin} needs a justification")
            rules[plugin] = (action, comment)
    return rules


def load_alerts(path):
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        raise InputError(f"cannot read report {path}: {exc}")
    sites = data.get("site") if isinstance(data, dict) else None
    if not sites:
        # A ZAP run that never reached its target can still write a report; treating it as
        # "no alerts" would let the gate pass without anything having been scanned.
        raise InputError(f"report {path} has no scanned sites")
    if isinstance(sites, dict):
        sites = [sites]
    alerts = []
    try:
        for site in sites:
            for a in site.get("alerts", []):
                instances = a.get("instances") or []
                count = int(a.get("count") or len(instances) or 1)
                alerts.append({
                    "risk": int(a.get("riskcode", 0)),
                    "plugin": str(a.get("pluginid", "?")),
                    "name": a.get("alert") or a.get("name") or "?",
                    "count": count,
                    "url": instances[0].get("uri", "") if instances else "",
                })
    except (AttributeError, TypeError, ValueError, IndexError) as exc:
        # A malformed report is an input error (exit 2), never a "finding" (exit 1).
        raise InputError(f"report {path} has an unexpected shape: {exc}")
    return alerts


def main(argv):
    rules_path = DEFAULT_RULES
    reports = []
    args = list(argv)
    while args:
        arg = args.pop(0)
        if arg == "--rules":
            if not args:
                print("--rules needs a path", file=sys.stderr)
                return 2
            rules_path = args.pop(0)
        else:
            reports.append(arg)
    if not reports:
        print(__doc__, file=sys.stderr)
        return 2

    try:
        rules = load_rules(rules_path)
        per_report = [(p, load_alerts(p)) for p in reports]
    except InputError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2

    broken = 0
    for path, alerts in per_report:
        print(f"\n== {path}")
        print(f"{'RISK':<7} {'PLUGIN':<7} {'COUNT':>5}  {'STATUS':<8} NAME (first URL)")
        for a in sorted(alerts, key=lambda x: (-x["risk"], x["plugin"])):
            action, comment = rules.get(a["plugin"], ("", ""))
            if a["risk"] >= THRESHOLD and action != "IGNORE":
                status = "BREAKS"
                broken += 1
            elif a["risk"] >= THRESHOLD:
                status = "IGNORED"
            elif a["risk"] == 1:
                status = "warning"
            else:
                status = "info"
            print(f"{RISK_NAMES.get(a['risk'], '?'):<7} {a['plugin']:<7} {a['count']:>5}  "
                  f"{status:<8} {a['name']} ({a['url']})")
        if not alerts:
            print("(no alerts)")

    if broken:
        print(f"\nGATE BROKEN: {broken} alert(s) of severity Medium or higher.")
        return 1
    print("\nGate passed: no alerts of severity Medium or higher.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
