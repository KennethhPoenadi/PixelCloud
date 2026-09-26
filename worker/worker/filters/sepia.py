"""sepia: CSS `sepia()` colour matrix. Internal op, only reachable through presets."""

from __future__ import annotations

from PIL import Image

from worker.filters._array import apply_matrix, map_rgb


def sepia_matrix(amount: float) -> list[list[float]]:
    a = 1.0 - amount
    return [
        [0.393 + 0.607 * a, 0.769 - 0.769 * a, 0.189 - 0.189 * a],
        [0.349 - 0.349 * a, 0.686 + 0.314 * a, 0.168 - 0.168 * a],
        [0.272 - 0.272 * a, 0.534 - 0.534 * a, 0.131 + 0.869 * a],
    ]


def sepia(img: Image.Image, value: float) -> Image.Image:
    if value <= 0:
        return img
    matrix = sepia_matrix(min(value, 1.0))
    return map_rgb(img, lambda rgb: apply_matrix(rgb, matrix))
