package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestValidateAccepts(t *testing.T) {
	cases := []string{
		`{"version":1,"operations":[]}`,
		`{"version":1,"operations":[{"op":"preset","name":"vintage"},{"op":"brightness","value":1.1},{"op":"contrast","value":1.2},{"op":"vignette","value":0.4},{"op":"resize","max_width":1920,"max_height":1920}]}`,
		`{"version":1,"operations":[{"op":"temperature","value":-100},{"op":"blur","value":20},{"op":"sharpen","value":3}]}`,
		`{"version":1,"operations":[{"op":"rotate","angle":270},{"op":"flip","direction":"h"},{"op":"resize","max_height":10}]}`,
		`{"version":1,"operations":[{"op":"saturation","value":0},{"op":"brightness","value":2}]}`,
	}
	for _, c := range cases {
		if _, err := Validate(json.RawMessage(c)); err != nil {
			t.Errorf("Validate(%s) = %v, want ok", c, err)
		}
	}
}

func TestValidateCanonicalizes(t *testing.T) {
	out, err := Validate(json.RawMessage(`{"operations":[{"value":1.5,"op":"contrast"},{"op":"rotate","angle":90.0}],"version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"operations":[{"op":"contrast","value":1.5},{"angle":90,"op":"rotate"}],"version":1}`
	if string(out) != want {
		t.Fatalf("got %s\nwant %s", out, want)
	}
}

func TestValidateRejects(t *testing.T) {
	tooMany := make([]string, MaxOperations+1)
	for i := range tooMany {
		tooMany[i] = `{"op":"brightness","value":1}`
	}
	cases := map[string]string{
		"empty":             ``,
		"not object":        `[1,2]`,
		"wrong version":     `{"version":2,"operations":[]}`,
		"missing ops":       `{"version":1}`,
		"extra top field":   `{"version":1,"operations":[],"x":1}`,
		"unknown op":        `{"version":1,"operations":[{"op":"hdr","value":1}]}`,
		"missing op":        `{"version":1,"operations":[{"value":1}]}`,
		"above range":       `{"version":1,"operations":[{"op":"brightness","value":2.01}]}`,
		"below range":       `{"version":1,"operations":[{"op":"temperature","value":-101}]}`,
		"blur too big":      `{"version":1,"operations":[{"op":"blur","value":21}]}`,
		"value not number":  `{"version":1,"operations":[{"op":"contrast","value":"1"}]}`,
		"missing value":     `{"version":1,"operations":[{"op":"contrast"}]}`,
		"extra op field":    `{"version":1,"operations":[{"op":"contrast","value":1,"foo":2}]}`,
		"unknown preset":    `{"version":1,"operations":[{"op":"preset","name":"lomo"}]}`,
		"bad rotate":        `{"version":1,"operations":[{"op":"rotate","angle":45}]}`,
		"bad flip":          `{"version":1,"operations":[{"op":"flip","direction":"x"}]}`,
		"resize no dims":    `{"version":1,"operations":[{"op":"resize"}]}`,
		"resize fractional": `{"version":1,"operations":[{"op":"resize","max_width":10.5}]}`,
		"resize too big":    `{"version":1,"operations":[{"op":"resize","max_width":10001}]}`,
		"too many ops":      fmt.Sprintf(`{"version":1,"operations":[%s]}`, strings.Join(tooMany, ",")),
	}
	for name, c := range cases {
		if _, err := Validate(json.RawMessage(c)); err == nil {
			t.Errorf("%s: Validate(%s) succeeded, want error", name, c)
		}
	}
}

func TestErrorIncludesIndex(t *testing.T) {
	_, err := Validate(json.RawMessage(`{"version":1,"operations":[{"op":"brightness","value":1},{"op":"nope"}]}`))
	if err == nil || !strings.HasPrefix(err.Error(), "operations[1]:") {
		t.Fatalf("error = %v, want operations[1] prefix", err)
	}
}
