package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unicode"
)

type sample struct {
	CodePoint string `json:"code_point"`
	IsSpace   bool   `json:"is_space"`
	TrimEmpty bool   `json:"trim_empty"`
}

func main() {
	points := []int{}
	count := 0
	for cp := rune(0); cp <= 0x10ffff; cp++ {
		if cp >= 0xd800 && cp <= 0xdfff {
			continue
		}
		count++
		space := unicode.IsSpace(cp)
		if space != (strings.TrimSpace(string(cp)) == "") {
			panic(fmt.Sprintf("IsSpace/TrimSpace disagreement at U+%04X", cp))
		}
		if space {
			points = append(points, int(cp))
		}
	}
	samples := []sample{}
	for _, cp := range []rune{0x85, 0xfeff, 0x200b, 0xa0} {
		samples = append(samples, sample{fmt.Sprintf("U+%04X", cp), unicode.IsSpace(cp), strings.TrimSpace(string(cp)) == ""})
	}
	out := map[string]any{"runtime": runtime.Version(), "unicode_version": unicode.Version, "domain": "Unicode scalar values U+0000..U+10FFFF excluding U+D800..U+DFFF", "examined": count, "whitespace_code_points": points, "whitespace_count": len(points), "is_space_trim_disagreements": 0, "samples": samples}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(out); err != nil {
		panic(err)
	}
}
