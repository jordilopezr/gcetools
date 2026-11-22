#!/usr/bin/env python3
"""Draws the DMG window background (660x420 and @2x) from the brand logo.

    python3 build/darwin/dmg-background.py

Icon positions must match the create-dmg call in .github/workflows/build.yml:
gceTools.app at (170, 215) and the Applications link at (490, 215).
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter, ImageFont

ROOT = Path(__file__).resolve().parents[2]
LOGO = ROOT / "assets/brand/png/gcetools-horizontal-light.png"
OUT = Path(__file__).resolve().parent
FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
W, H = 660, 420
COLORS = ["#EA4335", "#FBBC04", "#34A853", "#4A86E8"]


def draw(scale: int) -> Image.Image:
    s = lambda v: int(round(v * scale))
    img = Image.new("RGB", (s(W), s(H)), "#F5F7FB")
    d = ImageDraw.Draw(img)
    # soft vertical gradient
    for y in range(s(H)):
        t = y / s(H)
        c = tuple(int(a + (b - a) * t) for a, b in zip((250, 251, 253), (234, 239, 247)))
        d.line([(0, y), (s(W), y)], fill=c)

    # logo
    logo = Image.open(LOGO).convert("RGBA")
    lh = s(54)
    logo = logo.resize((int(logo.width * lh / logo.height), lh), Image.LANCZOS)
    img.paste(logo, ((s(W) - logo.width) // 2, s(34)), logo)

    # brand colour strip under the logo
    seg = s(22)
    x0 = (s(W) - seg * 4) // 2
    for i, col in enumerate(COLORS):
        d.rounded_rectangle([x0 + i * seg, s(102), x0 + (i + 1) * seg - s(3), s(106)], radius=s(2), fill=col)

    # soft halos where the two icons sit
    halo = Image.new("L", img.size, 0)
    hd = ImageDraw.Draw(halo)
    for cx in (170, 490):
        hd.ellipse([s(cx - 86), s(215 - 86), s(cx + 86), s(215 + 86)], fill=70)
    halo = halo.filter(ImageFilter.GaussianBlur(s(18)))
    img.paste(Image.new("RGB", img.size, "#FFFFFF"), (0, 0), halo)
    d = ImageDraw.Draw(img)

    # dashed arrow from the app to Applications, in the brand colours
    y, xa, xb = s(215), s(262), s(384)
    dash, gap = s(14), s(8)
    x, i = xa, 0
    while x < xb - s(16):
        d.rounded_rectangle([x, y - s(2.5), min(x + dash, xb - s(16)), y + s(2.5)], radius=s(2.5), fill=COLORS[i % 4])
        x += dash + gap
        i += 1
    d.polygon([(xb, y), (xb - s(20), y - s(12)), (xb - s(20), y + s(12))], fill=COLORS[3])

    # instructions
    f1 = ImageFont.truetype(FONT, s(15))
    f2 = ImageFont.truetype(FONT, s(12))
    for text, font, top, col in (("Arrastra gceTools a la carpeta Aplicaciones", f1, 342, "#3C4250"),
                                 ("Drag gceTools to the Applications folder", f2, 366, "#7A8294")):
        w = d.textlength(text, font=font)
        d.text(((s(W) - w) / 2, s(top)), text, font=font, fill=col)
    return img


if __name__ == "__main__":
    draw(1).save(OUT / "dmg-background.png", optimize=True)
    draw(2).save(OUT / "dmg-background@2x.png", optimize=True)
    print("written", OUT / "dmg-background.png", OUT / "dmg-background@2x.png")
