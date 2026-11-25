#!/usr/bin/env python3
"""Generate the Katenary man page from the Zensical build."""

from __future__ import annotations

import datetime
import subprocess
import tempfile
from importlib.metadata import version
from pathlib import Path

from bs4 import BeautifulSoup, Comment

from preprocess import to_remove

HERE = Path(__file__).resolve().parent
SITE = HERE / "site"
OUTPUT = HERE / "share" / "man" / "man1" / "katenary.1"
INPUT_PAGES = [SITE / "usage" / "index.html", SITE / "labels" / "index.html"]
TITLE = "Katenary"
SECTION = "1"
HEADER = "Katenary helm chart generator"


def page_content(path: Path) -> str:
    soup = BeautifulSoup(path.read_text(encoding="utf8"), "html.parser")
    article = soup.select_one("article.md-content__inner")
    if article is None:
        raise SystemExit(f"cannot find the page content in {path}")
    for element in article.find_all(to_remove):
        element.decompose()
    for comment in article.find_all(string=lambda text: isinstance(text, Comment)):
        comment.extract()
    return article.decode_contents()


def main() -> None:
    for page in INPUT_PAGES:
        if not page.is_file():
            raise SystemExit(
                f"{page} is missing, run 'zensical build' before generating the man page"
            )

    html = "\n\n".join(page_content(page) for page in INPUT_PAGES)

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        "w", prefix="katenary_manpage_", suffix=".1.html", encoding="utf8"
    ) as temp_file:
        temp_file.write(html)
        temp_file.flush()

        try:
            zensical_version = version("zensical")
        except Exception:  # noqa: BLE001 - the version is only informative
            zensical_version = "unknown"

        pandoc = [
            "pandoc",
            "--verbose",
            "--standalone",
            "--wrap=none",
            f"-Vtitle:{TITLE}",
            f"-Vsection:{SECTION}",
            f"-Vdate:{datetime.date.today().isoformat()}",
            f"-Vfooter:zensical v{zensical_version}",
            f"-Vheader:{HEADER}",
            "--to",
            "man",
            temp_file.name,
            "-o",
            str(OUTPUT),
        ]
        subprocess.run(pandoc, check=True)  # noqa: S603

    print(f"Generated manpage {OUTPUT}")


if __name__ == "__main__":
    main()
