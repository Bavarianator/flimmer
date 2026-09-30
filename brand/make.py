#!/usr/bin/env python3
"""Erzeugt alle Flimmer-Logos aus einem Ort.

Aufruf:  python3 brand/make.py
Schreibt brand/svg/*.svg und brand/png/*.png (rsvg-convert muss da sein).

Die Wortmarke kommt aus FlimmerPlakat-800.woff2 (web/public/fonts) als Pfad,
damit die Logos ohne Schriftenart im Browser, in Reddit-Posts und in der CI
gleich aussehen. Wer die Datei austauscht, erzeugt die Logos einfach neu.
"""

from __future__ import annotations

import os
import subprocess

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FONT = os.path.join(ROOT, "web", "public", "fonts", "FlimmerPlakat-800.woff2")
SVG_DIR = os.path.join(ROOT, "brand", "svg")
PNG_DIR = os.path.join(ROOT, "brand", "png")

# Farben aus web/src/design/tokens.json
SAAL = "#11100e"
PAPER = "#ece7dd"
TINTE = "#16140f"
SALBEI = "#8fae86"
OCKER = "#cfae5c"
ZIEGEL = "#d4826f"

# --- Die Marke -------------------------------------------------------------
# Ein Plakat-F im 1000er-Raster. Der rechte Teil des oberen Arms ist um 90
# Einheiten nach unten gerutscht und ocker: der Flacker.
F_PAPER = "M180 80 H600 V280 H380 V460 H700 V660 H380 V920 H180 Z"
F_FLACK = "M640 170 H820 V370 H640 Z"

# Ampel-Marke: Punkt, Halbpunkt, Ring. Ohne Farbe an den Formen zu lesen.
AMPEL = (
    f'<circle cx="500" cy="300" r="130" fill="{SALBEI}"/>'
    f'<path d="M500 170 A130 130 0 0 1 500 430 Z" fill="{OCKER}"/>'
    f'<circle cx="500" cy="700" r="130" fill="none" stroke="{ZIEGEL}" stroke-width="56"/>'
)


# --- Wortmarke -------------------------------------------------------------


def wordmark(text: str = "FLIMMER", tracking_em: float = 0.05, cap: float = 1.0):
    """Wortmarke als Pfade: (markup, breite, hoehe) mit Oberkante bei y=0."""
    font = TTFont(FONT)
    glyphs = font.getGlyphSet()
    upem = font["head"].unitsPerEm
    tracking = tracking_em * upem

    # Erst alle Buchstaben einzeln setzen, damit die Laufweite stimmt.
    placed: list[tuple[float, str]] = []
    pen = BoundsPen(glyphs)
    x = 0.0
    for ch in text:
        name = font.getBestCmap().get(ord(ch))
        spen = SVGPathPen(glyphs)
        glyphs[name].draw(spen)
        placed.append((x, spen.getCommands()))
        glyphs[name].draw(pen)
        x += glyphs[name].width + tracking
    x_min, y_min, _x_max, _y_max = pen.bounds
    total_em = x - tracking

    k = cap / (pen.bounds[3] - pen.bounds[1])
    paths = []
    for dx, d in placed:
        if d:
            paths.append(
                f'<path transform="translate({(dx - x_min) * k:.2f} {-y_min * k:.2f}) '
                f'scale({k:.6f} {-k:.6f})" d="{d}"/>'
            )
    return "".join(paths), total_em * k, cap


# --- Dateien schreiben -----------------------------------------------------


def svg(
    w: int, h: int, body: str, bg: str | None = None, title: str = "Flimmer"
) -> str:
    b = f'  <rect width="{w}" height="{h}" fill="{bg}"/>\n' if bg else ""
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" '
        f'width="{w}" height="{h}" role="img" aria-label="{title}">\n'
        f"  <title>{title}</title>\n{b}  {body}\n</svg>\n"
    )


def mark(paper: str, flack: str) -> str:
    return f'<path d="{F_PAPER}" fill="{paper}"/><path d="{F_FLACK}" fill="{flack}"/>'


def plain_mark(paper: str) -> str:
    return f'<path d="{F_PAPER}" fill="{paper}"/>'


def main() -> None:
    os.makedirs(SVG_DIR, exist_ok=True)
    os.makedirs(PNG_DIR, exist_ok=True)
    wm, wm_w, wm_h = wordmark()

    files: dict[str, str] = {}

    # Marke auf hellem Grund, in Farbe
    files["flimmer-mark-hell.svg"] = svg(
        1000, 1000, mark(PAPER, OCKER), TINTE, "Flimmer-Marke"
    )
    # Marke einfarbig, für dunkle Untergründe
    files["flimmer-mark.svg"] = svg(
        1000, 1000, mark(PAPER, PAPER), None, "Flimmer-Marke"
    )
    # Marke einfarbig, für helle Untergründe
    files["flimmer-mark-dunkel.svg"] = svg(
        1000, 1000, mark(TINTE, TINTE), None, "Flimmer-Marke"
    )
    # Kachel dunkel: App-Icon, Profilbild, GitHub-Avatar
    files["flimmer-icon.svg"] = svg(1000, 1000, mark(PAPER, OCKER), SAAL, "Flimmer")
    # Kachel hell: Papier-Thema
    files["flimmer-icon-hell.svg"] = svg(
        1000, 1000, mark(TINTE, OCKER), "#f3efe6", "Flimmer"
    )
    # Kachel einfarbig
    files["flimmer-icon-mono.svg"] = svg(
        1000, 1000, mark(PAPER, PAPER), SAAL, "Flimmer"
    )

    # Wortmarke allein (Versalien, bündig auf 1000er Oberkante)
    files["flimmer-wordmark.svg"] = svg(int(wm_w) + 8, 1000, wm, None, "Flimmer")

    # Lockup quer: Marke links, Wortmarke rechts
    lw = int(1000 * 0.62 + 140 + wm_w) + 20
    lock = (
        f'<g transform="translate(0 20) scale(0.62)">{mark(PAPER, OCKER)}</g>'
        f'<g transform="translate({1000 * 0.62 + 140:.0f} 310)">{wm}</g>'
    )
    files["flimmer-lockup-quer.svg"] = svg(lw, 1000, lock, None, "Flimmer")
    files["flimmer-lockup-quer-hell.svg"] = (
        svg(lw, 1000, lock.replace(flack_placeholder := OCKER, PAPER), None, "Flimmer")
        if False
        else svg(
            lw,
            1000,
            lock.replace(f'fill="{OCKER}"', f'fill="{PAPER}"'),
            None,
            "Flimmer",
        )
    )

    # Lockup quer auf Kachel, für Header und Social
    kh = 520
    k_scale = kh / 1000.0
    kw = int(lw * k_scale) + 200
    files["flimmer-lockup-kachel.svg"] = svg(
        kw,
        kh,
        f'<g transform="translate(100 100) scale({k_scale:.4f})">'
        f'<g transform="translate(0 20) scale(0.62)">{mark(PAPER, OCKER)}</g>'
        f'<g transform="translate({1000 * 0.62 + 140:.0f} 310)">{wm}</g></g>',
        SAAL,
        "Flimmer",
    )

    # Lockup hoch: Wortmarke über der Marke, Posterformat für Social
    sh = 380 + 120 + 1000 + 120
    files["flimmer-lockup-hoch.svg"] = svg(
        int(wm_w) + 400,
        sh,
        f'<g transform="translate(200 120)">{wm}</g>'
        f'<g transform="translate(200 620) scale(1.0)">{mark(PAPER, OCKER)}</g>',
        SAAL,
        "Flimmer",
    )

    # Ampel-Marke
    files["flimmer-ampel.svg"] = svg(1000, 1000, AMPEL, SAAL, "Flimmer-Ampel")
    files["flimmer-ampel-hell.svg"] = svg(1000, 1000, AMPEL, None, "Flimmer-Ampel")

    # Favicon: nur das F, sonst zu viel auf 16 px
    files["favicon.svg"] = svg(1000, 1000, plain_mark(PAPER), SAAL, "Flimmer")

    for name, content in files.items():
        with open(os.path.join(SVG_DIR, name), "w", encoding="utf-8") as fh:
            fh.write(content)

    # PNGs
    sizes = {
        "flimmer-mark-1000.png": ("flimmer-mark.svg", 1000, 1000),
        "flimmer-mark-hell-1000.png": ("flimmer-mark-hell.svg", 1000, 1000),
        "flimmer-icon-512.png": ("flimmer-icon.svg", 512, 512),
        "flimmer-icon-1024.png": ("flimmer-icon.svg", 1024, 1024),
        "flimmer-icon-hell-512.png": ("flimmer-icon-hell.svg", 512, 512),
        "flimmer-icon-mono-512.png": ("flimmer-icon-mono.svg", 512, 512),
        "flimmer-ampel-512.png": ("flimmer-ampel.svg", 512, 512),
        "flimmer-lockup-quer-1200.png": ("flimmer-lockup-quer.svg", 1200, None),
        "flimmer-lockup-quer-hell-1200.png": (
            "flimmer-lockup-quer-hell.svg",
            1200,
            None,
        ),
        "flimmer-lockup-kachel-1200.png": ("flimmer-lockup-kachel.svg", 1200, None),
        "flimmer-lockup-hoch-1200.png": ("flimmer-lockup-hoch.svg", 1200, None),
        "favicon-16.png": ("favicon.svg", 16, 16),
        "favicon-32.png": ("favicon.svg", 32, 32),
        "favicon-180.png": ("favicon.svg", 180, 180),
    }
    for out, (src, w, h) in sizes.items():
        cmd = ["rsvg-convert", "-w", str(w), "-o", os.path.join(PNG_DIR, out)]
        if h:
            cmd += ["-h", str(h)]
        cmd.append(os.path.join(SVG_DIR, src))
        subprocess.run(cmd, check=True, timeout=60)

    print(f"{len(files)} SVGs, {len(sizes)} PNGs geschrieben.")


if __name__ == "__main__":
    main()
