#!/usr/bin/env python3
"""Logo-Paket für Block-F. Aufruf: python3 brand/entwuerfe/make_kit.py  ->  brand/block-f/*.svg
Danach export_variants.py für Schwarz/Weiß/PNG/Web-Icons (siehe brand/block-f/README.md)."""

import os

from make_entwuerfe import PAPER, SAAL, TINTE, svg
from make_terminal import F_GRAU, F_LOCH, GRAU, pixel_text

OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "block-f")
U = 40  # Zelle der Wortmarke; Symbol: 8 Zellen à 80 = 640


def _pts(poly, u=80, ox=2, oy=1):
    return "M" + " L".join(f"{(ox + x) * u} {(oy + y) * u}" for x, y in poly) + " Z"


def symbol(fill, grey=GRAU, hole=None):
    """Block 640x640 mit ausgestanztem F. hole=Farbe: F wird gefüllt statt durchsichtig (Favicon)."""
    if hole:
        return (f'<path d="M0 0 h640 v640 h-640 Z" fill="{fill}"/><path d="{_pts(F_LOCH)}" fill="{hole}"/>')
    art = f'<path d="M0 0 h640 v640 h-640 Z {_pts(F_LOCH)}" fill="{fill}" fill-rule="evenodd"/>'
    return art + (f'<path d="{_pts(F_GRAU)}" fill="{grey}"/>' if grey else "")


def wort(mer, flim=GRAU, dx=0, dy=0):
    return pixel_text("flimmer", U, [flim] * 4 + [mer] * 3, dx, dy)


def quer(fill, mer, grey=GRAU, flim=GRAU):
    wm, w = wort(mer, flim, 520, 60)
    return f'<g transform="scale(0.625)">{symbol(fill, grey)}</g>{wm}', 520 + w, 400


def hoch(fill, mer, grey=GRAU, flim=GRAU):
    wm, w = wort(mer, flim, 0, 700)
    s = 560
    return f'<g transform="translate({(w - s) / 2:g} 0) scale({s / 640})">{symbol(fill, grey)}</g>{wm}', w, 980


def main():
    os.makedirs(OUT, exist_ok=True)
    files = {
        "flimmer-symbol-dunkel": (symbol(PAPER), 640, 640),
        "flimmer-symbol-hell": (symbol(TINTE), 640, 640),
        "flimmer-symbol-mono": (symbol("#000000", None), 640, 640),
        "flimmer-symbol-klein": (symbol(PAPER, hole=SAAL), 640, 640),
        "flimmer-quer-dunkel": (*quer(PAPER, PAPER),),
        "flimmer-quer-hell": (*quer(TINTE, TINTE),),
        "flimmer-quer-mono": (*quer("#000000", "#000000", None, "#000000"),),
        "flimmer-hoch-dunkel": (*hoch(PAPER, PAPER),),
        "flimmer-hoch-hell": (*hoch(TINTE, TINTE),),
    }
    wm, w = wort(PAPER)
    files["flimmer-wortmarke-dunkel"] = (wm, w, 280)
    wm, w = wort(TINTE)
    files["flimmer-wortmarke-hell"] = (wm, w, 280)
    for name, (body, W, H) in files.items():
        with open(f"{OUT}/{name}.svg", "w") as f:
            f.write(svg(int(W), int(H), body, "Flimmer"))
    # App-Icon: dunkle Kachel, Block auf 60 % der Kante
    icon = f'<rect width="1000" height="1000" fill="{SAAL}"/><g transform="translate(180 180)">{symbol(PAPER)}</g>'
    with open(f"{OUT}/flimmer-app-icon.svg", "w") as f:
        f.write(svg(1000, 1000, icon, "Flimmer"))


if __name__ == "__main__":
    main()
