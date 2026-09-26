from __future__ import annotations

import numpy as np
import pytest
from PIL import Image


def make_sample(width: int = 64, height: int = 48) -> Image.Image:
    """Deterministic test image: RGB gradients plus flat colour patches."""
    xs = np.linspace(0, 255, width, dtype=np.float32)
    ys = np.linspace(0, 255, height, dtype=np.float32)
    r = np.tile(xs, (height, 1))
    g = np.tile(ys[:, None], (1, width))
    b = 255 - (r + g) / 2
    arr = np.stack([r, g, b], axis=-1)
    arr[4:14, 4:14] = (220, 40, 40)
    arr[4:14, 20:30] = (40, 200, 60)
    arr[30:44, 40:60] = (128, 128, 128)
    return Image.fromarray(arr.round().astype(np.uint8), mode="RGB")


@pytest.fixture
def sample() -> Image.Image:
    return make_sample()


def pixels(img: Image.Image) -> np.ndarray:
    return np.asarray(img.convert("RGB"), dtype=np.int16)
