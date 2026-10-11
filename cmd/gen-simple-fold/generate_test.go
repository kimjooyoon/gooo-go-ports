package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode"
)

func pinnedIdentity(t *testing.T) upstream {
	t.Helper()
	raw, err := os.ReadFile("../../upstream.json")
	if err != nil {
		t.Fatal(err)
	}
	var pin upstream
	if err := json.Unmarshal(raw, &pin); err != nil {
		t.Fatal(err)
	}
	if err := verifyIdentity(pin, runtime.Version(), unicode.Version, runtime.GOROOT()); err != nil {
		t.Fatal(err)
	}
	return pin
}

func generatedFixture(t *testing.T) []byte {
	t.Helper()
	source, err := generate(pinnedIdentity(t), pinnedTables(t))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// Evaluate the emitted Gooo body, not a second copy of the table algorithm.
// The standard library's SimpleFold remains an independent test oracle only.
func TestGeneratedBodyMatchesGoForEveryUnicodeCodePoint(t *testing.T) {
	fold := compileGeneratedBody(t, generatedFixture(t))
	for input := int64(0); input <= unicode.MaxRune; input++ {
		want := int64(unicode.SimpleFold(rune(input)))
		if actual := fold(input); actual != want {
			t.Fatalf("U+%04X: emitted body=%d Go=%d", input, actual, want)
		}
	}
}

func TestGeneratedBodyPreservesRuneDomainAndExplicitIntegerExtension(t *testing.T) {
	fold := compileGeneratedBody(t, generatedFixture(t))
	// These original Go-rune inputs are separate from the wider Integer API.
	runes := []int64{-1 << 31, -1<<31 + 1, -1000000, -1, 0xd800, 0xdfff, 0x110000, 0x110001, 1<<31 - 1}
	rng := rand.New(rand.NewSource(17))
	for range 256 {
		runes = append(runes, int64(int32(rng.Uint32())))
	}
	for _, input := range runes {
		if got, want := fold(input), int64(unicode.SimpleFold(rune(input))); got != want {
			t.Fatalf("rune %d: %d != %d", input, got, want)
		}
	}
	for _, input := range []int64{-1 << 63, -9007199254740993, -1<<31 - 1, 1 << 31, 9007199254740993, 1<<63 - 1} {
		if got := fold(input); got != input {
			t.Fatalf("outside-int32 extension narrowed %d to %d", input, got)
		}
	}
}

func TestGeneratedSpecialOrbitsAndRelativeParity(t *testing.T) {
	fold := compileGeneratedBody(t, generatedFixture(t))
	for _, pair := range [][2]int64{
		{'K', 'k'}, {'k', 0x212a}, {0x212a, 'K'}, {'S', 's'}, {'s', 0x17f}, {0x17f, 'S'},
		{0x130, 0x130}, {0x131, 0x131}, {0x139, 0x13a}, {0x13a, 0x139},
		{0x1c4, 0x1c5}, {0x1c5, 0x1c6}, {0x1c6, 0x1c4},
		{0x3a3, 0x3c2}, {0x3c2, 0x3c3}, {0x3c3, 0x3a3},
		{0xdf, 0x1e9e}, {0x1e9e, 0xdf}, {0x10400, 0x10428}, {0x10428, 0x10400},
	} {
		if got := fold(pair[0]); got != pair[1] {
			t.Fatalf("special fold U+%04X: U+%04X != U+%04X", pair[0], got, pair[1])
		}
	}
}

func TestGeneratedSourceIsCurrentDeterministicAndBounded(t *testing.T) {
	first, second := generatedFixture(t), generatedFixture(t)
	if !bytes.Equal(first, second) || len(first) > 128<<10 {
		t.Fatal("generated source is nondeterministic or exceeds compiler bound")
	}
	current, err := os.ReadFile(filepath.Join("../..", targetSource))
	if err != nil || !bytes.Equal(first, current) {
		t.Fatal("checked-in source is stale", err)
	}
	for _, want := range []string{"Copyright 2009 The Go Authors", "third_party/GO-LICENSE", pinnedGoRevision, `id "gooo://go/unicode/SimpleFold"`} {
		if !bytes.Contains(first, []byte(want)) {
			t.Fatal("generated provenance or identity missing", want)
		}
	}
}

func TestCLIWriteAndCheckDoNotRepairStaleSource(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ports/unicode"), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../upstream.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "upstream.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"--root", root}, &output); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "--check"}, &output); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, targetSource)
	stale := []byte("stale\n")
	if err := os.WriteFile(path, stale, 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--root", root, "--check"}, &output); err == nil {
		t.Fatal("stale output accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, stale) {
		t.Fatal("--check wrote output", err)
	}
}
