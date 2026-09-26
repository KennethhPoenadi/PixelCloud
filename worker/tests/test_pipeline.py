from __future__ import annotations

import io

import pytest
from PIL import Image

from worker.pipeline import MAX_OPERATIONS, PipelineError, encode, run, validate

VALID = [
    {"version": 1, "operations": []},
    {
        "version": 1,
        "operations": [
            {"op": "preset", "name": "vintage"},
            {"op": "brightness", "value": 1.1},
            {"op": "contrast", "value": 1.2},
            {"op": "vignette", "value": 0.4},
            {"op": "resize", "max_width": 1920, "max_height": 1920},
        ],
    },
    {
        "version": 1,
        "operations": [{"op": "temperature", "value": -100}, {"op": "blur", "value": 20}],
    },
    {
        "version": 1,
        "operations": [{"op": "rotate", "angle": 270}, {"op": "flip", "direction": "h"}],
    },
]

INVALID = {
    "not object": [1, 2],
    "wrong version": {"version": 2, "operations": []},
    "bool version": {"version": True, "operations": []},
    "missing ops": {"version": 1},
    "extra field": {"version": 1, "operations": [], "x": 1},
    "unknown op": {"version": 1, "operations": [{"op": "hdr", "value": 1}]},
    "internal op not allowed": {"version": 1, "operations": [{"op": "sepia", "value": 1}]},
    "above range": {"version": 1, "operations": [{"op": "brightness", "value": 2.01}]},
    "below range": {"version": 1, "operations": [{"op": "temperature", "value": -101}]},
    "string value": {"version": 1, "operations": [{"op": "contrast", "value": "1"}]},
    "bool value": {"version": 1, "operations": [{"op": "contrast", "value": True}]},
    "nan": {"version": 1, "operations": [{"op": "contrast", "value": float("nan")}]},
    "extra op field": {"version": 1, "operations": [{"op": "contrast", "value": 1, "x": 2}]},
    "unknown preset": {"version": 1, "operations": [{"op": "preset", "name": "lomo"}]},
    "bad rotate": {"version": 1, "operations": [{"op": "rotate", "angle": 45}]},
    "bad flip": {"version": 1, "operations": [{"op": "flip", "direction": "x"}]},
    "resize no dims": {"version": 1, "operations": [{"op": "resize"}]},
    "resize fractional": {"version": 1, "operations": [{"op": "resize", "max_width": 10.5}]},
    "too many ops": {
        "version": 1,
        "operations": [{"op": "brightness", "value": 1}] * (MAX_OPERATIONS + 1),
    },
}


@pytest.mark.parametrize("pipeline", VALID)
def test_valid(pipeline: object) -> None:
    validate(pipeline)


@pytest.mark.parametrize("name", sorted(INVALID))
def test_invalid(name: str) -> None:
    with pytest.raises(PipelineError):
        validate(INVALID[name])


def test_error_mentions_index() -> None:
    with pytest.raises(PipelineError, match=r"^operations\[1\]"):
        validate({"version": 1, "operations": [{"op": "blur", "value": 1}, {"op": "nope"}]})


def test_run_applies_in_order(sample: Image.Image) -> None:
    out = run(
        sample,
        {
            "version": 1,
            "operations": [{"op": "rotate", "angle": 90}, {"op": "resize", "max_width": 24}],
        },
    )
    assert out.size == (24, 32)


def test_run_honours_exif_orientation() -> None:
    img = Image.new("RGB", (40, 20))
    exif = Image.Exif()
    exif[0x0112] = 6  # rotate 90 CW when displayed
    buf = io.BytesIO()
    img.save(buf, format="JPEG", exif=exif.tobytes())
    out = run(Image.open(io.BytesIO(buf.getvalue())), {"version": 1, "operations": []})
    assert out.size == (20, 40)


@pytest.mark.parametrize(
    ("fmt", "magic"), [("jpeg", b"\xff\xd8"), ("png", b"\x89PNG"), ("webp", b"RIFF")]
)
def test_encode_formats(sample: Image.Image, fmt: str, magic: bytes) -> None:
    data, mime, ext = encode(sample, fmt, 85)
    assert data.startswith(magic)
    assert mime.startswith("image/") and ext
    assert Image.open(io.BytesIO(data)).size == sample.size


def test_encode_jpeg_flattens_alpha() -> None:
    img = Image.new("RGBA", (4, 4), (0, 0, 0, 0))
    data, _, _ = encode(img, "jpeg", 90)
    assert Image.open(io.BytesIO(data)).getpixel((1, 1)) == pytest.approx((255, 255, 255), abs=2)


def test_encode_strips_metadata(sample: Image.Image) -> None:
    exif = Image.Exif()
    exif[0x010F] = "SecretCam"
    sample.info["exif"] = exif.tobytes()
    for fmt in ("jpeg", "png", "webp"):
        data, _, _ = encode(sample, fmt, 90)
        assert b"SecretCam" not in data


def test_encode_rejects_unknown_format(sample: Image.Image) -> None:
    with pytest.raises(PipelineError):
        encode(sample, "gif", 90)
