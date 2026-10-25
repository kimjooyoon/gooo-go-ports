// Command check-runtime-package-surface is a bounded I/O adapter for the Gooo evaluator.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-go-ports/internal/runtimepackagesurface"
)

const maxInputBytes = 512 << 10
const maxPaths = 256
const maxTextBytes = 1024

// encoding/json replaces lone UTF-16 surrogate escapes. Reject those instead
// of silently changing the observed text; valid replacement characters remain valid.
func strictString(raw json.RawMessage) (string, error) {
	var value string
	if len(raw) < 2 || raw[0] != '"' {
		return value, fmt.Errorf("JSON string required")
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		code, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return "", err
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return "", fmt.Errorf("unpaired Unicode surrogate")
		}
		if code >= 0xd800 && code <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return "", fmt.Errorf("unpaired Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return "", fmt.Errorf("unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	if len(value) > maxTextBytes {
		return "", fmt.Errorf("string exceeds the adapter byte limit")
	}
	return value, nil
}

// Decode only the observation. The declared surface, counts and decision belong
// to Gooo, including empty and duplicate observations.
func readObservation(input io.Reader) (runtimepackagesurface.Observation, error) {
	var observation runtimepackagesurface.Observation
	raw, err := io.ReadAll(io.LimitReader(input, maxInputBytes+1))
	if err != nil {
		return observation, err
	}
	if len(raw) > maxInputBytes || !utf8.Valid(raw) {
		return observation, fmt.Errorf("bounded UTF-8 JSON observation required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return observation, fmt.Errorf("observation object required")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return observation, err
		}
		key, ok := token.(string)
		if !ok || (key != "origin_ref" && key != "import_paths") {
			return observation, fmt.Errorf("unknown observation field")
		}
		if _, exists := fields[key]; exists {
			return observation, fmt.Errorf("duplicate observation field: %s", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return observation, err
		}
		fields[key] = value
	}
	if _, err = decoder.Token(); err != nil {
		return observation, err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return observation, fmt.Errorf("one JSON observation required")
	}
	if len(fields) != 2 || len(fields["origin_ref"]) == 0 || fields["origin_ref"][0] != '"' || len(fields["import_paths"]) == 0 || fields["import_paths"][0] != '[' {
		return observation, fmt.Errorf("origin_ref string and import_paths array required")
	}
	observation.OriginRef, err = strictString(fields["origin_ref"])
	if err != nil {
		return observation, err
	}
	var paths []json.RawMessage
	if err := json.Unmarshal(fields["import_paths"], &paths); err != nil {
		return observation, err
	}
	if len(paths) > maxPaths {
		return observation, fmt.Errorf("at most %d import paths required", maxPaths)
	}
	observation.ImportPaths = make([]string, len(paths))
	for i, rawPath := range paths {
		observation.ImportPaths[i], err = strictString(rawPath)
		if err != nil {
			return observation, err
		}
	}
	return observation, nil
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "no arguments; provide one observation on stdin")
		return 2
	}
	observation, err := readObservation(stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	assessment := runtimepackagesurface.Evaluate(observation)
	if err := json.NewEncoder(stdout).Encode(assessment); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
