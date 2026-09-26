"""Pipeline validation (mirrors api/internal/pipeline) and execution."""

from __future__ import annotations

import io
import math
from collections.abc import Mapping
from typing import Any

from PIL import Image, ImageOps

from worker.filters import apply_op
from worker.filters._array import normalize_mode
from worker.filters.presets import PRESETS, Op, expand

VERSION = 1
MAX_OPERATIONS = 20
MAX_RESIZE_SIDE = 10000

ADJUSTMENTS: dict[str, tuple[float, float]] = {
    "brightness": (0.0, 2.0),
    "contrast": (0.0, 2.0),
    "saturation": (0.0, 2.0),
    "temperature": (-100.0, 100.0),
    "blur": (0.0, 20.0),
    "sharpen": (0.0, 3.0),
    "vignette": (0.0, 1.0),
}

OUTPUT_FORMATS: dict[str, tuple[str, str, str]] = {
    # format -> (PIL format, content type, file extension)
    "jpeg": ("JPEG", "image/jpeg", "jpg"),
    "png": ("PNG", "image/png", "png"),
    "webp": ("WEBP", "image/webp", "webp"),
}


class PipelineError(ValueError):
    """The pipeline is invalid; retrying the job will not help."""


def _only_keys(op: Mapping[str, Any], *allowed: str) -> None:
    extra = sorted(set(op) - set(allowed))
    if extra:
        raise PipelineError(f"unknown field(s): {', '.join(extra)}")


def _number(op: Mapping[str, Any], key: str) -> float:
    if key not in op:
        raise PipelineError(f"{key!r} is required")
    value = op[key]
    # bool is an int subclass in Python; JSON true must not pass as 1
    if isinstance(value, bool) or not isinstance(value, int | float) or not math.isfinite(value):
        raise PipelineError(f"{key!r} must be a number")
    return float(value)


def _validate_op(op: Any) -> Op:
    if not isinstance(op, dict):
        raise PipelineError("operation must be an object")
    name = op.get("op")
    if not isinstance(name, str) or not name:
        raise PipelineError('"op" must be a non-empty string')

    if name in ADJUSTMENTS:
        _only_keys(op, "op", "value")
        lo, hi = ADJUSTMENTS[name]
        value = _number(op, "value")
        if not lo <= value <= hi:
            raise PipelineError(f"{name} value must be between {lo:g} and {hi:g}, got {value:g}")
        return {"op": name, "value": value}

    if name == "preset":
        _only_keys(op, "op", "name")
        preset = op.get("name")
        if not isinstance(preset, str) or preset not in PRESETS:
            raise PipelineError(f"unknown preset {preset!r}")
        return {"op": name, "name": preset}

    if name == "rotate":
        _only_keys(op, "op", "angle")
        angle = _number(op, "angle")
        if angle not in (90, 180, 270):
            raise PipelineError("rotate angle must be 90, 180 or 270")
        return {"op": name, "angle": int(angle)}

    if name == "flip":
        _only_keys(op, "op", "direction")
        direction = op.get("direction")
        if direction not in ("h", "v"):
            raise PipelineError('flip direction must be "h" or "v"')
        return {"op": name, "direction": direction}

    if name == "resize":
        _only_keys(op, "op", "max_width", "max_height")
        out: dict[str, Any] = {"op": name}
        for key in ("max_width", "max_height"):
            if key not in op:
                continue
            value = _number(op, key)
            if value != int(value) or not 1 <= value <= MAX_RESIZE_SIDE:
                raise PipelineError(f"{key} must be an integer between 1 and {MAX_RESIZE_SIDE}")
            out[key] = int(value)
        if len(out) == 1:
            raise PipelineError("resize needs max_width and/or max_height")
        return out

    raise PipelineError(f"unknown operation {name!r}")


def validate(pipeline: Any) -> list[Op]:
    """Validate a decoded pipeline document and return its operations."""
    if not isinstance(pipeline, dict):
        raise PipelineError("pipeline must be a JSON object")
    _only_keys(pipeline, "version", "operations")
    version = pipeline.get("version")
    if isinstance(version, bool) or version != VERSION:
        raise PipelineError(f"version must be {VERSION}")
    ops = pipeline.get("operations")
    if not isinstance(ops, list):
        raise PipelineError("operations must be an array of objects")
    if len(ops) > MAX_OPERATIONS:
        raise PipelineError(f"at most {MAX_OPERATIONS} operations are allowed, got {len(ops)}")
    validated: list[Op] = []
    for i, op in enumerate(ops):
        try:
            validated.append(_validate_op(op))
        except PipelineError as exc:
            raise PipelineError(f"operations[{i}]: {exc}") from None
    return validated


def run(img: Image.Image, pipeline: Any) -> Image.Image:
    """Validate, expand presets and apply every operation in order."""
    ops = expand(validate(pipeline))
    # Apply camera orientation first so the pipeline sees the image as displayed.
    img = normalize_mode(ImageOps.exif_transpose(img) or img)
    for op in ops:
        img = apply_op(img, op)
    return img


def encode(img: Image.Image, fmt: str, quality: int) -> tuple[bytes, str, str]:
    """Encode to the output format without any metadata. Returns (bytes, mime, ext)."""
    if fmt not in OUTPUT_FORMATS:
        raise PipelineError(f"unsupported output format {fmt!r}")
    pil_format, mime, ext = OUTPUT_FORMATS[fmt]
    buf = io.BytesIO()
    if pil_format == "JPEG":
        if img.mode != "RGB":
            # JPEG has no alpha: composite transparent areas onto white
            background = Image.new("RGB", img.size, (255, 255, 255))
            rgba = img.convert("RGBA")
            background.paste(rgba, mask=rgba.getchannel("A"))
            img = background
        img.save(buf, format="JPEG", quality=quality, optimize=True, progressive=True)
    elif pil_format == "WEBP":
        img.save(buf, format="WEBP", quality=quality, method=4)
    else:
        img.save(buf, format="PNG", optimize=True)
    return buf.getvalue(), mime, ext
