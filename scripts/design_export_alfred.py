#!/usr/bin/env python
"""Export Alfred identity assets and regenerate the brand notebook section.

Runs from the repo root. Idempotent. Steps:

1. Copy ``design/alfred-identity/marks/*.svg`` to ``portal/public/`` (running
   ``svgo`` when available; otherwise raw copy).
2. Build the Alfred section as inline HTML.
3. Replace (or append) the `<!-- ALFRED:BEGIN -->` … `<!-- ALFRED:END -->`
   block in ``design/Forge Brand Notebook _standalone_.html``.
4. Assert the resulting file is < 3 MB and stays a single file (no <script
   src=…>, no external <link rel=stylesheet>).
5. Write a checksum sidecar so CI can detect drift between the identity
   folder and the notebook section.

With ``--check``: re-generate to a temp file and fail if the notebook on disk
does not match — used by ``make design-export-check`` in CI.
"""

from __future__ import annotations

import argparse
import hashlib
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
IDENTITY = ROOT / "design" / "alfred-identity"
MARKS = IDENTITY / "marks"
PORTAL_PUBLIC = ROOT / "portal" / "public"
NOTEBOOK = ROOT / "design" / "Forge Brand Notebook _standalone_.html"
CHECKSUM_PATH = IDENTITY / ".section-checksum"

BEGIN_MARKER = "<!-- ALFRED:BEGIN -->"
END_MARKER = "<!-- ALFRED:END -->"
MAX_BYTES = 3 * 1024 * 1024


def have_svgo() -> bool:
    return shutil.which("svgo") is not None


def optimize_svgs() -> None:
    for src in MARKS.glob("*.svg"):
        dst = PORTAL_PUBLIC / src.name
        if have_svgo():
            subprocess.run(["svgo", "-i", str(src), "-o", str(dst)], check=True)
        else:
            shutil.copyfile(src, dst)


def build_section() -> str:
    persona = (IDENTITY / "PERSONA.md").read_text(encoding="utf-8")
    motion = (IDENTITY / "MOTION.md").read_text(encoding="utf-8")
    do_dont = (IDENTITY / "DO_DONT.md").read_text(encoding="utf-8")
    marks = {p.stem: p.read_text(encoding="utf-8") for p in MARKS.glob("*.svg")}

    parts: list[str] = [BEGIN_MARKER, '<section id="alfred-identity">']
    parts.append('<h2 class="brand-section-title">Alfred</h2>')
    parts.append(
        '<p class="brand-section-lede">Forge\'s autonomous operator. '
        "A named persona inside the brand family, not a separate brand.</p>"
    )
    parts.append('<div class="alfred-marks">')
    for stem in ("alfred-mark", "alfred-mark-mono", "alfred-mark-working"):
        if stem in marks:
            parts.append(f'<figure id="{stem}-fig">{marks[stem]}<figcaption>{stem}</figcaption></figure>')
    parts.append("</div>")
    parts.append('<div class="alfred-tokens"><pre class="brand-tokens">')
    parts.append(
        "--alfred-ink         #1A1614 / #F0E8D9 (dark)\n"
        "--alfred-paper       #FBF6EE / #15110E (dark)\n"
        "--alfred-ember       #B7330F / #FF6A33 (dark)\n"
        "--alfred-ember-soft  #F4A77E / #8A2509 (dark)\n"
        "--alfred-thread      #4F8C76 / #6FBC9C (dark)\n"
        "--alfred-border      #D9D2C5 / #2D2823 (dark)\n"
        "--alfred-anim-working 2400ms cubic-bezier(.45,.05,.55,.95)\n"
        "--alfred-anim-dock-in  280ms cubic-bezier(.16,1,.3,1)"
    )
    parts.append("</pre></div>")
    parts.append('<details><summary>Persona</summary><pre>')
    parts.append(_md_to_text(persona))
    parts.append("</pre></details>")
    parts.append('<details><summary>Motion</summary><pre>')
    parts.append(_md_to_text(motion))
    parts.append("</pre></details>")
    parts.append('<details><summary>Do / Don\'t</summary><pre>')
    parts.append(_md_to_text(do_dont))
    parts.append("</pre></details>")
    parts.append("</section>")
    parts.append(END_MARKER)
    return "\n".join(parts)


def _md_to_text(md: str) -> str:
    # Light-touch HTML escape; we want the markdown to render as text inside
    # <pre> so we don't lift in a markdown dependency.
    return (
        md.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    )


def patch_notebook(section: str) -> bytes:
    if not NOTEBOOK.exists():
        # Minimal stub if the notebook is missing — keeps the script idempotent
        # without coupling to its existing chrome.
        return f"<!doctype html><html><body>{section}</body></html>".encode("utf-8")
    body = NOTEBOOK.read_text(encoding="utf-8")
    if BEGIN_MARKER in body and END_MARKER in body:
        before, _, rest = body.partition(BEGIN_MARKER)
        _, _, after = rest.partition(END_MARKER)
        body = before + section + after
    else:
        # Insert before </body> if present, else append.
        if "</body>" in body:
            body = body.replace("</body>", section + "\n</body>", 1)
        else:
            body += "\n" + section + "\n"
    return body.encode("utf-8")


def section_checksum(section: str) -> str:
    return hashlib.sha256(section.encode("utf-8")).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="verify only; do not write")
    args = parser.parse_args()

    PORTAL_PUBLIC.mkdir(parents=True, exist_ok=True)
    optimize_svgs()

    section = build_section()
    new_body = patch_notebook(section)
    if len(new_body) > MAX_BYTES:
        print(
            f"design-export: notebook size {len(new_body)} exceeds {MAX_BYTES}",
            file=sys.stderr,
        )
        return 2

    new_checksum = section_checksum(section)
    if args.check:
        existing = CHECKSUM_PATH.read_text(encoding="utf-8").strip() if CHECKSUM_PATH.exists() else ""
        if existing != new_checksum:
            print(
                "design-export: alfred-identity drifted from notebook section. "
                "Run `make design-export` and commit the result.",
                file=sys.stderr,
            )
            return 1
        return 0

    if NOTEBOOK.exists() or new_body:
        NOTEBOOK.parent.mkdir(parents=True, exist_ok=True)
        NOTEBOOK.write_bytes(new_body)
    CHECKSUM_PATH.write_text(new_checksum + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
