package main

import (
	"math"
	"math/rand/v2"
	"net/http"
	"unicode/utf16"
	"unicode/utf8"
)

// Expected values come only from the pinned Go standard library, never the port.
type testCase struct {
	Inputs   []any `json:"inputs"`
	Expected any   `json:"expected"`
}
type suite struct {
	Package  string     `json:"package"`
	Activity string     `json:"activity"`
	Domain   string     `json:"domain"`
	Cases    []testCase `json:"cases"`
}

func corpus() ([]suite, error) {
	var all []suite
	add := func(pkg, activity, domain string, inputs [][]any, oracle func([]any) any) {
		s := suite{Package: pkg, Activity: activity, Domain: domain}
		for _, in := range inputs {
			s.Cases = append(s.Cases, testCase{in, oracle(in)})
		}
		all = append(all, s)
	}
	var codes [][]any
	for n := int64(-10); n <= 610; n++ {
		codes = append(codes, []any{n})
	}
	for _, n := range []int64{math.MinInt64, math.MaxInt64, -100000, 100000} {
		codes = append(codes, []any{n})
	}
	add("go/net/http", "StatusText", "int64; every integer -10..610 plus 4 outliers", codes, func(in []any) any { return http.StatusText(int(in[0].(int64))) })
	edges := []int64{math.MinInt32, -1, 0, 1, 126, 127, 128, 129, 2046, 2047, 2048, 2049, 55294, 55295, 55296, 55297, 56318, 56319, 56320, 56321, 57342, 57343, 57344, 57345, 65533, 65534, 65535, 65536, 65537, 1114110, 1114111, 1114112, 1114113, math.MaxInt32}
	rng := rand.New(rand.NewPCG(17, 29))
	seen := map[int64]bool{}
	var runes [][]any
	for _, n := range edges {
		seen[n] = true
		runes = append(runes, []any{n})
	}
	for i := 0; i < 128; i++ {
		n := int64(rng.Int32N(0x120000))
		if !seen[n] {
			seen[n] = true
			runes = append(runes, []any{n})
		}
	}
	runeDomain := "int32 rune values; boundary corpus + deterministic PCG samples (17,29)"
	add("go/unicode/utf8", "ValidRune", runeDomain, runes, func(in []any) any { return utf8.ValidRune(rune(in[0].(int64))) })
	add("go/unicode/utf8", "RuneLen", runeDomain, runes, func(in []any) any { return utf8.RuneLen(rune(in[0].(int64))) })
	var bytes [][]any
	for n := int64(0); n <= 255; n++ {
		bytes = append(bytes, []any{n})
	}
	add("go/unicode/utf8", "RuneStart", "all 256 byte values; this does not validate a complete UTF-8 sequence", bytes, func(in []any) any { return utf8.RuneStart(byte(in[0].(int64))) })
	add("go/unicode/utf16", "IsSurrogate", runeDomain, runes, func(in []any) any { return utf16.IsSurrogate(rune(in[0].(int64))) })
	add("go/unicode/utf16", "RuneLen", runeDomain, runes, func(in []any) any { return utf16.RuneLen(rune(in[0].(int64))) })
	add("go/unicode/utf16", "EncodeRune", runeDomain, runes, func(in []any) any {
		h, l := utf16.EncodeRune(rune(in[0].(int64)))
		return map[string]any{"high": h, "low": l}
	})
	var pairs [][]any
	for _, h := range []int64{-1, 0, 55295, 55296, 55297, 56319, 56320, 57343, 57344, math.MaxInt32} {
		for _, l := range []int64{-1, 0, 55295, 55296, 56319, 56320, 56321, 57343, 57344, math.MaxInt32} {
			pairs = append(pairs, []any{h, l})
		}
	}
	for i := 0; i < 64; i++ {
		pairs = append(pairs, []any{int64(55296 + rng.IntN(1024)), int64(56320 + rng.IntN(1024))})
	}
	add("go/unicode/utf16", "DecodeRune", "int32 rune pairs; boundary Cartesian grid and 64 valid surrogate pairs", pairs, func(in []any) any { return utf16.DecodeRune(rune(in[0].(int64)), rune(in[1].(int64))) })
	add("app/response", "Describe", "importing consumer; known and unknown HTTP status codes", [][]any{{int64(200)}, {int64(404)}, {int64(418)}, {int64(0)}, {int64(599)}, {int64(103)}}, func(in []any) any {
		n := in[0].(int64)
		text := http.StatusText(int(n))
		return map[string]any{"code": n, "text": text, "known": text != ""}
	})
	byteSuites, err := mathBitsCorpus()
	if err != nil {
		return nil, err
	}
	return append(all, byteSuites...), nil
}
