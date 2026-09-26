"""blur: Gaussian blur with the given radius in pixels (0 = off)."""

from __future__ import annotations

from PIL import Image, ImageFilter

from worker.filters._array import filter_rgb


def blur(img: Image.Image, value: float) -> Image.Image:
    if value <= 0:
        return img
    return filter_rgb(img, lambda rgb: rgb.filter(ImageFilter.GaussianBlur(radius=value)))
