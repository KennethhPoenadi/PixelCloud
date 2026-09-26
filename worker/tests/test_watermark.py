from __future__ import annotations

import numpy as np
from PIL import Image

from worker import watermark


def test_watermark_marks_bottom_right_only() -> None:
    img = Image.new("RGB", (400, 300), (30, 30, 30))
    out = watermark.apply(img)
    assert out.mode == "RGB" and out.size == img.size
    arr = np.asarray(out, dtype=np.int16)
    corner = arr[-60:, -200:]
    rest = arr[:150, :150]
    assert (corner > 60).any(), "expected light text pixels near the corner"
    assert (rest == 30).all(), "the rest of the image must be untouched"


def test_watermark_keeps_alpha_mode() -> None:
    img = Image.new("RGBA", (200, 200), (10, 10, 10, 255))
    assert watermark.apply(img).mode == "RGBA"


def test_watermark_tiny_image() -> None:
    assert watermark.apply(Image.new("RGB", (16, 16))).size == (16, 16)
