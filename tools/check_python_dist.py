"""Verify the generated Python binding wheel publishes under the expected name."""

from __future__ import annotations

import argparse
import email.parser
import sys
import zipfile

from pathlib import Path


def _wheel_metadata(path: Path) -> tuple[str, str]:
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
        version = metadata.get("Version")
        if not version:
            message = f"{path} does not declare a distribution Version"
            raise ValueError(message)
        return name, version


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist-dir", required=True, type=Path)
    parser.add_argument("--name", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    wheels = sorted(args.dist_dir.glob("*.whl"))
    if not wheels:
        print(f"No wheels found in {args.dist_dir}", file=sys.stderr)
        return 1

    failures: list[str] = []
    for wheel in wheels:
        try:
            actual_name, actual_version = _wheel_metadata(wheel)
        except ValueError as exc:
            failures.append(f"{wheel.name}: failed to parse metadata: {exc}")
            continue

        if actual_name != args.name:
            failures.append(
                f"{wheel.name}: expected Name {args.name!r}, got {actual_name!r}"
            )
        if actual_version != args.version:
            failures.append(
                f"{wheel.name}: expected Version {args.version!r}, "
                f"got {actual_version!r}"
            )

    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
