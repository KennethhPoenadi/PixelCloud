"""rotate: clockwise by 90, 180 or 270 degrees (lossless transpose)."""

from __future__ import annotations

from PIL import Image

_TRANSPOSE = {
    90: Image.Transpose.ROTATE_270,  # PIL rotates counter-clockwise
    180: Image.Transpose.ROTATE_180,
    270: Image.Transpose.ROTATE_90,
}


def rotate(img: Image.Image, angle: int) -> Image.Image:
    return img.transpose(_TRANSPOSE[angle])
