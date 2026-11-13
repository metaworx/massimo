#!/usr/bin/env python3
"""Massimo's mark, and every file made from it.

The mark is one parameterised drawing: the bold M with round corners, the
clock hands along its V, a thermometer cut into each leg with a five-step
scale. Its flavours (VARIANTS below) differ in the hands, the hub and a gray
M behind; "grayorange" is the chosen one. Each is drawn for light and for
dark pages. The SVGs are plain filled paths - the M's stroke outlined, the
cut-outs subtracted - so that every renderer draws them alike.

Run with --help for the command line; tools/brand/README.md has the rest.
"""

from __future__ import annotations

import io
import math
import multiprocessing
import sys
from dataclasses import dataclass, replace
from pathlib import Path

try:
    import cairosvg
    import pathops
    from fontTools.pens.recordingPen import DecomposingRecordingPen
    from fontTools.pens.svgPathPen import SVGPathPen
    from fontTools.pens.transformPen import TransformPen
    from fontTools.ttLib import TTFont
    from PIL import Image
except ImportError as _err:  # --help, --version and --list still work
    _MISSING: ImportError | None = _err
else:
    _MISSING = None

BLUE = "#1F63C6"
BLUE_ON_DARK = "#5B95EC"
RED = "#D0342C"
RED_ON_DARK = "#F0675C"
ORANGE = "#ED6E21"  # the orange of the MAX! logo's "!"
METAWORX_ORANGE = "#CC6600"  # metaworx's logo: "worx"
METAWORX_GRAY = "#8F8F8F"  # metaworx's logo: "meta"
BACKING = METAWORX_GRAY
BACKING_ON_DARK = "#555555"  # behind the lighter blue: darker, for contrast
BACKING_INSET = 4  # inside the M's edge, so that the gray never shows there
TEXT = "#1C1C1C"
TEXT_ON_DARK = "#F2F2F2"
LATO = Path(__file__).resolve().parent / "fonts" / "Lato-Regular.ttf"  # --font overrides it
JOBS: int | None = None  # rendering processes; None: every core (--jobs)

# The M as one stroke, in a 512 box; shifted down 14 to centre it.
M_POINTS = [(96, 452), (96, 100), (256, 260), (416, 100), (416, 452)]
M_WIDTH = 140
SHIFT = 14
# The M's outline, for cropping the logo: the stroke's extent.
M_BOX = (26, 30, 460, 422)

HUB = (256, 260)
HOUR_TIP = (186, 190)
MINUTE_LENGTH = 150
LEFT_X, RIGHT_X = 96, 416
COLUMN_FOOT = 360
BULB_Y, BULB_R = 392, 34
COLUMN_W = 38
HAND_W = 30
HUB_R = 32
# The scale's five steps, the same on both thermometers. In the mercury a
# step is a dot left standing; above it a hole, lifted a little so that the
# holes sit higher than the dots would, evenly spaced from the topmost.
STEPS = (150, 190, 230, 270, 310)
HOLE_LIFT = 20
DOT_R, HOLE_R = 6.5, 9
# The mercury's tops at rest, and at their lowest.
LEFT_FULL, RIGHT_REST, LOW = 120, 222, 330
K = 0.5522847498  # a quarter circle as one cubic
SUPERSAMPLE = 4
# The mark in the full square: its farthest corner from the centre, 306 of
# 512, within the safe circle of 205 (0.62 x 306 = 190).
SQUARE_SCALE = 0.62


@dataclass(frozen=True)
class Pose:
    """What changes between the mark's states."""

    left_top: float = LEFT_FULL  # the left mercury's top
    right_top: float = RIGHT_REST  # the right mercury's top
    minute_deg: float = 45  # from twelve o'clock, clockwise
    scale: bool = True
    colour: str = BLUE
    # The minute hand: "hand" cut out of the M (in its colour outside it);
    # or the MAX! logo's exclamation mark, solid orange on top: "bang" the
    # bar alone; "bang-both" both hands as bars, one square dot on the
    # closed hub that fits both, the hands being 90 degrees apart;
    # "bang-min" the same with the hour hand carved instead.
    finger: str = "hand"
    finger_colour: str = ORANGE
    # A gray M behind, a little smaller, so that what is cut out shows gray
    # on any page instead of the page: its colour, or none.
    backing: str = ""
    # The hour hand's shape: "bar" as it is; "round" a capsule, both ends
    # rounded outward, carved; "round-orange" the same capsule in the
    # finger's colour, on top.
    hour: str = "bar"


def _circle(cx: float, cy: float, r: float) -> pathops.Path:
    p = pathops.Path()
    p.moveTo(cx + r, cy)
    p.cubicTo(cx + r, cy + K * r, cx + K * r, cy + r, cx, cy + r)
    p.cubicTo(cx - K * r, cy + r, cx - r, cy + K * r, cx - r, cy)
    p.cubicTo(cx - r, cy - K * r, cx - K * r, cy - r, cx, cy - r)
    p.cubicTo(cx + K * r, cy - r, cx + r, cy - K * r, cx + r, cy)
    p.close()
    return p


def _stroke(points, width: float, cap=None) -> pathops.Path:
    cap = pathops.LineCap.ROUND_CAP if cap is None else cap
    p = pathops.Path()
    p.moveTo(*points[0])
    for pt in points[1:]:
        p.lineTo(*pt)
    p.stroke(width, cap, pathops.LineJoin.ROUND_JOIN, 4)
    p.convertConicsToQuads(0.05)  # SVG knows no conics
    return p


def _op(a: pathops.Path, b: pathops.Path, how: pathops.PathOp) -> pathops.Path:
    return pathops.op(a, b, how)


def _union(paths) -> pathops.Path:
    paths = list(paths)
    out = paths[0]
    for p in paths[1:]:
        out = _op(out, p, pathops.PathOp.UNION)
    return out


def outline(pose: Pose) -> pathops.Path:
    """The mark as one filled outline."""
    m = _stroke(M_POINTS, M_WIDTH, pathops.LineCap.BUTT_CAP)
    hub = _circle(*HUB, HUB_R)
    cut = [
        _stroke([(LEFT_X, COLUMN_FOOT), (LEFT_X, pose.left_top)], COLUMN_W),
        _stroke([(RIGHT_X, COLUMN_FOOT), (RIGHT_X, pose.right_top)], COLUMN_W),
        _circle(LEFT_X, BULB_Y, BULB_R),
        _circle(RIGHT_X, BULB_Y, BULB_R),
    ]
    if pose.finger not in CLOSED_HUB:  # there the dot sits on the closed hub
        cut.append(hub)
    dots = []
    if pose.scale:
        for x, top in ((LEFT_X, pose.left_top), (RIGHT_X, pose.right_top)):
            for y in STEPS:
                if y >= top - 2:  # in the mercury: a dot left standing
                    dots.append(_circle(x, y, DOT_R))
                else:  # above it: a hole, lifted
                    cut.append(_circle(x, y - HOLE_LIFT, HOLE_R))
    shape = _op(m, _union(cut), pathops.PathOp.DIFFERENCE)
    if dots:
        shape = _union([shape, _op(_union(dots), m, pathops.PathOp.INTERSECTION)])
    # The hands: cut out where they lie on the M, in its colour where they
    # leave it - so they pass in front of the thermometers. The hub stays open.
    a = math.radians(pose.minute_deg)
    tip = (HUB[0] + MINUTE_LENGTH * math.sin(a), HUB[1] - MINUTE_LENGTH * math.cos(a))
    if pose.finger == "bang-both":
        strokes = []
    elif pose.hour == "round":
        strokes = [_hour_capsule()]
    elif pose.hour == "round-orange":
        strokes = []  # drawn on top, in the finger's colour
    else:
        strokes = [_stroke([HUB, HOUR_TIP], HAND_W)]
    if pose.finger == "hand":
        strokes.append(_stroke([HUB, tip], HAND_W))
    if not strokes:
        return shape
    hands = _union(strokes)
    if pose.hour == "bar":  # the hand stops at the hub's rim
        hands = _op(hands, hub, pathops.PathOp.DIFFERENCE)
    return _op(shape, hands, pathops.PathOp.XOR)


def _num(v: float) -> str:
    return f"{v:.2f}".rstrip("0").rstrip(".")


def _d(path: pathops.Path, dx: float = 0, dy: float = 0) -> str:
    pen = SVGPathPen(None, ntos=_num)
    path.draw(TransformPen(pen, (1, 0, 0, 1, dx, dy)))
    return pen.getCommands()


# The flavours whose hub is closed, a square dot on it.
CLOSED_HUB = ("bang-both", "bang-min")

# The hour hand's direction and reach.
HOUR_DEG = math.degrees(math.atan2(HOUR_TIP[0] - HUB[0], HUB[1] - HOUR_TIP[1]))
HOUR_REACH = math.dist(HUB, HOUR_TIP) + HAND_W / 2


def _hour_capsule() -> pathops.Path:
    """The hour hand as a capsule, moved out from the hub so that the
    square, turning with the minute hand, clears it even corner first - by
    the gap the minute hand's bar keeps; its length unchanged."""
    a = math.radians(HOUR_DEG)
    corner = HUB_R * 0.6 * math.sqrt(2)  # the square's half diagonal
    r0 = corner + BANG_GAP + HAND_W / 2
    r1 = math.dist(HUB, HOUR_TIP) + (r0 - (HUB_R * 0.6 + BANG_GAP + HAND_W / 2))
    return _stroke([(HUB[0] + r * math.sin(a), HUB[1] - r * math.cos(a)) for r in (r0, r1)], HAND_W)


def _round_bar(deg: float, r0: float, r1: float, w0: float, w1: float) -> pathops.Path:
    """A tapering bar from r0 to r1 along deg, both ends rounded outward:
    half circles of its width there, within r0 and r1."""
    a = math.radians(deg)
    u = (math.sin(a), -math.cos(a))
    n = (math.cos(a), math.sin(a))

    def at(r: float, w: float) -> tuple[float, float]:
        return HUB[0] + u[0] * r + n[0] * w, HUB[1] + u[1] * r + n[1] * w

    h0, h1 = w0 / 2, w1 / 2
    s0, s1 = r0 + h0, r1 - h1  # the straight part between the ends
    p = pathops.Path()
    p.moveTo(*at(s0, -h0))
    p.lineTo(*at(s1, -h1))
    p.cubicTo(*at(s1 + K * h1, -h1), *at(s1 + h1, -K * h1), *at(s1 + h1, 0))
    p.cubicTo(*at(s1 + h1, K * h1), *at(s1 + K * h1, h1), *at(s1, h1))
    p.lineTo(*at(s0, h0))
    p.cubicTo(*at(s0 - K * h0, h0), *at(s0 - h0, K * h0), *at(s0 - h0, 0))
    p.cubicTo(*at(s0 - h0, -K * h0), *at(s0 - K * h0, -h0), *at(s0, -h0))
    p.close()
    return p


# The exclamation mark of the MAX! logo, as measured there: a bar tapering
# from 8 to 4 units over 28, a gap of 2, a square dot of 7.
BANG_WIDE, BANG_NARROW = 36, 18
BANG_GAP = 9


def finger_d(pose: Pose, dy: float = 0) -> str:
    """The exclamation-mark finger(s) as a path, along the hands."""
    return " ".join(p for p in finger_parts(pose, dy) if p)


def finger_parts(pose: Pose, dy: float = 0) -> tuple[str, str]:
    """The fingers as two paths: the minute hand with the hub's square,
    which turn together, and the hour hand; "" where one is not drawn on
    top."""

    def frame(deg: float):
        a = math.radians(deg)
        u = (math.sin(a), -math.cos(a))  # outward
        n = (math.cos(a), math.sin(a))  # across

        def at(r: float, w: float) -> tuple[float, float]:
            return HUB[0] + u[0] * r + n[0] * w, HUB[1] + u[1] * r + n[1] * w + dy

        def bar(r0: float, w0: float, r1: float, w1: float) -> str:
            pts = [at(r0, -w0 / 2), at(r1, -w1 / 2), at(r1, w1 / 2), at(r0, w0 / 2)]
            return "M" + " L".join(f"{_num(x)} {_num(y)}" for x, y in pts) + " Z"

        def dot(rc: float, side: float) -> str:
            return bar(rc - side / 2, side, rc + side / 2, side)

        return bar, dot

    bar, dot = frame(pose.minute_deg)
    tip = MINUTE_LENGTH + HAND_W / 2  # as far as the cut-out hand reached
    side = HUB_R * 1.2  # a dot in the open hub, its corners clear of the rim
    if pose.finger == "bang":
        return bar(HUB_R + BANG_GAP, BANG_NARROW, tip, BANG_WIDE), ""
    if pose.finger in CLOSED_HUB:
        hour_bar, _ = frame(HOUR_DEG)
        # The dot square to the minute hand is square to the hour hand too,
        # and turns with the minute hand.
        minute = dot(0, side) + " " + bar(side / 2 + BANG_GAP, BANG_NARROW, tip, BANG_WIDE)
        hour = []
        if pose.hour == "round-orange":
            hour.append(_d(_hour_capsule(), 0, dy))
        if pose.finger == "bang-both":
            if pose.hour == "round":
                rounded = _round_bar(HOUR_DEG, side / 2 + BANG_GAP, HOUR_REACH, BANG_NARROW, BANG_WIDE)
                hour.append(_d(rounded, 0, dy))
            else:
                hour.append(hour_bar(side / 2 + BANG_GAP, BANG_NARROW, HOUR_REACH, BANG_WIDE))
        return minute, " ".join(hour)
    return "", ""


def mark_parts(pose: Pose, dy: float = 0) -> dict[str, str]:
    """The mark's paths in the 512 box, moved down by dy, by what they
    are: "back", the gray M behind, where the flavour has one; "m", the M
    with its cut-outs (even-odd); "minute" and "hour", the hands drawn on
    top, where they are."""
    parts = {}
    if pose.backing:
        legs = [(x, y - BACKING_INSET if y == M_POINTS[0][1] else y) for x, y in M_POINTS]
        back = _stroke(legs, M_WIDTH - 2 * BACKING_INSET, pathops.LineCap.BUTT_CAP)
        parts["back"] = _d(back, 0, dy)
    parts["m"] = _d(outline(pose), 0, dy)
    if pose.finger != "hand":
        minute, hour = finger_parts(pose, dy)
        for name, d in (("minute", minute), ("hour", hour)):
            if d:
                parts[name] = d
    return parts


def mark(pose: Pose, dy: float = 0) -> str:
    """The mark as filled paths in the 512 box, moved down by dy."""
    parts = mark_parts(pose, dy)
    body = f'<path d="{parts["m"]}" fill="{pose.colour}" fill-rule="evenodd"/>'
    if "back" in parts:
        body = f'<path d="{parts["back"]}" fill="{pose.backing}"/>' + body
    if pose.finger == "hand":
        return body
    return body + f'<path d="{finger_d(pose, dy)}" fill="{pose.finger_colour}"/>'


def icon_svg(pose: Pose, title: str = "Massimo") -> str:
    """The mark alone, transparent, 512 box."""
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">'
        f"<title>{title}</title>{mark(pose, SHIFT)}</svg>\n"
    )


def tile_svg(pose: Pose) -> str:
    """The mark in white on a blue tile, for an app icon; a little smaller,
    inside the rounded square."""
    white = replace(pose, colour="#FFFFFF")
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">'
        f'<title>Massimo</title><rect width="512" height="512" rx="112" fill="{pose.colour}"/>'
        f'<g transform="translate(256,256) scale(0.72) translate(-256,-256)">{mark(white, SHIFT)}</g></svg>\n'
    )


def square_svg(pose: Pose, scale: float = SQUARE_SCALE) -> str:
    """The tile's blue as a full square, the mark in white within the
    circle a maskable icon keeps (40 % of the side), for Android's maskable
    icons and Apple's touch icon, which round or cut the corners
    themselves."""
    white = replace(pose, colour="#FFFFFF")
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">'
        f'<title>Massimo</title><rect width="512" height="512" fill="{pose.colour}"/>'
        f'<g transform="translate(256,256) scale({_num(scale)}) translate(-256,-256)">{mark(white, SHIFT)}</g></svg>\n'
    )


# The web UI's sprite (the ui scope): the parts of the logo and of the small
# mark as symbols without a fill, each drawn by a <use> whose class gives it
# its colour (brand.css); the cover first, in the background of the place,
# so that the word under the drawing shows until the sprite has come.
UI_PARTS = (("back", "brand-back"), ("m", "brand-m"), ("hour", "brand-hands"), ("minute", "brand-hands"))


def _symbol(sid: str, vb: str, body: str) -> str:
    return f'<symbol id="{sid}" viewBox="{vb}">{body}</symbol>'


def _rect_of(vb: str) -> str:
    x, y, w, h = vb.split()
    return f'<rect x="{x}" y="{y}" width="{w}" height="{h}"/>'


def _parts_symbols(prefix: str, vb: str, parts: dict[str, str]) -> list[str]:
    out = [_symbol(f"{prefix}-cover", vb, _rect_of(vb))]
    for name, _ in UI_PARTS:
        if name in parts:
            rule = ' fill-rule="evenodd"' if name == "m" else ""
            out.append(_symbol(f"{prefix}-{name}", vb, f'<path d="{parts[name]}"{rule}/>'))
    return out


def sprite_svg(pose: Pose, i_style: str) -> str:
    """The web UI's sprite: logo-cover, logo-back, logo-m, logo-hour,
    logo-minute, logo-letters and logo-dot (the i's parts in the accent),
    and the small mark's mark-cover, mark-back, mark-m, mark-hour and
    mark-minute. Their paths have no fill: the page's classes colour them."""
    vb, path, i_parts = logo_parts(i_style)
    symbols = _parts_symbols("logo", vb, mark_parts(pose))
    plain = "".join(f'<path transform="{t}" d="{d}"/>' for t, d, lit in i_parts if not lit)
    lit = "".join(f'<path transform="{t}" d="{d}"/>' for t, d, lit in i_parts if lit)
    symbols.append(_symbol("logo-letters", vb, f'<path d="{path}"/>{plain}'))
    if lit:
        symbols.append(_symbol("logo-dot", vb, lit))
    symbols += _parts_symbols("mark", "0 0 512 512", mark_parts(replace(pose, scale=False), SHIFT))
    return (
        '<svg xmlns="http://www.w3.org/2000/svg">\n'
        "<!-- Massimo's mark for the web UI, written by tools/brand/make_brand.py (make ui-brand): "
        "edit the tool, not this file. -->\n" + "\n".join(symbols) + "\n</svg>\n"
    )


def brand_css(v: Variant, i_style: str) -> str:
    """The web UI's brand.css: the mark's colours for light and dark pages
    as tokens, in style.css's pattern, so that the chosen mode applies; the
    sprite's classes; the letters in the page's text colour; the logo's
    proportions."""
    light, _ = themed(v, "light")
    dark, _ = themed(v, "dark")
    vb = logo_parts(i_style)[0].split()

    def tokens(pose: Pose) -> str:
        back = pose.backing or pose.colour
        return f"--brand-back: {back}; --brand-m: {pose.colour}; --brand-hands: {pose.finger_colour};"

    ratio = float(vb[2]) / float(vb[3])

    return (
        "/* SPDX-License-Identifier: AGPL-3.0-or-later */\n"
        "/* Massimo's mark in the web UI, written by tools/brand/make_brand.py (make ui-brand): edit the tool,\n"
        "   not this file. The parts of brand.svg take these colours; a place sets --brand-cover to its\n"
        "   background, which the cover hides the word under the drawing with; --brand-logo-ratio is the\n"
        "   logo's width to its height, for a box the logo will take before it has come. */\n"
        f":root {{ {tokens(light)} --brand-logo-ratio: {ratio:.4f}; }}\n"
        "@media (prefers-color-scheme: dark) {\n"
        f'  :root:not([data-theme="light"]) {{ {tokens(dark)} }}\n'
        "}\n"
        f':root[data-theme="dark"] {{ {tokens(dark)} }}\n'
        ".brand-cover { fill: var(--brand-cover, var(--bg)); }\n"
        ".brand-back { fill: var(--brand-back); }\n"
        ".brand-m { fill: var(--brand-m); }\n"
        ".brand-hands { fill: var(--brand-hands); }\n"
        ".brand-letters { fill: var(--fg); }\n"
        f".brand-logo {{ aspect-ratio: {vb[2]} / {vb[3]}; }}\n"
        ".brand-mark { aspect-ratio: 1; }\n"
    )


def favicon_svg(v: Variant) -> str:
    """The small mark as a favicon: the light page's colours, and the dark
    one's under the browser's dark mode, as the tab strip has it."""
    light, _ = themed(v, "light")
    dark, _ = themed(v, "dark")
    parts = mark_parts(replace(light, scale=False), SHIFT)
    fills = {"back": light.backing, "m": light.colour, "hour": light.finger_colour, "minute": light.finger_colour}
    evenodd = ' fill-rule="evenodd"'
    body = "".join(
        f'<path class="{name}" d="{parts[name]}" fill="{fills[name]}"{evenodd if name == "m" else ""}/>'
        for name, _ in UI_PARTS
        if name in parts
    )
    dark_rules = f".back{{fill:{dark.backing}}}" if dark.backing else ""
    dark_rules += f".m{{fill:{dark.colour}}}"
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">'
        f"<title>Massimo</title><style>@media (prefers-color-scheme: dark){{{dark_rules}}}</style>{body}</svg>\n"
    )


def _letters(text: str, size: float, x: float, baseline: float) -> tuple[str, float]:
    """The text as one path in Lato Regular; returns it and its width."""
    font = TTFont(LATO)
    upm = font["head"].unitsPerEm
    cmap = font.getBestCmap()
    glyphs = font.getGlyphSet()
    hmtx = font["hmtx"]
    scale = size / upm
    pen = SVGPathPen(glyphs, ntos=_num)
    cursor = 0.0
    for ch in text:
        name = cmap[ord(ch)]
        glyphs[name].draw(TransformPen(pen, (scale, 0, 0, -scale, x + cursor, baseline)))
        cursor += hmtx[name][0] * scale
    return pen.getCommands(), cursor


def cap_height() -> float:
    """Lato's capital height, as a share of the em."""
    font = TTFont(LATO)
    return font["OS/2"].sCapHeight / font["head"].unitsPerEm


# The "i" of the logo: as Lato draws it ("plain"), all in the accent
# ("orange"), only its dot in it ("dot"), or as the MAX! logo's exclamation
# mark in the accent - "bang" the mark as it is (Mass!mo), "bang-up" upside
# down, its dot on top, so that it still reads as an i.
I_STYLES = ("plain", "orange", "dot", "bang", "bang-up")


def _i_parts(style: str) -> list[tuple[str, bool]]:
    """The i's outlines in Lato's units, each with whether it takes the
    accent: SVG path data, y upward."""
    font = TTFont(LATO)
    glyphs = font.getGlyphSet()
    rec = DecomposingRecordingPen(glyphs)
    glyphs["i"].draw(rec)
    contours, cur = [], []
    for op, args in rec.value:
        cur.append((op, args))
        if op in ("closePath", "endPath"):
            contours.append(cur)
            cur = []

    def d(contour) -> str:
        pen = SVGPathPen(None, ntos=_num)
        for op, args in contour:
            getattr(pen, op)(*args)
        return pen.getCommands()

    def is_dot(contour) -> bool:
        return min(y for _, args in contour for _, y in args) > font["OS/2"].sxHeight

    if style in ("plain", "orange", "dot"):
        return [(d(c), style == "orange" or (style == "dot" and is_dot(c))) for c in contours]
    # The exclamation mark, on the i's own places: the dot square, as wide as
    # Lato's dot; the bar where the stem is, tapering 2:1 as the MAX! one,
    # as heavy as the stem on average; the gap as the i's.
    stem = next(c for c in contours if not is_dot(c))
    dot = next(c for c in contours if is_dot(c))
    sx = [x for _, args in stem for x, _ in args]
    dy_ = [y for _, args in dot for _, y in args]
    cx = (min(sx) + max(sx)) / 2
    stem_w, x_h = max(sx) - min(sx), font["OS/2"].sxHeight
    side, gap = max(dy_) - min(dy_), min(dy_) - x_h
    wide, narrow = stem_w * 4 / 3, stem_w * 2 / 3

    def quad(y0: float, w0: float, y1: float, w1: float) -> str:
        pts = [(cx - w0 / 2, y0), (cx + w0 / 2, y0), (cx + w1 / 2, y1), (cx - w1 / 2, y1)]
        return "M" + " L".join(f"{_num(x)} {_num(y)}" for x, y in pts) + " Z"

    top = x_h + gap + side
    if style == "bang-up":  # dot on top, the bar widening down to the baseline
        return [(quad(top - side, side, top, side), True), (quad(0, wide, x_h, narrow), True)]
    # "bang": the bar from the top narrowing down, the dot on the baseline
    return [(quad(side + gap, narrow, top, wide), True), (quad(0, side, side, side), True)]


def logo_svg(pose: Pose, text_colour: str, i_style: str = "plain", accent: str = ORANGE) -> str:
    """The mark as the M of "Massimo", the letters on its baseline."""
    vb, path, i_parts = logo_parts(i_style)
    i_paths = "".join(
        f'<path transform="{transform}" d="{d}" fill="{accent if lit else text_colour}"/>'
        for transform, d, lit in i_parts
    )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb}">'
        f"<title>Massimo</title>{mark(pose)}"
        f'<path d="{path}" fill="{text_colour}"/>{i_paths}</svg>\n'
    )


def logo_parts(i_style: str = "plain") -> tuple[str, str, list[tuple[str, str, bool]]]:
    """The logo's viewBox, its letters but a drawn i as one path, and the
    i's parts where it is drawn by itself: (transform, path, whether in the
    accent). The mark, as mark(pose), is the M."""
    mx, my, mw, mh = M_BOX
    size = mh / cap_height()  # the letters' capitals as tall as the M
    gap = 0.045 * size
    x0, baseline = mx + mw + gap, my + mh
    path, width = _letters("assimo", size, x0, baseline)
    if i_style != "plain":
        # "ass" and "mo" as before; the i drawn by itself, in its parts.
        font = TTFont(LATO)
        scale = size / font["head"].unitsPerEm
        _, before = _letters("ass", size, x0, baseline)
        rest, _ = _letters("mo", size, x0 + before + font["hmtx"]["i"][0] * scale, baseline)
        ass, _ = _letters("ass", size, x0, baseline)
        path = ass + " " + rest
        transform = f"matrix({_num(scale)} 0 0 {_num(-scale)} {_num(x0 + before)} {_num(baseline)})"
        i_parts = [(transform, d, lit) for d, lit in _i_parts(i_style)]
    else:
        i_parts = []
    total = mw + gap + width
    pad = 0.06 * mh
    vb = f"{mx - pad:.1f} {my - pad:.1f} {total + 2 * pad:.1f} {mh + 2 * pad:.1f}"
    return vb, path, i_parts


def png(svg: str, width: int | None = None, height: int | None = None) -> Image.Image:
    data = cairosvg.svg2png(bytestring=svg.encode(), output_width=width, output_height=height)
    return Image.open(io.BytesIO(data)).convert("RGBA")


def smooth(svg: str, size: int) -> Image.Image:
    """Rendered at four times the size and scaled down: the cleanest edges."""
    big = png(svg, size * SUPERSAMPLE, size * SUPERSAMPLE)
    return big.resize((size, size), Image.Resampling.LANCZOS)


def _rgb(colour: str) -> tuple[int, int, int]:
    return tuple(int(colour[i : i + 2], 16) for i in (1, 3, 5))


BLEND_STEPS = 8  # in-between shades for each pair of colours: 7
TRANSPARENT = 255  # the GIF's transparent index


def gif_palette(colours: list[str]) -> list[tuple[int, int, int]]:
    """A GIF's colours: the drawing's own, and seven shades between each
    pair of them, so that where two meet inside the mark the edge is
    smoothed as in the WebP - the same in every frame, so nothing flickers."""
    pure = list(dict.fromkeys(_rgb(c) for c in colours))
    blends = [
        tuple(round(a[k] + (b[k] - a[k]) * t / BLEND_STEPS) for k in range(3))
        for i, a in enumerate(pure)
        for b in pure[i + 1 :]
        for t in range(1, BLEND_STEPS)
    ]
    return pure + blends


def gif_frame(img: Image.Image, colours: list[str]) -> Image.Image:
    """A GIF frame. GIF knows transparency only on or off: a pixel the
    drawing half covers at the outline is either one of its colours or
    transparent, so no background is blended into the outline on any page.
    Inside the mark, its colours and their shades smooth the edges."""
    pal = [v for c in gif_palette(colours) for v in c]
    n = len(pal) // 3
    # The transparent index gets a colour of its own, far from every one of
    # the drawing's, that no pixel is mapped to; the unused ones repeat the
    # first colour.
    lookup = Image.new("P", (1, 1))
    lookup.putpalette(pal + pal[:3] * (TRANSPARENT - n) + [255, 0, 255])
    frame = img.convert("RGB").quantize(palette=lookup, dither=Image.Dither.NONE)
    frame.putpalette(pal + [0, 0, 0] * (256 - n))
    frame.paste(TRANSPARENT, mask=img.getchannel("A").point(lambda v: 255 if v < 128 else 0))
    frame.info["transparency"] = TRANSPARENT
    return frame


def ease(t: float) -> float:
    t = min(max(t, 0.0), 1.0)
    return t * t * (3 - 2 * t)


def _render_one(job: tuple[str, int, int]) -> bytes:
    svg, width, height = job
    big = png(svg, width * SUPERSAMPLE, height * SUPERSAMPLE)
    buf = io.BytesIO()
    big.resize((width, height), Image.Resampling.LANCZOS).save(buf, "PNG")
    return buf.getvalue()


def render(frames_dir: Path | None, svgs: list[str], width: int, height: int) -> list[Image.Image]:
    """Render a series' frames - at four times the size, scaled down, the
    cleanest edges - in parallel; and keep them as SVGs where frames_dir is
    given."""
    if frames_dir is not None:
        frames_dir.mkdir(parents=True, exist_ok=True)
        digits = max(3, len(str(len(svgs) - 1)))
        for i, svg in enumerate(svgs):
            (frames_dir / f"frame-{i:0{digits}d}.svg").write_text(svg)
    with multiprocessing.get_context("fork").Pool(JOBS) as pool:
        data = pool.map(_render_one, [(svg, width, height) for svg in svgs], chunksize=8)
    return [Image.open(io.BytesIO(d)).convert("RGBA") for d in data]


def harden(soft: list[Image.Image], colours: list[list[str]]) -> list[Image.Image]:
    """The frames for a GIF."""
    return [gif_frame(img, cs) for img, cs in zip(soft, colours, strict=True)]


def write(
    base: Path, soft: list[Image.Image], hard: list[Image.Image], durations: list[int], webp: bool = True
) -> None:
    """One animation: a GIF (hard edges, any page) and, where asked, an
    animated WebP (soft edges, true transparency)."""
    base.parent.mkdir(parents=True, exist_ok=True)
    hard[0].save(
        base.with_name(base.name + ".gif"),
        save_all=True,
        append_images=hard[1:],
        duration=durations,
        loop=0,
        disposal=2,
        transparency=hard[0].info["transparency"],
        optimize=False,
    )
    if webp:
        soft[0].save(
            base.with_name(base.name + ".webp"),
            save_all=True,
            append_images=soft[1:],
            duration=durations,
            loop=0,
            lossless=True,
        )


# The right mercury's highest point in the full turn: between the first and
# second step, so that one step always stays a hole above it.
RIGHT_HIGH = 178


def _ease_inverse(y: float) -> float:
    """The t at which ease(t) is y, by bisection."""
    lo, hi = 0.0, 1.0
    for _ in range(60):
        mid = (lo + hi) / 2
        lo, hi = (mid, hi) if ease(mid) < y else (lo, mid)
    return (lo + hi) / 2


# The right column's fall runs from noon (360 deg) through the loop's seam
# as one curve, so that it never stops there; it is timed to pass the
# icon's resting level exactly at the turn's start (405 deg).
_FALL_END = 360 + 45 / _ease_inverse((RIGHT_REST - RIGHT_HIGH) / (LOW - RIGHT_HIGH))


def cycle(deg: float) -> Pose:
    """A full turn of the minute hand, from the icon's own pose (45 deg) to
    it again (405 deg), the hour hand still, as a seamless loop:

    - the right column falls in one smooth curve from noon, through the
      start, to its lowest (at about 4:15), waits there until five o'clock,
      and climbs back by noon - never to the top: one step stays a hole;
    - the left goes down one step while the right falls, on down from five
      until nine o'clock, and rises behind the hand from there to the end.
    """

    def seg(d, d0, d1, v0, v1):
        return v0 + (v1 - v0) * ease((d - d0) / (d1 - d0))

    if 150 <= deg <= 360:
        right = seg(deg, 150, 360, LOW, RIGHT_HIGH)
    else:
        d = deg if deg > 360 else deg + 360  # the fall, unwrapped across the seam
        right = seg(d, 360, _FALL_END, RIGHT_HIGH, LOW) if d <= _FALL_END else LOW
    step = STEPS[1] - STEPS[0]
    if deg <= 100:
        left = seg(deg, 45, 100, LEFT_FULL, LEFT_FULL + step)
    elif deg <= 150:
        left = LEFT_FULL + step
    elif deg <= 270:
        left = seg(deg, 150, 270, LEFT_FULL + step, LOW)
    else:
        left = seg(deg, 270, 405, LOW, LEFT_FULL)
    return Pose(left_top=left, right_top=right, minute_deg=deg % 360)


VERSION = "1.1.0"


@dataclass(frozen=True)
class Variant:
    """A flavour: its pose for light pages, and its letters' colours."""

    description: str
    pose: Pose
    text_light: str = TEXT
    text_dark: str = TEXT_ON_DARK


VARIANTS = {
    "classic": Variant("the first blue mark: hands and thermometers carved, cut-outs transparent", Pose()),
    "bang": Variant(
        "the minute hand the MAX! exclamation mark in orange, the hour hand carved, the hub open", Pose(finger="bang")
    ),
    "twin": Variant("both hands orange exclamation bars, the square on the closed hub", Pose(finger="bang-both")),
    "metaworx": Variant(
        "twin in metaworx's orange, the letters in its gray",
        Pose(finger="bang-both", finger_colour=METAWORX_ORANGE),
        METAWORX_GRAY,
        METAWORX_GRAY,
    ),
    "graycarved": Variant(
        "a gray M behind, so that cut-outs show gray on any page; the minute hand the orange exclamation bar, "
        "the hour hand carved",
        Pose(finger="bang-min", backing=BACKING),
    ),
    "graycapsule": Variant(
        "graycarved, the hour hand a carved capsule clear of the turning square",
        Pose(finger="bang-min", hour="round", backing=BACKING),
    ),
    "grayorange": Variant(
        "graycapsule, the capsule in orange (the chosen one)",
        Pose(finger="bang-min", hour="round-orange", backing=BACKING),
    ),
}
THEMES = ("light", "dark")
I_CHOICES = ("dot", "plain", "orange", "bang", "bang-up")
SCOPES = {
    "svg": "the icon, the logo and the small icon, as SVG",
    "png": "the icon at 256 and 512 px, the logo 128 and 256 px high",
    "ladder": "the icon at 64, 128, 180, 192, 1024 px, the small one at 16, 24, 32, 48 px",
    "tile": "the mark in white on a blue square, for an app: SVG, and PNG at 180, 192, 512 px",
    "favicon": "favicon.ico, 16 to 256 px",
    "ui": "the web UI's files, flat and by fixed names: brand.svg (a sprite), brand.css, favicon.svg, "
    "favicon.ico, tile-192/512.png, maskable-192/512.png, apple-touch-180.png (make ui-brand)",
    "homeassistant": "homeassistant-brand/: icon.png, icon@2x.png, logo.png, logo@2x.png, and dark_ ones",
    "turn-fast": "the turn at 40, 80 and 160 ms a frame, 80 frames, as GIF and WebP",
    "turn-slow": "the turn at 1 s and 3 s a frame, 800 frames, as GIF",
    "tick": "the tick turn, 60 frames of a second, as GIF",
    "frames": "every frame of the animations, as SVG, under frames/",
}
FRAMES = 800  # a turn, for the slow speeds
FAST_STEP = 10  # the fast speeds take every tenth: 80 frames, 4.5 degrees each
FAST_SPEEDS = (40, 80, 160)  # ms a frame
SLOW_SPEEDS = (1000, 3000)  # ms a frame
TICKS = 60  # 6 degrees a second, a turn a minute
LOGO_H = 128
MARKER = ".make_brand"
EXIT_USAGE, EXIT_REQUIREMENT = 2, 3


def themed(v: Variant, theme: str) -> tuple[Pose, str]:
    """A flavour's pose and letters' colour for a page."""
    if theme == "light":
        return v.pose, v.text_light
    return replace(v.pose, colour=BLUE_ON_DARK, backing=BACKING_ON_DARK if v.pose.backing else ""), v.text_dark


def colours_of(pose: Pose, text: str | None = None) -> list[str]:
    """The colours a frame is drawn in, for its GIF palette."""
    cs = [pose.colour]
    if pose.finger != "hand":
        cs.append(pose.finger_colour)
    if pose.backing:
        cs.append(pose.backing)
    if text:
        cs.append(text)
    return cs


def speed_name(ms: int) -> str:
    return f"{ms}ms" if ms < 1000 else f"{ms // 1000}s"


def logo_size(svg: str) -> tuple[int, int]:
    vb = [float(v) for v in svg.split('viewBox="')[1].split('"')[0].split()]
    return round(LOGO_H * vb[2] / vb[3]), LOGO_H


@dataclass
class Job:
    """A part of the set: the files it writes, relative to the target, and
    how."""

    label: str
    outputs: list[str]
    run: object  # a callable taking the target folder


def plan(flavour: str, base: str, scopes: set[str], themes: tuple[str, ...], i_style: str) -> list[Job]:
    """What a build of a flavour writes, scope by scope."""
    v = VARIANTS[flavour]
    jobs: list[Job] = []

    def svgs(theme: str) -> dict[str, str]:
        pose, text = themed(v, theme)
        return {
            "icon": icon_svg(pose),
            "logo": logo_svg(pose, text, i_style),
            "small": icon_svg(replace(pose, scale=False)),
        }

    for theme in themes:
        if "svg" in scopes:
            names = [f"svg/{base}-{form}-{theme}.svg" for form in ("icon", "logo", "small")]

            def run(out: Path, theme=theme, names=names) -> None:
                for name, svg in zip(names, svgs(theme).values(), strict=True):
                    (out / name).write_text(svg)

            jobs.append(Job(f"SVG, {theme}", names, run))
        if "png" in scopes:
            names = [f"png/{base}-icon-{theme}-{px}.png" for px in (256, 512)]
            names += [f"png/{base}-logo-{theme}-h{h}.png" for h in (128, 256)]

            def run(out: Path, theme=theme) -> None:
                s = svgs(theme)
                for px in (256, 512):
                    png(s["icon"], px, px).save(out / f"png/{base}-icon-{theme}-{px}.png")
                for h in (128, 256):
                    png(s["logo"], height=h).save(out / f"png/{base}-logo-{theme}-h{h}.png")

            jobs.append(Job(f"PNG, {theme}", names, run))
        if "ladder" in scopes:
            names = [f"png/{base}-small-{theme}-{px}.png" for px in (16, 24, 32, 48)]
            names += [f"png/{base}-icon-{theme}-{px}.png" for px in (64, 128, 180, 192, 1024)]

            def run(out: Path, theme=theme) -> None:
                s = svgs(theme)
                for px in (16, 24, 32, 48):
                    png(s["small"], px, px).save(out / f"png/{base}-small-{theme}-{px}.png")
                for px in (64, 128, 180, 192, 1024):
                    png(s["icon"], px, px).save(out / f"png/{base}-icon-{theme}-{px}.png")

            jobs.append(Job(f"sizes, {theme}", names, run))
        if scopes & {"turn-fast", "turn-slow", "tick"}:
            for form in ("icon", "logo"):
                jobs.append(turn_job(v, base, form, theme, scopes, i_style))
    if "tile" in scopes:
        names = [f"svg/{base}-tile.svg"] + [f"png/{base}-tile-{px}.png" for px in (180, 192, 512)]

        def run(out: Path) -> None:
            svg = tile_svg(v.pose)
            (out / f"svg/{base}-tile.svg").write_text(svg)
            for px in (180, 192, 512):
                png(svg, px, px).save(out / f"png/{base}-tile-{px}.png")

        jobs.append(Job("tile", names, run))

    def favicon(out: Path, theme: str) -> None:
        s = svgs(theme)
        sizes = [16, 24, 32, 48, 64, 128, 256]
        images = [png(s["small"] if px <= 48 else s["icon"], px, px) for px in sizes]
        images[-1].save(out / "favicon.ico", sizes=[(px, px) for px in sizes], append_images=images[:-1])

    if "favicon" in scopes:
        theme = "light" if "light" in themes else themes[0]

        def run(out: Path, theme=theme) -> None:
            favicon(out, theme)

        jobs.append(Job("favicon", ["favicon.ico"], run))
    if "ui" in scopes:
        names = ["brand.svg", "brand.css", "favicon.svg", "favicon.ico"]
        names += [f"tile-{px}.png" for px in (192, 512)] + [f"maskable-{px}.png" for px in (192, 512)]
        names += ["apple-touch-180.png"]

        def run(out: Path) -> None:
            (out / "brand.svg").write_text(sprite_svg(v.pose, i_style))
            (out / "brand.css").write_text(brand_css(v, i_style))
            (out / "favicon.svg").write_text(favicon_svg(v))
            favicon(out, "light")
            tile, square = tile_svg(v.pose), square_svg(v.pose)
            for px in (192, 512):
                png(tile, px, px).save(out / f"tile-{px}.png")
                png(square, px, px).save(out / f"maskable-{px}.png")
            png(square, 180, 180).save(out / "apple-touch-180.png")

        jobs.append(Job("the web UI's files", names, run))
    if "homeassistant" in scopes:
        prefixes = {"light": "", "dark": "dark_"}
        names = [
            f"homeassistant-brand/{prefixes[t]}{n}.png" for t in themes for n in ("icon", "icon@2x", "logo", "logo@2x")
        ]

        def run(out: Path) -> None:
            for theme in themes:
                s, p = svgs(theme), prefixes[theme]
                png(s["icon"], 256, 256).save(out / f"homeassistant-brand/{p}icon.png")
                png(s["icon"], 512, 512).save(out / f"homeassistant-brand/{p}icon@2x.png")
                png(s["logo"], height=128).save(out / f"homeassistant-brand/{p}logo.png")
                png(s["logo"], height=256).save(out / f"homeassistant-brand/{p}logo@2x.png")

        jobs.append(Job("Home Assistant's brand images", names, run))
    return jobs


def turn_job(v: Variant, base: str, form: str, theme: str, scopes: set[str], i_style: str) -> Job:
    """The animations of one form and theme: one drawing of the turn - 800
    frames, or every tenth where the slow speeds are not asked for - and the
    tick turn."""
    pose, text = themed(v, theme)
    if form == "icon":
        svg_of, colours, size = icon_svg, colours_of(pose), (256, 256)
    else:

        def svg_of(p: Pose) -> str:
            return logo_svg(p, text, i_style)

        colours, size = colours_of(pose, text), logo_size(svg_of(pose))
    name = f"{base}-{form}-{theme}"
    keep = "frames" in scopes
    outputs = []
    if "turn-fast" in scopes:
        outputs += [f"anim/{name}-turn-{speed_name(ms)}.{ext}" for ms in FAST_SPEEDS for ext in ("gif", "webp")]
    if "turn-slow" in scopes:
        outputs += [f"anim/{name}-turn-{speed_name(ms)}.gif" for ms in SLOW_SPEEDS]
    if "tick" in scopes:
        outputs.append(f"anim/{name}-tick-1s.gif")
    if keep and scopes & {"turn-fast", "turn-slow"}:
        count = FRAMES if "turn-slow" in scopes else FRAMES // FAST_STEP
        outputs.append(f"frames/{name}/ ({count} SVG)")
    if keep and "tick" in scopes:
        outputs.append(f"frames/{name}-tick/ ({TICKS} SVG)")

    def posed(deg: float) -> Pose:
        c = cycle(deg)
        return replace(pose, left_top=c.left_top, right_top=c.right_top, minute_deg=c.minute_deg)

    def run(out: Path) -> None:
        if scopes & {"turn-fast", "turn-slow"}:
            step = 1 if "turn-slow" in scopes else FAST_STEP
            poses = [posed(45 + 360 * i / FRAMES) for i in range(0, FRAMES, step)]
            soft = render(out / "frames" / name if keep else None, [svg_of(p) for p in poses], *size)
            hard = harden(soft, [colours] * len(soft))
            fast = slice(None, None, FAST_STEP // step)
            if "turn-fast" in scopes:
                for ms in FAST_SPEEDS:
                    write(
                        out / "anim" / f"{name}-turn-{speed_name(ms)}", soft[fast], hard[fast], [ms] * len(soft[fast])
                    )
            if "turn-slow" in scopes:
                for ms in SLOW_SPEEDS:
                    write(out / "anim" / f"{name}-turn-{speed_name(ms)}", soft, hard, [ms] * FRAMES, webp=False)
        if "tick" in scopes:
            poses = [posed(45 + 360 * i / TICKS) for i in range(TICKS)]
            soft = render(out / "frames" / f"{name}-tick" if keep else None, [svg_of(p) for p in poses], *size)
            hard = harden(soft, [colours] * TICKS)
            write(out / "anim" / f"{name}-tick-1s", soft, hard, [1000] * TICKS, webp=False)

    return Job(f"animations, {form}, {theme}", outputs, run)


def default_out(base: str) -> Path:
    """<guidelines root>/temp/brand/<basename> in this repository: the root
    as its config.ini names it."""
    import subprocess

    here = Path(__file__).resolve().parent
    try:
        root = Path(
            subprocess.run(
                ["git", "-C", str(here), "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True
            ).stdout.strip()
        )
    except (OSError, subprocess.CalledProcessError):
        root = here.parent.parent
    guidelines = "GUIDELINES"
    config = root / "GUIDELINES" / "config.ini"
    if config.is_file():
        try:
            guidelines = (
                subprocess.run(
                    ["git", "config", "--file", str(config), "guidelines.root"],
                    capture_output=True,
                    text=True,
                    check=True,
                ).stdout.strip()
                or guidelines
            )
        except (OSError, subprocess.CalledProcessError):
            pass
    return root / guidelines / "temp" / "brand" / base


def _parser():
    import argparse

    flavours = "\n".join(f"  {n:12} {v.description}" for n, v in VARIANTS.items())
    scopes = "\n".join(f"  {n:14} {d}" for n, d in SCOPES.items())
    epilog = f"""\
flavours:
{flavours}

scopes (default: all):
{scopes}

files: <out>/<kind>/<basename>-<form>-<theme>[-<size> | -turn-<speed> | -tick-1s].<ext>,
  form icon, logo or small, theme the page it is for (light or dark).

examples:
  make_brand.py --list
  make_brand.py --flavour grayorange --dry-run
  make_brand.py --flavour grayorange --scope svg,png,homeassistant --yes
  make_brand.py --flavour all --scope turn-fast --theme dark --yes

exit codes:
  0  done (or the help, the version, the list, a dry run)
  1  a prune was asked about and declined
  2  could not run: a bad option, a target it will not prune
  3  an unmet requirement: a Python package or the font is missing
"""
    p = argparse.ArgumentParser(
        prog="make_brand.py",
        description="Draw Massimo's mark and write the files made from it, for one flavour or several. "
        "The target folder is pruned first.",
        epilog=epilog,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("-V", "--version", action="version", version=f"make_brand.py {VERSION}")
    p.add_argument(
        "--flavour",
        action="append",
        metavar="NAME",
        choices=[*VARIANTS, "all"],
        help="the flavour to build; repeat it, or 'all'",
    )
    p.add_argument("--scope", metavar="SCOPE[,SCOPE...]", help="only these parts of the set (default: all)")
    p.add_argument("--theme", choices=THEMES, help="only for light or dark pages (default: both)")
    p.add_argument(
        "--i",
        dest="i_style",
        choices=I_CHOICES,
        default="dot",
        help="the logo's 'i': its dot in orange (default), plain, all orange, the exclamation mark, or upside down",
    )
    p.add_argument(
        "--out",
        type=Path,
        metavar="DIR",
        help="the target folder (default: <guidelines root>/temp/brand/<basename>); one flavour only",
    )
    p.add_argument(
        "--name", metavar="BASENAME", help="the files' first name part (default: the flavour); one flavour only"
    )
    p.add_argument("-y", "--yes", action="store_true", help="prune the target without asking")
    p.add_argument("--dry-run", action="store_true", help="say what would be pruned and written, and do nothing")
    p.add_argument("--jobs", type=int, metavar="N", help="rendering processes (default: every core)")
    p.add_argument(
        "--font", type=Path, metavar="TTF", help="the logo's letters' font (default: fonts/Lato-Regular.ttf)"
    )
    p.add_argument("--list", action="store_true", help="list the flavours and the scopes")
    v = p.add_mutually_exclusive_group()
    v.add_argument("-q", "--quiet", action="store_true", help="errors only")
    v.add_argument("-v", "--verbose", action="store_true", help="name every file written")
    return p


def _prune(out: Path, yes: bool) -> int | None:
    """Empty the target, if it is ours; a non-zero exit code where not."""
    import shutil

    if out.exists() and any(out.iterdir()):
        if not (out / MARKER).is_file():
            print(
                f"make_brand.py: {out} was not made by make_brand.py (no {MARKER}); "
                "not pruning it - choose another --out, or empty it",
                file=sys.stderr,
            )
            return EXIT_USAGE
        count = sum(1 for f in out.rglob("*") if f.is_file())
        if not yes:
            if not sys.stdin.isatty():
                print(f"make_brand.py: {out} holds {count} files; give --yes to prune it", file=sys.stderr)
                return EXIT_USAGE
            answer = input(f"Prune {out} ({count} files)? [y/N] ").strip().lower()
            if answer not in ("y", "yes"):
                print("make_brand.py: not pruned; nothing written", file=sys.stderr)
                return 1
        shutil.rmtree(out)
    out.mkdir(parents=True, exist_ok=True)
    (out / MARKER).write_text("Written by Massimo's tools/brand/make_brand.py; pruned by its next run.\n")
    return None


def main(argv: list[str]) -> int:
    global LATO, JOBS
    p = _parser()
    if not argv:
        p.print_usage(sys.stderr)
        print("make_brand.py: give --flavour to build, or --list, or --help", file=sys.stderr)
        return EXIT_USAGE
    args = p.parse_args(argv)
    if args.list:
        print("flavours:")
        for n, v in VARIANTS.items():
            print(f"  {n:12} {v.description}")
        print("scopes:")
        for n, d in SCOPES.items():
            print(f"  {n:14} {d}")
        return 0
    if not args.flavour:
        p.error("--flavour is needed: a flavour's name, or 'all'")
    flavours = list(VARIANTS) if "all" in args.flavour else list(dict.fromkeys(args.flavour))
    if len(flavours) > 1 and (args.out or args.name):
        p.error("--out and --name take one flavour")
    scopes = set(SCOPES)
    if args.scope:
        scopes = {s.strip() for s in args.scope.split(",") if s.strip()}
        if unknown := scopes - set(SCOPES):
            p.error(f"unknown scope: {', '.join(sorted(unknown))} (see --list)")
    themes = (args.theme,) if args.theme else THEMES
    if args.jobs is not None and args.jobs < 1:
        p.error("--jobs takes 1 or more")
    if _MISSING is not None:
        print(
            f"make_brand.py: a Python package is missing ({_MISSING.name}); "
            "pip install -r tools/brand/requirements.txt",
            file=sys.stderr,
        )
        return EXIT_REQUIREMENT
    if args.font:
        LATO = args.font
    if not LATO.is_file():
        print(f"make_brand.py: no font at {LATO}", file=sys.stderr)
        return EXIT_REQUIREMENT
    JOBS = args.jobs
    if not args.quiet:
        print(f"make_brand.py v{VERSION} - Massimo's mark and its files", file=sys.stderr)
    for flavour in flavours:
        base = args.name or flavour
        out = (args.out or default_out(base)).resolve()
        jobs = plan(flavour, base, scopes, themes, args.i_style)
        if args.dry_run:
            if out.exists() and any(out.iterdir()):
                state = (
                    "would prune it" if (out / MARKER).is_file() else "would refuse to prune it: not made by this tool"
                )
                print(f"{out}: {state}")
            print(f"{out}:")
            for job in jobs:
                for name in job.outputs:
                    print(f"  {name}")
            continue
        if (code := _prune(out, args.yes)) is not None:
            return code
        for sub in ("svg", "png", "anim", "frames", "homeassistant-brand"):
            (out / sub).mkdir(exist_ok=True)
        for job in jobs:
            if not args.quiet:
                print(f"  {flavour}: {job.label}", file=sys.stderr)
            job.run(out)
            if args.verbose:
                for name in job.outputs:
                    print(f"    {name}", file=sys.stderr)
        for sub in ("svg", "png", "anim", "frames", "homeassistant-brand"):
            if not any((out / sub).iterdir()):
                (out / sub).rmdir()
        if not args.quiet:
            print(f"  {flavour}: {out}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
