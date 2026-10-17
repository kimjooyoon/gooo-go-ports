// Strict expected-ledger loading and bounded native-recorder source bindings.
// Loading expected data does not establish native execution or CI acceptance.
package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxContractBytes = 1 << 20
const maxContractInt64 int64 = 1<<63 - 1

const nativeVerificationContractPath = "cmd/verify/testdata/native-verification-contract-v1.json"
const nativeVerificationContractVersionSHA256 = "sha256:b15cb329b36da4c0e19ef26cbb3f3c72f3e639701bc790cbd873cf75c0a24c13"

// The whole version hash binds the current suffix and every declared reader,
// in addition to the independently frozen historical prefix/source baseline.
// This source baseline binds immutable main 057bc6d8 path/mode identities.
// Loading an expected definition does not establish runtime acceptance.
// A later source revision must update the baseline metadata/guard and
// whole-version hash explicitly.
//
//go:embed testdata/native-verification-contract-v1.json
var nativeVerificationContractJSON []byte

func nativeExpectedVerificationContractFrom(raw []byte) (*NativeVerificationContract, error) {
	sum := sha256.Sum256(raw)
	if "sha256:"+hex.EncodeToString(sum[:]) != nativeVerificationContractVersionSHA256 {
		return nil, fmt.Errorf("native expected ledger version bytes differ")
	}
	return LoadNativeVerificationContract(raw, nativeFrozenPrefixGuard(), nativeFrozenSourceGuard())
}

func nativeFrozenPrefixGuard() FrozenGuard {
	return FrozenGuard{"b19b5517f8e9716affb0f75fd07045b8217cdaf1", 80, "sha256:9020c39f7746e766ef38b6d5854d97c14fcd386638d038937cf8f178aec52478"}
}

func nativeFrozenSourceGuard() FrozenGuard {
	return FrozenGuard{"057bc6d8d328a6ab2e7a3b37845a7450a6b19d0b", 283, "sha256:2b7247008db8ee9d7d92374bb3d01a317e3e35c2df009cfe943ac153037f2184"}
}

func nativeExpectedVerificationContract() (*NativeVerificationContract, error) {
	return nativeExpectedVerificationContractFrom(nativeVerificationContractJSON)
}

func nativeVerificationTimingSourceNames(c *NativeVerificationContract) []string {
	names := []string{}
	for _, source := range c.Sources {
		for _, role := range source.Roles {
			if role == "explicit-observer-input" || role == "contract-reader" {
				names = append(names, source.Path)
				break
			}
		}
	}
	return names
}

func mustNativeVerificationTimingSourceNames() []string {
	c, err := nativeExpectedVerificationContract()
	if err != nil {
		panic(fmt.Sprintf("embedded native expected ledger: %v", err))
	}
	return nativeVerificationTimingSourceNames(c)
}

func nativeVerificationSourceModes(c *NativeVerificationContract) map[string]string {
	modes := make(map[string]string, len(c.Sources))
	for _, source := range c.Sources {
		modes[source.Path] = source.Mode
	}
	return modes
}

func nativeVerificationCheckSourceMode(path, mode string) error {
	if mode != "100644" && mode != "100755" {
		return fmt.Errorf("unknown expected regular Git mode")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("native expected source is not a regular file")
	}
	// Windows does not represent Git's executable bit in its filesystem mode.
	// Its caller must check Git mode separately; regular/path/byte checks still run.
	if runtime.GOOS == "windows" {
		return nil
	}
	// Git ce_permissions uses the owner executable bit, not group/other bits.
	executable := info.Mode().Perm()&0100 != 0
	if executable != (mode == "100755") {
		return fmt.Errorf("native expected source executable bit differs")
	}
	return nil
}

func nativeVerificationCheckDiskLedger(raw []byte) error {
	if !bytes.Equal(raw, nativeVerificationContractJSON) {
		return fmt.Errorf("on-disk native expected ledger differs from embedded bytes")
	}
	return nil
}

// FrozenGuard must come from a separately reviewed caller, never from the ledger.
type FrozenGuard struct {
	Commit string `json:"commit"`
	Count  int64  `json:"count"`
	SHA256 string `json:"sha256"`
}

type ContractProfiles struct {
	Case      string `json:"case"`
	Workspace string `json:"workspace"`
}

// Field order is the normative canonical prefix field order.
type ContractSuite struct {
	Index       int64    `json:"index"`
	Package     string   `json:"package"`
	Activity    string   `json:"activity"`
	Cases       int64    `json:"cases"`
	Batches     int64    `json:"batches"`
	SourcePaths []string `json:"source_paths"`
}

type ContractSource struct {
	Path     string   `json:"path"`
	Mode     string   `json:"mode"`
	Owner    string   `json:"owner"`
	Roles    []string `json:"roles"`
	Baseline bool     `json:"baseline"`
}

type baselineSourceIdentity struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type NativeVerificationContract struct {
	Schema         string           `json:"schema"`
	Profiles       ContractProfiles `json:"profiles"`
	Prefix         FrozenGuard      `json:"prefix"`
	Suites         []ContractSuite  `json:"suites"`
	SourceBaseline FrozenGuard      `json:"source_baseline"`
	Sources        []ContractSource `json:"sources"`
	TotalCases     int64            `json:"-"`
	TotalBatches   int64            `json:"-"`
	TimingEvents   int64            `json:"-"`
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var integerPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var activityPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var contractRoles = map[string]bool{
	"catalog-source": true, "explicit-observer-input": true,
	"catalog-definition": true, "upstream-pins": true, "contract-reader": true,
}

// readStrictValue rejects repeated decoded keys recursively before schema reads.
func readStrictValue(d *json.Decoder, depth int) (any, error) {
	// Every valid schema value is at most four containers deep. Bound malformed
	// unknown structures too, without recursing over an attacker-sized stack.
	if depth > 8 {
		return nil, fmt.Errorf("JSON exceeds contract structural depth")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	opening, isDelimiter := t.(json.Delim)
	if !isDelimiter {
		return t, nil
	}
	switch opening {
	case '{':
		m := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			if _, found := m[name]; found {
				return nil, fmt.Errorf("duplicate decoded key %q", name)
			}
			value, err := readStrictValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[name] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("invalid object close: %v", err)
		}
		return m, nil
	case '[':
		values := []any{}
		for d.More() {
			value, err := readStrictValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("invalid array close: %v", err)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", opening)
	}
}

func contractObject(v any, keys ...string) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok || len(m) != len(keys) {
		return nil, fmt.Errorf("object has missing or unknown fields")
	}
	for _, key := range keys {
		if _, found := m[key]; !found {
			return nil, fmt.Errorf("object missing exact field %q", key)
		}
	}
	return m, nil
}

func contractString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected string")
	}
	return s, nil
}

func contractInteger(v any, positive bool) (int64, error) {
	n, ok := v.(json.Number)
	if !ok || !integerPattern.MatchString(string(n)) {
		return 0, fmt.Errorf("expected exact nonnegative decimal int64")
	}
	value, err := strconv.ParseInt(string(n), 10, 64)
	if err != nil || positive && value == 0 {
		return 0, fmt.Errorf("integer outside required int64 range")
	}
	return value, nil
}

func safeContractPath(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\:") {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func contractStrings(v any) ([]string, error) {
	values, ok := v.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("expected nonempty string array")
	}
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		s, err := contractString(value)
		if err != nil || seen[s] {
			return nil, fmt.Errorf("nonstring or duplicate array entry")
		}
		seen[s] = true
		result = append(result, s)
	}
	return result, nil
}

func validFrozenGuard(g FrozenGuard) bool {
	return commitPattern.MatchString(g.Commit) && g.Count > 0 && digestPattern.MatchString(g.SHA256)
}

func contractGuard(v any) (FrozenGuard, error) {
	m, err := contractObject(v, "commit", "count", "sha256")
	if err != nil {
		return FrozenGuard{}, err
	}
	commit, e1 := contractString(m["commit"])
	count, e2 := contractInteger(m["count"], true)
	digest, e3 := contractString(m["sha256"])
	g := FrozenGuard{commit, count, digest}
	if e1 != nil || e2 != nil || e3 != nil || !validFrozenGuard(g) {
		return FrozenGuard{}, fmt.Errorf("invalid frozen guard metadata")
	}
	return g, nil
}

func checkedContractAdd(a, b int64) (int64, error) {
	if a < 0 || b < 0 || b > maxContractInt64-a {
		return 0, fmt.Errorf("int64 total overflow")
	}
	return a + b, nil
}

// canonicalContractDigest uses no LF and disables Go's HTML escaping.
// All strings in these two hashed shapes are constrained to ASCII.
func canonicalContractDigest(v any) (string, error) {
	var buffer bytes.Buffer
	e := json.NewEncoder(&buffer)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return "", err
	}
	raw := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// LoadNativeVerificationContract reads an expected ledger using external guards.
// Current Git bytes, historical inputs and actual CI equality remain mandatory
// caller checks; successful loading alone proves none of those observations.
func LoadNativeVerificationContract(raw []byte, frozenPrefix, frozenSources FrozenGuard) (*NativeVerificationContract, error) {
	if !validFrozenGuard(frozenPrefix) || !validFrozenGuard(frozenSources) {
		return nil, fmt.Errorf("invalid external guard")
	}
	if len(raw) > MaxContractBytes || !utf8.Valid(raw) {
		return nil, fmt.Errorf("contract exceeds1MiB or is not valid UTF8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := readStrictValue(d, 0)
	if err != nil {
		return nil, fmt.Errorf("strict JSON: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON or invalid suffix")
	}
	m, err := contractObject(v, "schema", "profiles", "prefix", "suites", "source_baseline", "sources")
	if err != nil {
		return nil, err
	}
	if m["schema"] != "gooo/native-verification-contract/v1" {
		return nil, fmt.Errorf("unknown contract schema")
	}
	p, err := contractObject(m["profiles"], "case", "workspace")
	if err != nil {
		return nil, err
	}
	if p["case"] != "uuid-and-literal-report-compact-v1" || p["workspace"] != "dependency-closure-v1" {
		return nil, fmt.Errorf("unexpected exact profiles")
	}
	prefix, err := contractGuard(m["prefix"])
	if err != nil {
		return nil, fmt.Errorf("prefix metadata: %w", err)
	}
	if prefix != frozenPrefix {
		return nil, fmt.Errorf("prefix metadata differs from external guard")
	}
	baseline, err := contractGuard(m["source_baseline"])
	if err != nil {
		return nil, fmt.Errorf("source baseline metadata: %w", err)
	}
	if baseline != frozenSources {
		return nil, fmt.Errorf("source baseline metadata differs from external guard")
	}
	c := &NativeVerificationContract{Schema: "gooo/native-verification-contract/v1", Profiles: ContractProfiles{"uuid-and-literal-report-compact-v1", "dependency-closure-v1"}, Prefix: prefix, SourceBaseline: baseline}
	rows, ok := m["sources"].([]any)
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("sources must be a nonempty array")
	}
	qualified := map[string]bool{}
	baselinePaths := []baselineSourceIdentity{}
	for _, row := range rows {
		s, err := contractObject(row, "path", "mode", "owner", "roles", "baseline")
		if err != nil {
			return nil, err
		}
		path, e1 := contractString(s["path"])
		roles, e2 := contractStrings(s["roles"])
		flag, e3 := s["baseline"].(bool)
		mode, e4 := contractString(s["mode"])
		if e1 != nil || !safeContractPath(path) || e2 != nil || !e3 || e4 != nil || (mode != "100644" && mode != "100755") || s["owner"] != "native-ci" {
			return nil, fmt.Errorf("invalid source row")
		}
		if len(c.Sources) > 0 && path <= c.Sources[len(c.Sources)-1].Path {
			return nil, fmt.Errorf("source paths are not strictly sorted")
		}
		if !sort.StringsAreSorted(roles) {
			return nil, fmt.Errorf("source roles are not sorted")
		}
		for _, role := range roles {
			if !contractRoles[role] {
				return nil, fmt.Errorf("unknown source role %q", role)
			}
			if role == "catalog-source" {
				qualified[path] = true
			}
		}
		if flag {
			baselinePaths = append(baselinePaths, baselineSourceIdentity{path, mode})
		}
		c.Sources = append(c.Sources, ContractSource{path, mode, "native-ci", roles, flag})
	}
	if int64(len(baselinePaths)) != frozenSources.Count {
		return nil, fmt.Errorf("baseline path count differs")
	}
	baselineDigest, err := canonicalContractDigest(baselinePaths)
	if err != nil || baselineDigest != frozenSources.SHA256 {
		return nil, fmt.Errorf("baseline path digest differs")
	}
	suiteRows, ok := m["suites"].([]any)
	if !ok || len(suiteRows) == 0 || frozenPrefix.Count > int64(len(suiteRows)) {
		return nil, fmt.Errorf("suite roster shorter than frozen prefix")
	}
	seen := map[string]bool{}
	for i, row := range suiteRows {
		s, err := contractObject(row, "index", "package", "activity", "cases", "batches", "source_paths")
		if err != nil {
			return nil, err
		}
		index, e1 := contractInteger(s["index"], false)
		pkg, e2 := contractString(s["package"])
		activity, e3 := contractString(s["activity"])
		cases, e4 := contractInteger(s["cases"], true)
		batches, e5 := contractInteger(s["batches"], true)
		paths, e6 := contractStrings(s["source_paths"])
		if e1 != nil || index != int64(i) || e2 != nil || !safeContractPath(pkg) || e3 != nil || !activityPattern.MatchString(activity) || e4 != nil || e5 != nil || batches > cases || e6 != nil {
			return nil, fmt.Errorf("invalid suite row %d", i)
		}
		key := pkg + ":" + activity
		if seen[key] {
			return nil, fmt.Errorf("duplicate package:activity")
		}
		seen[key] = true
		for _, path := range paths {
			if !safeContractPath(path) || !qualified[path] {
				return nil, fmt.Errorf("suite source is missing or lacks catalog-source role")
			}
		}
		c.Suites = append(c.Suites, ContractSuite{index, pkg, activity, cases, batches, paths})
		c.TotalCases, err = checkedContractAdd(c.TotalCases, cases)
		if err != nil {
			return nil, err
		}
		c.TotalBatches, err = checkedContractAdd(c.TotalBatches, batches)
		if err != nil {
			return nil, err
		}
	}
	prefixDigest, err := canonicalContractDigest(c.Suites[:int(frozenPrefix.Count)])
	if err != nil || prefixDigest != frozenPrefix.SHA256 {
		return nil, fmt.Errorf("suite prefix digest differs")
	}
	c.TimingEvents = 2
	for _, amount := range []int64{int64(len(c.Suites)), int64(len(c.Suites)), c.TotalBatches} {
		c.TimingEvents, err = checkedContractAdd(c.TimingEvents, amount)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}
