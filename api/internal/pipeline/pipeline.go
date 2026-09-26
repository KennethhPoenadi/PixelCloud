// Package pipeline validates filter pipeline JSON before a job is enqueued.
// The worker re-validates with identical rules (defense in depth); keep both in
// sync with worker/worker/pipeline.py.
package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	Version       = 1
	MaxOperations = 20
	MaxResizeSide = 10000
)

var Presets = []string{"grayscale", "sepia", "vintage", "warm", "cool", "vivid", "noir", "fade", "invert"}

type valueRange struct{ min, max float64 }

// Adjustment ranges from the plan (§2.1).
var adjustments = map[string]valueRange{
	"brightness":  {0, 2},
	"contrast":    {0, 2},
	"saturation":  {0, 2},
	"temperature": {-100, 100},
	"blur":        {0, 20},
	"sharpen":     {0, 3},
	"vignette":    {0, 1},
}

// Error describes the first problem found, with the operation index when relevant.
type Error struct {
	Index   int // -1 when not tied to a single operation
	Message string
}

func (e *Error) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("operations[%d]: %s", e.Index, e.Message)
	}
	return e.Message
}

func fail(i int, format string, args ...any) *Error {
	return &Error{Index: i, Message: fmt.Sprintf(format, args...)}
}

// Validate checks raw pipeline JSON and returns it re-encoded in canonical form.
func Validate(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fail(-1, "pipeline is required")
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fail(-1, "pipeline must be a JSON object")
	}
	if err := onlyKeys(doc, "version", "operations"); err != nil {
		return nil, fail(-1, "%s", err)
	}
	var version int
	if err := json.Unmarshal(doc["version"], &version); err != nil || version != Version {
		return nil, fail(-1, "version must be %d", Version)
	}
	var ops []map[string]json.RawMessage
	if err := json.Unmarshal(doc["operations"], &ops); err != nil || doc["operations"] == nil {
		return nil, fail(-1, "operations must be an array of objects")
	}
	if len(ops) > MaxOperations {
		return nil, fail(-1, "at most %d operations are allowed, got %d", MaxOperations, len(ops))
	}

	canonical := make([]map[string]any, 0, len(ops))
	for i, op := range ops {
		c, err := validateOp(op)
		if err != nil {
			return nil, fail(i, "%s", err)
		}
		canonical = append(canonical, c)
	}
	out, err := json.Marshal(map[string]any{"version": Version, "operations": canonical})
	if err != nil {
		return nil, fmt.Errorf("encode pipeline: %w", err)
	}
	return out, nil
}

func validateOp(op map[string]json.RawMessage) (map[string]any, error) {
	var name string
	if err := json.Unmarshal(op["op"], &name); err != nil || name == "" {
		return nil, fmt.Errorf(`"op" must be a non-empty string`)
	}

	if r, ok := adjustments[name]; ok {
		if err := onlyKeys(op, "op", "value"); err != nil {
			return nil, err
		}
		v, err := number(op, "value")
		if err != nil {
			return nil, err
		}
		if v < r.min || v > r.max {
			return nil, fmt.Errorf("%s value must be between %g and %g, got %g", name, r.min, r.max, v)
		}
		return map[string]any{"op": name, "value": v}, nil
	}

	switch name {
	case "preset":
		if err := onlyKeys(op, "op", "name"); err != nil {
			return nil, err
		}
		var preset string
		if err := json.Unmarshal(op["name"], &preset); err != nil {
			return nil, fmt.Errorf(`preset "name" must be a string`)
		}
		if !contains(Presets, preset) {
			return nil, fmt.Errorf("unknown preset %q (valid: %s)", preset, strings.Join(Presets, ", "))
		}
		return map[string]any{"op": name, "name": preset}, nil

	case "rotate":
		if err := onlyKeys(op, "op", "angle"); err != nil {
			return nil, err
		}
		v, err := number(op, "angle")
		if err != nil {
			return nil, err
		}
		if v != 90 && v != 180 && v != 270 {
			return nil, fmt.Errorf("rotate angle must be 90, 180 or 270")
		}
		return map[string]any{"op": name, "angle": int(v)}, nil

	case "flip":
		if err := onlyKeys(op, "op", "direction"); err != nil {
			return nil, err
		}
		var dir string
		if err := json.Unmarshal(op["direction"], &dir); err != nil || (dir != "h" && dir != "v") {
			return nil, fmt.Errorf(`flip direction must be "h" or "v"`)
		}
		return map[string]any{"op": name, "direction": dir}, nil

	case "resize":
		if err := onlyKeys(op, "op", "max_width", "max_height"); err != nil {
			return nil, err
		}
		out := map[string]any{"op": name}
		for _, key := range []string{"max_width", "max_height"} {
			if _, present := op[key]; !present {
				continue
			}
			v, err := number(op, key)
			if err != nil {
				return nil, err
			}
			if v != math.Trunc(v) || v < 1 || v > MaxResizeSide {
				return nil, fmt.Errorf("%s must be an integer between 1 and %d", key, MaxResizeSide)
			}
			out[key] = int(v)
		}
		if len(out) == 1 {
			return nil, fmt.Errorf("resize needs max_width and/or max_height")
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown operation %q", name)
}

func number(op map[string]json.RawMessage, key string) (float64, error) {
	raw, ok := op[key]
	if !ok {
		return 0, fmt.Errorf("%q is required", key)
	}
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%q must be a number", key)
	}
	return v, nil
}

func onlyKeys(m map[string]json.RawMessage, allowed ...string) error {
	var extra []string
	for k := range m {
		if !contains(allowed, k) {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return fmt.Errorf("unknown field(s): %s", strings.Join(extra, ", "))
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
