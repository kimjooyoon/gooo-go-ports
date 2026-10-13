package servicepolicy

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestEveryExactMatcherAndImpactFieldMalformed(t *testing.T) {
	for _, mode := range []string{"matcher", "impact"} {
		obj := map[string]json.RawMessage{"req_type": []byte(`"mcp"`), "req_name": []byte(`"a"`)}
		if mode == "matcher" {
			obj["pattern"] = []byte(`"mcp://a"`)
		} else {
			obj["before"] = []byte(`"mcp://a"`)
			obj["after"] = []byte(`"*"`)
		}
		decode := func(raw []byte) error {
			if mode == "matcher" {
				_, err := DecodeMatcher(raw)
				return err
			}
			_, err := DecodeImpact(raw)
			return err
		}
		good, _ := json.Marshal(obj)
		if err := decode(good); err != nil {
			t.Fatal(err)
		}
		for field := range obj {
			for _, kind := range []string{"missing", "null", "bool", "number", "array", "object", "over128", "non_ascii", "control", "high_surrogate", "low_surrogate", "duplicate", "escaped_duplicate"} {
				t.Run(mode+"/"+field+"/"+kind, func(t *testing.T) {
					copy := map[string]json.RawMessage{}
					for k, v := range obj {
						copy[k] = v
					}
					var raw []byte
					switch kind {
					case "missing":
						delete(copy, field)
					case "null":
						copy[field] = []byte("null")
					case "bool":
						copy[field] = []byte("true")
					case "number":
						copy[field] = []byte("7")
					case "array":
						copy[field] = []byte("[]")
					case "object":
						copy[field] = []byte("{}")
					case "over128":
						copy[field], _ = json.Marshal(strings.Repeat("x", 129))
					case "non_ascii":
						copy[field] = []byte(`"é"`)
					case "control":
						copy[field] = []byte(`"a\u0000b"`)
					case "high_surrogate":
						copy[field] = []byte(`"\ud800"`)
					case "low_surrogate":
						copy[field] = []byte(`"\udc00"`)
					}
					raw, _ = json.Marshal(copy)
					if kind == "duplicate" || kind == "escaped_duplicate" {
						key, _ := json.Marshal(field)
						if kind == "escaped_duplicate" {
							key = []byte(fmt.Sprintf(`"\u%04x%s"`, []rune(field)[0], field[1:]))
						}
						raw = []byte(strings.TrimSuffix(string(good), "}") + "," + string(key) + ":" + string(obj[field]) + "}")
					}
					if err := decode(raw); err == nil {
						t.Fatal("malformed exact field accepted")
					}
				})
			}
		}
		for _, bad := range [][]byte{[]byte("null"), []byte("[]"), []byte("{}"), append(good, []byte(" {}")...), append(good, 0xff), []byte(strings.TrimSuffix(string(good), "}") + `,"unknown":true}`), []byte(strings.TrimSuffix(string(good), "}") + `,"\ud800":true}`)} {
			if err := decode(bad); err == nil {
				t.Fatal("malformed envelope accepted")
			}
		}
	}
}

func TestUnsupportedDomainRefusalsAndBoundaries(t *testing.T) {
	bad := []MatcherFacts{
		{Pattern: "mcp://a/path", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a?q=x", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a#x", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp:a", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a*b", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://*.*", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://*.a.*", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a..b", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a", ReqType: "mcp", ReqName: "a/path"}, {Pattern: "mcp://a", ReqType: "mcp", ReqName: "é"}, {Pattern: "mcp://a", ReqType: "mcp", ReqName: strings.Repeat("a", 129)}, {Pattern: strings.Repeat("a", 129), ReqType: "mcp", ReqName: "a"}, {Pattern: "*", ReqType: strings.Repeat("a", 129), ReqName: "a"}, {Pattern: "*", ReqType: "mcp", ReqName: "a\x00b"}, {Pattern: "mcp://user@a", ReqType: "mcp", ReqName: "a"}, {Pattern: "mcp://a:443", ReqType: "mcp", ReqName: "a"},
	}
	for i, f := range bad {
		if _, err := EvaluateMatcher(f); err == nil {
			t.Fatalf("unsupported matcher facts%d accepted", i)
		}
		if _, err := EvaluateImpact(ImpactFacts{Before: f.Pattern, After: "*", ReqType: f.ReqType, ReqName: f.ReqName}); err == nil {
			t.Fatalf("unsupported impact facts%d accepted", i)
		}
	}
	for _, f := range []MatcherFacts{{Pattern: "*", ReqType: strings.Repeat("A", 128), ReqName: strings.Repeat("x", 128)}, {Pattern: "mcp://" + strings.Repeat("a", 122), ReqType: "mcp", ReqName: strings.Repeat("a", 122)}, {Pattern: "", ReqType: "", ReqName: ""}, {Pattern: "MCP://A", ReqType: "MCP", ReqName: "A"}} {
		if _, err := EvaluateMatcher(f); err != nil {
			t.Fatal("valid canonical boundary refused", err)
		}
	}
}
