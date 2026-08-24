#!/usr/bin/env python3
"""Verify Sourcey's static output contains the required public artifacts."""

from __future__ import annotations

import sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

DIST = Path(__file__).resolve().parent.parent / "docs" / "dist"
REQUIRED = (
    "index.html",
    "getting-started.html",
    "pipeline.html",
    "security.html",
    "api.html",
    "api/index.html",
    "llms.txt",
    "llms-full.txt",
    "search-index.json",
    "sitemap.xml",
)
FORBIDDEN = ("sphinx", "docs/_build", "jbcom.github.io/secrets-sync")
BASE_PATH = "/secrets-sync/"


class LocalLinkCollector(HTMLParser):
    """Collect static asset and document links from one generated HTML page."""

    def __init__(self) -> None:
        super().__init__()
        self.links: list[str] = []

    def handle_starttag(self, _tag: str, attrs: list[tuple[str, str | None]]) -> None:
        for name, value in attrs:
            if name in {"href", "src"} and value:
                self.links.append(value)


def local_target(page: Path, href: str) -> Path | None:
    """Resolve a generated same-site link to its expected output file."""
    parts = urlsplit(href)
    if parts.scheme or parts.netloc or href.startswith(("#", "mailto:", "data:")):
        return None

    path = unquote(parts.path)
    if not path:
        return None
    if path.startswith("/"):
        if not path.startswith(BASE_PATH):
            return None
        candidate = DIST / path.removeprefix(BASE_PATH)
    else:
        candidate = page.parent / path

    if path.endswith("/"):
        return candidate / "index.html"
    return candidate


def ensure_api_compatibility_route() -> None:
    """Create the Go-doc tab endpoint Sourcey links to in static HTML mode."""
    route_page = DIST / "api" / "index.html"
    route_page.parent.mkdir(parents=True, exist_ok=True)
    route_page.write_text(
        '<!doctype html><html lang="en"><head>'
        '<meta charset="utf-8"><meta http-equiv="refresh" '
        'content="0; url=../api.html">'
        '<link rel="canonical" href="../api.html">'
        "<title>SecretSync Go API</title></head><body>"
        '<p>Opening the <a href="../api.html">SecretSync Go API</a>…</p>'
        "<script>location.replace('../api.html')</script></body></html>\n",
        encoding="utf-8",
    )


def validate_local_links() -> list[str]:
    broken: list[str] = []
    for page in DIST.rglob("*.html"):
        parser = LocalLinkCollector()
        parser.feed(page.read_text(encoding="utf-8"))
        for href in parser.links:
            target = local_target(page, href)
            if target is not None and not target.is_file():
                broken.append(f"{page.relative_to(DIST)} -> {href}")
    return broken


def main() -> int:
    if not (DIST / "api.html").is_file():
        print(f"Sourcey output is missing {DIST / 'api.html'}", file=sys.stderr)
        return 1
    ensure_api_compatibility_route()

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

    broken_links = validate_local_links()
    if broken_links:
        print(
            "Sourcey output has broken same-site links:\n"
            + "\n".join(broken_links[:20]),
            file=sys.stderr,
        )
        return 1

    print(f"Validated Sourcey output in {DIST}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
