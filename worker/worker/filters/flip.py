"""flip: mirror horizontally ("h") or vertically ("v")."""

from __future__ import annotations

from PIL import Image


def flip(img: Image.Image, direction: str) -> Image.Image:
    if direction == "h":
        return img.transpose(Image.Transpose.FLIP_LEFT_RIGHT)
    return img.transpose(Image.Transpose.FLIP_TOP_BOTTOM)
