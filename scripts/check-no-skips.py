#!/usr/bin/env python3
"""Fail if any test was skipped.

A skipped test is indistinguishable from a passing one in a green build. That is
how seven tests went completely unexecuted in CI (see issue #201): they were
gated on euid at runtime via t.Skip, and no job ever ran them in a context where
the guard passed.

Policy: under normal circumstances no test should be skipped. A test that cannot
run in a given context must be excluded by *build tag* (`rootonly` / `nonroot`)
so it is never scheduled, rather than scheduled and skipped. That way a coverage
gap shows up as "test does not exist here" instead of hiding behind a green
check.

Usage: check-no-skips.py <junit.xml> [<junit.xml> ...]
"""

import sys
import xml.etree.ElementTree as ET


def skipped_tests(path):
    """Return [(classname, name, message)] for every skipped testcase."""
    root = ET.parse(path).getroot()
    out = []
    for case in root.iter("testcase"):
        for skip in case.findall("skipped"):
            out.append(
                (
                    case.get("classname", "?"),
                    case.get("name", "?"),
                    (skip.get("message") or skip.text or "").strip(),
                )
            )
    return out


def main(argv):
    if len(argv) < 2:
        print("usage: check-no-skips.py <junit.xml> [...]", file=sys.stderr)
        return 2

    failed = False
    for path in argv[1:]:
        try:
            skips = skipped_tests(path)
        except FileNotFoundError:
            print(f"::error::expected JUnit report not found: {path}")
            failed = True
            continue
        except ET.ParseError as exc:
            print(f"::error::could not parse JUnit report {path}: {exc}")
            failed = True
            continue

        if skips:
            failed = True
            print(f"::error::{len(skips)} skipped test(s) in {path}")
            for classname, name, message in skips:
                print(f"  - {classname}.{name}: {message}")
        else:
            print(f"no skipped tests in {path}")

    if failed:
        print(
            "\nA skipped test means a precondition silently went unmet. Either fix the\n"
            "environment so the test runs, or move the test behind a build tag so it is\n"
            "never scheduled in this context. See issue #201."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
