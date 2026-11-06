"""Renders raven sprite options to raven-options.png. Run: python3 raven-sprites.py"""
import sys
from PIL import Image, ImageDraw, ImageFont

THEMES = {
    'dark':  dict(bg='#141414', X='#8aa9ff', W='#2457d6', S='#5f86f0', B='#2a2f3d', o='#0e1422', h='#ffffff', F='#2a2f3d', L='#b4c8ff', K='#262d45', Y='#ffb84d'),
    'light': dict(bg='#ffffff', X='#2457d6', W='#163a8f', S='#1d48b4', B='#1b1f2a', o='#0b1a40', h='#ffffff', F='#1b1f2a', L='#4a78e8', K='#0f1a33', Y='#f5a524'),
}
# X body, W wing, S shaggy throat (hackles), B beak, o eye, h glint, F feet, K near-black (black variant)

CURRENT = [
    '.....XXXXXXX.....', '...XXXXXXXXXXX...', '..XXXXXXXXXXXXX..', '..XXhoXXXXXhoXX..',
    '..XXooXXXXXooXX..', '..XXXXXBBBXXXXX..', '...XXXXXBXXXXX...', '....XXXXXXXXX....',
    '.XX.XXLLLLLXX.XX.', 'XWWXXLLLLLLLXXWWX', 'XWWWXLLLLLLLXWWWX', 'XWWWXLLLLLLLXWWWX',
    '.WWWXXLLLLLXXWWW.', '..WWXXXLLLXXXWW..', '...WXXXXXXXXXW...', '....XXXXXXXXX....',
    '.....FF...FF.....', '....FFF...FFF....',
]

# A: still facing you (keeps today's wing-flap animation), but with a big dark beak,
# a flat brow, shaggy throat, no pale belly, dark feet and a wedge tail showing below.
FRONT = [
    '....XXXXXXXXX....',
    '...XXXXXXXXXXX...',
    '..XXXXXXXXXXXXX..',
    '..XXhoXXXXXhoXX..',
    '..XXooXBBBXooXX..',
    '..XXXXBBBBBXXXX..',
    '...XXXBBBBBXXX...',
    '...XXXXBBBXXXX...',
    '.XX.SXSXBXSXS.XX.',
    'XWWXXSXSXSXSXXWWX',
    'XWWWXXSXXXSXXWWWX',
    'XWWWXXXXXXXXXWWWX',
    '.WWWXXXXXXXXXWWW.',
    '..WWXXXXXXXXXWW..',
    '...WXXXXXXXXXW...',
    '....XWWWWWWWX....',
    '.....FWWWWWF.....',
    '....FFFWWWFFF....',
]

# B: side-on, facing left toward the bubble: long heavy beak, flat head, hackles, folded wing, wedge tail.
SIDE = [
    '........XXXXX...........',
    '......XXXXXXXXX.........',
    '.....XXXXXXXXXXX........',
    '..BBBXhoXXXXXXXXX.......',
    '.BBBBBooXXXXXXXXXX......',
    'BBBBBBBXXXXXXXXXXXW.....',
    'B..BBBSXXXXWWWWWWWWW....',
    '....SXSXSXWWWWWWWWWWW...',
    '....XSXSXXWWWWWWWWWWWWW.',
    '.....XXXXXXWWWWWWWWWWWWW',
    '.....XXXXXXXWWWWWWWWWWWW',
    '......XXXXXXXXWWWWWWWWW.',
    '.......XXXXXXX....WWWWWW',
    '.........F..F......WWWW.',
    '.........F..F...........',
    '........FF.FF...........',
]

def recolor_black(grid):
    # C: the real raven: near-black all over, the brand blue only as a sheen on the wing and a highlight.
    out = []
    for r in grid:
        out.append(r.replace('X', 'K').replace('S', 'W'))
    return out

OPTIONS = [
    ('Today', [r.replace('B','Y').replace('F','Y') for r in CURRENT], {}),
    ('A  Front-on raven', FRONT, {}),
    ('B  Side-on, blue', SIDE, {}),
    ('C  Side-on, raven black', recolor_black(SIDE), {'W': None}),
]

PX = 12
def sprite(grid, pal, black=False):
    w, h = len(grid[0]), len(grid)
    im = Image.new('RGB', (w * PX, h * PX), pal['bg'])
    d = ImageDraw.Draw(im)
    for y, row in enumerate(grid):
        for x, c in enumerate(row):
            if c == '.':
                continue
            col = pal[c]
            if black and c == 'W':
                col = '#2457d6' if pal['bg'] == '#141414' else '#163a8f'  # blue sheen on the wing
            if black and c in 'BF':
                col = '#000000' if pal['bg'] == '#ffffff' else '#4a5068'
            d.rectangle([x * PX, y * PX, x * PX + PX - 1, y * PX + PX - 1], fill=col)
    return im

try:
    font = ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf', 22)
except OSError:
    font = ImageFont.load_default()
cellw, cellh = 360, 300
sheet = Image.new('RGB', (cellw * len(OPTIONS), cellh * 2), '#000')
for ti, (tname, pal) in enumerate(THEMES.items()):
    for oi, (name, grid, _) in enumerate(OPTIONS):
        cell = Image.new('RGB', (cellw, cellh), pal['bg'])
        s = sprite(grid, pal, black=name.startswith('C'))
        cell.paste(s, ((cellw - s.width) // 2, 60 + (220 - s.height) // 2))
        ImageDraw.Draw(cell).text((20, 18), name, fill='#a3a39e' if tname == 'dark' else '#62625e', font=font)
        sheet.paste(cell, (oi * cellw, ti * cellh))
sheet.save(sys.argv[1] if len(sys.argv) > 1 else 'raven-options.png')
