#!/usr/bin/env python3
"""Bump direct Go module dependencies whose proxy-reported release is at least 7 days old."""
import json
import subprocess
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path

CUTOFF = datetime.now(timezone.utc) - timedelta(days=7)
ROOT = Path(__file__).resolve().parents[1]


def eligible_updates(cwd: Path) -> list[str]:
    r = subprocess.run(
        ["go", "list", "-m", "-u", "-json", "all"],
        cwd=cwd,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print(f"go list failed in {cwd}: {r.stderr}", file=sys.stderr)
        return []
    dec = json.JSONDecoder()
    idx = 0
    specs: list[str] = []
    data = r.stdout
    while idx < len(data):
        while idx < len(data) and data[idx].isspace():
            idx += 1
        if idx >= len(data):
            break
        obj, end = dec.raw_decode(data, idx)
        idx = end
        if obj.get("Main"):
            continue
        if obj.get("Indirect"):
            continue
        up = obj.get("Update")
        if not up:
            continue
        t = datetime.fromisoformat(up["Time"].replace("Z", "+00:00"))
        if t > CUTOFF:
            continue
        specs.append(f"{up['Path']}@{up['Version']}")
    return specs


def main() -> int:
    failed = False
    for go_mod in sorted(ROOT.glob("plugins/*/go.mod")):
        plug = go_mod.parent
        if plug.name == "_template":
            continue
        cwd = plug
        specs = eligible_updates(cwd)
        if not specs:
            print(f"{plug.name}: no direct updates (7d rule)")
            continue
        print(f"{plug.name}: go get {' '.join(specs)}")
        r = subprocess.run(
            ["go", "get"] + specs,
            cwd=cwd,
        )
        if r.returncode != 0:
            failed = True
            continue
        subprocess.run(["go", "mod", "tidy"], cwd=cwd, check=False)
        r = subprocess.run(["go", "test", "./..."], cwd=cwd)
        if r.returncode != 0:
            print(f"TESTS FAILED: {plug.name}", file=sys.stderr)
            failed = True
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
