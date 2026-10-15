// Independent stdlib references for the explicit prefix-rule-pair profile.
// No Gooo source is read. conflict=false is not permission or ACL safety.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"unicode/utf8"
)

const profileOutputSchema = "gooo/ipv4-prefix-rule-pair-independent-oracle/v1"
const profileNetipSHA = "32295f4761825d8c6fdade4dad093b08683bdd1945010bee8a90a298e18cf208"
const profileContractSHA = "864c62caa16e4b900cad4bc7579e35cd56473f618a9b8be9151109ed7d218891"
const prefixCorpusSHA = "573f6a836ceed5f28a7ed6eee92ea8dbfb00896b78e04711a6b291c2386c9e5c"
const pairCorpusSHA = "8a7b9470c17f3244ef9bf4d9e6016b4b83d0f61b2bec0ca0a22801416d51d698"

type profileRawRef struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type networkPrefix struct {
	A    int64 `json:"a"`
	B    int64 `json:"b"`
	C    int64 `json:"c"`
	D    int64 `json:"d"`
	Bits int64 `json:"bits"`
}

type prefixFacts struct {
	Contains bool `json:"contains"`
	Overlaps bool `json:"overlaps"`
}
type prefixProfileInputs struct {
	Prefix      networkPrefix `json:"prefix"`
	Address     octets        `json:"address"`
	OtherPrefix networkPrefix `json:"other_prefix"`
}
type prefixReference struct {
	Index    int                 `json:"index"`
	Name     string              `json:"name"`
	Inputs   prefixProfileInputs `json:"inputs"`
	Expected prefixFacts         `json:"expected"`
}
type pairRule struct {
	A        int64 `json:"a"`
	B        int64 `json:"b"`
	C        int64 `json:"c"`
	D        int64 `json:"d"`
	Bits     int64 `json:"bits"`
	Effect   int64 `json:"effect"`
	Scope    int64 `json:"scope"`
	Priority int64 `json:"priority"`
}

// No omitempty: a valid zero-address witness differs from a false zero sentinel.
type pairFacts struct {
	Valid          bool  `json:"valid"`
	Comparable     bool  `json:"comparable"`
	Overlap        bool  `json:"overlap"`
	OpposingEffect bool  `json:"opposing_effect"`
	Conflict       bool  `json:"conflict"`
	WitnessValid   bool  `json:"witness_valid"`
	WitnessA       int64 `json:"witness_a"`
	WitnessB       int64 `json:"witness_b"`
	WitnessC       int64 `json:"witness_c"`
	WitnessD       int64 `json:"witness_d"`
	Reason         int64 `json:"reason"`
}
type pairProfileInputs struct {
	Left  pairRule `json:"left"`
	Right pairRule `json:"right"`
}
type pairReference struct {
	Index    int               `json:"index"`
	Name     string            `json:"name"`
	Inputs   pairProfileInputs `json:"inputs"`
	Expected pairFacts         `json:"expected"`
}
type prefixPairOutput struct {
	Schema            string            `json:"schema"`
	GoVersion         string            `json:"go_version"`
	NetipSourceSHA256 string            `json:"netip_source_sha256"`
	Contract          profileRawRef     `json:"contract"`
	PrefixCorpus      profileRawRef     `json:"prefix_corpus"`
	PairCorpus        profileRawRef     `json:"pair_corpus"`
	PrefixCaseCount   int               `json:"prefix_case_count"`
	PairCaseCount     int               `json:"pair_case_count"`
	PrefixCases       []prefixReference `json:"prefix_cases"`
	PairCases         []pairReference   `json:"pair_cases"`
}

func validNetworkOctets(a, b, c, d int64) bool {
	return a >= 0 && a <= 255 && b >= 0 && b <= 255 && c >= 0 && c <= 255 && d >= 0 && d <= 255
}
func validNetworkPrefix(p networkPrefix) bool {
	return validNetworkOctets(p.A, p.B, p.C, p.D) && p.Bits >= 0 && p.Bits <= 32
}
func actualPrefix(p networkPrefix) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{byte(p.A), byte(p.B), byte(p.C), byte(p.D)}), int(p.Bits))
}
func assessPrefix(in prefixProfileInputs) prefixFacts {
	var out prefixFacts
	if validNetworkPrefix(in.Prefix) && validNetworkOctets(in.Address.A, in.Address.B, in.Address.C, in.Address.D) {
		addr := netip.AddrFrom4([4]byte{byte(in.Address.A), byte(in.Address.B), byte(in.Address.C), byte(in.Address.D)})
		out.Contains = actualPrefix(in.Prefix).Contains(addr)
	}
	if validNetworkPrefix(in.Prefix) && validNetworkPrefix(in.OtherPrefix) {
		out.Overlaps = actualPrefix(in.Prefix).Overlaps(actualPrefix(in.OtherPrefix))
	}
	return out
}
func ruleNetwork(r pairRule) networkPrefix { return networkPrefix{r.A, r.B, r.C, r.D, r.Bits} }
func validPairRule(r pairRule) bool {
	return validNetworkPrefix(ruleNetwork(r)) && (r.Effect == 0 || r.Effect == 1) && r.Scope > 0
}
func assessPair(left, right pairRule) (pairFacts, error) {
	var out pairFacts
	if !validPairRule(left) || !validPairRule(right) {
		return out, nil
	}
	lp, rp := actualPrefix(ruleNetwork(left)), actualPrefix(ruleNetwork(right))
	if !lp.IsValid() || !rp.IsValid() {
		return out, fmt.Errorf("bounded PrefixFrom unexpectedly invalid")
	}
	out.Valid = true
	out.Comparable = left.Scope == right.Scope && left.Priority == right.Priority
	out.Overlap = lp.Overlaps(rp)
	out.OpposingEffect = left.Effect != right.Effect
	out.Conflict = out.Comparable && out.Overlap && out.OpposingEffect
	out.WitnessValid = out.Conflict
	switch {
	case left.Scope != right.Scope:
		out.Reason = 1
	case left.Priority != right.Priority:
		out.Reason = 2
	case !out.Overlap:
		out.Reason = 3
	case !out.OpposingEffect:
		out.Reason = 4
	default:
		out.Reason = 5
	}
	if out.Conflict {
		chosen := lp
		if right.Bits > left.Bits {
			chosen = rp
		}
		w := chosen.Masked().Addr()
		if !w.Is4() || !lp.Contains(w) || !rp.Contains(w) {
			return pairFacts{}, fmt.Errorf("witness not in both actual Go prefixes")
		}
		a := w.As4()
		out.WitnessA, out.WitnessB, out.WitnessC, out.WitnessD = int64(a[0]), int64(a[1]), int64(a[2]), int64(a[3])
	}
	return out, nil
}

// Preserve raw token kinds: encoding/json's int64 fields alone can silently
// default null. Exact object keys also avoid case-insensitive struct matching.
func profileObject(raw json.RawMessage, keys ...string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if len(m) != len(keys) {
		return nil, fmt.Errorf("exact object field roster required")
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing field %s", k)
		}
	}
	return m, nil
}
func profileIntegers(raw json.RawMessage, keys ...string) ([]int64, error) {
	m, err := profileObject(raw, keys...)
	if err != nil {
		return nil, err
	}
	values := make([]int64, len(keys))
	for i, k := range keys {
		values[i], err = strconv.ParseInt(string(bytes.TrimSpace(m[k])), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("require int64 integer token: %s", k)
		}
	}
	return values, nil
}

// The new profile rejects unpaired escaped surrogates as well as raw bad UTF8;
// scanJSON subsequently checks complete JSON syntax and duplicate decoded keys.
func profileUnicode(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid raw UTF8")
	}
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return fmt.Errorf("unfinished JSON escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return fmt.Errorf("short Unicode escape")
		}
		x, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return fmt.Errorf("bad Unicode escape")
		}
		i += 4
		if x >= 0xd800 && x <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return fmt.Errorf("unpaired high surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("unpaired high surrogate")
			}
			i += 6
		} else if x >= 0xdc00 && x <= 0xdfff {
			return fmt.Errorf("unpaired low surrogate")
		}
	}
	return nil
}
func profileRows(raw []byte, schema string, rowKeys ...string) ([]map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 65536 {
		return nil, fmt.Errorf("profile JSON byte bound")
	}
	if err := profileUnicode(raw); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := scanJSON(d, 0); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	doc, err := profileObject(raw, "schema", "cases")
	if err != nil {
		return nil, err
	}
	var actualSchema string
	if err := json.Unmarshal(doc["schema"], &actualSchema); err != nil || actualSchema != schema {
		return nil, fmt.Errorf("profile schema differs")
	}
	var cases []json.RawMessage
	if err := json.Unmarshal(doc["cases"], &cases); err != nil || len(cases) == 0 || len(cases) > 112 {
		return nil, fmt.Errorf("bounded nonempty case array required")
	}
	rows := make([]map[string]json.RawMessage, 0, len(cases))
	names := make(map[string]bool)
	for _, rawRow := range cases {
		row, err := profileObject(rawRow, rowKeys...)
		if err != nil {
			return nil, err
		}
		var name string
		if err := json.Unmarshal(row["name"], &name); err != nil || name == "" || len(name) > 128 || names[name] {
			return nil, fmt.Errorf("unique bounded name required")
		}
		names[name] = true
		rows = append(rows, row)
	}
	return rows, nil
}
func decodePrefixRows(raw []byte) ([]prefixReference, error) {
	items, err := profileRows(raw, "gooo/ipv4-prefix-pilot-inputs/v1", "name", "prefix", "address", "other_prefix")
	if err != nil {
		return nil, err
	}
	rows := make([]prefixReference, 0, len(items))
	for i, item := range items {
		p, err := profileIntegers(item["prefix"], "a", "b", "c", "d", "bits")
		if err != nil {
			return nil, err
		}
		a, err := profileIntegers(item["address"], "a", "b", "c", "d")
		if err != nil {
			return nil, err
		}
		o, err := profileIntegers(item["other_prefix"], "a", "b", "c", "d", "bits")
		if err != nil {
			return nil, err
		}
		var name string
		if err := json.Unmarshal(item["name"], &name); err != nil {
			return nil, err
		}
		in := prefixProfileInputs{networkPrefix{p[0], p[1], p[2], p[3], p[4]}, octets{a[0], a[1], a[2], a[3]}, networkPrefix{o[0], o[1], o[2], o[3], o[4]}}
		rows = append(rows, prefixReference{i, name, in, assessPrefix(in)})
	}
	return rows, nil
}
func decodePairRows(raw []byte) ([]pairReference, error) {
	items, err := profileRows(raw, "gooo/ipv4-rule-pair-inputs/v1", "name", "left", "right")
	if err != nil {
		return nil, err
	}
	rows := make([]pairReference, 0, len(items))
	for i, item := range items {
		l, err := profileIntegers(item["left"], "a", "b", "c", "d", "bits", "effect", "scope", "priority")
		if err != nil {
			return nil, err
		}
		r, err := profileIntegers(item["right"], "a", "b", "c", "d", "bits", "effect", "scope", "priority")
		if err != nil {
			return nil, err
		}
		left, right := pairRule{l[0], l[1], l[2], l[3], l[4], l[5], l[6], l[7]}, pairRule{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7]}
		expected, err := assessPair(left, right)
		if err != nil {
			return nil, err
		}
		var name string
		if err := json.Unmarshal(item["name"], &name); err != nil {
			return nil, err
		}
		rows = append(rows, pairReference{i, name, pairProfileInputs{left, right}, expected})
	}
	return rows, nil
}

func readProfileFile(path string, size int, digest string) ([]byte, profileRawRef, error) {
	var empty profileRawRef
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || canonical != path {
		return nil, empty, fmt.Errorf("canonical absolute profile path required")
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != int64(size) {
		return nil, empty, fmt.Errorf("frozen regular profile byte size differs")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, empty, err
	}
	opened, statErr := f.Stat()
	if statErr != nil || !os.SameFile(before, opened) {
		f.Close()
		return nil, empty, fmt.Errorf("profile identity changed before read")
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, int64(size)+1))
	closeErr := f.Close()
	if readErr != nil {
		return nil, empty, readErr
	}
	if closeErr != nil {
		return nil, empty, closeErr
	}
	after, err := os.Lstat(path)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() || len(raw) != size || fmt.Sprintf("%x", sha256.Sum256(raw)) != digest {
		return nil, empty, fmt.Errorf("frozen profile identity or SHA256 differs")
	}
	return raw, profileRawRef{path, size, digest}, nil
}
func runPrefixRulePair(args []string, out io.Writer, goVersion string) error {
	f := flag.NewFlagSet("ipv4-prefix-rule-pair-reference", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	prefixPath := f.String("prefix-corpus", "", "absolute frozen18 prefix corpus")
	pairPath := f.String("corpus", "", "absolute frozen112 rule-pair corpus")
	contractPath := f.String("contract", "", "absolute frozen public rule-pair contract")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *prefixPath == "" || *pairPath == "" || *contractPath == "" || f.NArg() != 0 {
		return fmt.Errorf("require --prefix-corpus --corpus --contract and no positional args")
	}
	if goVersion != pinnedGo {
		return fmt.Errorf("require %s", pinnedGo)
	}
	stdlib, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "src/net/netip/netip.go"))
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(stdlib)) != profileNetipSHA {
		return fmt.Errorf("pinned Go netip source differs")
	}
	_, contractRef, err := readProfileFile(*contractPath, 6091, profileContractSHA)
	if err != nil {
		return err
	}
	prefixRaw, prefixRef, err := readProfileFile(*prefixPath, 6983, prefixCorpusSHA)
	if err != nil {
		return err
	}
	pairRaw, pairRef, err := readProfileFile(*pairPath, 49496, pairCorpusSHA)
	if err != nil {
		return err
	}
	prefixCases, err := decodePrefixRows(prefixRaw)
	if err != nil {
		return err
	}
	pairCases, err := decodePairRows(pairRaw)
	if err != nil {
		return err
	}
	if len(prefixCases) != 18 || len(pairCases) != 112 {
		return fmt.Errorf("frozen profile case population differs")
	}
	result := prefixPairOutput{profileOutputSchema, goVersion, profileNetipSHA, contractRef, prefixRef, pairRef, 18, 112, prefixCases, pairCases}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
