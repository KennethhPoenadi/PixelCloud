"""saturation: CSS `saturate()` colour matrix; 0 = greyscale, 1.0 = unchanged."""

from __future__ import annotations

from PIL import Image

from worker.filters._array import apply_matrix, map_rgb


def saturation_matrix(s: float) -> list[list[float]]:
    # Filter Effects Module Level 1, feColorMatrix type="saturate"
    return [
        [0.213 + 0.787 * s, 0.715 - 0.715 * s, 0.072 - 0.072 * s],
        [0.213 - 0.213 * s, 0.715 + 0.285 * s, 0.072 - 0.072 * s],
        [0.213 - 0.213 * s, 0.715 - 0.715 * s, 0.072 + 0.928 * s],
    ]


def saturation(img: Image.Image, value: float) -> Image.Image:
    if value == 1.0:
        return img
    matrix = saturation_matrix(value)
    return map_rgb(img, lambda rgb: apply_matrix(rgb, matrix))
