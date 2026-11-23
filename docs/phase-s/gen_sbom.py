#!/usr/bin/env python3
"""Phase S SBOM generator (no third-party deps).

Reads `go list -m -json all` output (concatenated JSON objects) from
_modules.json and go.sum (for h1: hashes) from the repo root, and emits:
  - sbom.cyclonedx.json : CycloneDX 1.5 SBOM (auditable inventory)
  - DEPENDENCIES.md     : human-readable inventory table

This is a Phase-S artifact. It does not modify the Go module or its dependencies.
Run from the repo root:  python -I docs/phase-s/gen_sbom.py
"""
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
MODJSON = os.path.join(ROOT, "docs", "phase-s", "modules.json")
GOSUM = os.path.join(ROOT, "go.sum")
OUT_SBOM = os.path.join(ROOT, "docs", "phase-s", "sbom.cyclonedx.json")
OUT_MD = os.path.join(ROOT, "docs", "phase-s", "DEPENDENCIES.md")


def load_modules():
    """Parse the stream of concatenated JSON objects from go list -m -json all."""
    mods = []
    with open(MODJSON, "r", encoding="utf-8") as f:
        dec = json.JSONDecoder()
        buf = f.read().strip()
    i, n = 0, len(buf)
    while i < n:
        while i < n and buf[i] in " \t\r\n":
            i += 1
        if i >= n:
            break
        obj, end = dec.raw_decode(buf, i)
        mods.append(obj)
        i = end
    return mods


def load_gosum():
    """Map module@version -> h1: hash from go.sum."""
    h = {}
    if not os.path.exists(GOSUM):
        return h
    with open(GOSUM, "r", encoding="utf-8") as f:
        for line in f:
            parts = line.split()
            if len(parts) >= 3 and parts[2].startswith("h1:"):
                h[parts[0] + "@" + parts[1]] = parts[2]
    return h


def main():
    mods = load_modules()
    gosum = load_gosum()
    components, rows = [], []
    for m in sorted(mods, key=lambda x: x.get("Path", "")):
        path = m.get("Path", "")
        ver = m.get("Version", "")
        if m.get("Main"):
            continue  # the main module is the subject, not a component
        if not ver:
            continue
        hashes = []
        h1 = gosum.get(path + "@" + ver)
        if h1:
            hashes.append({"alg": "SHA-256", "content": h1.split(":", 1)[1]})
        comp = {
            "type": "library",
            "name": path,
            "version": ver,
            "purl": "pkg:golang/%s@%s" % (path, ver),
        }
        if hashes:
            comp["hashes"] = hashes
        if m.get("Indirect"):
            comp["scope"] = "required"
        components.append(comp)
        rows.append((path, ver, "indirect" if m.get("Indirect") else "direct", h1 or ""))

    sbom = {
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "version": 1,
        "metadata": {
            "timestamp": "2026-11-23T00:00:00Z",
            "tools": [{"vendor": "BLVChain", "name": "phase-s-gen_sbom", "version": "1.0"}],
            "component": {
                "type": "application",
                "name": "blvchain/core",
                "version": "phase-2",
            },
        },
        "components": components,
    }
    with open(OUT_SBOM, "w", encoding="utf-8") as f:
        json.dump(sbom, f, indent=2)
        f.write("\n")

    with open(OUT_MD, "w", encoding="utf-8") as f:
        f.write("# Phase S dependency inventory (SBOM, go.sum-pinned)\n\n")
        f.write("Generated from `go list -m -json all` + `go.sum`. Machine-readable ")
        f.write("form: `sbom.cyclonedx.json` (CycloneDX 1.5).\n\n")
        f.write("| Module | Version | Direct/Indirect | go.sum h1 |\n")
        f.write("|---|---|---|---|\n")
        for path, ver, di, h1 in rows:
            f.write("| `%s` | `%s` | %s | `%s` |\n" % (path, ver, di, (h1[:16] + "…" ) if h1 else ""))
    print("components:", len(components), "-> ", OUT_SBOM)


if __name__ == "__main__":
    sys.exit(main())