"""Helpers to run per-pixel math on the RGB channels while preserving alpha."""

from __future__ import annotations

from collections.abc import Callable

import numpy as np
from numpy.typing import NDArray
from PIL import Image

FloatArray = NDArray[np.float32]


def normalize_mode(img: Image.Image) -> Image.Image:
    """Convert any input mode to RGB or RGBA so every filter sees the same layout."""
    if img.mode in ("RGB", "RGBA"):
        return img
    has_alpha = img.mode in ("LA", "PA") or (img.mode == "P" and "transparency" in img.info)
    return img.convert("RGBA" if has_alpha else "RGB")


def map_rgb(img: Image.Image, fn: Callable[[FloatArray], FloatArray]) -> Image.Image:
    """Apply fn to an HxWx3 float32 array in 0..255, clamp, and restore alpha."""
    img = normalize_mode(img)
    alpha = img.getchannel("A") if img.mode == "RGBA" else None
    rgb = np.asarray(img.convert("RGB"), dtype=np.float32)
    out = np.clip(fn(rgb), 0.0, 255.0)
    # round-half-up to match browser 8-bit quantisation closely
    result = Image.fromarray((out + 0.5).astype(np.uint8), mode="RGB")
    if alpha is not None:
        result.putalpha(alpha)
    return result


def apply_matrix(rgb: FloatArray, matrix: list[list[float]]) -> FloatArray:
    """Multiply every pixel by a 3x3 colour matrix (rows = output channels)."""
    m = np.asarray(matrix, dtype=np.float32)
    return rgb @ m.T


def filter_rgb(img: Image.Image, fn: Callable[[Image.Image], Image.Image]) -> Image.Image:
    """Run a PIL filter on the RGB channels only, leaving alpha untouched."""
    img = normalize_mode(img)
    if img.mode == "RGBA":
        alpha = img.getchannel("A")
        out = fn(img.convert("RGB"))
        out.putalpha(alpha)
        return out
    return fn(img)
