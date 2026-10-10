// Generate independent Go expectations for the browser's bounded text workflow.
// Original UTF-16 units and expected decoding positions are authored separately;
// no Go string decoding can replace a lone surrogate before the oracle sees it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const maxCodePoints = 256
const maxUTF16Units = 512

type upstream struct {
	GoVersion        string            `json:"go_version"`
	GoRepository     string            `json:"go_repository"`
	GoRevision       string            `json:"go_revision"`
	CompilerRevision string            `json:"compiler_revision"`
	SourceSHA        map[string]string `json:"source_sha256"`
}

type point struct {
	CodePoint int `json:"codePoint"`
	Offset    int `json:"offset"`
	Units     int `json:"units"`
}

type example struct {
	ID     string
	Units  []uint16
	Points []point
}

type expectedPoint struct {
	point
	ValidScalar bool `json:"validScalar"`
	GoSpace     bool `json:"goSpace"`
	GoLetter    bool `json:"goLetter"`
	GoDigit     bool `json:"goDigit"`
	UTF8Bytes   int  `json:"utf8Bytes"`
	UTF16Units  int  `json:"utf16Units"`
}

type fixtureCase struct {
	ID       string          `json:"id"`
	Units    []uint16        `json:"units"`
	Expected []expectedPoint `json:"expected"`
}

type fixtures struct {
	Schema           string            `json:"schema"`
	GoVersion        string            `json:"go_version"`
	GoRepository     string            `json:"go_repository"`
	GoRevision       string            `json:"go_revision"`
	UnicodeVersion   string            `json:"unicode_version"`
	CompilerRevision string            `json:"compiler_revision"`
	GoSourceSHA      map[string]string `json:"go_source_sha256"`
	Limits           struct {
		MaxCodePoints int `json:"max_code_points"`
		MaxUTF16Units int `json:"max_utf16_units"`
	} `json:"limits"`
	Scope string        `json:"scope"`
	Cases []fixtureCase `json:"cases"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "new output JSON file (must not exist)")
	flag.Parse()
	if *out == "" || flag.NArg() != 0 {
		return fmt.Errorf("provide --out for a new fixture file")
	}
	raw, err := os.ReadFile("upstream.json")
	if err != nil {
		return err
	}
	var lock upstream
	if err := json.Unmarshal(raw, &lock); err != nil {
		return err
	}
	if err := checkOracleIdentity(lock, runtime.Version(), runtime.GOROOT()); err != nil {
		return err
	}
	doc, err := makeFixtures(lock)
	if err != nil {
		return err
	}
	return writeFixtures(*out, doc)
}

// Check the same pinned upstream files as the native differential verifier.
// In particular, the White_Space table and both encoding-length implementations
// must be identified before any reference output is generated.
func checkOracleIdentity(lock upstream, version, goroot string) error {
	if lock.GoVersion != "go1.27.2" || version != lock.GoVersion {
		return fmt.Errorf("oracle toolchain %s, require pinned go1.27.2", version)
	}
	if unicode.Version != "17.0.0" {
		return fmt.Errorf("unexpected Unicode version %s", unicode.Version)
	}
	if lock.GoRepository != "https://github.com/golang/go" {
		return fmt.Errorf("unexpected Go source repository")
	}
	if _, err := hex.DecodeString(lock.GoRevision); err != nil || len(lock.GoRevision) != 40 {
		return fmt.Errorf("missing Go source revision")
	}
	if _, err := hex.DecodeString(lock.CompilerRevision); err != nil || len(lock.CompilerRevision) != 40 {
		return fmt.Errorf("missing compiler revision")
	}
	for _, name := range []string{"src/unicode/graphic.go", "src/unicode/digit.go", "src/unicode/letter.go", "src/unicode/tables.go", "src/unicode/utf8/utf8.go", "src/unicode/utf16/utf16.go", "LICENSE"} {
		if lock.SourceSHA[name] == "" {
			return fmt.Errorf("missing pinned Go source %s", name)
		}
	}
	for name, want := range lock.SourceSHA {
		if !filepath.IsLocal(name) || len(want) != 64 {
			return fmt.Errorf("invalid pinned Go source %s", name)
		}
		raw, err := os.ReadFile(filepath.Join(goroot, name))
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != want {
			return fmt.Errorf("Go upstream source hash differs: %s", name)
		}
	}
	return nil
}

func makeFixtures(lock upstream) (fixtures, error) {
	doc := fixtures{
		Schema: "gooo/text-inspector-fixtures/v2", GoVersion: lock.GoVersion,
		GoRepository: lock.GoRepository, GoRevision: lock.GoRevision,
		UnicodeVersion: unicode.Version, CompilerRevision: lock.CompilerRevision,
		GoSourceSHA: lock.SourceSHA,
		Scope:       "Finite browser application observations. The host decodes original UTF-16 units into authored code-point rows and calls six existing compiled Gooo functions. Go supplies independent scalar validity, White_Space, Letter and decimal digit (Nd) membership, and encoding lengths. Lone surrogate integers are preserved; this is not a whole-text Go port, original-file UTF-8 validation, grapheme segmentation, or new native-port coverage.",
		Cases:       []fixtureCase{},
	}
	doc.Limits.MaxCodePoints, doc.Limits.MaxUTF16Units = maxCodePoints, maxUTF16Units
	seen := map[string]bool{}
	for _, e := range examples() {
		if e.ID == "" || seen[e.ID] {
			return fixtures{}, fmt.Errorf("missing or duplicate example ID %q", e.ID)
		}
		seen[e.ID] = true
		if err := checkExample(e); err != nil {
			return fixtures{}, fmt.Errorf("example %s: %w", e.ID, err)
		}
		row := fixtureCase{ID: e.ID, Units: slices.Clone(e.Units), Expected: []expectedPoint{}}
		for _, p := range e.Points {
			r := rune(p.CodePoint) // The complete authored numeric domain was checked above.
			row.Expected = append(row.Expected, expectedPoint{p, utf8.ValidRune(r), unicode.IsSpace(r), unicode.IsLetter(r), unicode.IsDigit(r), utf8.RuneLen(r), utf16.RuneLen(r)})
		}
		doc.Cases = append(doc.Cases, row)
	}
	return doc, nil
}

// Validate authored positions by encoding their numeric points back to the
// original units. This never obtains expected points from a production decoder.
func checkExample(e example) error {
	if e.Units == nil || e.Points == nil || len(e.Units) > maxUTF16Units || len(e.Points) > maxCodePoints {
		return fmt.Errorf("missing arrays or outside application limits")
	}
	offset := 0
	for _, p := range e.Points {
		if p.CodePoint < 0 || p.CodePoint > utf8.MaxRune || p.Offset != offset {
			return fmt.Errorf("invalid original point or offset")
		}
		var units []uint16
		if p.CodePoint <= 0xffff {
			units = []uint16{uint16(p.CodePoint)} // Preserve a lone surrogate as one original unit.
		} else {
			hi, lo := utf16.EncodeRune(rune(p.CodePoint))
			units = []uint16{uint16(hi), uint16(lo)}
		}
		if p.Units != len(units) || offset+len(units) > len(e.Units) || !slices.Equal(units, e.Units[offset:offset+len(units)]) {
			return fmt.Errorf("authored point does not match original units")
		}
		if p.CodePoint >= 0xd800 && p.CodePoint <= 0xdbff && offset+1 < len(e.Units) && e.Units[offset+1] >= 0xdc00 && e.Units[offset+1] <= 0xdfff {
			return fmt.Errorf("a valid surrogate pair was split into two rows")
		}
		offset += len(units)
	}
	if offset != len(e.Units) {
		return fmt.Errorf("unaccounted original units")
	}
	return nil
}

func writeFixtures(path string, doc fixtures) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(doc)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func examples() []example {
	return []example{
		{"empty", []uint16{}, []point{}},
		{"copied-separators", []uint16{0x41, 0xa0, 0x42, 0x0d, 0x0a, 0x200b, 0xd83d, 0xde00}, []point{{0x41, 0, 1}, {0xa0, 1, 1}, {0x42, 2, 1}, {0x0d, 3, 1}, {0x0a, 4, 1}, {0x200b, 5, 1}, {0x1f600, 6, 2}}},
		{"all-go-white-space", []uint16{9, 10, 11, 12, 13, 32, 133, 160, 5760, 8192, 8193, 8194, 8195, 8196, 8197, 8198, 8199, 8200, 8201, 8202, 8232, 8233, 8239, 8287, 12288}, []point{{9, 0, 1}, {10, 1, 1}, {11, 2, 1}, {12, 3, 1}, {13, 4, 1}, {32, 5, 1}, {133, 6, 1}, {160, 7, 1}, {5760, 8, 1}, {8192, 9, 1}, {8193, 10, 1}, {8194, 11, 1}, {8195, 12, 1}, {8196, 13, 1}, {8197, 14, 1}, {8198, 15, 1}, {8199, 16, 1}, {8200, 17, 1}, {8201, 18, 1}, {8202, 19, 1}, {8232, 20, 1}, {8233, 21, 1}, {8239, 22, 1}, {8287, 23, 1}, {12288, 24, 1}}},
		{"space-lookalikes", []uint16{0, 28, 29, 30, 31, 133, 0x180e, 0x200b, 0x200c, 0x200d, 0x200e, 0x200f, 0x2060, 0x2800, 0xfeff, 0xfffd}, []point{{0, 0, 1}, {28, 1, 1}, {29, 2, 1}, {30, 3, 1}, {31, 4, 1}, {133, 5, 1}, {0x180e, 6, 1}, {0x200b, 7, 1}, {0x200c, 8, 1}, {0x200d, 9, 1}, {0x200e, 10, 1}, {0x200f, 11, 1}, {0x2060, 12, 1}, {0x2800, 13, 1}, {0xfeff, 14, 1}, {0xfffd, 15, 1}}},
		{"mixed-line-endings", []uint16{0x41, 13, 10, 0x42, 13, 0x43, 10}, []point{{0x41, 0, 1}, {13, 1, 1}, {10, 2, 1}, {0x42, 3, 1}, {13, 4, 1}, {0x43, 5, 1}, {10, 6, 1}}},
		{"combining-sequence", []uint16{0x65, 0x301}, []point{{0x65, 0, 1}, {0x301, 1, 1}}},
		{"joined-emoji", []uint16{0xd83d, 0xdc69, 0x200d, 0xd83d, 0xdcbb}, []point{{0x1f469, 0, 2}, {0x200d, 2, 1}, {0x1f4bb, 3, 2}}},
		{"regional-indicators", []uint16{0xd83c, 0xddf0, 0xd83c, 0xddf7}, []point{{0x1f1f0, 0, 2}, {0x1f1f7, 2, 2}}},
		{"lone-high-surrogate", []uint16{0xd800}, []point{{0xd800, 0, 1}}},
		{"lone-low-surrogate", []uint16{0xdfff}, []point{{0xdfff, 0, 1}}},
		{"reversed-surrogates", []uint16{0xdc00, 0xd800}, []point{{0xdc00, 0, 1}, {0xd800, 1, 1}}},
		{"high-high-low", []uint16{0xd800, 0xdbff, 0xdc00}, []point{{0xd800, 0, 1}, {0x10fc00, 1, 2}}},
		{"mixed-invalid", []uint16{0xd800, 0x41, 0xdc00, 0xd83d, 0xde00, 0xdbff}, []point{{0xd800, 0, 1}, {0x41, 1, 1}, {0xdc00, 2, 1}, {0x1f600, 3, 2}, {0xdbff, 5, 1}}},
		{"maximum-scalar", []uint16{0xdbff, 0xdfff}, []point{{0x10ffff, 0, 2}}},
		{"scalar-boundaries", []uint16{0xd7ff, 0xe000, 0xfffd, 0xffff, 0xd800, 0xdc00}, []point{{0xd7ff, 0, 1}, {0xe000, 1, 1}, {0xfffd, 2, 1}, {0xffff, 3, 1}, {0x10000, 4, 2}}},
		{"nul-and-bom", []uint16{0, 0xfeff, 0x41, 0}, []point{{0, 0, 1}, {0xfeff, 1, 1}, {0x41, 2, 1}, {0, 3, 1}}},
		{"multilingual-letters", []uint16{0x41, 0xe9, 0xac00, 0xd801, 0xdc00, 0xd840, 0xdc00}, []point{{0x41, 0, 1}, {0xe9, 1, 1}, {0xac00, 2, 1}, {0x10400, 3, 2}, {0x20000, 5, 2}}},
		{"decimal-and-other-numbers", []uint16{0x30, 0x660, 0x967, 0xd835, 0xdfd8, 0xb2, 0xbd, 0x216b, 0x2460}, []point{{0x30, 0, 1}, {0x660, 1, 1}, {0x967, 2, 1}, {0x1d7d8, 3, 2}, {0xb2, 5, 1}, {0xbd, 6, 1}, {0x216b, 7, 1}, {0x2460, 8, 1}}},
		{"letter-stride-members-and-holes", []uint16{0xaa, 0xab, 0xb4, 0xb5, 0xb6, 0xb9, 0xba, 0x2ec, 0x2ed, 0x2ee}, []point{{0xaa, 0, 1}, {0xab, 1, 1}, {0xb4, 2, 1}, {0xb5, 3, 1}, {0xb6, 4, 1}, {0xb9, 5, 1}, {0xba, 6, 1}, {0x2ec, 7, 1}, {0x2ed, 8, 1}, {0x2ee, 9, 1}}},
		repeatExample("limit-256-ascii", []uint16{0x41}, 0x41),
		repeatExample("limit-256-supplementary", []uint16{0xd83d, 0xde00}, 0x1f600),
	}
}

// Boundary inputs repeat explicit units and an explicit point; no decoding or
// oracle calculation supplies their expected positions.
func repeatExample(id string, units []uint16, codePoint int) example {
	e := example{ID: id, Units: []uint16{}, Points: []point{}}
	for i := range maxCodePoints {
		e.Units = append(e.Units, units...)
		e.Points = append(e.Points, point{codePoint, i * len(units), len(units)})
	}
	return e
}
