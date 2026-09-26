"""invert: photographic negative. Internal op, only reachable through presets."""

from __future__ import annotations

from PIL import Image

from worker.filters._array import map_rgb


def invert(img: Image.Image) -> Image.Image:
    return map_rgb(img, lambda rgb: 255.0 - rgb)
