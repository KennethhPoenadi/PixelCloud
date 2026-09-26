"""sharpen: unsharp mask; value 0..3 maps to 0..300% strength (0 = off)."""

from __future__ import annotations

from PIL import Image, ImageFilter

from worker.filters._array import filter_rgb

RADIUS = 2.0


def sharpen(img: Image.Image, value: float) -> Image.Image:
    if value <= 0:
        return img
    mask = ImageFilter.UnsharpMask(radius=RADIUS, percent=round(value * 100), threshold=0)
    return filter_rgb(img, lambda rgb: rgb.filter(mask))
