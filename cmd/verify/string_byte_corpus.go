package main

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const textEntity = "gooo://std/text"
const stringBytePackage = "go/strings"
const stringByteConsumerPackage = "app/string-byte-search"
const stringByteResultEntity = "gooo://go-ports/examples/string-byte-search/result"

type stringByteObservations struct {
	Suites                                         []suite
	PortReferenceCases, PortExtensionCases         int
	ConsumerReferenceCases, ConsumerExtensionCases int
}

// Text is the pinned runtime's valid UTF-8 string domain, limited to 1024 bytes.
// No trimming, normalization, rune conversion or byte narrowing precedes this
// validation. Invalid Integer needles belong to the explicit extension corpus.
func readStringByteInput(row []any) (string, int64, error) {
	if len(row) != 2 {
		return "", 0, fmt.Errorf("string byte search requires Text and Integer")
	}
	text, ok := row[0].(string)
	if !ok || !utf8.ValidString(text) || len(text) > 1024 {
		return "", 0, fmt.Errorf("string byte search requires valid UTF-8 Text of at most 1024 bytes")
	}
	needle, ok := row[1].(int64)
	if !ok {
		return "", 0, fmt.Errorf("string byte needle must preserve an exact int64")
	}
	return text, needle, nil
}

// Validate the whole input roster before calling the independent Go oracle.
func stringByteReferenceSuite(pkg, activity string, inputs [][]any, oracle func(string, byte) any) (suite, error) {
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty string byte reference corpus")
	}
	seen := map[struct {
		text   string
		needle int64
	}]bool{}
	for _, row := range inputs {
		text, needle, err := readStringByteInput(row)
		if err != nil {
			return suite{}, err
		}
		if needle < 0 || needle > 255 {
			return suite{}, fmt.Errorf("string byte reference needle outside byte domain: %d", needle)
		}
		key := struct {
			text   string
			needle int64
		}{text, needle}
		if seen[key] {
			return suite{}, fmt.Errorf("duplicate string byte reference tuple")
		}
		seen[key] = true
	}
	s := suite{Package: pkg, Activity: activity, Domain: "six valid UTF-8 Text values, including empty, NUL, multibyte and 1024-byte inputs, crossed with all 256 byte needles; byte offsets, not rune positions"}
	for _, row := range inputs {
		text, needle, _ := readStringByteInput(row)
		s.Cases = append(s.Cases, testCase{[]any{text, needle}, oracle(text, byte(needle))})
	}
	return s, nil
}

func stringByteTexts() []string {
	return []string{"", "ababa", "Go IndexByte 0123456789 XYZ", "A\x00B\x00", "é가🙂xé", strings.Repeat("a", 1023) + "Z"}
}

func stringByteInvalidNeedles() []int64 {
	return []int64{-1, 256, -256, 257, math.MinInt32, math.MaxInt32,
		math.MinInt32 - 1, math.MaxInt32 + 1, -4294967296, 4294967296,
		-9007199254740993, 9007199254740993, math.MinInt64, math.MaxInt64,
		-9007199254740992, 9007199254740992}
}

func stringByteSearchOracle(text string, needle byte) any {
	offset := strings.IndexByte(text, needle)
	return map[string]any{"offset": offset, "found": offset >= 0, "in_byte_domain": true, "byte_length": len(text)}
}

func stringByteCorpus() (stringByteObservations, error) {
	var inputs [][]any
	texts := stringByteTexts()
	for _, text := range texts {
		for needle := int64(0); needle <= 255; needle++ {
			inputs = append(inputs, []any{text, needle})
		}
	}
	port, err := stringByteReferenceSuite(stringBytePackage, "IndexByte", inputs, func(text string, needle byte) any {
		return strings.IndexByte(text, needle)
	})
	if err != nil {
		return stringByteObservations{}, err
	}
	consumer, err := stringByteReferenceSuite(stringByteConsumerPackage, "Search", inputs, stringByteSearchOracle)
	if err != nil {
		return stringByteObservations{}, err
	}
	result := stringByteObservations{PortReferenceCases: len(port.Cases), ConsumerReferenceCases: len(consumer.Cases)}
	for i, needle := range stringByteInvalidNeedles() {
		text := texts[i%len(texts)]
		row := []any{text, needle}
		if _, original, err := readStringByteInput(row); err != nil || (original >= 0 && original <= 255) {
			return stringByteObservations{}, fmt.Errorf("invalid string byte extension roster")
		}
		port.Cases = append(port.Cases, testCase{Inputs: row, Expected: -1})
		consumer.Cases = append(consumer.Cases, testCase{Inputs: append([]any(nil), row...), Expected: map[string]any{
			"offset": -1, "found": false, "in_byte_domain": false, "byte_length": len(text),
		}})
		result.PortExtensionCases++
		result.ConsumerExtensionCases++
	}
	port.Domain += "; 16 separately counted out-of-byte int64 needles return -1 before narrowing"
	consumer.Domain += "; 16 separately counted out-of-byte int64 needles report invalid domain, offset -1 and original byte length"
	result.Suites = []suite{port, consumer}
	return result, nil
}
