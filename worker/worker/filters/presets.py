"""Preset definitions: the single source of truth for what each preset does.

Each preset expands to base operations. `sepia` and `invert` are internal ops
that users can only reach through presets. The frontend mirrors this table in
frontend/src/lib/presets.ts for the live preview — keep them in sync.
"""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

Op = Mapping[str, Any]

PRESETS: dict[str, list[Op]] = {
    "grayscale": [{"op": "saturation", "value": 0.0}],
    "sepia": [{"op": "sepia", "value": 1.0}],
    "vintage": [
        {"op": "sepia", "value": 0.6},
        {"op": "contrast", "value": 0.85},
        {"op": "vignette", "value": 0.5},
    ],
    "warm": [{"op": "temperature", "value": 35.0}],
    "cool": [{"op": "temperature", "value": -35.0}],
    "vivid": [{"op": "saturation", "value": 1.4}, {"op": "contrast", "value": 1.15}],
    "noir": [{"op": "saturation", "value": 0.0}, {"op": "contrast", "value": 1.5}],
    # lifted blacks: contrast < 1 raises the black point, brightness restores whites
    "fade": [
        {"op": "contrast", "value": 0.9},
        {"op": "brightness", "value": 1.05},
        {"op": "saturation", "value": 0.7},
    ],
    "invert": [{"op": "invert"}],
}


def expand(ops: list[Op]) -> list[Op]:
    """Replace every preset op with its base operations, preserving order."""
    out: list[Op] = []
    for op in ops:
        if op["op"] == "preset":
            out.extend(PRESETS[str(op["name"])])
        else:
            out.append(op)
    return out
