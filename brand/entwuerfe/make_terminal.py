#!/usr/bin/env python3
"""Drei Flimmer-Entwürfe im Terminal-Stil (Pixelblöcke, warmes Grau, zweifarbig).
Aufruf: python3 brand/entwuerfe/make_terminal.py  ->  t-*-symbol.svg, t-*-quer.svg"""

import os

from make_entwuerfe import HERE, OCKER, PAPER, SAAL, rect, svg

GRAU = "#78716a"  # --linie-stark aus tokens.json

# Kleinbuchstaben auf 5 Zeilen x-Höhe, Oberlängen 7 Zeilen; "X" = gefüllter Block.
GLYPHEN = {
    "f": ["XXX.", "X...", "XXX.", "X...", "X...", "X...", "X..."],
    "l": ["XX.", ".X.", ".X.", ".X.", ".X.", ".X.", ".XX"],
    "i": [".X.", "...", "XX.", ".X.", ".X.", ".X.", ".XX"],
    "m": [".....", ".....", "XXXXX", "X.X.X", "X.X.X", "X.X.X", "X.X.X"],
    "e": [".....", ".....", "XXXX", "X..X", "XXXX", "X...", "XXXX"],
    "r": [".....", ".....", "X.XX", "XX..", "X...", "X...", "X..."],
}


def pixel_text(text, u, colors, x0=0, y0=0):
    """Text als Blöcke der Kantenlänge u. colors[i] färbt Buchstabe i. Gibt (Markup, Breite) zurück.
    Läufe werden 0,5 Einheiten überlappt, damit zwischen Blöcken keine Haarlinien entstehen."""
    out, x = "", x0
    for ch, col in zip(text, colors):
        rows = GLYPHEN[ch]
        d = ""
        for r, row in enumerate(rows):
            c = 0
            while c < len(row):
                if row[c] == "X":
                    n = 1
                    while c + n < len(row) and row[c + n] == "X":
                        n += 1
                    d += f"M{x + c * u - .5:g} {y0 + r * u - .5:g} h{n * u + 1:g} v{u + 1:g} h{-(n * u + 1):g} Z"
                    c += n
                else:
                    c += 1
        out += f'<path d="{d}" fill="{col}"/>'
        x += (len(rows[0]) + 1) * u
    return out, x - x0 - u


def block(hole, grey, u=80, x0=180, y0=180, cells=8, hx=2, hy=1):
    """Heller Block mit ausgeschnittener Form; die untere Hälfte des Lochs ist grau (zweifarbig)."""
    n = cells * u

    # Loch als Polygon aus Zellkoordinaten in Tile-Koordinaten umrechnen
    def tile(poly):
        return "M" + " L".join(f"{x0 + (hx + px) * u} {y0 + (hy + py) * u}" for px, py in poly) + " Z"

    d = f"M{x0} {y0} h{n} v{n} h{-n} Z {tile(hole)}"
    return (
        f'<path d="{d}" fill="{PAPER}" fill-rule="evenodd"/>'
        f'<path d="{tile(grey)}" fill="{GRAU}"/>'
    )


F_LOCH = [(0, 0), (4, 0), (4, 1), (1, 1), (1, 2), (3, 2), (3, 3), (1, 3), (1, 6), (0, 6)]
F_GRAU = [(0, 3), (1, 3), (1, 6), (0, 6)]
PLAY_LOCH = [(0, 0), (2, 0), (2, 1), (4, 1), (4, 2), (6, 2), (6, 4), (4, 4), (4, 5), (2, 5), (2, 6), (0, 6)]
PLAY_GRAU = [(0, 3), (6, 3), (6, 4), (4, 4), (4, 5), (2, 5), (2, 6), (0, 6)]


def t_a():
    return SAAL, block(F_LOCH, F_GRAU)


def t_b():
    """Kleines f mit blinkendem Cursor. Der Cursor ist der Flacker."""
    f, w = pixel_text("f", 100, [PAPER], 150, 150)
    return SAAL, f + rect(150 + w + 100 - 1, 150 + 200 - 1, 202, 502, OCKER)


def t_c():
    return SAAL, block(PLAY_LOCH, PLAY_GRAU, u=64, cells=10, hx=2.5, hy=2)


KONZEPTE = {
    "t-a-block-f": (t_a, "Block-F", "Ein heller Block mit F ausgestanzt, die untere Hälfte im Schatten."),
    "t-b-cursor": (t_b, "Cursor", "Ein f und der Cursor, der blinkt: der Flacker."),
    "t-c-block-play": (t_c, "Block-Play", "Derselbe Block, ein Pixel-Play ausgestanzt."),
}


def main():
    U = 40
    for key, (fn, name, _) in KONZEPTE.items():
        bg, art = fn()
        with open(f"{HERE}/{key}-symbol.svg", "w") as f:
            f.write(svg(1000, 1000, f'<rect width="1000" height="1000" fill="{bg}"/>{art}', f"Flimmer {name}"))

        H, pad = 640, 120
        if key == "t-b-cursor":
            wm, w = pixel_text("flimmer", U, [PAPER] * 7, pad, (H - 7 * U) // 2)
            cur = rect(pad + w + U - 1, (H - 7 * U) // 2 + 2 * U - 1, 2 * U + 2, 5 * U + 2, OCKER)
            body, W = wm + cur, pad + w + U + 2 * U + pad
        else:
            sym = f'<g transform="translate({pad} {pad}) scale(0.625) translate(-180 -180)">{art}</g>'
            wm, w = pixel_text("flimmer", U, [GRAU] * 4 + [PAPER] * 3, pad + 400 + pad, (H - 7 * U) // 2)
            body, W = sym + wm, pad + 400 + pad + w + pad
        with open(f"{HERE}/{key}-quer.svg", "w") as f:
            f.write(svg(int(W), H, f'<rect width="{int(W)}" height="{H}" fill="{SAAL}"/>{body}', f"Flimmer {name}"))


if __name__ == "__main__":
    main()
