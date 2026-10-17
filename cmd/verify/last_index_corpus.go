package main

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

const lastIndexSource = "ports/strings/last_index.gooo"
const lastIndexConsumerPackage = "app/literal-final-marker-report"
const lastIndexConsumerSource = "examples/literal-final-marker-report/main.gooo"
const lastIndexConsumerID = "gooo://go-ports/examples/literal-final-marker-report/v1/report"
const lastIndexResultEntity = "gooo://go-ports/examples/literal-final-marker-report/v1/result"
const lastIndexSeed1 uint64 = 0x6c617374696e6478
const lastIndexSeed2 uint64 = 0x66696e616c323032

type lastIndexAnchor struct {
	Name string
	literalTextPair
	First, Last, Count int
}

type lastIndexObservations struct {
	Suites                                     []suite
	PortReferenceCases, ConsumerReferenceCases int
	PortPairs, ConsumerPairs                   []literalTextPair
}

// Independently stated byte offsets include overlapping final occurrences;
// the last offset is never inferred from first offset or non-overlap count.
func lastIndexAnchors() []lastIndexAnchor {
	return []lastIndexAnchor{
		{"empty-both", literalTextPair{"", ""}, 0, 0, 1},
		{"empty-text", literalTextPair{"", "x"}, -1, -1, 0},
		{"empty-needle", literalTextPair{"abc", ""}, 0, 3, 4},
		{"equal", literalTextPair{"abc", "abc"}, 0, 0, 1},
		{"longer-needle", literalTextPair{"abc", "abcd"}, -1, -1, 0},
		{"absent", literalTextPair{"abc", "x"}, -1, -1, 0},
		{"adjacent", literalTextPair{"aaaa", "aa"}, 0, 2, 2},
		{"overlap", literalTextPair{"ababa", "aba"}, 0, 2, 1},
		{"prefixed-overlap", literalTextPair{"xaaaaay", "aa"}, 1, 4, 2},
		{"diagnostic-lines", literalTextPair{"request_id=1\nrequest_id=2", "request_id="}, 0, 13, 2},
		{"three-byte-offset", literalTextPair{"가x가", "x"}, 3, 3, 1},
		{"three-byte-token", literalTextPair{"가x가", "가"}, 0, 4, 2},
		{"mixed-width", literalTextPair{"é🙂é", "é"}, 0, 6, 2},
		{"four-byte-overlap", literalTextPair{"🙂🙂🙂", "🙂🙂"}, 0, 4, 1},
		{"no-compose", literalTextPair{"e\u0301", "é"}, -1, -1, 0},
		{"no-decompose", literalTextPair{"é", "e\u0301"}, -1, -1, 0},
		{"nul", literalTextPair{"A\x00B\x00", "\x00"}, 1, 3, 2},
		{"crlf", literalTextPair{" CR\r\nLF\n ", "\n"}, 4, 7, 2},
		{"empty-multibyte-needle", literalTextPair{"가🙂\x00", ""}, 0, 8, 4},
		{"last-byte", literalTextPair{strings.Repeat("a", 1023) + "Z", "Z"}, 1023, 1023, 1},
		{"maximum-adjacent", literalTextPair{strings.Repeat("a", 1024), "aa"}, 0, 1022, 512},
		{"maximum-empty-runes", literalTextPair{strings.Repeat("🙂", 256), ""}, 0, 1024, 257},
		{"maximum-equal", literalTextPair{strings.Repeat("a", 1024), strings.Repeat("a", 1024)}, 0, 0, 1},
		{"maximum-absent", literalTextPair{strings.Repeat("a", 1024), strings.Repeat("b", 1024)}, -1, -1, 0},
		{"odd-overlap", literalTextPair{"aaaaa", "aa"}, 0, 3, 2},
		{"final-marker", literalTextPair{"status=ready\r\nstatus=retry\r\nstatus=ready", "status="}, 0, 28, 3},
	}
}

func isLastIndexSuite(s suite) bool {
	return s.Package == literalTextPackage && s.Activity == "LastIndex" || s.Package == lastIndexConsumerPackage && s.Activity == "Report"
}

func lastIndexReferenceSuite(pkg, activity string, inputs [][]any, oracle func(string, string) any) (suite, error) {
	if !isLastIndexSuite(suite{Package: pkg, Activity: activity}) || oracle == nil {
		return suite{}, fmt.Errorf("unsupported LastIndex reference suite or missing oracle")
	}
	return literalTextReferenceSuite(pkg, activity, inputs, oracle)
}

func lastIndexReportOracle(text, needle string) any {
	first := strings.Index(text, needle)
	return map[string]any{
		"text": text, "needle": needle,
		"first_byte_offset": first, "last_byte_offset": strings.LastIndex(text, needle),
		"found": first >= 0, "non_overlapping_count": strings.Count(text, needle),
		"text_byte_length": len(text), "needle_byte_length": len(needle),
	}
}

func lastIndexInputs() (port, consumer []literalTextPair) {
	seenPort, seenConsumer := map[literalTextPair]bool{}, map[literalTextPair]bool{}
	add := func(pair literalTextPair, report bool) {
		if !seenPort[pair] {
			port = append(port, pair)
			seenPort[pair] = true
		}
		if report && !seenConsumer[pair] {
			consumer = append(consumer, pair)
			seenConsumer[pair] = true
		}
	}
	for _, a := range lastIndexAnchors() {
		add(a.literalTextPair, true)
	}
	short, level := []string{""}, []string{""}
	for length := 1; length <= 2; length++ {
		var next []string
		for _, prefix := range level {
			for _, token := range []string{"a", "b", "\x00", "é"} {
				next = append(next, prefix+token)
			}
		}
		short, level = append(short, next...), next
	}
	for _, text := range short {
		for _, needle := range short {
			add(literalTextPair{text, needle}, false)
		}
	}
	for offset := 0; offset < 1024; offset++ {
		add(literalTextPair{strings.Repeat("a", offset) + "Z" + strings.Repeat("a", 1023-offset), "Z"}, false)
	}
	for _, n := range []int{0, 1, 2, 3, 4, 7, 8, 31, 32, 127, 128, 511, 512, 1023, 1024} {
		for _, text := range []string{strings.Repeat("a", n), literalTextOfLength(n)} {
			for _, needle := range []string{"", text, "Z", "a", "é"} {
				add(literalTextPair{text, needle}, false)
			}
			if n < 1024 {
				add(literalTextPair{text, text + "Z"}, false)
			}
		}
	}
	rng := rand.New(rand.NewPCG(lastIndexSeed1, lastIndexSeed2))
	tokens := []string{"a", "b", "\x00", "\r\n", "é", "가", "🙂", "e\u0301", "\"", "\\", "<>&", "\u2028", "\u2029"}
	for i := 0; i < 64; i++ {
		var b strings.Builder
		for n := rng.IntN(24); n > 0; n-- {
			b.WriteString(tokens[rng.IntN(len(tokens))])
		}
		text, needle := b.String(), tokens[rng.IntN(len(tokens))]
		switch i % 4 {
		case 0:
			needle = ""
		case 1:
			needle = text
		case 2:
			text = needle + text + needle
		}
		add(literalTextPair{text, needle}, true)
	}
	return port, consumer
}

func lastIndexCorpus() (lastIndexObservations, error) {
	portPairs, consumerPairs := lastIndexInputs()
	rows := func(pairs []literalTextPair) [][]any {
		inputs := make([][]any, len(pairs))
		for i, pair := range pairs {
			inputs[i] = []any{pair.Text, pair.Needle}
		}
		return inputs
	}
	port, err := lastIndexReferenceSuite(literalTextPackage, "LastIndex", rows(portPairs), func(text, needle string) any { return strings.LastIndex(text, needle) })
	if err != nil {
		return lastIndexObservations{}, err
	}
	consumer, err := lastIndexReferenceSuite(lastIndexConsumerPackage, "Report", rows(consumerPairs), lastIndexReportOracle)
	if err != nil {
		return lastIndexObservations{}, err
	}
	port.Domain = fmt.Sprintf("LastIndex finite corpus v1: %d unique decoded valid-UTF8 Text pairs, each argument <=1024 bytes. Independent pinned Go strings.LastIndex preserves the final literal byte offset, overlapping final matches, -1 on misses and byte length for an empty needle. Includes26 separately stated anchors, complete21x21 short pairs over [a,b,NUL,é] of0..2tokens, all1024 final byte offsets, length boundaries and64 fixed-PCG candidates, deduplicated in order. No arbitrary-byte or longer Go strings, normalization, whole-package, exhaustive-string, performance or adoption claim.", len(port.Cases))
	consumer.Domain = fmt.Sprintf("FinalMarkerReport v1 finite corpus: %d unique representative pairs from26 manual anchors and64 fixed-PCG candidates. Go strings.Index/LastIndex/Count are independently called for all8 fields; original Texts and UTF8 byte lengths are preserved. LastIndex includes overlaps while Count remains non-overlapping. This separate v1 result does not extend or rebind the existing7-field literal-text Report, its9313 pairs, compact policy or saved evidence. No production enforcement or safety claim.", len(consumer.Cases))
	return lastIndexObservations{[]suite{port, consumer}, len(port.Cases), len(consumer.Cases), portPairs, consumerPairs}, nil
}

func lastIndexResultContract(s suite) resultContract {
	if !isLastIndexSuite(s) {
		return resultContract{}
	}
	if s.Package == literalTextPackage {
		return resultContract{Entity: integerEntity}
	}
	contract := resultContract{Entity: lastIndexResultEntity}
	for _, name := range []string{"text", "needle", "first_byte_offset", "last_byte_offset", "found", "non_overlapping_count", "text_byte_length", "needle_byte_length"} {
		contract.Fields = append(contract.Fields, recordFieldContract{name, lastIndexResultEntity + "/" + strings.ReplaceAll(name, "_", "-")})
	}
	return contract
}
