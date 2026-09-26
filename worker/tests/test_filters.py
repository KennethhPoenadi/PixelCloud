from __future__ import annotations

import numpy as np
import pytest
from PIL import Image

from tests.conftest import pixels
from worker.filters import apply_op
from worker.filters.blur import blur
from worker.filters.brightness import brightness
from worker.filters.contrast import contrast
from worker.filters.flip import flip
from worker.filters.invert import invert
from worker.filters.presets import PRESETS, expand
from worker.filters.resize import fit_size, resize
from worker.filters.rotate import rotate
from worker.filters.saturation import saturation
from worker.filters.sepia import sepia
from worker.filters.sharpen import sharpen
from worker.filters.temperature import temperature
from worker.filters.vignette import vignette, vignette_mask


def solid(rgb: tuple[int, int, int], size: tuple[int, int] = (8, 8)) -> Image.Image:
    return Image.new("RGB", size, rgb)


@pytest.mark.parametrize(
    ("fn", "neutral"),
    [
        (brightness, 1.0),
        (contrast, 1.0),
        (saturation, 1.0),
        (temperature, 0.0),
        (blur, 0.0),
        (sharpen, 0.0),
        (vignette, 0.0),
        (sepia, 0.0),
    ],
)
def test_neutral_value_is_identity(sample: Image.Image, fn, neutral) -> None:  # type: ignore[no-untyped-def]
    assert np.array_equal(pixels(fn(sample, neutral)), pixels(sample))


def test_brightness_scales_and_clamps() -> None:
    assert pixels(brightness(solid((100, 50, 200)), 1.5))[0, 0].tolist() == [150, 75, 255]
    assert pixels(brightness(solid((100, 50, 200)), 0.0))[0, 0].tolist() == [0, 0, 0]


def test_contrast_pivots_on_mid_grey() -> None:
    assert pixels(contrast(solid((200, 55, 128)), 0.0))[0, 0].tolist() == [128, 128, 128]
    assert pixels(contrast(solid((200, 55, 128)), 2.0))[0, 0].tolist() == [255, 0, 129]


def test_saturation_zero_is_greyscale(sample: Image.Image) -> None:
    out = pixels(saturation(sample, 0.0))
    assert np.abs(out[..., 0] - out[..., 1]).max() <= 1
    assert np.abs(out[..., 1] - out[..., 2]).max() <= 1


def test_saturation_boost_moves_away_from_grey() -> None:
    before = pixels(solid((180, 100, 100)))[0, 0]
    after = pixels(saturation(solid((180, 100, 100)), 2.0))[0, 0]
    assert after[0] > before[0] and after[1] < before[1]


def test_temperature_direction() -> None:
    grey = solid((128, 128, 128))
    warm = pixels(temperature(grey, 100))[0, 0]
    cool = pixels(temperature(grey, -100))[0, 0]
    assert warm[0] > 128 > warm[2] and warm[1] == 128
    assert cool[2] > 128 > cool[0]


def test_blur_smooths_edges(sample: Image.Image) -> None:
    before = pixels(sample).astype(np.float32)
    after = pixels(blur(sample, 3)).astype(np.float32)
    assert np.abs(np.diff(after, axis=1)).mean() < np.abs(np.diff(before, axis=1)).mean()


def test_sharpen_increases_local_contrast(sample: Image.Image) -> None:
    soft = blur(sample, 2)
    before = np.abs(np.diff(pixels(soft).astype(np.float32), axis=1)).mean()
    after = np.abs(np.diff(pixels(sharpen(soft, 3)).astype(np.float32), axis=1)).mean()
    assert after > before


def test_vignette_darkens_corners_not_centre() -> None:
    img = solid((200, 200, 200), (101, 101))
    out = pixels(vignette(img, 1.0))
    assert out[50, 50].tolist() == [200, 200, 200]
    assert out[0, 0, 0] < 20
    mask = vignette_mask(10, 10, 0.5)
    assert mask.min() >= 0.5 and mask.max() == pytest.approx(1.0)


def test_sepia_full_matches_css_matrix() -> None:
    out = pixels(sepia(solid((100, 100, 100)), 1.0))[0, 0].tolist()
    # CSS sepia(1) of grey 100: (0.393+0.769+0.189)*100 etc.
    assert out == [135, 120, 94]


def test_invert() -> None:
    assert pixels(invert(solid((0, 128, 255))))[0, 0].tolist() == [255, 127, 0]


def test_rotate_clockwise() -> None:
    img = Image.new("RGB", (4, 2), (0, 0, 0))
    img.putpixel((0, 0), (255, 0, 0))  # top-left
    out = rotate(img, 90)
    assert out.size == (2, 4)
    assert out.getpixel((1, 0)) == (255, 0, 0)  # top-left moves to top-right
    assert rotate(img, 180).getpixel((3, 1)) == (255, 0, 0)
    assert rotate(img, 270).getpixel((0, 3)) == (255, 0, 0)


def test_flip() -> None:
    img = Image.new("RGB", (3, 3), (0, 0, 0))
    img.putpixel((0, 0), (255, 0, 0))
    assert flip(img, "h").getpixel((2, 0)) == (255, 0, 0)
    assert flip(img, "v").getpixel((0, 2)) == (255, 0, 0)


def test_resize_keeps_ratio_and_never_upscales() -> None:
    assert fit_size(4000, 3000, 1920, 1920) == (1920, 1440)
    assert fit_size(3000, 4000, 1920, None) == (1920, 2560)
    assert fit_size(800, 600, 1920, 1920) == (800, 600)
    assert resize(solid((1, 2, 3), (100, 50)), 10, None).size == (10, 5)


def test_alpha_is_preserved() -> None:
    img = Image.new("RGBA", (4, 4), (100, 100, 100, 77))
    for op in ({"op": "brightness", "value": 1.5}, {"op": "blur", "value": 1}, {"op": "invert"}):
        out = apply_op(img, op)
        assert out.mode == "RGBA"
        assert out.getpixel((1, 1))[3] == 77


def test_palette_image_is_normalised() -> None:
    img = Image.new("P", (4, 4), 3)
    assert apply_op(img, {"op": "brightness", "value": 1.2}).mode == "RGB"


def test_every_preset_expands_to_known_ops(sample: Image.Image) -> None:
    for name in PRESETS:
        ops = expand([{"op": "preset", "name": name}])
        assert ops and all(op["op"] != "preset" for op in ops)
        out = sample
        for op in ops:
            out = apply_op(out, op)
        assert out.size == sample.size
