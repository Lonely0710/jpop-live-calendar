#!/usr/bin/env python3
"""Render assets/readme/icons.svg, the stamp album of the icon library.

The sheet shows every icon of packages/spotlight-icons, one stamp each, in
the library's own order: a white stamp with perforated edges, its number in
the corner, the drawing, and the name under it. Run it after adding an icon
to the library; the READMEs embed the sheet at full width.
"""

import html
import json
import math
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LIB = ROOT / "packages" / "spotlight-icons"
OUT = ROOT / "assets" / "readme" / "icons.svg"

# Two drawings are variants the interface uses rather than icons of the set:
# the line drawing of the map beside its filled original, and the plain globe
# beside the globe with a cursor. The album shows the set.
HIDDEN = {"globe", "map-route-linear"}

COLS = 10          # stamps across, unless the set fills a nearer count
MARGIN = 28        # around the sheet
PITCH_X, PITCH_Y = 126, 150
STAMP_W, STAMP_H = 112, 136
INNER_W, INNER_H = 94, 118
ICON = 38          # the drawing, in a 24x24 box
BG = "#EADFCB"     # the album's paper, also the perforations
STAMP_BG = "#FFFDF8"
HAIRLINE = "#D8D1C7"
NUMBER = "#E85A3A"
NAME = "#8C847B"
MONO = "'SF Mono',Menlo,Consolas,'Liberation Mono',monospace"


def drawing(name):
    """The icon's drawing, and the attributes its root hands down to it.

    The outer <svg> carries the presentation attributes too — a line icon
    says fill="none" stroke="#E85A3A" there — so the nested <svg> of a stamp
    must take them over, or the drawing falls back to a black fill.
    """
    svg = (LIB / "svg" / f"{name}.svg").read_text().strip()
    svg = re.sub(r"^<\?xml[^>]*\?>\s*", "", svg)
    root = re.match(r"<svg\s([^>]*)>", svg)
    attrs = root.group(1) if root else ""
    keep = []
    for m in re.finditer(r'([\w:-]+)="([^"]*)"', attrs):
        name_, value = m.group(1), m.group(2)
        if name_ in ("xmlns", "width", "height", "viewBox", "x", "y"):
            continue
        keep.append(f'{name_}="{value}"')
    inner = re.sub(r"^<svg[^>]*>", "", svg)
    inner = re.sub(r"</svg>\s*$", "", inner)
    return " ".join(keep), inner.strip()


def perforations(x, y):
    """The teeth of a stamp: circles of the sheet's colour on its edges."""
    out = []
    for cx in range(x + 4, x + STAMP_W - 8 + 1, 9):
        out.append(f'<circle cx="{cx}" cy="{y}" r="3.4"/>')
        out.append(f'<circle cx="{cx}" cy="{y + STAMP_H}" r="3.4"/>')
    for cy in range(y + 4, y + STAMP_H - 5 + 1, 9):
        out.append(f'<circle cx="{x}" cy="{cy}" r="3.4"/>')
        out.append(f'<circle cx="{x + STAMP_W}" cy="{cy}" r="3.4"/>')
    return f'<g fill="{BG}">' + "".join(out) + "</g>"


def stamp(i, name, total, rows):
    row, col = divmod(i, COLS)
    x = MARGIN + col * PITCH_X
    # The album's last row is short: centre it rather than leave a gap at
    # the right, as a collection is laid out on its page.
    if row == rows - 1:
        x += (COLS - (total - row * COLS)) * PITCH_X // 2
    y = MARGIN + row * PITCH_Y
    delay = i * 0.12
    root_attrs, inner = drawing(name)
    # A name longer than the stamp is condensed into it rather than broken
    # out over the paper.
    # A name wider than the stamp is set a size down rather than broken out
    # over the paper: a monospaced face takes about 0.6 em a character.
    label = min(9.5, (INNER_W - 10) / (len(name) * 0.6))
    return (
        f'<rect x="{x}" y="{y}" width="{STAMP_W}" height="{STAMP_H}" fill="{STAMP_BG}"/>'
        + perforations(x, y)
        + f'<rect x="{x + 9}" y="{y + 9}" width="{INNER_W}" height="{INNER_H}" fill="none" stroke="{HAIRLINE}"/>'
        + f'<text x="{x + 98}" y="{y + 24}" text-anchor="end" font-family="{MONO}" '
        f'font-size="10" font-weight="700" fill="{NUMBER}">{i + 1:02d}</text>'
        + f'<g class="i" style="animation-delay:{delay:.2f}s">'
        f'<svg x="{x + 37}" y="{y + 38}" width="{ICON}" height="{ICON}" viewBox="0 0 24 24" {root_attrs}>{inner}</svg></g>'
        + f'<text x="{x + 56}" y="{y + 114}" text-anchor="middle" font-family="{MONO}" '
        f'font-size="{label:.1f}" fill="{NAME}">{html.escape(name)}</text>'
    )


def columns(n):
    """The columns that leave the shortest last row, nine or so across."""
    best = (10, (10 - n % 10) % 10)
    for c in range(8, 12):
        waste = (c - n % c) % c
        if waste < best[1]:
            best = (c, waste)
    return best[0]


def main():
    icons = json.loads((LIB / "icons.json").read_text())
    names = [i["name"] for i in icons if i["name"] not in HIDDEN]
    global COLS
    COLS = columns(len(names))
    rows = math.ceil(len(names) / COLS)
    width = MARGIN * 2 + STAMP_W + (COLS - 1) * PITCH_X
    height = MARGIN * 2 + STAMP_H + (rows - 1) * PITCH_Y
    body = "".join(stamp(i, n, len(names), rows) for i, n in enumerate(names))
    svg = (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}" '
        f'width="{width}" height="{height}">\n'
        "<style>.i{transform-box:fill-box;transform-origin:center;animation:w 6s ease-in-out infinite}"
        "@keyframes w{0%,10%,100%{transform:none}5%{transform:scale(1.22) rotate(-8deg)}}"
        "@media (prefers-reduced-motion:reduce){.i{animation:none}}</style>\n"
        f'<rect width="{width}" height="{height}" rx="14" fill="{BG}"/>\n'
        f"{body}\n</svg>\n"
    )
    OUT.write_text(svg)
    print(f"→ {OUT.relative_to(ROOT)} ({len(names)} icons, {COLS}×{rows}, {width}×{height})")


if __name__ == "__main__":
    main()
