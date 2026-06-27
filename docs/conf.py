"""Sphinx configuration for the SecretSync documentation site."""

from __future__ import annotations

import os
import subprocess

from pathlib import Path


project = "SecretSync"
author = "Jon Bogaty"
copyright = "2025-2026, Jon Bogaty"  # noqa: A001
html_title = project
html_baseurl = "https://jbcom.github.io/secrets-sync/"


def _release() -> str:
    if env := os.environ.get("DOCS_VERSION"):
        return env
    repo_root = Path(__file__).resolve().parent.parent
    try:
        result = subprocess.run(
            ["git", "describe", "--tags", "--always", "--dirty"],
            cwd=repo_root,
            check=True,
            capture_output=True,
            text=True,
        )
    except (FileNotFoundError, subprocess.CalledProcessError):
        return "dev"
    return result.stdout.strip() or "dev"


release = version = _release()

extensions = [
    "myst_parser",
    "sphinx_copybutton",
    "sphinx.ext.githubpages",
    "sphinx.ext.napoleon",
    "sphinx.ext.intersphinx",
]

source_suffix = {
    ".md": "markdown",
    ".rst": "restructuredtext",
}

exclude_patterns = ["_build", "Thumbs.db", ".DS_Store"]

html_theme = "furo"
html_static_path = ["_static"]
html_css_files = ["secrets-sync.css"]
html_theme_options = {
    "source_repository": "https://github.com/jbcom/secrets-sync/",
    "source_branch": "main",
    "source_directory": "docs/",
    "light_css_variables": {
        "color-brand-primary": "#0f766e",
        "color-brand-content": "#115e59",
        "color-api-name": "#1d4ed8",
        "color-api-pre-name": "#374151",
    },
    "dark_css_variables": {
        "color-brand-primary": "#5eead4",
        "color-brand-content": "#99f6e4",
        "color-api-name": "#93c5fd",
        "color-api-pre-name": "#d1d5db",
    },
}

myst_enable_extensions = [
    "colon_fence",
    "deflist",
    "fieldlist",
    "tasklist",
]
myst_heading_anchors = 3

# gomarkdoc emits package-local symbol links that MyST cannot resolve as
# Sphinx cross-reference targets. Keep warnings strict for hand-written docs
# while suppressing the generated API reference noise.
suppress_warnings = [
    "myst.header",
    "myst.xref_missing",
]

intersphinx_mapping = {
    "python": ("https://docs.python.org/3", None),
}

napoleon_google_docstring = True
napoleon_numpy_docstring = False
napoleon_include_init_with_doc = True
napoleon_use_param = True
napoleon_use_rtype = True
