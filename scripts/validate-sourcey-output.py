#!/usr/bin/env python3
"""Verify Sourcey's static output contains the required public artifacts."""

from __future__ import annotations

import sys
from pathlib import Path

DIST = Path(__file__).resolve().parent.parent / "docs" / "dist"
REQUIRED = (
    "index.html",
    "getting-started.html",
    "pipeline.html",
    "security.html",
    "api.html",
    "llms.txt",
    "llms-full.txt",
    "search-index.json",
    "sitemap.xml",
)
FORBIDDEN = ("sphinx", "docs/_build", "jbcom.github.io/secrets-sync")


def main() -> int:
    missing = [name for name in REQUIRED if not (DIST / name).is_file()]
    if missing:
        print(
            f"Sourcey output is missing required files: {', '.join(missing)}",
            file=sys.stderr,
        )
        return 1

    for context_file in (DIST / "llms.txt", DIST / "llms-full.txt"):
        content = context_file.read_text(encoding="utf-8")
        if not content.strip():
            print(f"{context_file} is empty", file=sys.stderr)
            return 1
        stale = [needle for needle in FORBIDDEN if needle in content]
        if stale:
            print(
                f"{context_file} contains stale documentation references: {', '.join(stale)}",
                file=sys.stderr,
            )
            return 1

    print(f"Validated Sourcey output in {DIST}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
