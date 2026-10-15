package main

import (
	"fmt"
	"strings"
)

const literalCutPackage = "go/strings"
const literalCutConsumerPackage = "app/literal-cut-report"
const literalCutResultEntity = "gooo://go-ports/examples/literal-cut-report/result"
const cutResultEntity = "gooo://go/strings/cut-result"
const cutAffixResultEntity = "gooo://go/strings/affix-result"

type literalCutObservations struct {
	Suites                                                                                    []suite
	CutReferenceCases, CutPrefixReferenceCases, CutSuffixReferenceCases, ReportReferenceCases int
	Pairs                                                                                     []literalTextPair
}

// These are the unchanged 32 input pairs frozen before the private original-Go
// oracle and native observation. The independently stated eleven-field manual
// expectations are retained literally in literal_cut_corpus_test.go.
func literalCutInputs() []literalTextPair {
	return []literalTextPair{
		{"", ""},                               // empty-both
		{"", "x"},                              // empty-source
		{"abc", ""},                            // empty-needle
		{"abc", "abc"},                         // exact-source
		{"abc", "x"},                           // absent
		{"abc", "abcd"},                        // longer-needle
		{"abc", "a"},                           // prefix-only
		{"abc", "c"},                           // suffix-only
		{"abc", "b"},                           // middle-only
		{"aaaa", "aa"},                         // duplicate-aa
		{"ababa", "aba"},                       // overlap-aba
		{"aaaaa", "aa"},                        // odd-aa
		{"one::two::three", "::"},              // repeated-multibyte-separator
		{"a=a=a", "="},                         // repeated-equals
		{" a ", " "},                           // whitespace-preserved
		{"A\u0000B\u0000", "\u0000"},           // nul-repeated
		{"a\r\nb\r\n", "\r\n"},                 // crlf-repeated
		{"\tfield\t", "\t"},                    // tab-both-ends
		{"\r\n", "\r"},                         // cr-is-not-crlf
		{"가x가", "가"},                           // korean-both-ends
		{"가x가", "x"},                           // korean-byte-middle
		{"🙂🙂x", "🙂"},                           // emoji-prefix
		{"x🙂", "🙂"},                            // emoji-suffix
		{"é", "é"},                            // no-normalize-decomposed-source
		{"é", "é"},                            // no-normalize-composed-source
		{"éé", "é"},                         // combining-sequence-repeated
		{"\ufeffabc", "\ufeff"},                // bom-not-trimmed
		{strings.Repeat("a", 1023) + "Z", "Z"}, // last-byte-1024
		{strings.Repeat("a", 1024), strings.Repeat("a", 1024)}, // whole-needle-1024
		{strings.Repeat("a", 1023), strings.Repeat("a", 1024)}, // needle-1024-source-1023
		{strings.Repeat("🙂", 255) + "ABCD", "D"},               // multibyte-last-byte-1024
		{strings.Repeat("é", 512), "é"},                        // multibyte-whole-1024
	}
}

func isLiteralCutSuite(s suite) bool {
	return s.Package == literalCutPackage && (s.Activity == "Cut" || s.Activity == "CutPrefix" || s.Activity == "CutSuffix") || s.Package == literalCutConsumerPackage && s.Activity == "Report"
}

// Validate every original pair before invoking any oracle, including a bad row
// after valid rows. The reused reader rejects conversion, malformed UTF-8,
// overlong arguments, wrong arity and duplicates without normalizing Text.
func literalCutReferenceSuite(pkg, activity string, inputs [][]any, oracle func(string, string) any) (suite, error) {
	if !isLiteralCutSuite(suite{Package: pkg, Activity: activity}) || oracle == nil {
		return suite{}, fmt.Errorf("unsupported literal Cut reference suite or missing oracle")
	}
	return literalTextReferenceSuite(pkg, activity, inputs, oracle)
}

func literalCutOracle(text, needle string) any {
	before, after, found := strings.Cut(text, needle)
	return map[string]any{"before": before, "after": after, "found": found}
}

func literalCutPrefixOracle(text, needle string) any {
	after, found := strings.CutPrefix(text, needle)
	return map[string]any{"text": after, "found": found}
}

func literalCutSuffixOracle(text, needle string) any {
	before, found := strings.CutSuffix(text, needle)
	return map[string]any{"text": before, "found": found}
}

// Recompute each original function independently for the importing consumer;
// never read expected results from another suite, candidate or saved receipt.
func literalCutReportOracle(text, needle string) any {
	before, after, found := strings.Cut(text, needle)
	prefix, prefixFound := strings.CutPrefix(text, needle)
	suffix, suffixFound := strings.CutSuffix(text, needle)
	return map[string]any{
		"text": text, "needle": needle,
		"cut_before": before, "cut_after": after, "cut_found": found,
		"prefix_after": prefix, "prefix_found": prefixFound,
		"suffix_before": suffix, "suffix_found": suffixFound,
		"text_byte_length": len(text), "needle_byte_length": len(needle),
	}
}

func literalCutCorpus() (literalCutObservations, error) {
	pairs := literalCutInputs()
	inputs := make([][]any, len(pairs))
	for i, pair := range pairs {
		inputs[i] = []any{pair.Text, pair.Needle}
	}
	cut, err := literalCutReferenceSuite(literalCutPackage, "Cut", inputs, literalCutOracle)
	if err != nil {
		return literalCutObservations{}, err
	}
	prefix, err := literalCutReferenceSuite(literalCutPackage, "CutPrefix", inputs, literalCutPrefixOracle)
	if err != nil {
		return literalCutObservations{}, err
	}
	suffix, err := literalCutReferenceSuite(literalCutPackage, "CutSuffix", inputs, literalCutSuffixOracle)
	if err != nil {
		return literalCutObservations{}, err
	}
	report, err := literalCutReferenceSuite(literalCutConsumerPackage, "Report", inputs, literalCutReportOracle)
	if err != nil {
		return literalCutObservations{}, err
	}
	domain := "Exactly the frozen32 unique pairs of explicitly present decoded valid UTF-8 Text, each original argument at most1024 bytes. Independent pinned Go strings.Cut/CutPrefix/CutSuffix tuple references preserve the first literal occurrence, empty-needle success, original miss outputs, case, whitespace, NUL, CRLF, BOM and byte lengths without normalization. Finite unchanged manual anchors include overlapping/repeated separators and1024-byte boundaries. No arbitrary-byte Go strings, longer inputs, expanded/exhaustive corpus, whole-package compatibility, performance, demand or production-safety claim."
	cut.Domain, prefix.Domain, suffix.Domain = domain, domain, domain
	report.Domain = domain + " The importing Gooo Report must preserve all11 fields, including both original inputs; Go supplies only independent expected results."
	return literalCutObservations{Suites: []suite{cut, prefix, suffix, report}, CutReferenceCases: len(cut.Cases), CutPrefixReferenceCases: len(prefix.Cases), CutSuffixReferenceCases: len(suffix.Cases), ReportReferenceCases: len(report.Cases), Pairs: pairs}, nil
}
