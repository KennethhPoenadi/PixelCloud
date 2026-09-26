"""resize: fit inside max_width x max_height keeping aspect ratio; never upscales."""

from __future__ import annotations

from PIL import Image


def fit_size(
    width: int, height: int, max_width: int | None, max_height: int | None
) -> tuple[int, int]:
    scale = 1.0
    if max_width is not None:
        scale = min(scale, max_width / width)
    if max_height is not None:
        scale = min(scale, max_height / height)
    if scale >= 1.0:
        return width, height
    return max(1, round(width * scale)), max(1, round(height * scale))


def resize(img: Image.Image, max_width: int | None, max_height: int | None) -> Image.Image:
    size = fit_size(img.width, img.height, max_width, max_height)
    if size == img.size:
        return img
    return img.resize(size, Image.Resampling.LANCZOS)
