// Package cropbridge validates the browser protocol around the unmodified
// compiled CropPlan activity. Geometry belongs exclusively to the Gooo program.
package cropbridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type Rectangle struct {
	MinX int64 `json:"min_x"`
	MinY int64 `json:"min_y"`
	MaxX int64 `json:"max_x"`
	MaxY int64 `json:"max_y"`
}

type Output struct {
	MinX      string `json:"min_x"`
	MinY      string `json:"min_y"`
	MaxX      string `json:"max_x"`
	MaxY      string `json:"max_y"`
	HasPixels bool   `json:"has_pixels"`
}

type response struct {
	OK     bool     `json:"ok"`
	Inputs []string `json:"inputs,omitempty"`
	Output *Output  `json:"output,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// Run accepts two rectangles as eight canonical decimal int64 strings, in
// min_x, min_y, max_x, max_y order. It does not normalize or restrict bounds.
// Only valid input reaches compute; all integer output remains decimal text.
func Run(inputs []string, compute func(Rectangle, Rectangle) any) (result string) {
	defer func() {
		if recover() != nil {
			result = Failure("computation failed")
		}
	}()
	if len(inputs) != 8 {
		return Failure("expected exactly eight decimal input strings")
	}
	inputs = append([]string(nil), inputs...)
	var values [8]int64
	for i, raw := range inputs {
		n, err := decimal(raw)
		if err != nil {
			return Failure(fmt.Sprintf("input %d is not a canonical decimal int64", i+1))
		}
		values[i] = n
	}
	if compute == nil {
		return Failure("compiled program is unavailable")
	}
	a := Rectangle{values[0], values[1], values[2], values[3]}
	b := Rectangle{values[4], values[5], values[6], values[7]}
	raw, err := json.Marshal(compute(a, b))
	if err != nil {
		return Failure("compiled program returned an invalid record")
	}
	out, err := DecodeOutput(raw)
	if err != nil {
		return Failure("compiled program returned an invalid record")
	}
	return encode(response{OK: true, Inputs: inputs, Output: &out})
}

// DecodeOutput verifies exact field names and types, preserving every int64.
// It performs no intersection or emptiness calculation.
func DecodeOutput(raw []byte) (Output, error) {
	var out Output
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return out, fmt.Errorf("output must be an object")
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return out, err
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return out, fmt.Errorf("invalid or duplicate output field")
		}
		seen[name] = true
		var value any
		if err := d.Decode(&value); err != nil {
			return out, err
		}
		if name == "has_pixels" {
			flag, ok := value.(bool)
			if !ok {
				return out, fmt.Errorf("has_pixels must be Boolean")
			}
			out.HasPixels = flag
			continue
		}
		number, ok := value.(json.Number)
		if !ok {
			return out, fmt.Errorf("coordinate must be an integer")
		}
		if _, err := decimal(number.String()); err != nil {
			return out, err
		}
		switch name {
		case "min_x":
			out.MinX = number.String()
		case "min_y":
			out.MinY = number.String()
		case "max_x":
			out.MaxX = number.String()
		case "max_y":
			out.MaxY = number.String()
		default:
			return out, fmt.Errorf("unexpected output field")
		}
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(seen) != 5 {
		return out, fmt.Errorf("incomplete output record")
	}
	if _, err := d.Token(); err != io.EOF {
		return out, fmt.Errorf("trailing output content")
	}
	return out, nil
}

func decimal(raw string) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != raw {
		return 0, fmt.Errorf("not a canonical decimal int64")
	}
	return n, nil
}

func Failure(reason string) string { return encode(response{Error: reason}) }

func encode(value response) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
