package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kimjooyoon/gooo-go-ports/internal/demobridge"
)

func TestBrowserFixturesKeepOracleCoverageAndExactStrings(t *testing.T) {
	suites, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	lock := upstream{GoVersion: "go1.27.2", CompilerRevision: "fixture-revision"}
	doc, err := makeBrowserFixtures(suites, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 4437 || len(doc.Rejections) != 172 || doc.GoVersion != lock.GoVersion || doc.CompilerRevision != lock.CompilerRevision {
		t.Fatalf("changed fixture scope: %d cases, %d rejections", len(doc.Cases), len(doc.Rejections))
	}
	counts := map[string]int{}
	byteInputs := map[string]map[string]bool{}
	precise := map[string]bool{"-9223372036854775808": false, "9223372036854775807": false,
		"9007199254740992": false, "9007199254740993": false}
	for _, c := range doc.Cases {
		counts[c.Operation]++
		if c.Operation == "http.StatusText" {
			if _, ok := precise[c.Inputs[0]]; ok {
				precise[c.Inputs[0]] = true
				if c.Expected.Type != "text" || c.Expected.Value != "" {
					t.Fatalf("large HTTP value got a different oracle result: %+v", c)
				}
			}
		}
		for _, input := range c.Inputs {
			n, err := strconv.ParseInt(input, 10, 64)
			if err != nil || strconv.FormatInt(n, 10) != input {
				t.Fatalf("fixture lost an int64 input: %q", input)
			}
		}
		switch c.Expected.Type {
		case "integer":
			if _, ok := c.Expected.Value.(string); !ok {
				t.Fatal("integer fixture would enter JavaScript as a number")
			}
		case "pair", "rgb", "cmyk":
			pair, ok := c.Expected.Value.(map[string]string)
			want := map[string]int{"pair": 2, "rgb": 3, "cmyk": 4}[c.Expected.Type]
			if !ok || len(pair) != want {
				t.Fatal("pair fixture lost string-valued integers")
			}
		case "text":
			if _, ok := c.Expected.Value.(string); !ok {
				t.Fatal("text fixture has wrong type")
			}
		case "boolean":
			if _, ok := c.Expected.Value.(bool); !ok {
				t.Fatal("Boolean fixture has wrong type")
			}
		default:
			t.Fatal("unknown fixture output type")
		}
		if c.Operation == "utf8.RuneStart" || len(c.Operation) > 5 && c.Operation[:5] == "bits." {
			if byteInputs[c.Operation] == nil {
				byteInputs[c.Operation] = map[string]bool{}
			}
			n, _ := strconv.ParseInt(c.Inputs[0], 10, 64)
			if n < 0 || n > 255 || byteInputs[c.Operation][c.Inputs[0]] || c.Inputs[1] != "0" {
				t.Fatal("byte fixture changed its exhaustive input domain")
			}
			byteInputs[c.Operation][c.Inputs[0]] = true
		}
	}
	if len(counts) != 15 || counts["color.RGBToCMYK"] != 637 || counts["color.CMYKToRGB"] != 663 || counts["http.StatusText"] != 627 || counts["utf16.DecodeRune"] != 164 || len(byteInputs) != 6 {
		t.Fatalf("wrong operation roster: %v", counts)
	}
	for operation, inputs := range byteInputs {
		if len(inputs) != 256 {
			t.Fatalf("incomplete byte domain: %s (%d)", operation, len(inputs))
		}
	}
	for value, present := range precise {
		if !present {
			t.Fatalf("missing precision boundary %s", value)
		}
	}
	for _, c := range doc.Rejections {
		calls := 0
		raw := demobridge.Run(c.Operation, c.Inputs, func(string, int64, int64, int64, int64) any { calls++; return nil })
		var result struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil || result.OK || result.Error == "" || calls != 0 {
			t.Fatalf("rejection fixture reached computation: %+v, %s", c, raw)
		}
	}
}

func TestBrowserFixtureFileDoesNotOverwritePriorEvidence(t *testing.T) {
	suites, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixtures.json")
	if err := writeBrowserFixtures(path, suites, upstream{}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeBrowserFixtures(path, suites, upstream{}); err == nil {
		t.Fatal("overwrote an existing fixture file")
	}
	again, err := os.ReadFile(path)
	if err != nil || string(original) != string(again) {
		t.Fatal("existing fixture file changed")
	}
}

func TestBrowserRowRejectsMalformedOracleShapes(t *testing.T) {
	for _, test := range []struct {
		operation string
		row       testCase
	}{
		{"http.StatusText", testCase{Inputs: []any{float64(404)}, Expected: "Not Found"}},
		{"http.StatusText", testCase{Inputs: []any{int64(404), int64(0)}, Expected: "Not Found"}},
		{"utf16.DecodeRune", testCase{Inputs: []any{int64(0)}, Expected: int64(65533)}},
		{"bits.Len8", testCase{Inputs: []any{int64(1)}, Expected: 1.5}},
		{"utf16.EncodeRune", testCase{Inputs: []any{int64(0)}, Expected: map[string]any{"high": true, "low": 0}}},
		{"color.RGBToCMYK", testCase{Inputs: []any{int64(0), int64(0), int64(256)}, Expected: map[string]int{"cyan": 0, "magenta": 0, "yellow": 0, "black": 0}}},
		{"color.CMYKToRGB", testCase{Inputs: []any{int64(0), int64(0), int64(0)}, Expected: map[string]int{"red": 0, "green": 0, "blue": 0}}},
		{"color.CMYKToRGB", testCase{Inputs: []any{int64(0), int64(0), int64(0), int64(0)}, Expected: map[string]any{"red": "255", "green": 255, "blue": 255}}},
		{"color.RGBToCMYK", testCase{Inputs: []any{int64(0), int64(0), int64(0)}, Expected: map[string]int{"cyan": 0, "magenta": 0, "yellow": 0, "other": 255}}},
		{"color.RGBToCMYK", testCase{Inputs: []any{int64(0), int64(0), int64(0)}, Expected: map[string]int{"cyan": 0, "magenta": 0, "yellow": 0, "black": 256}}},
		{"unknown.Operation", testCase{Inputs: []any{int64(0)}, Expected: 0}},
	} {
		if _, err := browserRow(test.operation, test.row); err == nil {
			t.Fatalf("accepted malformed browser oracle: %+v", test)
		}
	}
}
