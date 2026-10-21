package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/http/httpguts"
)

const headerCorpusFile = "examples/header-field-facts/corpus.json"
const headerCorpusSHA = "16d88045a78fe43f239f7b22ad12635a852529abb74733aa82b5073fdd892053"

type headerInputPair struct{ Name, Value string }
type headerInputPopulation struct {
	Names, Values []string
	Pairs         []headerInputPair
}

// This validates the whole finite population before any upstream or native call.
// The pinned raw input hash fixes order; no candidate or retained expected output
// supplies an oracle. Text validation is also used by the actual batch writer.
func headerInputs(raw []byte) (headerInputPopulation, error) {
	var out headerInputPopulation
	if err := decimalPayloadJSON(raw); err != nil {
		return out, err
	}
	v, err := decode(raw)
	if err != nil {
		return out, err
	}
	top, err := decimalPayloadObject(v, "schema", "names", "values", "pairs")
	if err != nil {
		return out, err
	}
	if top["schema"] != "gooo/header-native-inputs/v1" {
		return out, fmt.Errorf("header corpus schema")
	}
	pools := []map[string]bool{{}, {}}
	for i, key := range []string{"names", "values"} {
		values, ok := top[key].([]any)
		if !ok || len(values) != 232 {
			return headerInputPopulation{}, fmt.Errorf("header scalar population")
		}
		texts := make([]string, 0, 232)
		for _, value := range values {
			text, e := decimalPayloadText(value)
			if e != nil || pools[i][text] {
				return headerInputPopulation{}, fmt.Errorf("header invalid or duplicate scalar Text")
			}
			pools[i][text] = true
			texts = append(texts, text)
		}
		if i == 0 {
			out.Names = texts
		} else {
			out.Values = texts
		}
	}
	pairs, ok := top["pairs"].([]any)
	if !ok || len(pairs) != 476 {
		return headerInputPopulation{}, fmt.Errorf("header pair population")
	}
	seen := map[headerInputPair]bool{}
	for _, row := range pairs {
		pair, e := decimalPayloadObject(row, "name", "value")
		if e != nil {
			return headerInputPopulation{}, e
		}
		name, e := decimalPayloadText(pair["name"])
		if e != nil {
			return headerInputPopulation{}, e
		}
		value, e := decimalPayloadText(pair["value"])
		if e != nil {
			return headerInputPopulation{}, e
		}
		p := headerInputPair{name, value}
		if !pools[0][name] || !pools[1][value] || seen[p] {
			return headerInputPopulation{}, fmt.Errorf("header pair outside scalar pools or duplicate")
		}
		seen[p] = true
		out.Pairs = append(out.Pairs, p)
	}
	return out, nil
}

// The validity bits and first bad byte witnesses use the real upstream APIs.
// Prefix/single-byte calls deliberately avoid duplicating the Gooo byte formula.
func headerReference(name, value string) map[string]any {
	nv, vv := httpguts.ValidHeaderFieldName(name), httpguts.ValidHeaderFieldValue(value)
	no, nb, vo, vb := int64(-1), int64(-1), int64(-1), int64(-1)
	if !nv && name != "" {
		for at := 0; at < len(name); at++ {
			if !httpguts.ValidHeaderFieldName(name[:at+1]) {
				no, nb = int64(at), int64(name[at])
				break
			}
		}
	}
	if !vv {
		for at := 0; at < len(value); at++ {
			if !httpguts.ValidHeaderFieldValue(value[at : at+1]) {
				vo, vb = int64(at), int64(value[at])
				break
			}
		}
	}
	return map[string]any{"name": name, "value": value, "name_valid": nv, "value_valid": vv, "name_empty": name == "", "name_has_uppercase": strings.ContainsAny(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"), "value_has_obs_text": strings.IndexFunc(value, func(r rune) bool { return r >= 128 }) >= 0, "value_edge_whitespace": strings.HasPrefix(value, " ") || strings.HasPrefix(value, "\t") || strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t"), "name_invalid_offset": no, "name_invalid_octet": nb, "value_invalid_offset": vo, "value_invalid_octet": vb}
}

func headerCorpus(root string) ([]suite, error) {
	raw, err := decimalPayloadReadFixture(root, headerCorpusFile, headerCorpusSHA)
	if err != nil {
		return nil, err
	}
	in, err := headerInputs(raw)
	if err != nil {
		return nil, err
	}
	domain := "Decoded valid UTF8 Text <=1024 bytes; finite independent x/net/httpguts reference. No arbitrary Go-string, HTTP2, HTTP parsing, authorization or effect claim."
	name := suite{Package: headerPortPackage, Activity: headerNameActivity, Domain: domain}
	value := suite{Package: headerPortPackage, Activity: headerValueActivity, Domain: domain}
	report := suite{Package: headerReportPackage, Activity: headerReportActivity, Domain: "Original12 header facts: exact Text echoes, validity, uppercase/obs-text/edge whitespace, first invalid zero-based byte offset/octet or -1. Flags scan beyond first error. Caller facts do not authorize effects."}
	for _, text := range in.Names {
		name.Cases = append(name.Cases, testCase{Inputs: []any{text}, Expected: httpguts.ValidHeaderFieldName(text)})
	}
	for _, text := range in.Values {
		value.Cases = append(value.Cases, testCase{Inputs: []any{text}, Expected: httpguts.ValidHeaderFieldValue(text)})
	}
	for _, p := range in.Pairs {
		report.Cases = append(report.Cases, testCase{Inputs: []any{p.Name, p.Value}, Expected: headerReference(p.Name, p.Value)})
	}
	return []suite{name, value, report}, nil
}

func (f frozenVerificationCatalog) headerWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	packages := []sourcePackage{{Path: headerPortPackage, Sources: []string{headerPortSource}}}
	if s.Package == headerReportPackage {
		packages = append(packages, sourcePackage{Path: headerReportPackage, Sources: []string{headerReportSource}})
	}
	for _, want := range packages {
		found := 0
		for _, p := range f.Catalog.Packages {
			if p.Path == want.Path {
				found++
				if !equalSourceNames(p.Sources, want.Sources) {
					return w, fmt.Errorf("header exact whole-package sources")
				}
			}
		}
		if found != 1 {
			return w, fmt.Errorf("header exact package roster")
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("header frozen source differs")
			}
			w.Names = append(w.Names, name)
			w.Sources[name] = bytes.Clone(raw)
		}
	}
	for name, raw := range w.Sources {
		imports := catalogImport.FindAllSubmatch(raw, -1)
		want := 0
		if name == headerReportSource {
			want = 1
		}
		if len(imports) != want || want == 1 && string(imports[0][1]) != headerPortPackage {
			return w, fmt.Errorf("header exact import closure")
		}
	}
	var err error
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	return w, err
}

func headerBatchPolicy(s suite, selected suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if !isHeaderSuite(s) || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.3" {
		return nativeCaseBatchPolicy{}, fmt.Errorf("header requires compiler839 and Go1.27.3")
	}
	if selected.Manifest.Schema != "gooo/package-workspace-manifest/v1" || selected.Manifest.Entry.Package != s.Package || selected.Manifest.Entry.Activity != s.Activity || len(selected.Names) == 0 || len(selected.Names) != len(selected.Sources) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("header selected source closure")
	}
	seen, total := map[string]bool{}, 0
	for _, name := range selected.Names {
		raw, ok := selected.Sources[name]
		if !filepath.IsLocal(name) || seen[name] || !ok || len(raw) == 0 || len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("header missing/duplicate/oversized source")
		}
		seen[name] = true
		total += len(raw)
	}
	lowerMax := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	return nativeCaseBatchPolicy{header: true, sourceBytes: total, loweredKeyBytesUpperBound: lowerMax}, nil
}

func (p nativeCaseBatchPolicy) prepareHeader(s suite, start int) (nativePreparedCaseBatch, error) {
	if !isHeaderSuite(s) || start < 0 || start >= len(s.Cases) || p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes || p.loweredKeyBytesUpperBound != len("GoooPackage")+len(strconv.Itoa(p.sourceBytes))+len("Activity")+len(s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("header invalid batch or lowering bound")
	}
	arity := 1
	if s.Package == headerReportPackage {
		arity = 2
	}
	growth := max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
	low, high, best := start+1, min(start+128, len(s.Cases)), start
	var chosen []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		for _, c := range batch.Cases {
			if len(c.Inputs) != arity {
				return nativePreparedCaseBatch{}, fmt.Errorf("header input arity")
			}
			for _, v := range c.Inputs {
				if _, err := decimalPayloadText(v); err != nil {
					return nativePreparedCaseBatch{}, fmt.Errorf("header Text outside domain: %w", err)
				}
			}
		}
		doc := caseDocument(batch)
		raw, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		compact, err := json.Marshal(doc)
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		if len(raw)+1 <= 30000 && len(compact)+(arity+1)*(end-start)*growth <= 32768 {
			best, chosen, low = end, append(raw, '\n'), end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one header row exceeds budget")
	}
	return nativePreparedCaseBatch{End: best, Raw: chosen}, nil
}
