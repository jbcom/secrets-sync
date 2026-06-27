from __future__ import annotations

import subprocess
import sys
import zipfile

from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
PYTHON_DIST = "secrets-sync-python-binding"
VERSION = "2.3.0"


def write_minimal_wheel(
    path: Path,
    *,
    name: str = PYTHON_DIST,
    version: str = VERSION,
    tag: str = "py3-none-any",
) -> None:
    with zipfile.ZipFile(path, "w") as archive:
        archive.writestr(
            f"{name}-{version}.dist-info/METADATA",
            f"Name: {name}\nVersion: {version}\n",
        )
        archive.writestr(
            f"{name}-{version}.dist-info/WHEEL",
            f"Wheel-Version: 1.0\nGenerator: test\nRoot-Is-Purelib: false\nTag: {tag}\n",
        )


def run_tool(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, *args],
        cwd=REPO_ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


def test_patch_python_dist_updates_setup_py_metadata(tmp_path: Path) -> None:
    package_dir = tmp_path / "secrets_sync"
    init_py = package_dir / "secrets_sync" / "__init__.py"
    init_py.parent.mkdir(parents=True)
    init_py.write_text("# generated\n", encoding="utf-8")
    setup_py = package_dir / "setup.py"
    setup_py.write_text(
        'setup(name="secrets_sync", version="0.1.0")\n',
        encoding="utf-8",
    )

    result = run_tool(
        "tools/patch_python_dist.py",
        "--package-dir",
        str(package_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )

    assert result.returncode == 0, result.stderr
    assert setup_py.read_text(encoding="utf-8") == (
        f'setup(name="{PYTHON_DIST}", version="{VERSION}")\n'
    )
    assert "from .secrets_sync import *" in init_py.read_text(encoding="utf-8")


def test_patch_python_dist_updates_pyproject_metadata(tmp_path: Path) -> None:
    package_dir = tmp_path / "secrets_sync"
    package_dir.mkdir()
    pyproject = package_dir / "pyproject.toml"
    pyproject.write_text(
        '[project]\n  name = "secrets_sync"\n  version = "0.1.0"\n',
        encoding="utf-8",
    )

    result = run_tool(
        "tools/patch_python_dist.py",
        "--package-dir",
        str(package_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )

    assert result.returncode == 0, result.stderr
    assert pyproject.read_text(encoding="utf-8") == (
        f'[project]\n  name = "{PYTHON_DIST}"\n  version = "{VERSION}"\n'
    )


def test_check_python_dist_requires_expected_name_and_version(tmp_path: Path) -> None:
    dist_dir = tmp_path / "dist"
    dist_dir.mkdir()
    wheel = dist_dir / f"{PYTHON_DIST}-{VERSION}-py3-none-any.whl"
    write_minimal_wheel(wheel)

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )
    assert result.returncode == 0, result.stderr

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        "wrong-name",
        "--version",
        VERSION,
    )
    assert result.returncode == 1
    assert "expected Name 'wrong-name'" in result.stderr

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        "9.9.9",
    )
    assert result.returncode == 1
    assert "expected Version '9.9.9'" in result.stderr


def test_check_python_dist_rejects_unrepaired_linux_wheel_tags(
    tmp_path: Path,
) -> None:
    dist_dir = tmp_path / "dist"
    dist_dir.mkdir()
    wheel = dist_dir / f"{PYTHON_DIST}-{VERSION}-cp312-cp312-linux_x86_64.whl"
    write_minimal_wheel(wheel, tag="cp312-cp312-linux_x86_64")

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )

    assert result.returncode == 1
    assert "unsupported Linux wheel tag(s) cp312-cp312-linux_x86_64" in result.stderr
    assert "auditwheel repair" in result.stderr


def test_check_python_dist_accepts_repaired_manylinux_wheel_tags(
    tmp_path: Path,
) -> None:
    dist_dir = tmp_path / "dist"
    dist_dir.mkdir()
    wheel = dist_dir / f"{PYTHON_DIST}-{VERSION}-cp312-cp312-manylinux_2_34_x86_64.whl"
    write_minimal_wheel(wheel, tag="cp312-cp312-manylinux_2_34_x86_64")

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )

    assert result.returncode == 0, result.stderr


def test_check_python_dist_reports_malformed_wheel_without_traceback(
    tmp_path: Path,
) -> None:
    dist_dir = tmp_path / "dist"
    dist_dir.mkdir()
    wheel = dist_dir / f"{PYTHON_DIST}-{VERSION}-py3-none-any.whl"
    with zipfile.ZipFile(wheel, "w") as archive:
        archive.writestr("secrets_sync/__init__.py", "")

    result = run_tool(
        "tools/check_python_dist.py",
        "--dist-dir",
        str(dist_dir),
        "--name",
        PYTHON_DIST,
        "--version",
        VERSION,
    )

    assert result.returncode == 1
    assert "failed to parse metadata" in result.stderr
    assert "Traceback" not in result.stderr
