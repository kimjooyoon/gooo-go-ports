package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type browserOutput struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type browserCase struct {
	Operation string        `json:"operation"`
	Inputs    [2]string     `json:"inputs"`
	Expected  browserOutput `json:"expected"`
}

type browserRejection struct {
	Operation string    `json:"operation"`
	Inputs    [2]string `json:"inputs"`
}

type browserFixtures struct {
	Schema           string             `json:"schema"`
	GoVersion        string             `json:"go_version"`
	CompilerRevision string             `json:"compiler_revision"`
	GoSourceSHA      map[string]string  `json:"go_source_sha256"`
	Cases            []browserCase      `json:"cases"`
	Rejections       []browserRejection `json:"rejections"`
}

// writeBrowserFixtures runs only after run verifies the native Go toolchain and
// upstream file hashes. Expectations reuse the existing independent oracle
// corpus; the two additional HTTP inputs observe adjacent integers above 2^53.
func writeBrowserFixtures(path string, suites []suite, lock upstream) error {
	fixtures, err := makeBrowserFixtures(suites, lock)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(fixtures)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func makeBrowserFixtures(suites []suite, lock upstream) (browserFixtures, error) {
	result := browserFixtures{Schema: "gooo/browser-fixtures/v1", GoVersion: lock.GoVersion,
		CompilerRevision: lock.CompilerRevision, GoSourceSHA: lock.SourceSHA}
	for _, s := range suites {
		var prefix string
		switch s.Package {
		case "app/response", "app/byte-summary":
			continue
		case "go/net/http":
			prefix = "http."
		case "go/unicode/utf8":
			prefix = "utf8."
		case "go/unicode/utf16":
			prefix = "utf16."
		case "go/math/bits":
			prefix = "bits."
		default:
			return browserFixtures{}, fmt.Errorf("unknown browser fixture package %s", s.Package)
		}
		for _, c := range s.Cases {
			row, err := browserRow(prefix+s.Activity, c)
			if err != nil {
				return browserFixtures{}, err
			}
			result.Cases = append(result.Cases, row)
		}
	}
	for _, n := range []int64{9007199254740992, 9007199254740993} {
		row, err := browserRow("http.StatusText", testCase{Inputs: []any{n}, Expected: http.StatusText(int(n))})
		if err != nil {
			return browserFixtures{}, err
		}
		result.Cases = append(result.Cases, row)
	}
	result.Rejections = browserRejections()
	return result, nil
}

func browserRow(operation string, c testCase) (browserCase, error) {
	row := browserCase{Operation: operation, Inputs: [2]string{"0", "0"}}
	arity := 1
	if operation == "utf16.DecodeRune" {
		arity = 2
	}
	if len(c.Inputs) != arity {
		return browserCase{}, fmt.Errorf("invalid browser fixture input arity: %s", operation)
	}
	for i, input := range c.Inputs {
		n, ok := input.(int64)
		if !ok {
			return browserCase{}, fmt.Errorf("browser fixture input must be int64: %s", operation)
		}
		row.Inputs[i] = strconv.FormatInt(n, 10)
	}
	raw, err := json.Marshal(c.Expected)
	if err != nil {
		return browserCase{}, err
	}
	expected, err := decode(raw) // Existing exact-number decoder; no float64 conversion.
	if err != nil {
		return browserCase{}, err
	}
	switch operation {
	case "http.StatusText":
		value, ok := expected.(string)
		if !ok {
			return browserCase{}, fmt.Errorf("expected text oracle output")
		}
		row.Expected = browserOutput{"text", value}
	case "utf8.ValidRune", "utf8.RuneStart", "utf16.IsSurrogate":
		value, ok := expected.(bool)
		if !ok {
			return browserCase{}, fmt.Errorf("expected Boolean oracle output")
		}
		row.Expected = browserOutput{"boolean", value}
	case "utf16.EncodeRune":
		pair, ok := expected.(map[string]any)
		if !ok || len(pair) != 2 {
			return browserCase{}, fmt.Errorf("expected pair oracle output")
		}
		high, err := browserInteger(pair["high"])
		if err != nil {
			return browserCase{}, err
		}
		low, err := browserInteger(pair["low"])
		if err != nil {
			return browserCase{}, err
		}
		row.Expected = browserOutput{"pair", map[string]string{"high": high, "low": low}}
	case "utf8.RuneLen", "utf16.RuneLen", "utf16.DecodeRune", "bits.Len8", "bits.LeadingZeros8", "bits.TrailingZeros8", "bits.OnesCount8", "bits.Reverse8":
		value, err := browserInteger(expected)
		if err != nil {
			return browserCase{}, err
		}
		row.Expected = browserOutput{"integer", value}
	default:
		return browserCase{}, fmt.Errorf("unknown browser fixture operation %s", operation)
	}
	return row, nil
}

func browserInteger(value any) (string, error) {
	n, ok := value.(json.Number)
	if !ok {
		return "", fmt.Errorf("expected integer oracle output")
	}
	i, err := n.Int64()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(i, 10), nil
}

func browserRejections() []browserRejection {
	result := []browserRejection{{"unknown.Operation", [2]string{"0", "0"}}}
	for _, raw := range []string{"", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "0x10", "NaN", "9223372036854775808", "-9223372036854775809"} {
		result = append(result, browserRejection{"http.StatusText", [2]string{raw, "0"}})
	}
	for _, operation := range []string{"utf8.ValidRune", "utf8.RuneLen", "utf16.IsSurrogate", "utf16.RuneLen", "utf16.DecodeRune", "utf16.EncodeRune"} {
		for _, raw := range []string{"-2147483649", "2147483648"} {
			result = append(result, browserRejection{operation, [2]string{raw, "0"}})
			if operation == "utf16.DecodeRune" {
				result = append(result, browserRejection{operation, [2]string{"0", raw}})
			}
		}
	}
	for _, operation := range []string{"utf8.RuneStart", "bits.Len8", "bits.LeadingZeros8", "bits.TrailingZeros8", "bits.OnesCount8", "bits.Reverse8"} {
		for _, raw := range []string{"-1", "256"} {
			result = append(result, browserRejection{operation, [2]string{raw, "0"}})
		}
	}
	for _, operation := range []string{"http.StatusText", "utf8.ValidRune", "utf8.RuneLen", "utf8.RuneStart", "utf16.IsSurrogate", "utf16.RuneLen", "utf16.EncodeRune", "bits.Len8", "bits.LeadingZeros8", "bits.TrailingZeros8", "bits.OnesCount8", "bits.Reverse8"} {
		result = append(result, browserRejection{operation, [2]string{"0", "1"}})
	}
	return result
}
