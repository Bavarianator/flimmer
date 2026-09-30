#!/usr/bin/env python3
"""Fünf Logo-Entwürfe für Flimmer. Aufruf: python3 brand/entwuerfe/make_entwuerfe.py
Schreibt pro Entwurf <id>-symbol.svg (Kachel) und <id>-quer.svg (Lockup). Farben aus tokens.json."""

import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
from make import wordmark  # noqa: E402  (Wortmarke als Pfade aus FlimmerPlakat-800)

SAAL, PAPER, TINTE = "#11100e", "#ece7dd", "#16140f"
SALBEI, OCKER, ZIEGEL = "#8fae86", "#cfae5c", "#d4826f"
SALBEI_D, ZIEGEL_D = "#3d6b38", "#a13f2c"


def poly(pts, fill):
    return f'<path d="M{" L".join(f"{x:.1f} {y:.1f}" for x, y in pts)} Z" fill="{fill}"/>'


def rect(x, y, w, h, fill):
    return f'<path d="M{x} {y} h{w} v{h} h{-w} Z" fill="{fill}"/>'


def a_judder():
    """Play-Dreieck, dessen Bild nicht ganz sitzt: Ocker rutscht hinter dem Papier."""
    tri = [(320, 260), (320, 820), (800, 540)]
    ghost = [(x - 100, y - 70) for x, y in tri]
    return SAAL, poly(ghost, OCKER) + poly(tri, PAPER)


def b_filmband():
    """Filmstreifen, dessen drei Bilder die Ampel sind: läuft, umgepackt, umgerechnet."""
    strip = "M300 100 H700 V900 H300 Z"
    holes = ""
    for y in (158, 405, 652):
        for x in (322, 638):
            holes += f"M{x} {y + 60} h40 v70 h-40 Z "
    art = f'<path d="{strip} {holes}" fill="{PAPER}" fill-rule="evenodd"/>'
    for y, c in zip((158, 405, 652), (SALBEI, OCKER, ZIEGEL)):
        art += rect(380, y, 240, 190, c)
    return SAAL, art


def c_lichtkegel():
    """Projektor: Linse und Lichtkegel in drei Strahlen, der mittlere flackert ocker."""
    cx, cy, x_end, r0, g = 230, 500, 820, 120, 4
    span = math.degrees(math.atan(270 / (x_end - cx)))
    w = (2 * span - 2 * g) / 3
    art = f'<circle cx="{cx}" cy="{cy}" r="70" fill="{PAPER}"/>'
    for i in range(3):
        a1 = -span + i * (w + g)
        a2 = a1 + w
        t1, t2 = (math.radians(a) for a in (a1, a2))
        pts = [
            (cx + r0 * math.cos(t1), cy + r0 * math.sin(t1)),
            (cx + r0 * math.cos(t2), cy + r0 * math.sin(t2)),
            (x_end, cy + (x_end - cx) * math.tan(t2)),
            (x_end, cy + (x_end - cx) * math.tan(t1)),
        ]
        art += poly(pts, OCKER if i == 1 else PAPER)
    return ZIEGEL_D, art


def d_zu_zweit():
    """Leinwand und zwei Köpfe davor: Gemeinsam schauen. Ziegel und Ocker sind die beiden Gäste."""
    art = f'<path d="M140 150 h720 v420 h-720 Z M450 265 V455 L615 360 Z" fill="{PAPER}" fill-rule="evenodd"/>'
    for cx, c in ((335, ZIEGEL), (665, OCKER)):
        art += f'<circle cx="{cx}" cy="730" r="100" fill="{c}"/>'
        art += f'<path d="M{cx - 150} 950 A150 100 0 0 1 {cx + 150} 950 Z" fill="{c}"/>'
    return SAAL, art


def e_pixel():
    """F aus 3×3 Bildpunkten; der Punkt oben rechts ist ocker und hängt eine Zeile zu tief."""
    art = ""
    for cx, cy in [(0, 0), (1, 0), (0, 1), (1, 1), (0, 2)]:
        art += rect(160 + cx * 240, 160 + cy * 240, 200, 200, PAPER)
    art += rect(160 + 2 * 240, 160 + 70, 200, 200, OCKER)
    return SALBEI_D, art


CONCEPTS = {
    "a-judder": (a_judder, "Judder", "Ein Play-Dreieck, dessen zweites Bild nicht ganz sitzt: so sieht Flimmern aus."),
    "b-filmband": (b_filmband, "Ampelband", "Ein Filmstreifen, dessen drei Bilder die Wiedergabe-Ampel sind."),
    "c-lichtkegel": (c_lichtkegel, "Lichtkegel", "Der Projektor wirft drei Strahlen, der mittlere flackert."),
    "d-zu-zweit": (d_zu_zweit, "Zu zweit", "Zwei Köpfe vor der Leinwand: gemeinsam schauen."),
    "e-pixel": (e_pixel, "Pixel-F", "Ein F aus neun Bildpunkten, einer flackert ocker."),
}


def svg(w, h, body, title):
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}" '
        f'role="img" aria-label="{title}"><title>{title}</title>{body}</svg>\n'
    )


def main():
    wm, wm_w, _ = wordmark(cap=300)
    for key, (fn, name, _) in CONCEPTS.items():
        bg, art = fn()
        tile = f'<rect width="1000" height="1000" fill="{bg}"/>{art}'
        with open(f"{HERE}/{key}-symbol.svg", "w") as f:
            f.write(svg(1000, 1000, tile, f"Flimmer {name}"))
        x = 620 + 120
        lock = (
            f'<g transform="translate(0 190) scale(0.62)">{tile}</g>'
            f'<g transform="translate({x} 650)" fill="{TINTE}">{wm}</g>'
        )
        with open(f"{HERE}/{key}-quer.svg", "w") as f:
            f.write(svg(int(x + wm_w) + 40, 1000, lock, f"Flimmer {name}"))


if __name__ == "__main__":
    main()
