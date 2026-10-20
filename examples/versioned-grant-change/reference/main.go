// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2010 The Go Authors. All rights reserved.
// Copyright 2026 Gooo Go Ports contributors. All rights reserved.
// BSD-style terms: LICENSE and third_party/GO-LICENSE.
// Verification-only originals: Go1.27.2 revision022c8636110ebac86a9b88724cda168ed713c21f,
// src/net/http/fs.go435..513 (scanETag/etagStrongMatch/enum/checkIfMatch),
// SHA25621660cb61d17c258de99172683f9b41aca37d45c0af0556723f2a11179bdc45e.
// Only checkIfMatch's signature and two selected-header reads are substituted.
// The actual pinned textproto.TrimString and x/net0.59.0 httpguts are used.
// This tool preserves frozen final expectations as RawMessage. It observes only
// original Go components; it never implements the final Gooo grant decision.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"
)

const entry = "gooo/service-policy/versioned-change:AssessVersionedGrantChange"
const activity = "AssessVersionedGrantChange"
const newSource = "examples/versioned-grant-change/main.gooo"
const maxRaw = 30000
const maxLowered = 32768
const maxSources = 10 << 20

var originalPins = map[string]string{
	"examples/agentmesh-service-policy/domain.gooo":             "b866ea5aa827e6122541faa54fd920c9ec1942a308bf72adb82fb244f1b3c33c",
	"ports/agentmesh/service_pattern.gooo":                      "e9aabc13addf3db59a3a5448f8ad2e01e8b731a6da5f6e5eac7b749bf93bda7d",
	"examples/agentmesh-service-policy/impact.gooo":             "e7e84b71beab8661298ed2d6fdd85e8d01586d3546b1c0a7f5f86bb0d155f647",
	"examples/agentmesh-service-policy/refinement.gooo":         "90bcc738c88eb5bec8b41b5306cccf274a5740560d7dd62b7f3a57b30b441ff0",
	"examples/agentmesh-service-policy/checked_impact.gooo":     "c47c196e58bfc43f9a8d1e0faa11388b50f13012dc2190c105bef740b8ab5a02",
	"examples/agentmesh-service-policy/checked_refinement.gooo": "6628f15bf8a260e48bc7b58e5d05699c48138b145a1e42c6e6f872efd6f81772",
	"examples/agentmesh-service-policy/conditional_apply.gooo":  "09b1d63aa9ad970989c0857bdf9dbe8b02bb0c096df9b1256face673bc0e5f51",
	"ports/net/http/etag.gooo":                                  "2fc5b2c19acf8c040173859309dd687be37bca1c2c56b49efa293a8df98d42c4",
	"ports/net/httpguts/headers.gooo":                           "a033b96ab0adb394a9d79137715d7a4170e22061e4ae9aa282f8ab3671da9f76",
	"examples/strong-validator-facts/main.gooo":                 "a7002b9db59ff68e732b66ef27c86426b8fff325cfb15f2665a0d0cf04f17a01",
	"ports/net/http/precondition.gooo":                          "9924cde4a2e01222658929a87007d6e34db85f2d4f49c057d110bb8d1bf05aeb",
}

var sourcePackages = map[string]string{
	"examples/agentmesh-service-policy/domain.gooo":             "gooo/service-domain",
	"ports/agentmesh/service_pattern.gooo":                      "go/google/agentmesh/api",
	"examples/agentmesh-service-policy/impact.gooo":             "app/service-grant-impact",
	"examples/agentmesh-service-policy/refinement.gooo":         "app/service-grant-refinement",
	"examples/agentmesh-service-policy/checked_impact.gooo":     "gooo/service-policy/checked-impact",
	"examples/agentmesh-service-policy/checked_refinement.gooo": "gooo/service-policy/checked-refinement",
	"examples/agentmesh-service-policy/conditional_apply.gooo":  "gooo/service-policy/conditional-apply",
	"ports/net/http/etag.gooo":                                  "go/net/http/etag",
	"ports/net/httpguts/headers.gooo":                           "go/golang.org/x/net/http/httpguts",
	"examples/strong-validator-facts/main.gooo":                 "app/strong-validator-facts",
	"ports/net/http/precondition.gooo":                          "go/net/http/precondition",
	"examples/versioned-grant-change/main.gooo":                 "gooo/service-policy/versioned-change",
}

type caseRow struct {
	Raw      json.RawMessage
	Inputs   [6]string
	Expected json.RawMessage
}
type preparedBatch struct {
	Start, End        int
	Raw               []byte
	LoweredUpperBytes int
}
type sourceImage struct {
	Package      string `json:"package"`
	Filename     string `json:"filename"`
	SourceDigest string `json:"source_digest"`
}
type componentFacts struct {
	Tag         string `json:"tag"`
	Remainder   string `json:"remainder"`
	HeaderValid bool   `json:"header_valid"`
	Strong      bool   `json:"strong"`
}
type component struct {
	Selected         componentFacts `json:"selected"`
	Current          componentFacts `json:"current"`
	IfMatchCondition int64          `json:"if_match_condition"`
}
type batchFile struct {
	Start             int    `json:"start"`
	End               int    `json:"end"`
	File              string `json:"file"`
	RawBytes          int    `json:"raw_bytes"`
	LoweredUpperBytes int    `json:"lowered_upper_bytes"`
}
type workspace struct {
	Schema string `json:"schema"`
	Entry  struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
	} `json:"entry"`
	Packages []struct {
		Path    string   `json:"path"`
		Sources []string `json:"sources"`
	} `json:"packages"`
}

func sum(raw []byte) string { d := sha256.Sum256(raw); return hex.EncodeToString(d[:]) }
func strictJSON(raw []byte) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return errors.New("invalid UTF8 or JSON")
	}
	// encoding/json otherwise silently replaces unpaired escaped surrogates.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		i++
		for i < len(raw) && raw[i] != '"' {
			if raw[i] != '\\' {
				i++
				continue
			}
			i++
			if raw[i] != 'u' {
				i++
				continue
			}
			value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
			if err != nil {
				return err
			}
			i += 5
			if value >= 0xDC00 && value <= 0xDFFF {
				return errors.New("unpaired low surrogate")
			}
			if value >= 0xD800 && value <= 0xDBFF {
				if i+6 > len(raw) || raw[i] != '\\' || raw[i+1] != 'u' {
					return errors.New("unpaired high surrogate")
				}
				low, err := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
				if err != nil || low < 0xDC00 || low > 0xDFFF {
					return errors.New("invalid surrogate pair")
				}
				i += 6
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 64 {
			return errors.New("JSON nesting bound")
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					key, err := dec.Token()
					if err != nil {
						return err
					}
					name, ok := key.(string)
					if !ok || seen[name] {
						return errors.New("duplicate decoded JSON key")
					}
					seen[name] = true
					if err := visit(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for dec.More() {
					if err := visit(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("unexpected JSON delimiter")
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := visit(0); err != nil {
		return err
	}
	_, err := dec.Token()
	if err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func exactKeys(m map[string]json.RawMessage, names ...string) bool {
	if len(m) != len(names) {
		return false
	}
	for _, n := range names {
		if _, ok := m[n]; !ok {
			return false
		}
	}
	return true
}
func textValue(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", errors.New("Text required")
	}
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err
}
func decodeRows(raw []byte) ([]caseRow, error) {
	if len(raw) > 4<<20 {
		return nil, errors.New("master document bound")
	}
	if err := strictJSON(raw); err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil || !exactKeys(doc, "schema", "cases") {
		return nil, errors.New("case document keys")
	}
	schema, err := textValue(doc["schema"])
	if err != nil || schema != "gooo/body-composition-cases/v1" {
		return nil, errors.New("case schema")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(doc["cases"], &rows); err != nil || len(rows) == 0 || len(rows) > 4096 {
		return nil, errors.New("nonempty ordered case array required")
	}
	fields := map[string]string{"status": "string", "reason": "string", "expected_validator_strict": "bool", "current_validator_strict": "bool", "if_match_condition": "int", "version_match": "bool", "grant_status": "string", "grant_reason": "string", "invalid_field": "string", "required_before_match": "bool", "required_after_match": "bool", "subset_rule_id": "string"}
	result := make([]caseRow, 0, len(rows))
	for index, row := range rows {
		var members map[string]json.RawMessage
		if err := json.Unmarshal(row, &members); err != nil || !exactKeys(members, "inputs", "expected") {
			return nil, fmt.Errorf("row%d keys", index)
		}
		var inputs map[string]json.RawMessage
		if err := json.Unmarshal(members["inputs"], &inputs); err != nil || len(inputs) != 6 {
			return nil, fmt.Errorf("row%d six inputs", index)
		}
		parsed := caseRow{Raw: bytes.Clone(row), Expected: bytes.Clone(members["expected"])}
		for i := 0; i < 6; i++ {
			value, ok := inputs[entry+".input"+strconv.Itoa(i)]
			if !ok {
				return nil, errors.New("exact input keys required")
			}
			s, err := textValue(value)
			if err != nil || len(s) > 1024 || !utf8.ValidString(s) {
				return nil, fmt.Errorf("row%d input%d Text0..1024", index, i)
			}
			parsed.Inputs[i] = s
		}
		var expected map[string]json.RawMessage
		if err := json.Unmarshal(members["expected"], &expected); err != nil || !exactKeys(expected, entry) {
			return nil, errors.New("exact expected entry")
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(expected[entry], &record); err != nil || len(record) != len(fields) {
			return nil, errors.New("twelve expected fields required")
		}
		for name, kind := range fields {
			value, ok := record[name]
			if !ok {
				return nil, errors.New("expected field missing")
			}
			s := string(bytes.TrimSpace(value))
			switch kind {
			case "string":
				if _, err := textValue(value); err != nil {
					return nil, err
				}
			case "bool":
				if s != "true" && s != "false" {
					return nil, errors.New("expected Boolean required")
				}
			case "int":
				if _, err := strconv.ParseInt(s, 10, 64); err != nil {
					return nil, errors.New("expected int64 token required")
				}
			}
		}
		result = append(result, parsed)
	}
	return result, nil
}
func document(rows []caseRow) ([]byte, []byte, error) {
	values := make([]json.RawMessage, len(rows))
	for i, row := range rows {
		values[i] = row.Raw
	}
	doc := struct {
		Schema string            `json:"schema"`
		Cases  []json.RawMessage `json:"cases"`
	}{"gooo/body-composition-cases/v1", values}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	compact, err := json.Marshal(doc)
	return append(raw, '\n'), compact, err
}
func prepareBatches(rows []caseRow, sourceBytes int) ([]preparedBatch, error) {
	if len(rows) == 0 || sourceBytes < 1 || sourceBytes > maxSources {
		return nil, errors.New("nonempty rows/source bound")
	}
	lower := len("GoooPackage") + len(strconv.Itoa(sourceBytes)) + len("Activity") + len(activity)
	growth := max(0, lower-len(entry))
	result := []preparedBatch{}
	for start := 0; start < len(rows); {
		var selected preparedBatch
		for end := start + 1; end <= len(rows) && end-start <= 8 && end-start <= 128; end++ {
			raw, compact, err := document(rows[start:end])
			if err != nil {
				return nil, err
			}
			upper := len(compact) + 7*(end-start)*growth
			if len(raw) > maxRaw || upper > maxLowered {
				break
			}
			selected = preparedBatch{start, end, raw, upper}
		}
		if selected.End == 0 {
			return nil, fmt.Errorf("individual row%d exceeds aggregate evidence envelope", start)
		}
		result = append(result, selected)
		start = selected.End
	}
	return result, nil
}
func relative(name string) bool {
	return name != "" && !filepath.IsAbs(name) && !strings.Contains(name, "\\") && filepath.ToSlash(filepath.Clean(name)) == name && !strings.HasPrefix(name, "../") && name != ".." && name != "."
}
func canonical(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("absolute path required")
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil || resolved != path || clean != path {
		return "", errors.New("canonical non-symlink path required")
	}
	return path, nil
}
func bounded(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, errors.New("regular bounded input required")
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(raw) > limit || int64(len(raw)) != info.Size() {
		return nil, errors.New("bounded input changed")
	}
	return raw, nil
}
func read(root, name string, limit int) ([]byte, error) {
	if !relative(name) {
		return nil, errors.New("unsafe relative source")
	}
	p, err := canonical(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	return bounded(p, limit)
}

func images(root string, manifestRaw []byte) ([]sourceImage, int, error) {
	if err := strictJSON(manifestRaw); err != nil {
		return nil, 0, err
	}
	var manifest workspace
	dec := json.NewDecoder(bytes.NewReader(manifestRaw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return nil, 0, err
	}
	if manifest.Schema != "gooo/package-workspace-manifest/v1" || manifest.Entry.Package+":"+manifest.Entry.Activity != entry || len(manifest.Packages) != 12 {
		return nil, 0, errors.New("exact12 package workspace")
	}
	seen := map[string]bool{}
	packages := map[string]bool{}
	records := map[string]bool{}
	recordPattern := regexp.MustCompile(`(?m)^entity\s+\w+\s+id\s+"([^"]+)"\s+fields\s*\{`)
	result := []sourceImage{}
	total := 0
	for _, pkg := range manifest.Packages {
		if packages[pkg.Path] || len(pkg.Sources) != 1 {
			return nil, 0, errors.New("unique one-source packages required")
		}
		packages[pkg.Path] = true
		name := pkg.Sources[0]
		wantPackage, knownSource := sourcePackages[name]
		if seen[name] || !knownSource || wantPackage != pkg.Path {
			return nil, 0, errors.New("source/package roster")
		}
		seen[name] = true
		raw, err := read(root, name, maxSources)
		if err != nil {
			return nil, 0, err
		}
		if !utf8.Valid(raw) {
			return nil, 0, errors.New("source UTF8")
		}
		if pin, ok := originalPins[name]; ok && sum(raw) != pin {
			return nil, 0, fmt.Errorf("unchanged source pin: %s", name)
		}
		total += len(raw)
		if total > maxSources {
			return nil, 0, errors.New("source bytes bound")
		}
		for _, match := range recordPattern.FindAllSubmatch(raw, -1) {
			id := string(match[1])
			if records[id] {
				return nil, 0, errors.New("duplicate named record")
			}
			records[id] = true
		}
		encoded, err := json.Marshal(string(raw))
		if err != nil {
			return nil, 0, err
		}
		result = append(result, sourceImage{pkg.Path, name, "sha256:" + sum(encoded)})
	}
	if len(seen) != 12 || len(records) != 9 {
		return nil, 0, errors.New("exact12 sources/nine records")
	}
	return result, total, nil
}
func extractFunction(raw []byte, name string) ([]byte, error) {
	start := bytes.Index(raw, []byte("\nfunc "+name+"("))
	if start >= 0 {
		start++
	} else if bytes.HasPrefix(raw, []byte("func "+name+"(")) {
		start = 0
	}
	if start < 0 {
		return nil, errors.New("original function missing: " + name)
	}
	tail := bytes.Index(raw[start:], []byte("\n}\n"))
	if tail < 0 {
		return nil, errors.New("original function end missing")
	}
	return raw[start : start+tail+3], nil
}
func adaptedIfMatch(raw []byte) ([]byte, error) {
	body, err := extractFunction(raw, "checkIfMatch")
	if err != nil {
		return nil, err
	}
	pairs := [][2]string{{"func checkIfMatch(w ResponseWriter, r *Request) condResult {", "func checkIfMatch(selectedIfMatchHeader, currentETag string) condResult {"}, {`r.Header.Get("If-Match")`, "selectedIfMatchHeader"}, {`w.Header().get("Etag")`, "currentETag"}}
	for _, pair := range pairs {
		if bytes.Count(body, []byte(pair[0])) != 1 {
			return nil, errors.New("exact three original lookup substitutions")
		}
		body = bytes.Replace(body, []byte(pair[0]), []byte(pair[1]), 1)
	}
	return body, nil
}
func validateProvenance(root string) error {
	pins := map[string]string{"third_party/go/net/http/fs.go.txt": "21660cb61d17c258de99172683f9b41aca37d45c0af0556723f2a11179bdc45e", "third_party/go/net/textproto/textproto.go.txt": "e4f0d94752345b5dec4fe7f4805531e8036a9cd44131d807cafa700dd5323901", "third_party/GO-LICENSE": "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"}
	for name, want := range pins {
		raw, err := read(root, name, 1<<20)
		if err != nil {
			return err
		}
		if sum(raw) != want {
			return fmt.Errorf("original source/license pin: %s", name)
		}
	}
	own, err := read(root, "examples/versioned-grant-change/reference/main.go", 1<<20)
	if err != nil {
		return err
	}
	original, err := read(root, "third_party/go/net/http/fs.go.txt", 1<<20)
	if err != nil {
		return err
	}
	for _, name := range []string{"scanETag", "etagStrongMatch"} {
		a, err := extractFunction(original, name)
		if err != nil {
			return err
		}
		b, err := extractFunction(own, name)
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			return fmt.Errorf("byte-exact original: %s", name)
		}
	}
	adapted, err := adaptedIfMatch(original)
	if err != nil {
		return err
	}
	actual, err := extractFunction(own, "checkIfMatch")
	if err != nil {
		return err
	}
	if !bytes.Equal(adapted, actual) {
		return errors.New("original IfMatch adapter drift")
	}
	for _, raw := range [][]byte{original, own} {
		start := bytes.Index(raw, []byte("type condResult int\n"))
		if start < 0 {
			return errors.New("original enum missing")
		}
		end := bytes.Index(raw[start:], []byte("\n)\n"))
		if end < 0 {
			return errors.New("original enum close")
		}
		if !bytes.Equal(raw[start:start+end+3], []byte("type condResult int\n\nconst (\n\tcondNone condResult = iota\n\tcondTrue\n\tcondFalse\n)\n")) {
			return errors.New("original enum drift")
		}
	}
	if !bytes.Contains(own, []byte("Copyright 2009 The Go Authors.")) || !bytes.Contains(own, []byte("Copyright 2010 The Go Authors.")) || !bytes.Contains(own, []byte("third_party/GO-LICENSE")) {
		return errors.New("original license notice")
	}
	return nil
}
func observe(raw string) componentFacts {
	tag, remain := scanETag(raw)
	return componentFacts{tag, remain, httpguts.ValidHeaderFieldValue(raw), etagStrongMatch(tag, tag)}
}
func freshFile(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, e := f.Write(raw)
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return closeErr
}
func run(args []string) error {
	fs := flag.NewFlagSet("versioned-grant-reference", flag.ContinueOnError)
	root := fs.String("root", "", "canonical repository root")
	cases := fs.String("cases", "", "frozen master")
	manifest := fs.String("manifest", "", "workspace")
	out := fs.String("out", "", "fresh output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	for _, p := range []*string{root, cases, manifest} {
		value, err := canonical(*p)
		if err != nil {
			return err
		}
		*p = value
	}
	if _, err := canonical(filepath.Dir(*out)); err != nil || !filepath.IsAbs(*out) || filepath.Clean(*out) != *out {
		return errors.New("canonical fresh output parent")
	}
	if _, err := os.Lstat(*out); !errors.Is(err, os.ErrNotExist) {
		return errors.New("fresh output must not exist")
	}
	if runtime.Version() != "go1.27.3" {
		return errors.New("pinned Go1.27.3 required")
	}
	if err := validateProvenance(*root); err != nil {
		return err
	}
	master, err := bounded(*cases, 4<<20)
	if err != nil {
		return err
	}
	rows, err := decodeRows(master)
	if err != nil {
		return err
	}
	if len(rows) != 48 {
		return errors.New("exact frozen48 original rows required")
	}
	manifestRaw, err := bounded(*manifest, 128<<10)
	if err != nil {
		return err
	}
	imgs, total, err := images(*root, manifestRaw)
	if err != nil {
		return err
	}
	batches, err := prepareBatches(rows, total)
	if err != nil {
		return err
	}
	// Check the actual compiler stdlib and imported module source, not merely a version label.
	trim, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src/net/textproto/textproto.go"))
	if err != nil || sum(trim) != "e4f0d94752345b5dec4fe7f4805531e8036a9cd44131d807cafa700dd5323901" {
		return errors.New("actual pinned textproto source")
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("actual Go build metadata")
	}
	netOK := false
	for _, dep := range info.Deps {
		if dep.Path == "golang.org/x/net" {
			netOK = dep.Version == "v0.59.0" && dep.Replace == nil
		}
	}
	if !netOK {
		return errors.New("actual pinned x/net module")
	}
	cache := os.Getenv("GOMODCACHE")
	if cache == "" {
		goPath := os.Getenv("GOPATH")
		if goPath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			goPath = filepath.Join(home, "go")
		}
		cache = filepath.Join(strings.Split(goPath, string(os.PathListSeparator))[0], "pkg/mod")
	}
	guts, err := os.ReadFile(filepath.Join(cache, "golang.org/x/net@v0.59.0/http/httpguts/httplex.go"))
	if err != nil || sum(guts) != "8cfd5819dc821d45f82fde10afff4b2e652535b0091e0559fc8ad7766051a2c2" {
		return errors.New("actual pinned httpguts source")
	}
	plan := struct {
		Schema      string        `json:"schema"`
		Rows        int           `json:"rows"`
		Batches     []batchFile   `json:"batches"`
		Images      []sourceImage `json:"images"`
		Components  []component   `json:"components"`
		SourceBytes int           `json:"source_bytes"`
		MasterSHA   string        `json:"master_sha256"`
		ManifestSHA string        `json:"manifest_sha256"`
		GoVersion   string        `json:"go_version"`
		Scope       string        `json:"scope"`
	}{Schema: "gooo/versioned-grant-reference-plan/v1", Rows: 48, Images: imgs, SourceBytes: total, MasterSHA: "sha256:" + sum(master), ManifestSHA: "sha256:" + sum(manifestRaw), GoVersion: runtime.Version(), Scope: "Frozen RawMessage cases only; original Go scan/strong/IfMatch/header-valid components, not final policy/authority/effects or full joint-domain capacity. Existing AgentMesh matcher is observed separately."}
	if err := os.Mkdir(*out, 0700); err != nil {
		return err
	}
	for i, b := range batches {
		file := fmt.Sprintf("cases-%02d.json", i)
		if err := freshFile(filepath.Join(*out, file), b.Raw); err != nil {
			return err
		}
		plan.Batches = append(plan.Batches, batchFile{b.Start, b.End, file, len(b.Raw), b.LoweredUpperBytes})
	}
	for _, row := range rows {
		plan.Components = append(plan.Components, component{observe(row.Inputs[0]), observe(row.Inputs[1]), int64(checkIfMatch(row.Inputs[0], row.Inputs[1]))})
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := freshFile(filepath.Join(*out, "plan.json"), append(raw, '\n')); err != nil {
		return err
	}
	// Recheck exact inputs, including all source-image bytes, after writing evidence.
	now, err := bounded(*cases, 4<<20)
	if err != nil || !bytes.Equal(master, now) {
		return errors.New("master changed")
	}
	now, err = bounded(*manifest, 128<<10)
	if err != nil || !bytes.Equal(manifestRaw, now) {
		return errors.New("manifest changed")
	}
	again, againBytes, err := images(*root, manifestRaw)
	if err != nil || againBytes != total {
		return errors.New("source changed")
	}
	a, _ := json.Marshal(imgs)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		return errors.New("source images changed")
	}
	if err := validateProvenance(*root); err != nil {
		return err
	}
	return nil
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "VERSIONED_GRANT_REFERENCE_REFUSED:", err)
		os.Exit(1)
	}
}

// The original control flow below is kept byte-exact except the documented adapter.
func scanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	// ETag is either W/"text" or "text".
	// See RFC 7232 2.3.
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		// Character values allowed in ETags.
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

// condResult is the result of an HTTP request precondition check.
// See https://tools.ietf.org/html/rfc7232 section 3.
type condResult int

const (
	condNone condResult = iota
	condTrue
	condFalse
)

func checkIfMatch(selectedIfMatchHeader, currentETag string) condResult {
	im := selectedIfMatchHeader
	if im == "" {
		return condNone
	}
	for {
		im = textproto.TrimString(im)
		if len(im) == 0 {
			break
		}
		if im[0] == ',' {
			im = im[1:]
			continue
		}
		if im[0] == '*' {
			return condTrue
		}
		etag, remain := scanETag(im)
		if etag == "" {
			break
		}
		if etagStrongMatch(etag, currentETag) {
			return condTrue
		}
		im = remain
	}

	return condFalse
}
