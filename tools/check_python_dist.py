"""Verify the generated Python binding wheel publishes under the expected name."""

from __future__ import annotations

import argparse
import email.parser
import sys
import zipfile

from pathlib import Path


def _wheel_name(path: Path) -> str:
    with zipfile.ZipFile(path) as wheel:
        metadata_paths = [
            name for name in wheel.namelist() if name.endswith(".dist-info/METADATA")
        ]
        if len(metadata_paths) != 1:
            message = (
                f"{path} should contain exactly one METADATA file, "
                f"found {len(metadata_paths)}"
            )
            raise ValueError(message)

        metadata_text = wheel.read(metadata_paths[0]).decode("utf-8")
        metadata = email.parser.Parser().parsestr(metadata_text)
        name = metadata.get("Name")
        if not name:
            message = f"{path} does not declare a distribution Name"
            raise ValueError(message)
        return name


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist-dir", required=True, type=Path)
    parser.add_argument("--name", required=True)
    args = parser.parse_args()

    wheels = sorted(args.dist_dir.glob("*.whl"))
    if not wheels:
        print(f"No wheels found in {args.dist_dir}", file=sys.stderr)
        return 1

    failures: list[str] = []
    for wheel in wheels:
        actual = _wheel_name(wheel)
        if actual != args.name:
            failures.append(f"{wheel.name}: expected {args.name!r}, got {actual!r}")

    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
