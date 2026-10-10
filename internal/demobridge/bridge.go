// Package demobridge validates the browser protocol around a precompiled Gooo
// program. It contains no implementations or reference oracles for the ports.
package demobridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

type operation struct {
	minimum, maximum int64
	pairInput        bool
	output           string
}

func lookup(name string) (operation, bool) {
	const minRune, maxRune = -1 << 31, 1<<31 - 1
	const minInt, maxInt = -1 << 63, 1<<63 - 1
	switch name {
	case "http.StatusText":
		return operation{minInt, maxInt, false, "text"}, true
	case "utf8.ValidRune", "utf16.IsSurrogate":
		return operation{minRune, maxRune, false, "boolean"}, true
	case "utf8.RuneLen", "utf16.RuneLen":
		return operation{minRune, maxRune, false, "integer"}, true
	case "utf16.DecodeRune":
		return operation{minRune, maxRune, true, "integer"}, true
	case "utf16.EncodeRune":
		return operation{minRune, maxRune, false, "pair"}, true
	case "utf8.RuneStart":
		return operation{0, 255, false, "boolean"}, true
	case "bits.Len8", "bits.LeadingZeros8", "bits.TrailingZeros8", "bits.OnesCount8", "bits.Reverse8":
		return operation{0, 255, false, "integer"}, true
	default:
		return operation{}, false
	}
}

type output struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type response struct {
	OK        bool     `json:"ok"`
	Operation string   `json:"operation,omitempty"`
	Inputs    []string `json:"inputs,omitempty"`
	Output    *output  `json:"output,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Run accepts canonical decimal strings, validates the original Go API's input
// domain, and calls compute exactly once only after validation. compute must
// return a tagged Go record with text, value, flag, high and low fields. Its
// numeric values are decoded with UseNumber and emitted as decimal strings.
func Run(name, a, b string, compute func(string, int64, int64) any) (result string) {
	defer func() {
		if recover() != nil {
			result = failure("computation failed")
		}
	}()
	op, ok := lookup(name)
	if !ok {
		return failure("unknown operation")
	}
	first, err := decimal(a)
	if err != nil || first < op.minimum || first > op.maximum {
		return failure("first input is outside the operation's decimal integer domain")
	}
	second, err := decimal(b)
	if err != nil || (!op.pairInput && b != "0") || (op.pairInput && (second < op.minimum || second > op.maximum)) {
		return failure("second input must be an int32 rune for DecodeRune, otherwise exactly 0")
	}
	if compute == nil {
		return failure("compiled program is unavailable")
	}
	raw, err := json.Marshal(compute(name, first, second))
	if err != nil {
		return failure("compiled program returned an invalid reply")
	}
	fields, err := decodeReply(raw)
	if err != nil {
		return failure("compiled program returned an invalid reply")
	}
	value := fields["text"]
	switch op.output {
	case "integer":
		value = fields["value"].(json.Number).String()
	case "boolean":
		value = fields["flag"]
	case "pair":
		value = map[string]string{"high": fields["high"].(json.Number).String(), "low": fields["low"].(json.Number).String()}
	}
	return encode(response{OK: true, Operation: name, Inputs: []string{a, b}, Output: &output{Type: op.output, Value: value}})
}

func decimal(raw string) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != raw {
		return 0, fmt.Errorf("not a canonical decimal int64")
	}
	return n, nil
}

func decodeReply(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("reply must be an object")
	}
	fields := map[string]any{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if _, duplicate := fields[key]; !ok || duplicate {
			return nil, fmt.Errorf("invalid or duplicate reply field")
		}
		var value any
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		switch key {
		case "text":
			_, ok = value.(string)
		case "flag":
			_, ok = value.(bool)
		case "value", "high", "low":
			var number json.Number
			number, ok = value.(json.Number)
			if ok {
				_, err = decimal(number.String())
				ok = err == nil
			}
		default:
			ok = false
		}
		if !ok {
			return nil, fmt.Errorf("invalid reply field %q", key)
		}
		fields[key] = value
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') || len(fields) != 5 {
		return nil, fmt.Errorf("incomplete reply")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing reply content")
	}
	return fields, nil
}

func failure(reason string) string { return encode(response{Error: reason}) }

func encode(value response) string {
	raw, _ := json.Marshal(value) // Only validated scalars and string-keyed records reach this point.
	return string(raw)
}
