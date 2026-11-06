#!/usr/bin/env python3
"""Generates the Huginn mascot SVGs (static and animated) from a pixel grid.

Run from this directory:  python3 generate.py
"""

# X body, W wing feathers, B beak, o eye (left empty, so the background shows)
GRID = [
    "....XXXXXXXXX....",
    "...XXXXXXXXXXX...",
    "..XXXXXXXXXXXXX..",
    "..XXXoXXXXXoXXX..",
    "..XXXXXXBXXXXXX..",
    "..XXXXXBBBXXXXX..",
    "...XXXXBBBXXXX...",
    "....XXXXBXXXX....",
    ".XX.XXXXXXXXX.XX.",
    "XWWXXXXXXXXXXXWWX",
    "XWWWXXXXXXXXXWWWX",
    "XWWWXXXXXXXXXWWWX",
    ".WWWXXXXXXXXXWWW.",
    "..WWXXXXXXXXXWW..",
    "...WXXXXXXXXXW...",
    "....XXXXXXXXX....",
    ".....XX...XX.....",
    "....XXX...XXX....",
]

S = 16    # size of one pixel
PAD = 3   # empty pixels around the bird
WING_ROW = 8   # from this row down, cells left of BODY_L / right of BODY_R are wings
BODY_L, BODY_R = 4, 12
LEG_ROW = 16

THEMES = {
    "": dict(body="#8aa9ff", wing="#2457d6", beak="#ffb84d", bg="#141414"),
    "-light": dict(body="#141414", wing="#2457d6", beak="#ffb84d", bg="#f4f4f1"),
}

# One 5 s loop: breathe, blink twice, then a little hop with two wing flaps.
# step-end keeps every movement on the pixel grid.
CSS = f"""
* {{ animation-duration: 5s; animation-iteration-count: infinite; animation-timing-function: step-end; }}
.hop    {{ animation-name: hop; }}
.upper  {{ animation-name: breathe; }}
.lids   {{ animation-name: blink; opacity: 0; }}
.wing-l {{ animation-name: flap-l; }}
.wing-r {{ animation-name: flap-r; }}
@keyframes breathe {{ 0%,20%,40%,100% {{ transform: none; }} 10%,30% {{ transform: translateY({S}px); }} }}
@keyframes blink   {{ 0%,46%,49%,100% {{ opacity: 0; }} 44%,47.5% {{ opacity: 1; }} }}
@keyframes hop     {{ 0%,72%,100% {{ transform: none; }} 60% {{ transform: translateY(-{S}px); }} 62%,68% {{ transform: translateY(-{2*S}px); }} 70% {{ transform: translateY(-{S}px); }} }}
@keyframes flap-l  {{ 0%,58%,64%,70%,100% {{ transform: none; }} 61%,67% {{ transform: translate(-{S}px,-{S}px); }} }}
@keyframes flap-r  {{ 0%,58%,64%,70%,100% {{ transform: none; }} 61%,67% {{ transform: translate({S}px,-{S}px); }} }}
@media (prefers-reduced-motion: reduce) {{ * {{ animation: none !important; }} }}
"""


def rect(x, y, fill):
    return f'<rect x="{(x + PAD) * S}" y="{(y + PAD) * S}" width="{S}" height="{S}" fill="{fill}"/>'


def svg(t, animated):
    w, h = len(GRID[0]), len(GRID)
    W, H = (w + 2 * PAD) * S, (h + 2 * PAD) * S
    parts = {"upper": [], "wing-l": [], "wing-r": [], "legs": [], "lids": []}
    for y, row in enumerate(GRID):
        for x, c in enumerate(row):
            if c == "o":
                parts["lids"].append(rect(x, y, t["body"]))
                continue
            fill = {"X": t["body"], "W": t["wing"], "B": t["beak"]}.get(c)
            if not fill:
                continue
            if y >= LEG_ROW:
                part = "legs"
            elif y >= WING_ROW and x < BODY_L:
                part = "wing-l"
            elif y >= WING_ROW and x > BODY_R:
                part = "wing-r"
            else:
                part = "upper"
            parts[part].append(rect(x, y, fill))

    out = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" '
        f'shape-rendering="crispEdges" role="img" aria-label="Huginn mascot">',
        f'<rect width="{W}" height="{H}" fill="{t["bg"]}"/>',
    ]
    if not animated:
        for p in ("upper", "wing-l", "wing-r", "legs"):
            out += parts[p]
    else:
        out.append(f"<style>{CSS}</style>")
        out.append('<g class="hop">')
        out.append('<g class="upper">')
        out += parts["upper"]
        out.append('<g class="lids">' + "".join(parts["lids"]) + "</g>")
        out.append('<g class="wing-l">' + "".join(parts["wing-l"]) + "</g>")
        out.append('<g class="wing-r">' + "".join(parts["wing-r"]) + "</g>")
        out.append("</g>")
        out += parts["legs"]
        out.append("</g>")
    out.append("</svg>")
    return "\n".join(out) + "\n"


if __name__ == "__main__":
    for suffix, theme in THEMES.items():
        with open(f"huginn-mascot{suffix}.svg", "w") as f:
            f.write(svg(theme, animated=False))
        with open(f"huginn-mascot-animated{suffix}.svg", "w") as f:
            f.write(svg(theme, animated=True))
