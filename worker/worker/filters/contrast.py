"""contrast: scale distance from mid-grey (CSS `contrast()`); 1.0 = unchanged."""

from __future__ import annotations

from PIL import Image

from worker.filters._array import map_rgb

MID = 127.5


def contrast(img: Image.Image, value: float) -> Image.Image:
    if value == 1.0:
        return img
    return map_rgb(img, lambda rgb: (rgb - MID) * value + MID)
