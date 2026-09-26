"""temperature: shift white balance; positive = warmer (orange), negative = cooler (blue).

The frontend preview uses the same per-channel gains in an SVG feColorMatrix.
"""

from __future__ import annotations

import numpy as np
from PIL import Image

from worker.filters._array import FloatArray, map_rgb

# gain applied to red/blue at |value| = 100
MAX_SHIFT = 0.25


def channel_gains(value: float) -> tuple[float, float]:
    k = value / 100.0 * MAX_SHIFT
    return 1.0 + k, 1.0 - k


def temperature(img: Image.Image, value: float) -> Image.Image:
    if value == 0:
        return img
    red, blue = channel_gains(value)
    gains = np.asarray([red, 1.0, blue], dtype=np.float32)

    def shift(rgb: FloatArray) -> FloatArray:
        return rgb * gains

    return map_rgb(img, shift)
