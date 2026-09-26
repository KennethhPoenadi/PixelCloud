"""Image operations. Each module implements one operation; apply_op dispatches."""

from __future__ import annotations

from PIL import Image

from worker.filters.blur import blur
from worker.filters.brightness import brightness
from worker.filters.contrast import contrast
from worker.filters.flip import flip
from worker.filters.invert import invert
from worker.filters.presets import Op
from worker.filters.resize import resize
from worker.filters.rotate import rotate
from worker.filters.saturation import saturation
from worker.filters.sepia import sepia
from worker.filters.sharpen import sharpen
from worker.filters.temperature import temperature
from worker.filters.vignette import vignette


def _opt_int(op: Op, key: str) -> int | None:
    value = op.get(key)
    return None if value is None else int(value)


def apply_op(img: Image.Image, op: Op) -> Image.Image:
    """Apply one already-validated, preset-expanded operation."""
    match op["op"]:
        case "brightness":
            return brightness(img, float(op["value"]))
        case "contrast":
            return contrast(img, float(op["value"]))
        case "saturation":
            return saturation(img, float(op["value"]))
        case "temperature":
            return temperature(img, float(op["value"]))
        case "blur":
            return blur(img, float(op["value"]))
        case "sharpen":
            return sharpen(img, float(op["value"]))
        case "vignette":
            return vignette(img, float(op["value"]))
        case "sepia":
            return sepia(img, float(op["value"]))
        case "invert":
            return invert(img)
        case "rotate":
            return rotate(img, int(op["angle"]))
        case "flip":
            return flip(img, str(op["direction"]))
        case "resize":
            return resize(img, _opt_int(op, "max_width"), _opt_int(op, "max_height"))
    raise ValueError(f"unsupported operation {op['op']!r}")
