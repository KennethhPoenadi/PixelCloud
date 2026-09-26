"""vignette: darken towards the corners; value 0..1 is the corner opacity.

Matches the preview overlay `radial-gradient(ellipse at center, transparent 40%,
rgba(0,0,0,value) 100%)`: black is composited with an alpha that ramps linearly
from 0 at 40% of the centre-to-corner distance to `value` at the corners.
"""

from __future__ import annotations

import numpy as np
from PIL import Image

from worker.filters._array import FloatArray, map_rgb

INNER = 0.4


def vignette_mask(width: int, height: int, strength: float) -> FloatArray:
    """Per-pixel multiplier in [1 - strength, 1], shape (H, W, 1)."""
    ys = (np.arange(height, dtype=np.float32) + 0.5) / height * 2.0 - 1.0
    xs = (np.arange(width, dtype=np.float32) + 0.5) / width * 2.0 - 1.0
    # distance normalised so the corners are at 1.0 (ellipse "farthest-corner")
    dist = np.sqrt(xs[None, :] ** 2 + ys[:, None] ** 2) / np.sqrt(2.0)
    alpha = np.clip((dist - INNER) / (1.0 - INNER), 0.0, 1.0) * strength
    mask: FloatArray = (1.0 - alpha)[:, :, None].astype(np.float32)
    return mask


def vignette(img: Image.Image, value: float) -> Image.Image:
    if value <= 0:
        return img
    mask = vignette_mask(img.width, img.height, value)
    return map_rgb(img, lambda rgb: rgb * mask)
