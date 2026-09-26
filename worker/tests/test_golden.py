"""Golden image tests: render every preset and a mixed pipeline on a fixed sample
and compare with stored PNGs. Regenerate after an intentional change with:

    UPDATE_GOLDEN=1 pytest tests/test_golden.py
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import numpy as np
import pytest
from PIL import Image

from tests.conftest import make_sample, pixels
from worker.filters.presets import PRESETS
from worker.pipeline import run

GOLDEN_DIR = Path(__file__).parent / "golden"
UPDATE = os.environ.get("UPDATE_GOLDEN") == "1"

# small tolerance: numpy/Pillow versions may round a pixel differently
MAX_PIXEL_DIFF = 3
MAX_MEAN_DIFF = 0.5

CASES: dict[str, dict[str, Any]] = {
    f"preset_{name}": {"version": 1, "operations": [{"op": "preset", "name": name}]}
    for name in PRESETS
}
CASES["mixed"] = {
    "version": 1,
    "operations": [
        {"op": "preset", "name": "vintage"},
        {"op": "brightness", "value": 1.1},
        {"op": "temperature", "value": 20},
        {"op": "sharpen", "value": 1},
        {"op": "blur", "value": 1},
        {"op": "rotate", "angle": 90},
        {"op": "flip", "direction": "h"},
        {"op": "resize", "max_width": 32},
    ],
}


@pytest.mark.parametrize("name", sorted(CASES))
def test_golden(name: str) -> None:
    out = run(make_sample(), CASES[name])
    path = GOLDEN_DIR / f"{name}.png"
    if UPDATE or not path.exists():
        if not UPDATE:
            pytest.fail(f"missing golden image {path.name}; run with UPDATE_GOLDEN=1")
        GOLDEN_DIR.mkdir(exist_ok=True)
        out.save(path)
        return
    expected = Image.open(path)
    assert out.size == expected.size
    diff = np.abs(pixels(out) - pixels(expected))
    assert diff.max() <= MAX_PIXEL_DIFF, f"{name}: max pixel diff {diff.max()}"
    assert diff.mean() <= MAX_MEAN_DIFF, f"{name}: mean pixel diff {diff.mean():.3f}"
