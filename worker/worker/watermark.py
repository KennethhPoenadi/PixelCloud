"""Semi-transparent "PixelCloud" mark in the bottom-right corner (Free plan)."""

from __future__ import annotations

from PIL import Image, ImageDraw, ImageFont

TEXT = "PixelCloud"


def apply(img: Image.Image) -> Image.Image:
    base = img.convert("RGBA")
    size = max(12, round(min(base.width, base.height) * 0.045))
    font = ImageFont.load_default(size=size)
    overlay = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(overlay)

    left, top, right, bottom = draw.textbbox((0, 0), TEXT, font=font)
    margin = max(6, size // 2)
    x = base.width - (right - left) - margin - left
    y = base.height - (bottom - top) - margin - top
    shadow = max(1, size // 16)
    draw.text((x + shadow, y + shadow), TEXT, font=font, fill=(0, 0, 0, 90))
    draw.text((x, y), TEXT, font=font, fill=(255, 255, 255, 150))

    out = Image.alpha_composite(base, overlay)
    return out if img.mode == "RGBA" else out.convert("RGB")
