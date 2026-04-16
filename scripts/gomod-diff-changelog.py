#!/usr/bin/env python3
"""Parse git diff for a go.mod path vs origin/main; emit changelog bullet lines."""
from __future__ import annotations

import os
import re
import subprocess
import sys

BASE_REF = os.environ.get("BASE_REF", "origin/main")

# require line: module path, version, optional // comments
LINE_RE = re.compile(
    r"^(?P<mod>\S+)\s+(?P<ver>\S+)(?:\s+(?P<tail>//.*))?$"
)

SKIP_FIRST = frozenset(
    {
        "go",
        "toolchain",
        "module",
        "replace",
        "exclude",
        "retract",
        "require",
    }
)


def _is_indirect(tail: str | None) -> bool:
    if not tail:
        return False
    return "indirect" in tail.split()


def _parse_diff_line(body: str) -> tuple[str, str, bool] | None:
    body = body.strip()
    if not body or body.startswith("//"):
        return None
    if body in ("(", ")"):
        return None
    m = LINE_RE.match(body)
    if not m:
        return None
    mod, ver, tail = m.group("mod"), m.group("ver"), m.group("tail")
    if mod in SKIP_FIRST:
        return None
    return mod, ver, _is_indirect(tail)


def bullets_from_diff(diff_text: str) -> list[str]:
    removed: dict[str, tuple[str, bool]] = {}
    added: dict[str, tuple[str, bool]] = {}

    for line in diff_text.splitlines():
        if not line:
            continue
        if line.startswith("---") or line.startswith("+++") or line.startswith("@@"):
            continue
        sign = line[0]
        if sign == "-" and not line.startswith("---"):
            parsed = _parse_diff_line(line[1:])
            if parsed:
                mod, ver, ind = parsed
                removed[mod] = (ver, ind)
        elif sign == "+" and not line.startswith("+++"):
            parsed = _parse_diff_line(line[1:])
            if parsed:
                mod, ver, ind = parsed
                added[mod] = (ver, ind)

    direct: list[str] = []
    indirect_bump = False

    all_mods = set(removed) | set(added)
    for mod in sorted(all_mods):
        if mod not in added:
            continue
        new_ver, new_ind = added[mod]
        if mod not in removed:
            if new_ind:
                indirect_bump = True
            else:
                direct.append(f"Bump library {mod} to {new_ver}")
            continue

        old_ver, old_ind = removed[mod]
        if old_ver == new_ver:
            continue

        if old_ind and new_ind:
            indirect_bump = True
        elif not old_ind and not new_ind:
            direct.append(f"Bump library {mod} to {new_ver}")
        else:
            if new_ind:
                indirect_bump = True
            else:
                direct.append(f"Bump library {mod} to {new_ver}")

    out = direct[:]
    if indirect_bump:
        out.append("Bump indirect libraries dependencies")
    return out


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: gomod-diff-changelog.py <path/to/go.mod>", file=sys.stderr)
        return 2
    gomod_rel = sys.argv[1]
    p = subprocess.run(
        ["git", "diff", BASE_REF, "--", gomod_rel],
        capture_output=True,
        text=True,
    )
    if p.returncode != 0:
        return 1
    diff = p.stdout
    if not diff.strip():
        return 0
    for line in bullets_from_diff(diff):
        print(line)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
