package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

type declaration struct {
	Source, Package, Name, Namespace, ID, Body, Input, Output, Lowered string
	Imports                                                            map[string]string
}
type callSite struct {
	Caller  string `json:"caller"`
	Callee  string `json:"callee"`
	Surface string `json:"surface"`
	Start   int    `json:"start_byte"`
	End     int    `json:"end_byte"`
}

// This is a deliberately narrow source profile: the three unchanged UUID files
// contain only unary computes activities and explicit alias imports. Unknown
// activity declarations/call syntax are not silently treated as another pin.
func indexDeclarations(filename, pkg, source string, declarations map[string]declaration) error {
	names := map[string][]string{
		quotePackage: {"IsPrintable", "HexDigit", "Hex2", "Hex4", "Hex8", "QuotePrefix9"},
		uuidPackage:  {"DecimalLength", "HexDigit", "URNPrefix", "Validate"},
		appPackage:   {"HexValue", "ByteHex", "ParseCanonical"},
	}
	base := map[string]int{quotePackage: 0, uuidPackage: 6, appPackage: 10}[pkg]
	namespaceMatch := regexp.MustCompile(`(?m)^namespace ([A-Za-z_][A-Za-z_0-9]*)$`).FindStringSubmatch(source)
	if len(namespaceMatch) != 2 {
		return fmt.Errorf("unsupported UUID source namespace")
	}
	imports := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^import ([A-Za-z_][A-Za-z_0-9]*) "([^"]+)"$`).FindAllStringSubmatch(source, -1) {
		if imports[match[1]] != "" || !knownPackage(match[2]) {
			return fmt.Errorf("unsupported source import")
		}
		imports[match[1]] = match[2]
	}
	pattern := regexp.MustCompile("(?s)activity\\s+(\\w+)\\((Text|Integer)\\)\\s*->\\s*(\\w+)\\s+id\\s+\"([^\"]+)\"\\s+computes `(.*?)`")
	matches := pattern.FindAllStringSubmatch(source, -1)
	if len(matches) != len(names[pkg]) || len(regexp.MustCompile(`(?m)^activity `).FindAllString(source, -1)) != len(matches) {
		return fmt.Errorf("unsupported UUID declaration roster")
	}
	for i, m := range matches {
		if m[1] != names[pkg][i] {
			return fmt.Errorf("UUID declaration order differs")
		}
		key := pkg + ":" + m[1]
		if _, ok := declarations[key]; ok {
			return fmt.Errorf("duplicate activity")
		}
		declarations[key] = declaration{filename, pkg, m[1], namespaceMatch[1], m[4], m[5], m[2], m[3], fmt.Sprintf("GoooPackage%dActivity%s", base+i, m[1]), imports}
	}
	return nil
}

func sourceCalls(declarations map[string]declaration) ([]callSite, error) {
	queue := []string{appPackage + ":ParseCanonical"}
	seen := map[string]bool{queue[0]: true}
	var result []callSite
	for i := 0; i < len(queue); i++ {
		d, ok := declarations[queue[i]]
		if !ok {
			return nil, fmt.Errorf("missing entry declaration")
		}
		set := token.NewFileSet()
		file := set.AddFile(d.Source, -1, len(d.Body))
		var s scanner.Scanner
		var scanErr error
		s.Init(file, []byte(d.Body), func(_ token.Position, message string) { scanErr = fmt.Errorf("source scanner: %s", message) }, 0)
		type lexeme struct {
			start int
			kind  token.Token
			text  string
		}
		var tokens []lexeme
		for {
			pos, kind, text := s.Scan()
			if kind == token.EOF {
				break
			}
			tokens = append(tokens, lexeme{file.Offset(pos), kind, text})
		}
		if scanErr != nil {
			return nil, scanErr
		}
		for j := 0; j < len(tokens); j++ {
			if tokens[j].kind != token.IDENT {
				continue
			}
			pkg, name, end, next := d.Package, tokens[j].text, tokens[j].start+len(tokens[j].text), j+1
			if next+1 < len(tokens) && tokens[next].kind == token.PERIOD && tokens[next+1].kind == token.IDENT {
				pkg, name = d.Imports[name], tokens[next+1].text
				end = tokens[next+1].start + len(name)
				next += 2
			} else if j > 0 && tokens[j-1].kind == token.PERIOD {
				continue
			}
			if next >= len(tokens) || tokens[next].kind != token.LPAREN {
				continue
			}
			key := pkg + ":" + name
			if _, exists := declarations[key]; !exists {
				if pkg == d.Package && (name == "len" || name == "int64") {
					continue
				}
				return nil, fmt.Errorf("unsupported source call %s", key)
			}
			result = append(result, callSite{queue[i], key, "computes", tokens[j].start, end})
			if !seen[key] {
				queue = append(queue, key)
				seen[key] = true
			}
		}
	}
	return result, nil
}

func inspectCalls(calls map[string]any, id identity) error {
	if calls["schema"] != "gooo/workspace-pure-calls/v1" {
		return fmt.Errorf("pure call schema")
	}
	participants := map[string]bool{}
	expectedSites := map[callSite]int{}
	for _, site := range id.Calls {
		participants[site.Caller] = true
		participants[site.Callee] = true
		expectedSites[site]++
	}
	seen := map[string]bool{}
	for _, value := range array(calls["activities"]) {
		item := object(value)
		ref := object(item["activity"])
		key := str(ref["package_path"]) + ":" + str(ref["activity"])
		d, ok := id.Declarations[key]
		if !ok || !participants[key] || seen[key] || ref["lowered_name"] != d.Lowered || item["source"] != d.Source || item["source_body_sha256"] != digest([]byte(d.Body)) || item["activity_id"] != d.ID || item["lowered_activity_id"] != d.ID {
			return fmt.Errorf("pure call source/identity binding")
		}
		seen[key] = true
	}
	if !reflect.DeepEqual(seen, participants) {
		return fmt.Errorf("pure call activity roster")
	}
	for _, value := range array(calls["sites"]) {
		item := object(value)
		start, ok := item["start_byte"].(interface{ Int64() (int64, error) })
		if !ok {
			return fmt.Errorf("call start")
		}
		a, err := start.Int64()
		if err != nil {
			return err
		}
		end, ok := item["end_byte"].(interface{ Int64() (int64, error) })
		if !ok {
			return fmt.Errorf("call end")
		}
		b, err := end.Int64()
		if err != nil {
			return err
		}
		site := callSite{str(item["caller"]), str(item["callee"]), str(item["surface"]), int(a), int(b)}
		if !keys(item, "caller", "callee", "surface", "start_byte", "end_byte") || expectedSites[site] < 1 {
			return fmt.Errorf("pure call site binding")
		}
		expectedSites[site]--
	}
	for _, count := range expectedSites {
		if count != 0 {
			return fmt.Errorf("missing pure call site")
		}
	}
	return nil
}

// Historical namespace-derived aliases are retained for rejection controls.
// The current compiler preserves the explicit source ID across lowering.
func namespaceID(namespace, name string) string {
	var out strings.Builder
	dash := false
	var previous rune
	for _, current := range name {
		if unicode.IsUpper(current) && unicode.IsLower(previous) && out.Len() > 0 && !dash {
			out.WriteByte('-')
		}
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			out.WriteRune(unicode.ToLower(current))
			dash = false
		} else if out.Len() > 0 && !dash {
			out.WriteByte('-')
			dash = true
		}
		previous = current
	}
	return strings.ReplaceAll(namespace, "_", "-") + "://activity/" + strings.Trim(out.String(), "-")
}
