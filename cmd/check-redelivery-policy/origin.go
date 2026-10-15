package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

type rawRef struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type originBindings struct {
	Schema        string   `json:"schema"`
	Repository    string   `json:"repository_root"`
	Oracle        rawRef   `json:"oracle"`
	Process       rawRef   `json:"process"`
	Contract      rawRef   `json:"contract"`
	Witness       rawRef   `json:"witness"`
	NativeInputs  rawRef   `json:"native_inputs"`
	Policy        rawRef   `json:"oracle_policy"`
	SourceFreeze  rawRef   `json:"source_freeze"`
	OracleBinary  rawRef   `json:"oracle_binary"`
	GoTool        rawRef   `json:"go_tool"`
	SourceClosure []rawRef `json:"source_closure"`
}

const policySHA = "c45284edbed9aeeb44843c2d687452e8753d6cbd1cdba41aaa04291467ee76ae"
const referenceDir = "examples/automatic-redelivery/reference"

var referenceNames = []string{"go.mod", "main.go", "policy.go", "policy_test.go", "strict.go"}

func currentRef(path string, limit int64) (rawRef, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return rawRef{}, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return rawRef{}, err
	}
	if resolved != absolute || absolute != filepath.Clean(absolute) {
		return rawRef{}, fmt.Errorf("reference path must be absolute/canonical: %s", path)
	}
	f, err := os.Open(absolute)
	if err != nil {
		return rawRef{}, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return rawRef{}, err
	}
	if !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return rawRef{}, fmt.Errorf("reference file kind/size: %s", path)
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, limit+1))
	if err != nil {
		return rawRef{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return rawRef{}, err
	}
	atPath, err := os.Stat(absolute)
	if err != nil {
		return rawRef{}, err
	}
	if n > limit || n != before.Size() || n != after.Size() || !os.SameFile(before, after) || !os.SameFile(before, atPath) || !before.ModTime().Equal(after.ModTime()) {
		return rawRef{}, fmt.Errorf("reference changed while reading: %s", path)
	}
	return rawRef{Path: absolute, Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func parseRef(value any) (rawRef, error) {
	m := object(value)
	if !keys(m, "path", "bytes", "sha256") {
		return rawRef{}, fmt.Errorf("raw reference exact shape")
	}
	path, ok := m["path"].(string)
	if !ok || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return rawRef{}, fmt.Errorf("raw reference path")
	}
	n, ok := m["bytes"].(json.Number)
	if !ok {
		return rawRef{}, fmt.Errorf("raw reference bytes type")
	}
	size, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || size < 0 || size > 128<<20 || strconv.FormatInt(size, 10) != n.String() {
		return rawRef{}, fmt.Errorf("raw reference bytes bounds")
	}
	sha, ok := m["sha256"].(string)
	if !ok || len(sha) != 64 || sha != strings.ToLower(sha) {
		return rawRef{}, fmt.Errorf("raw reference SHA spelling")
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return rawRef{}, err
	}
	want := rawRef{path, size, sha}
	got, err := currentRef(path, 128<<20)
	if err != nil {
		return rawRef{}, err
	}
	if got != want {
		return rawRef{}, fmt.Errorf("current raw reference differs: %s", path)
	}
	return want, nil
}

func readCurrentBytes(r rawRef, limit int64) ([]byte, error) {
	raw, err := readLimited(r.Path, limit)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != r.Bytes || digest(raw) != "sha256:"+r.SHA256 {
		return nil, fmt.Errorf("bytes differ from bound raw reference")
	}
	return raw, nil
}

func loadRef(value any, limit int64) (rawRef, map[string]any, error) {
	r, err := parseRef(value)
	if err != nil {
		return rawRef{}, nil, err
	}
	raw, err := readLimited(r.Path, limit)
	if err != nil {
		return rawRef{}, nil, err
	}
	if digest(raw) != "sha256:"+r.SHA256 || int64(len(raw)) != r.Bytes {
		return rawRef{}, nil, fmt.Errorf("raw reference second read differs")
	}
	v, err := decode(raw)
	if err != nil {
		return rawRef{}, nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return rawRef{}, nil, fmt.Errorf("referenced JSON must be object")
	}
	return r, m, nil
}

func refIndex(value any) (map[string]rawRef, error) {
	a, ok := value.([]any)
	if !ok || len(a) == 0 || len(a) > 256 {
		return nil, fmt.Errorf("reference roster array/bounds")
	}
	out := map[string]rawRef{}
	for _, v := range a {
		r, err := parseRef(v)
		if err != nil {
			return nil, err
		}
		if _, exists := out[r.Path]; exists {
			return nil, fmt.Errorf("duplicate reference path")
		}
		out[r.Path] = r
	}
	return out, nil
}
func member(index map[string]rawRef, r rawRef) bool { return index[r.Path] == r }

func canonicalRepository(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return fmt.Errorf("repository must be absolute canonical directory")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	st, err := os.Stat(root)
	if err != nil {
		return err
	}
	if resolved != root || !st.IsDir() {
		return fmt.Errorf("repository directory alias/kind")
	}
	return nil
}
func loadRuntimeFreeze(value any, root string) (rawRef, map[string]rawRef, map[string]rawRef, error) {
	r, m, err := loadRef(value, 1<<20)
	if err != nil {
		return rawRef{}, nil, nil, err
	}
	if !keys(m, "schema", "repository_root", "files", "tools", "overall_seconds") || m["schema"] != "gooo/redelivery-public-runtime-source-freeze/v1" || m["repository_root"] != root {
		return rawRef{}, nil, nil, fmt.Errorf("current public runtime freeze schema/root")
	}
	n, ok := m["overall_seconds"].(json.Number)
	if !ok {
		return rawRef{}, nil, nil, fmt.Errorf("runtime freeze time integer")
	}
	seconds, err := strconv.ParseInt(n.String(), 10, 64)
	if err != nil || seconds < 1 || seconds > 300 || strconv.FormatInt(seconds, 10) != n.String() {
		return rawRef{}, nil, nil, fmt.Errorf("runtime freeze time bound")
	}
	files, err := refIndex(m["files"])
	if err != nil {
		return rawRef{}, nil, nil, err
	}
	tools, err := refIndex(m["tools"])
	if err != nil {
		return rawRef{}, nil, nil, err
	}
	for path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rawRef{}, nil, nil, fmt.Errorf("frozen source outside repository")
		}
	}
	return r, files, tools, nil
}
func checkOracleOrigin(originPath, oraclePath, contractPath string, goTool rawRef, root string, profile rawRef, originRef rawRef) (originBindings, error) {
	fail := func(s string) (originBindings, error) {
		return originBindings{}, fmt.Errorf("invalid independent oracle origin: %s", s)
	}
	if err := canonicalRepository(root); err != nil {
		return originBindings{}, err
	}
	if originRef.Path != originPath {
		return fail("origin reference path differs")
	}
	raw, err := readCurrentBytes(originRef, 1<<20)
	if err != nil {
		return originBindings{}, err
	}
	v, err := decode(raw)
	if err != nil {
		return originBindings{}, err
	}
	m := object(v)
	if !keys(m, "schema", "oracle", "process", "contract", "witness", "native_inputs", "oracle_policy", "source_freeze", "oracle_binary", "go_tool", "source_closure") || m["schema"] != "gooo/redelivery-public-oracle-origin/v1" {
		return fail("exact current public origin schema/roster")
	}
	out := originBindings{Schema: str(m["schema"]), Repository: root}
	for _, slot := range []struct {
		name string
		dest *rawRef
	}{{"oracle", &out.Oracle}, {"process", &out.Process}, {"contract", &out.Contract}, {"witness", &out.Witness}, {"native_inputs", &out.NativeInputs}, {"oracle_policy", &out.Policy}, {"source_freeze", &out.SourceFreeze}, {"oracle_binary", &out.OracleBinary}, {"go_tool", &out.GoTool}} {
		r, err := parseRef(m[slot.name])
		if err != nil {
			return originBindings{}, fmt.Errorf("%s: %w", slot.name, err)
		}
		*slot.dest = r
	}
	if out.Oracle.Path != oraclePath || out.Contract.Path != contractPath || out.Contract.Path != filepath.Join(root, "examples/automatic-redelivery/contract.json") || out.Contract.SHA256 != contractSHA || out.Witness.Path != filepath.Join(root, "examples/automatic-redelivery/manual-witnesses.json") || out.Witness.SHA256 != witnessSHA || out.NativeInputs.Path != filepath.Join(root, "examples/automatic-redelivery/manual-inputs.json") || out.NativeInputs.SHA256 != nativeInputsSHA || out.Policy.Path != filepath.Join(root, referenceDir, "policy.go") || out.Policy.SHA256 != policySHA || out.GoTool != goTool || filepath.Base(out.OracleBinary.Path) != "oracle" || filepath.Base(filepath.Dir(out.OracleBinary.Path)) != "bin" || profile.Path != filepath.Join(root, "cmd/check-path-facts/result-profile-83928fa.json") {
		return fail("public contract/witness/native/policy/tool/path binding")
	}
	closure := array(m["source_closure"])
	if len(closure) != 5 {
		return fail("five independent source references")
	}
	for i, v := range closure {
		r, err := parseRef(v)
		if err != nil {
			return originBindings{}, err
		}
		if r.Path != filepath.Join(root, referenceDir, referenceNames[i]) {
			return fail("public oracle source exact roster/order")
		}
		out.SourceClosure = append(out.SourceClosure, r)
	}
	frozen, files, tools, err := loadRuntimeFreeze(m["source_freeze"], root)
	if err != nil {
		return originBindings{}, err
	}
	if frozen != out.SourceFreeze || !member(tools, out.GoTool) {
		return fail("actual Go absent from runtime freeze")
	}
	for _, r := range append(append([]rawRef{}, out.SourceClosure...), out.Contract, out.Witness, out.NativeInputs, out.Policy, profile) {
		if !member(files, r) {
			return fail("current oracle/contract/profile source outside frozen closure")
		}
	}
	critical := []string{manifestName, sourceName, "go.mod", "LICENSE", "upstream.json", "tools/check-redelivery-policy.py", "cmd/check-redelivery-policy/main.go", "cmd/check-redelivery-policy/inspect.go", "cmd/check-redelivery-policy/binding.go", "cmd/check-redelivery-policy/origin.go", "cmd/check-redelivery-policy/wire.go", "cmd/check-redelivery-policy/observer_test.go", "cmd/check-redelivery-policy/wire_test.go", "cmd/check-redelivery-policy/result_digest.go", "cmd/check-redelivery-policy/result_digest_test.go"}
	for _, name := range critical {
		r, err := currentRef(filepath.Join(root, name), 1<<20)
		if err != nil {
			return originBindings{}, err
		}
		if !member(files, r) {
			return fail("required public checker/source absent from runtime freeze")
		}
	}
	_, process, err := loadRef(m["process"], 1<<20)
	if err != nil {
		return originBindings{}, err
	}
	if err := checkOracleProcess(process, out); err != nil {
		return originBindings{}, err
	}
	stdout, err := parseRef(process["stdout"])
	if err != nil {
		return originBindings{}, err
	}
	stderr, err := parseRef(process["stderr"])
	if err != nil {
		return originBindings{}, err
	}
	if stdout != out.Oracle || stderr.Bytes != 0 || "sha256:"+stderr.SHA256 != digest(nil) {
		return fail("actual oracle stdout or empty stderr")
	}
	_, started, err := loadRef(process["started_ledger"], 1<<20)
	if err != nil {
		return originBindings{}, err
	}
	if err := checkStarted(process, started); err != nil {
		return originBindings{}, err
	}
	before, err := parseRef(process["source_freeze_before"])
	if err != nil {
		return originBindings{}, err
	}
	after, err := parseRef(process["source_freeze_after"])
	if err != nil {
		return originBindings{}, err
	}
	if before != out.SourceFreeze || after != before {
		return fail("actual process current source freeze before/after")
	}
	return out, nil
}
func checkOracleProcess(p map[string]any, out originBindings) error {
	if err := checkProcessState(p, 30); err != nil {
		return fmt.Errorf("invalid independent oracle process: %w", err)
	}
	if p["name"] != "original-oracle-manual16" || !equal(p["argv"], []string{out.OracleBinary.Path, "--contract", out.Contract.Path, "--manual"}) || p["cwd"] != filepath.Join(out.Repository, referenceDir) {
		return fmt.Errorf("invalid independent oracle process: actual oracle command/cwd")
	}
	return nil
}
func checkProcessState(p map[string]any, max float64) error {
	if p["status"] != "COMPLETED" || !isInt(p["exit_code"], 0) || !isInt(p["expected_exit_code"], 0) || !isInt(p["automatic_retries"], 0) || p["process_reaped"] != true || p["owned_group_remaining"] != false || p["abort"] != nil || p["primary_error"] != nil || p["source_tools_unchanged_after"] != true || !equal(p["secondary_errors"], []any{}) || !equal(p["termination_errors"], []any{}) {
		return fmt.Errorf("process completion/ownership/errors/counters")
	}
	group := object(p["owned_group_state"])
	if !keys(group, "state", "error") || group["state"] != "absent" || group["error"] != nil {
		return fmt.Errorf("owned group absence unconfirmed")
	}
	pid, pok := p["pid"].(json.Number)
	pgid, gok := p["owned_pgid"].(json.Number)
	if !pok || !gok || pid != pgid {
		return fmt.Errorf("owned process/group identity")
	}
	id, err := strconv.ParseInt(pid.String(), 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != pid.String() {
		return fmt.Errorf("owned PID integer")
	}
	for _, name := range []string{"timeout_seconds", "elapsed_seconds"} {
		n, ok := p[name].(json.Number)
		if !ok {
			return fmt.Errorf("process timing type")
		}
		v, err := strconv.ParseFloat(n.String(), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > max || (name == "timeout_seconds" && v == 0) {
			return fmt.Errorf("bounded process timing")
		}
	}
	return nil
}
func checkStarted(process, started map[string]any) error {
	if started["status"] != "STARTED" {
		return fmt.Errorf("STARTED ledger status")
	}
	for _, name := range []string{"name", "pid", "owned_pgid", "argv", "cwd", "source_freeze_before", "timeout_seconds", "expected_exit_code", "automatic_retries"} {
		if !reflect.DeepEqual(started[name], process[name]) {
			return fmt.Errorf("STARTED process identity differs: %s", name)
		}
	}
	return nil
}
