package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"
)

const literalTextPackage = "go/strings"
const literalTextConsumerPackage = "app/literal-text-report"
const literalTextResultEntity = "gooo://go-ports/examples/literal-text-report/result"
const literalTextSeed1 uint64 = 0x6c69746572616c31
const literalTextSeed2 uint64 = 0x7465787432303236

type literalTextPair struct{ Text, Needle string }
type literalTextAnchor struct {
	Name string
	literalTextPair
	Index, Count int
}
type literalTextRoster struct {
	Pairs                             []literalTextPair
	CategoryCandidates, CategoryAdded map[string]int
}
type literalTextObservations struct {
	Suites                                                         []suite
	IndexReferenceCases, CountReferenceCases, ReportReferenceCases int
	Roster                                                         literalTextRoster
}

// These independently stated anchors include the exact 24 supplied example
// pairs. Byte offsets and empty/non-overlap counts are checked before any
// generated expectation is used by the native verifier.
func literalTextPilotAnchors() []literalTextAnchor {
	return []literalTextAnchor{
		{"empty-both", literalTextPair{"", ""}, 0, 1},
		{"empty-text", literalTextPair{"", "x"}, -1, 0},
		{"empty-needle", literalTextPair{"abc", ""}, 0, 4},
		{"equal", literalTextPair{"abc", "abc"}, 0, 1},
		{"longer-needle", literalTextPair{"abc", "abcd"}, -1, 0},
		{"absent", literalTextPair{"abc", "x"}, -1, 0},
		{"adjacent", literalTextPair{"aaaa", "aa"}, 0, 2},
		{"overlap", literalTextPair{"ababa", "aba"}, 0, 1},
		{"prefixed-overlap", literalTextPair{"xaaaaay", "aa"}, 1, 2},
		{"diagnostic-lines", literalTextPair{"request_id=1\nrequest_id=2", "request_id="}, 0, 2},
		{"three-byte-offset", literalTextPair{"가x가", "x"}, 3, 1},
		{"three-byte-token", literalTextPair{"가x가", "가"}, 0, 2},
		{"mixed-width", literalTextPair{"é🙂é", "é"}, 0, 2},
		{"four-byte-overlap", literalTextPair{"🙂🙂🙂", "🙂🙂"}, 0, 1},
		{"no-compose", literalTextPair{"e\u0301", "é"}, -1, 0},
		{"no-decompose", literalTextPair{"é", "e\u0301"}, -1, 0},
		{"nul", literalTextPair{"A\x00B\x00", "\x00"}, 1, 2},
		{"crlf", literalTextPair{" CR\r\nLF\n ", "\n"}, 4, 2},
		{"empty-multibyte-needle", literalTextPair{"가🙂\x00", ""}, 0, 4},
		{"last-byte", literalTextPair{strings.Repeat("a", 1023) + "Z", "Z"}, 1023, 1},
		{"maximum-adjacent", literalTextPair{strings.Repeat("a", 1024), "aa"}, 0, 512},
		{"maximum-empty-runes", literalTextPair{strings.Repeat("🙂", 256), ""}, 0, 257},
		{"maximum-equal", literalTextPair{strings.Repeat("a", 1024), strings.Repeat("a", 1024)}, 0, 1},
		{"maximum-absent", literalTextPair{strings.Repeat("a", 1024), strings.Repeat("b", 1024)}, -1, 0},
	}
}

func readLiteralTextPair(row []any) (literalTextPair, error) {
	if len(row) != 2 {
		return literalTextPair{}, fmt.Errorf("literal text requires exactly two Text arguments")
	}
	var pair literalTextPair
	for i, value := range row {
		text, ok := value.(string)
		if !ok || !utf8.ValidString(text) || len(text) > 1024 {
			return literalTextPair{}, fmt.Errorf("literal text argument %d requires valid UTF-8 Text <=1024 bytes", i)
		}
		if i == 0 {
			pair.Text = text
		} else {
			pair.Needle = text
		}
	}
	return pair, nil
}

// Validate the entire roster, including duplicates, before any Go oracle call.
// No normalization, trimming, conversion or malformed byte slicing occurs.
func literalTextReferenceSuite(pkg, activity string, inputs [][]any, oracle func(string, string) any) (suite, error) {
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty literal text reference roster")
	}
	pairs := make([]literalTextPair, len(inputs))
	seen := map[literalTextPair]bool{}
	for i, row := range inputs {
		pair, err := readLiteralTextPair(row)
		if err != nil {
			return suite{}, err
		}
		if seen[pair] {
			return suite{}, fmt.Errorf("duplicate literal text reference pair")
		}
		pairs[i] = pair
		seen[pair] = true
	}
	s := suite{Package: pkg, Activity: activity}
	for _, pair := range pairs {
		s.Cases = append(s.Cases, testCase{Inputs: []any{pair.Text, pair.Needle}, Expected: oracle(pair.Text, pair.Needle)})
	}
	return s, nil
}

func literalTextReportOracle(text, needle string) any {
	index := strings.Index(text, needle)
	return map[string]any{"text": text, "needle": needle, "first_byte_offset": index, "found": index >= 0, "non_overlapping_count": strings.Count(text, needle), "text_byte_length": len(text), "needle_byte_length": len(needle)}
}

func literalTextShortValues() []string {
	values := []string{""}
	alphabet := []string{"a", "b", "\x00", "é"}
	level := []string{""}
	for length := 1; length <= 3; length++ {
		var next []string
		for _, prefix := range level {
			for _, token := range alphabet {
				next = append(next, prefix+token)
			}
		}
		values = append(values, next...)
		level = next
	}
	return values
}

func literalTextOfLength(length int) string {
	var out strings.Builder
	tokens := []string{"é", "가", "🙂", "a"}
	for i := 0; out.Len() < length; i++ {
		token := tokens[i%len(tokens)]
		if len(token) > length-out.Len() {
			token = "a"
		}
		out.WriteString(token)
	}
	return out.String()
}

func literalTextInputs() literalTextRoster {
	roster := literalTextRoster{CategoryCandidates: map[string]int{}, CategoryAdded: map[string]int{}}
	seen := map[literalTextPair]bool{}
	add := func(category, text, needle string) {
		roster.CategoryCandidates[category]++
		pair := literalTextPair{text, needle}
		if !seen[pair] {
			seen[pair] = true
			roster.Pairs = append(roster.Pairs, pair)
			roster.CategoryAdded[category]++
		}
	}
	for _, anchor := range literalTextPilotAnchors() {
		add("pilot", anchor.Text, anchor.Needle)
	}
	short := literalTextShortValues()
	for _, text := range short {
		for _, needle := range short {
			add("short_cartesian", text, needle)
		}
	}
	for offset := 0; offset <= 1023; offset++ {
		add("byte_offsets", strings.Repeat("a", offset)+"Z"+strings.Repeat("a", 1023-offset), "Z")
	}
	for _, length := range []int{0, 1, 2, 3, 4, 7, 8, 15, 16, 31, 32, 63, 64, 127, 128, 255, 256, 511, 512, 1023, 1024} {
		for _, text := range []string{strings.Repeat("a", length), literalTextOfLength(length)} {
			for _, needle := range []string{"", text, "Z", "a", "é"} {
				add("length_boundaries", text, needle)
			}
			add("length_boundaries", "", text)
			add("length_boundaries", strings.Repeat("a", 1024), text)
			add("length_boundaries", literalTextOfLength(1024), text)
			if len(text) > 0 {
				_, width := utf8.DecodeLastRuneInString(text)
				add("length_boundaries", text, text[len(text)-width:])
			}
			if len(text) < 1024 {
				add("length_boundaries", text, text+"Z")
				add("length_boundaries", text+"Z", text)
			}
		}
	}
	for _, text := range []string{"aaaaa", "ababababa", "ééééé", "가가가가가", "🙂🙂🙂🙂🙂", "ababcababcab", "aaaaabaaaaab", "a" + strings.Repeat("b", 1021) + "ac"} {
		for _, needle := range []string{"aa", "aba", "abab", "éé", "가가", "🙂🙂", "ababc", "aaaaab", "a" + strings.Repeat("b", 1021) + "ad"} {
			add("non_overlap", text, needle)
		}
	}
	for _, text := range []string{"\x00a\x00", "\r\n\r\n", "\"\\<>&\u2028\u2029", "e\u0301éÉ", "👩\u200d💻", "\u007f\u0080\u07ff\u0800\uffff\U00010000\U0010ffff", "\ufdd0\u0378"} {
		for _, needle := range []string{"", text, "\x00", "\r\n", "\"", "\\", "<>&", "\u2028", "\u2029", "é", "e\u0301", "É", "\u200d", "\uffff", "\U0010ffff", "\u0378"} {
			add("unicode_serializer", text, needle)
		}
	}
	rng := rand.New(rand.NewPCG(literalTextSeed1, literalTextSeed2))
	alphabet := []string{"a", "b", "\x00", "é", "가", "🙂", "\\", "\"", "<", ">", "&", "\u2028", "\u2029", "\u0301", "\u200d", "\ufdd0", "\U0010ffff"}
	randomText := func(length int) string {
		var out strings.Builder
		for out.Len() < length {
			token := alphabet[rng.IntN(len(alphabet))]
			if len(token) > length-out.Len() {
				token = "a"
			}
			out.WriteString(token)
		}
		return out.String()
	}
	for i := 0; i < 512; i++ {
		text := randomText(rng.IntN(1025))
		needle := randomText(rng.IntN(1025))
		switch i % 6 {
		case 0:
			needle = ""
		case 1:
			needle = text
		case 2, 3, 4:
			// Slice only at rune boundaries; retain original byte positions.
			boundaries := []int{0}
			for offset := range text {
				if offset != 0 {
					boundaries = append(boundaries, offset)
				}
			}
			boundaries = append(boundaries, len(text))
			lo, hi := 0, len(text)
			if i%6 != 2 {
				lo = boundaries[rng.IntN(len(boundaries))]
			}
			if i%6 != 3 {
				hi = boundaries[rng.IntN(len(boundaries))]
			}
			if lo > hi {
				lo, hi = hi, lo
			}
			needle = text[lo:hi]
		}
		add("seeded", text, needle)
	}
	return roster
}

func literalTextCorpus() (literalTextObservations, error) {
	roster := literalTextInputs()
	inputs := make([][]any, len(roster.Pairs))
	for i, pair := range roster.Pairs {
		inputs[i] = []any{pair.Text, pair.Needle}
	}
	index, err := literalTextReferenceSuite(literalTextPackage, "Index", inputs, func(text, needle string) any { return strings.Index(text, needle) })
	if err != nil {
		return literalTextObservations{}, err
	}
	count, err := literalTextReferenceSuite(literalTextPackage, "Count", inputs, func(text, needle string) any { return strings.Count(text, needle) })
	if err != nil {
		return literalTextObservations{}, err
	}
	report, err := literalTextReferenceSuite(literalTextConsumerPackage, "Report", inputs, literalTextReportOracle)
	if err != nil {
		return literalTextObservations{}, err
	}
	domain := fmt.Sprintf("%d unique finite pairs of original decoded valid UTF-8 Text, each argument <=1024 bytes. Literal case-sensitive byte offsets, non-overlapping matches, and empty-needle Unicode code-point count plus1 are independently compared with pinned Go strings.Index/Count. Includes supplied24 anchors, symmetric85x85 short strings over [a,b,NUL,é], all1024 byte offsets, both-argument length/Unicode/non-overlap/serializer boundaries and fixed PCG samples. No normalization, arbitrary-byte Go strings, all-string-domain, whole-package, demand, adoption or performance claim.", len(inputs))
	index.Domain = domain
	count.Domain = domain
	report.Domain = domain + " Report echoes both original inputs and UTF-8 byte lengths; Go supplies the oracle only."
	return literalTextObservations{Suites: []suite{index, count, report}, IndexReferenceCases: len(index.Cases), CountReferenceCases: len(count.Cases), ReportReferenceCases: len(report.Cases), Roster: roster}, nil
}

func isLiteralTextSuite(s suite) bool {
	return s.Package == literalTextPackage && (s.Activity == "Index" || s.Activity == "Count") || s.Package == literalTextConsumerPackage && s.Activity == "Report"
}
