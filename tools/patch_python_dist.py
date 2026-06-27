"""Patch gopy-generated packaging metadata to the public distribution name."""

from __future__ import annotations

import argparse
import re
import sys

from pathlib import Path


SETUP_NAME_PATTERNS = (
    re.compile(r"(name\s*=\s*)(['\"])([^'\"]+)(['\"])"),
    re.compile(r"(['\"]name['\"]\s*:\s*)(['\"])([^'\"]+)(['\"])"),
)


def _patch_setup_py(path: Path, expected_name: str) -> bool:
    text = path.read_text(encoding="utf-8")
    for pattern in SETUP_NAME_PATTERNS:
        match = pattern.search(text)
        if match is None:
            continue
        updated = (
            text[: match.start()]
            + match.group(1)
            + match.group(2)
            + expected_name
            + match.group(4)
            + text[match.end() :]
        )
        path.write_text(updated, encoding="utf-8")
        return True
    return False


def _patch_pyproject(path: Path, expected_name: str) -> bool:
    text = path.read_text(encoding="utf-8")
    pattern = re.compile(r"(^name\s*=\s*)(['\"])([^'\"]+)(['\"])", re.MULTILINE)
    if pattern.search(text) is None:
        return False
    updated = pattern.sub(rf"\1\2{expected_name}\4", text, count=1)
    path.write_text(updated, encoding="utf-8")
    return True


def _patch_package_init(package_dir: Path) -> None:
    init_py = package_dir / "secrets_sync" / "__init__.py"
    if not init_py.exists():
        return
    init_py.write_text(
        '''"""Python bindings for the secrets-sync Go runtime."""

# gopy generates public package wrappers in ``secrets_sync.py`` and lower-level
# runtime helpers in ``go.py``. Re-export the package wrapper at top level so
# consumers can use ``import secrets_sync`` directly.
from .secrets_sync import *  # noqa: F401,F403
''',
        encoding="utf-8",
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--package-dir", required=True, type=Path)
    parser.add_argument("--name", required=True)
    args = parser.parse_args()

    _patch_package_init(args.package_dir)

    patched = False
    setup_py = args.package_dir / "setup.py"
    if setup_py.exists():
        patched = _patch_setup_py(setup_py, args.name) or patched

    pyproject = args.package_dir / "pyproject.toml"
    if pyproject.exists():
        patched = _patch_pyproject(pyproject, args.name) or patched

    if patched:
        return 0

    print(
        f"Could not find patchable Python distribution metadata in {args.package_dir}",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
