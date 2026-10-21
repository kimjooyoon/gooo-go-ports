// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2026 Gooo Go Ports contributors. All rights reserved.
// BSD-style terms: LICENSE and third_party/GO-LICENSE; provenance in NOTICE.
// Verification-only scanETag/etagStrongMatch originals are copied byte-exact
// from Go1.27.2 src/net/http/fs.go:435..467, revision
// 022c8636110ebac86a9b88724cda168ed713c21f. The actual pinned stdlib
// textproto.TrimString and x/net v0.59.0 httpguts implementation are used.
// This is one finite development check, never a Gooo runtime fallback.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"
)

const (
	revision   = "205896a413e5dea92366309c35ae85e542d055f6"
	corpusSHA  = "sha256:ce9015cc12bc37e785761f4ee6907a77603082cd69b4d8edc80a634ff4478345"
	core       = "go/net/http/etag"
	header     = "go/golang.org/x/net/http/httpguts"
	app        = "app/strong-validator-facts"
	scanID     = "gooo://go/net/http/etag-scan-result/v1"
	factsID    = "gooo://go-ports/examples/strong-validator-facts"
	textID     = "gooo://std/text"
	boolID     = "gooo://std/boolean"
	coreFile   = "ports/net/http/etag.gooo"
	headerFile = "ports/net/httpguts/headers.gooo"
	appFile    = "examples/strong-validator-facts/main.gooo"
	reportFile = "strong-validator-facts.workspace.json"
)

var files = []string{coreFile, headerFile, appFile, reportFile, "corpus.json", "check.go", "go.mod", "go.sum", "README.md", "NOTICE", "LICENSE", "third_party/GO-LICENSE", ".github/workflows/ci.yml"}
var scanFields = []string{"etag", "remain"}
var factsFields = []string{"raw", "tag", "remainder", "header_valid", "present", "weak", "complete", "strong", "single_strong", "input_bytes", "tag_bytes", "remainder_bytes", "reason"}
var factsTypes = []string{"string", "string", "string", "boolean", "boolean", "boolean", "boolean", "boolean", "boolean", "integer", "integer", "integer", "string"}

type inputRow struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Inputs []string `json:"inputs"`
}
type scanResult struct {
	ETag   string `json:"etag"`
	Remain string `json:"remain"`
}
type facts struct {
	Raw            string `json:"raw"`
	Tag            string `json:"tag"`
	Remainder      string `json:"remainder"`
	HeaderValid    bool   `json:"header_valid"`
	Present        bool   `json:"present"`
	Weak           bool   `json:"weak"`
	Complete       bool   `json:"complete"`
	Strong         bool   `json:"strong"`
	SingleStrong   bool   `json:"single_strong"`
	InputBytes     int64  `json:"input_bytes"`
	TagBytes       int64  `json:"tag_bytes"`
	RemainderBytes int64  `json:"remainder_bytes"`
	Reason         string `json:"reason"`
}
type batch struct {
	Activity, Kind, Package, ID string
	Start, End                  int
}

var batches = []batch{
	{"ScanETag", "scanner", core, "gooo://go/net/http/scan-etag", 0, 55},
	{"ScanETag", "scanner", core, "gooo://go/net/http/scan-etag", 55, 70},
	{"ScanETag", "scanner", core, "gooo://go/net/http/scan-etag", 70, 192},
	{"ETagStrongMatch", "strong_match", core, "gooo://go/net/http/etag-strong-match", 0, 28},
	{"ETagStrongMatch", "strong_match", core, "gooo://go/net/http/etag-strong-match", 28, 31},
	{"Report", "facts", app, factsID + "/report", 0, 19},
	{"Report", "facts", app, factsID + "/report", 19, 24},
}

// scanETag determines if a syntactically valid ETag is present at s. If so,
// the ETag and remaining text after consuming ETag is returned. Otherwise,
// it returns "", "".
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

// etagStrongMatch reports whether a and b match using strong ETag comparison.
// Assumes a and b are valid ETags.
func etagStrongMatch(a, b string) bool {
	return a == b && a != "" && a[0] == '"'
}

func reference(r inputRow) any {
	if r.Kind == "strong_match" {
		return etagStrongMatch(r.Inputs[0], r.Inputs[1])
	}
	tag, rest := scanETag(r.Inputs[0])
	if r.Kind == "scanner" {
		return scanResult{tag, rest}
	}
	valid, present := httpguts.ValidHeaderFieldValue(r.Inputs[0]), tag != ""
	weak, complete, strong := present && strings.HasPrefix(tag, "W/"), present && rest == "", etagStrongMatch(tag, tag)
	reason := "SINGLE_STRONG_TAG"
	switch {
	case !valid:
		reason = "HEADER_VALUE_INVALID"
	case !present:
		reason = "NO_VALID_TAG"
	case !complete:
		reason = "TRAILING_CONTENT"
	case weak:
		reason = "WEAK_TAG"
	}
	return facts{r.Inputs[0], tag, rest, valid, present, weak, complete, strong, valid && present && complete && strong, int64(len(r.Inputs[0])), int64(len(tag)), int64(len(rest)), reason}
}

func need(ok bool, message string) {
	if !ok {
		panic(errors.New(message))
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func object(v any) map[string]any {
	m, ok := v.(map[string]any)
	need(ok, "JSON object required")
	return m
}
func array(v any) []any             { a, ok := v.([]any); need(ok, "JSON array required"); return a }
func text(v any) string             { s, ok := v.(string); need(ok, "JSON string required"); return s }
func same(a, b any, message string) { need(reflect.DeepEqual(a, b), message) }
func integer(v any) int64 {
	n, ok := v.(json.Number)
	need(ok, "exact JSON integer required")
	x, err := strconv.ParseInt(string(n), 10, 64)
	must(err)
	return x
}
func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func encode(v any) []byte    { b, e := json.Marshal(v); must(e); return b }
func pretty(v any) []byte    { b, e := json.MarshalIndent(v, "", "  "); must(e); return append(b, '\n') }
func normalized(v any) any   { return decode(encode(v)) }
func keys(m map[string]any, names ...string) {
	need(len(m) == len(names), "JSON object key count differs")
	for _, n := range names {
		_, ok := m[n]
		need(ok, "missing JSON key: "+n)
	}
}
func empty(v any) bool {
	if v == nil {
		return true
	}
	a, ok := v.([]any)
	return ok && len(a) == 0
}

// The fixed corpus is hashed before parsing. This decoder also rejects lossy
// JSON: duplicate keys, malformed UTF8, unpaired surrogate escapes, extra roots.
func decode(b []byte) any {
	need(utf8.Valid(b), "malformed JSON UTF8")
	for i := 0; i < len(b); i++ {
		if b[i] != '"' {
			continue
		}
		i++
		for i < len(b) && b[i] != '"' {
			if b[i] != '\\' {
				i++
				continue
			}
			i++
			need(i < len(b), "unterminated JSON escape")
			if b[i] != 'u' {
				i++
				continue
			}
			need(i+4 < len(b), "short Unicode escape")
			n, err := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
			must(err)
			i += 5
			if n >= 0xD800 && n <= 0xDBFF {
				need(i+5 < len(b) && b[i] == '\\' && b[i+1] == 'u', "unpaired high surrogate")
				low, err := strconv.ParseUint(string(b[i+2:i+6]), 16, 16)
				must(err)
				need(low >= 0xDC00 && low <= 0xDFFF, "unpaired high surrogate")
				i += 6
			} else {
				need(n < 0xDC00 || n > 0xDFFF, "unpaired low surrogate")
			}
		}
		need(i < len(b), "unterminated JSON string")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var value func(int) any
	value = func(depth int) any {
		need(depth <= 64, "JSON depth exceeds64")
		tok, err := dec.Token()
		must(err)
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				m := map[string]any{}
				for dec.More() {
					k, err := dec.Token()
					must(err)
					s := text(k)
					_, seen := m[s]
					need(!seen, "duplicate JSON key: "+s)
					m[s] = value(depth + 1)
				}
				close, err := dec.Token()
				must(err)
				need(close == json.Delim('}'), "JSON object close")
				return m
			case '[':
				a := []any{}
				for dec.More() {
					a = append(a, value(depth+1))
				}
				close, err := dec.Token()
				must(err)
				need(close == json.Delim(']'), "JSON array close")
				return a
			default:
				need(false, "unexpected JSON delimiter")
			}
		}
		return tok
	}
	v := value(0)
	_, err := dec.Token()
	need(err == io.EOF, "extra JSON root")
	return v
}

func corpus(raw []byte) map[string][]inputRow {
	same(digest(raw), corpusSHA, "input-only corpus bytes differ")
	doc := object(decode(raw))
	keys(doc, "schema", "cases")
	same(doc["schema"], "gooo/etag-input-corpus/v1", "corpus schema")
	grouped := map[string][]inputRow{}
	seen := map[string]bool{}
	order := []string{}
	operands := 0
	for _, v := range array(doc["cases"]) {
		m := object(v)
		keys(m, "id", "kind", "inputs")
		r := inputRow{ID: text(m["id"]), Kind: text(m["kind"])}
		need(r.ID != "" && !seen[r.ID], "unique corpus ID required")
		seen[r.ID] = true
		arity := 1
		if r.Kind == "strong_match" {
			arity = 2
		}
		need(r.Kind == "scanner" || r.Kind == "strong_match" || r.Kind == "facts", "corpus kind")
		ins := array(m["inputs"])
		need(len(ins) == arity, "corpus input arity")
		for _, v := range ins {
			s := text(v)
			need(utf8.ValidString(s) && len(s) <= 1024, "Text outside Unicode-scalar validUTF8 0..1024-byte domain")
			r.Inputs = append(r.Inputs, s)
			operands++
		}
		if len(order) == 0 || order[len(order)-1] != r.Kind {
			order = append(order, r.Kind)
		}
		grouped[r.Kind] = append(grouped[r.Kind], r)
	}
	same(order, []string{"scanner", "strong_match", "facts"}, "ordered original corpus groups")
	need(len(grouped["scanner"]) == 192 && len(grouped["strong_match"]) == 31 && len(grouped["facts"]) == 24 && operands == 278, "247 rows/278 operands required")
	return grouped
}

// Files are checked as current bytes, not against old Gooo body hashes. Edits
// to computes are permitted; source/ABI/value checks still apply to that edit.
func readFile(path string, limit int64) []byte {
	need(filepath.IsAbs(path) && filepath.Clean(path) == path, "absolute clean file path required")
	resolved, err := filepath.EvalSymlinks(path)
	must(err)
	need(resolved == path, "symlink file/parent unsupported: "+path)
	st, err := os.Lstat(path)
	must(err)
	need(st.Mode().IsRegular() && st.Size() <= limit, "regular bounded file required: "+path)
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	must(err)
	need(int64(len(b)) <= limit, "file exceeds bound")
	return b
}
func fresh(path string, b []byte) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must(err)
	_, e := f.Write(b)
	ce := f.Close()
	must(e)
	must(ce)
}
func newDir(path string) { must(os.Mkdir(path, 0700)) }
func copyWorkspace(dir string, sources map[string][]byte) {
	newDir(dir)
	for _, name := range []string{coreFile, headerFile, appFile} {
		path := filepath.Join(dir, name)
		must(os.MkdirAll(filepath.Dir(path), 0700))
		fresh(path, sources[name])
	}
}
func manifest(b batch) any {
	packs := []any{map[string]any{"path": core, "sources": []string{coreFile}}}
	if b.Package == app {
		packs = append(packs, map[string]any{"path": header, "sources": []string{headerFile}}, map[string]any{"path": app, "sources": []string{appFile}})
	}
	return map[string]any{"schema": "gooo/package-workspace-manifest/v1", "entry": map[string]any{"package_path": b.Package, "activity": b.Activity}, "packages": packs}
}
func sourceBytes(b batch, sources map[string][]byte) int {
	n := len(sources[coreFile])
	if b.Package == app {
		n += len(sources[headerFile]) + len(sources[appFile])
	}
	need(n > 0 && n <= 10<<20, "selected sources exceed10MiB")
	return n
}
func cases(b batch, rows []inputRow, sources map[string][]byte) ([]byte, []any, []byte) {
	need(len(rows) == b.End-b.Start && len(rows) > 0 && len(rows) <= 128, "fixed batch row count")
	key := b.Package + ":" + b.Activity
	body := []any{}
	want := []any{}
	matrix := [][]any{}
	for _, r := range rows {
		original := reference(r)
		expected := normalized(original)
		inputs := map[string]any{}
		for i, s := range r.Inputs {
			k := key
			if len(r.Inputs) > 1 {
				k += ".input" + strconv.Itoa(i)
			}
			inputs[k] = s
		}
		body = append(body, map[string]any{"inputs": inputs, "expected": map[string]any{key: expected}})
		want = append(want, expected)
		matrix = append(matrix, []any{original})
	}
	doc := map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": body}
	raw := pretty(doc)
	lower := len("GoooPackage") + len(strconv.Itoa(sourceBytes(b, sources))) + len("Activity") + len(b.Activity)
	growth := max(0, lower-len(key))
	arity := len(rows[0].Inputs)
	need(len(raw) <= 30000 && len(encode(doc))+(arity+1)*len(rows)*growth <= 32768, "fixed batch exceeds existing839 raw/translated budgets; no rows omitted")
	return raw, want, append(encode(matrix), '\n')
}

// Each direct child owns a new process group. Cancellation/cap/Wait failure
// kills only that group. Bounded original streams and dispositions are retained.
const maxCommandStdout = 1 << 20

// The current seven baseline receipts plus one mutation receipt peak at
// 7,628,750 bytes. This finite 8 MiB budget is not a promise for arbitrary edits.
const maxReceiptStdout = 8 << 20
const maxCommandStderr = 256 << 10

// Only the explicit compiler package-execute JSON path receives a receipt budget.
func commandStdoutLimit(binary, compiler string, args []string) int {
	if binary == compiler && len(args) >= 3 && args[0] == "package" && args[1] == "execute" && args[2] == "--json" {
		return maxReceiptStdout
	}
	return maxCommandStdout
}

type stream struct {
	file   *os.File
	data   bytes.Buffer
	limit  int
	cancel context.CancelFunc
	capped bool
}

func (s *stream) Write(p []byte) (int, error) {
	n := len(p)
	room := s.limit - s.data.Len()
	if room < n {
		n = room
		s.capped = true
	}
	if n > 0 {
		_, err := s.file.Write(p[:n])
		if err != nil {
			s.cancel()
			return 0, err
		}
		s.data.Write(p[:n])
	}
	if s.capped {
		s.cancel()
		return n, errors.New("command output cap exceeded")
	}
	return n, nil
}

type process struct {
	Raw   []byte
	Exit  int
	Error string
}
type runner struct {
	ctx                                context.Context
	out, root                          string
	compiler, goTool, goVersion, goSHA string
	commands                           int
}

func cleanEnv() []string {
	fixed := map[string]string{"GOTOOLCHAIN": "local", "GOWORK": "off", "GOENV": "off", "GOFLAGS": "-mod=readonly", "GOPROXY": "off", "GOSUMDB": "off", "GOOO_LAYA_URL": "", "GOOO_LAYA_API_KEY": ""}
	env := []string{}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := fixed[k]; !ok {
			env = append(env, kv)
		}
	}
	for k, v := range fixed {
		env = append(env, k+"="+v)
	}
	return env
}
func (r *runner) run(name, binary, cwd string, args ...string) process {
	dir := filepath.Join(r.out, name)
	newDir(dir)
	stdout, err := os.OpenFile(filepath.Join(dir, "stdout"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	stderr, err := os.OpenFile(filepath.Join(dir, "stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		stdout.Close()
		must(err)
	}
	ctx, cancel := context.WithTimeout(r.ctx, 90*time.Second)
	defer cancel()
	out := &stream{file: stdout, limit: commandStdoutLimit(binary, r.compiler, args), cancel: cancel}
	diag := &stream{file: stderr, limit: maxCommandStderr, cancel: cancel}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = cwd
	cmd.Env = cleanEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdout, cmd.Stderr = out, diag
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		e := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if e == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return e
	}
	began := time.Now()
	startErr := cmd.Start()
	waitErr := startErr
	gone := true
	groupErr := ""
	if startErr == nil {
		waitErr = cmd.Wait()
		e := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if e != nil && e != syscall.ESRCH {
			groupErr = e.Error()
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			e = syscall.Kill(-cmd.Process.Pid, 0)
			if e == syscall.ESRCH {
				break
			}
			if time.Now().After(deadline) {
				gone = false
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	closeOut, closeErr := stdout.Close(), stderr.Close()
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	errorText := ""
	if waitErr != nil {
		errorText = waitErr.Error()
	}
	record := map[string]any{"argv": append([]string{binary}, args...), "cwd": cwd, "started": startErr == nil, "exit_code": exit, "error": errorText, "wall_ns": time.Since(began).Nanoseconds(), "command_limit_seconds": 90, "stdout_cap": out.limit, "stderr_cap": diag.limit, "stdout_sha256": digest(out.data.Bytes()), "stderr_sha256": digest(diag.data.Bytes()), "stdout_capped": out.capped, "stderr_capped": diag.capped, "own_group_gone": gone, "group_cleanup_error": groupErr}
	if cmd.Process != nil {
		record["pid"] = cmd.Process.Pid
	}
	if ctx.Err() != nil {
		record["context_error"] = ctx.Err().Error()
	}
	fresh(filepath.Join(dir, "process.json"), pretty(record))
	r.commands++
	must(closeOut)
	must(closeErr)
	need(startErr == nil && ctx.Err() == nil && !out.capped && !diag.capped && gone && groupErr == "", "bounded command/process failure: "+name)
	return process{bytes.Clone(out.data.Bytes()), exit, errorText}
}
func success(p process, name string) []byte {
	need(p.Exit == 0 && p.Error == "", "command failed: "+name+": "+p.Error)
	return p.Raw
}

func preflight(r *runner, snapshots map[string][]byte) {
	need(runtime.GOOS == "darwin" || runtime.GOOS == "linux", "supported platforms: macOS/Linux")
	same(runtime.Version(), "go1.27.3", "oracle itself must be built with Go1.27.3")
	info, ok := debug.ReadBuildInfo()
	need(ok, "oracle build metadata required")
	found := false
	for _, d := range info.Deps {
		need(d.Replace == nil, "oracle dependency replacement unsupported")
		if d.Path == "golang.org/x/net" {
			need(!found && d.Version == "v0.59.0" && d.Sum == "h1:5zfYln+w5XCxwrnMMJPufRgNoXEaGxl0wo5GqPXyues=", "pinned actual x/net import required")
			found = true
		}
	}
	need(found, "actual pinned httpguts dependency metadata required")
	identity := object(decode(success(r.run("compiler-identity", r.compiler, r.root, "version", "--build", "--json"), "compiler-identity")))
	for k, v := range map[string]any{"schema": "gooo/build-identity/v1", "language": "gooo", "version": "0.6.37-dev", "compiler_source_sha": revision, "source_status": "CLEAN_VCS", "vcs_revision": revision, "vcs_modified": "false", "native_go_required": "go1.27.3", "go_version": "go1.27.3", "build_info_observed": true} {
		same(identity[k], v, "compiler identity: "+k)
	}
	mod := object(identity["module"])
	same(mod["path"], "github.com/kimjooyoon/meta-ontology-go", "compiler module")
	version := strings.TrimSpace(string(success(r.run("go-version", r.goTool, r.root, "version"), "go-version")))
	need(version == "go version go1.27.3 "+runtime.GOOS+"/"+runtime.GOARCH, "explicit Go tool must match oracle platform/version")
	r.goVersion = version
	goroot := strings.TrimSpace(string(success(r.run("go-root", r.goTool, r.root, "env", "GOROOT"), "go-root")))
	for name, hash := range map[string]string{"src/net/http/fs.go": "sha256:21660cb61d17c258de99172683f9b41aca37d45c0af0556723f2a11179bdc45e", "src/net/textproto/textproto.go": "sha256:e4f0d94752345b5dec4fe7f4805531e8036a9cd44131d807cafa700dd5323901"} {
		path := filepath.Join(goroot, name)
		raw := readFile(path, 1<<20)
		same(digest(raw), hash, "pinned actual Go oracle source: "+name)
		snapshots[path] = raw
	}
	modcache := strings.TrimSpace(string(success(r.run("go-module-cache", r.goTool, r.root, "env", "GOMODCACHE"), "go-module-cache")))
	need(filepath.IsAbs(modcache) && filepath.Clean(modcache) == modcache, "absolute clean actual Go module cache required")
	httpgutsSource := filepath.Join(modcache, "golang.org", "x", "net@v0.59.0", "http", "httpguts", "httplex.go")
	httpgutsRaw := readFile(httpgutsSource, 1<<20)
	same(digest(httpgutsRaw), "sha256:8cfd5819dc821d45f82fde10afff4b2e652535b0091e0559fc8ad7766051a2c2", "pinned actual x/net httpguts oracle source")
	snapshots[httpgutsSource] = httpgutsRaw
}

// Only public ABI and direct source/value bindings are checked here. There is
// no copy of the engine's semantic/plan canonical hash implementation and no
// fixed computes body, pure-call order, or private lowering-number allowlist.
func image(b batch, program map[string]any, sources map[string][]byte) string {
	same(program["schema"], "gooo/workspace-body-program/v1", "program schema")
	entry := object(program["entry"])
	same(entry["package_path"], b.Package, "program entry package")
	same(entry["activity"], b.Activity, "program entry activity")
	lower := text(entry["lowered_name"])
	need(lower != "", "lowered entry required")
	im := object(program["workspace_image"])
	same(im["schema"], "gooo/package-runtime-image/v1", "workspace image schema")
	ep := object(im["entry"])
	same(ep["package_path"], b.Package, "image entry package")
	same(ep["activity"], b.Activity, "image entry activity")
	src, out := coreFile, "ETagScanResult"
	if b.Activity == "ETagStrongMatch" {
		out = "Boolean"
	}
	if b.Package == app {
		src, out = appFile, "StrongValidatorFacts"
	}
	same(ep["source"], src, "entry source")
	same(ep["output"], out, "entry output")
	ins := []string{"Text"}
	if b.Kind == "strong_match" {
		ins = append(ins, "Text")
	}
	same(ep["inputs"], normalized(ins), "entry input types")
	wanted := map[string]string{core: coreFile}
	if b.Package == app {
		wanted[header] = headerFile
		wanted[app] = appFile
	}
	packs := array(im["packages"])
	need(len(packs) == len(wanted), "exact workspace package/source roster")
	seen := map[string]bool{}
	for _, v := range packs {
		p := object(v)
		path := text(p["path"])
		file, ok := wanted[path]
		need(ok && !seen[path], "unexpected/duplicate workspace package")
		seen[path] = true
		name, ns := "httpetag", "httpetag"
		if path == header {
			name, ns = "httpguts", "gooo_ports_x_net_httpguts"
		}
		if path == app {
			name, ns = "strongvalidator", "strongvalidator"
		}
		same(p["name"], name, "package name")
		same(p["namespace"], ns, "namespace")
		imports := []string{}
		if path == app {
			imports = []string{header, core}
		}
		got := []string{}
		if !empty(p["imports"]) {
			for _, v := range array(p["imports"]) {
				got = append(got, text(v))
			}
		}
		sort.Strings(got)
		same(got, imports, "exact import closure")
		ss := array(p["sources"])
		need(len(ss) == 1, "one current source per package")
		s := object(ss[0])
		same(s["filename"], file, "source filename")
		same(s["source_digest"], digest(encode(string(sources[file]))), "current JSON-string source digest")
		expected := map[string]any{}
		entity := func(n, id string) { expected[n] = map[string]any{"name": n, "kind": "entity", "id": id} }
		activity := func(n, id, output string, nin int) {
			inputs := []string{}
			for i := 0; i < nin; i++ {
				inputs = append(inputs, textID)
			}
			expected[n] = map[string]any{"name": n, "kind": "activity", "id": id, "input_types": inputs, "output_type": output}
		}
		entity("Text", textID)
		switch path {
		case core:
			entity("Boolean", boolID)
			entity("ETagScanResult", scanID)
			activity("ScanETag", "gooo://go/net/http/scan-etag", scanID, 1)
			activity("ETagStrongMatch", "gooo://go/net/http/etag-strong-match", boolID, 2)
		case header:
			entity("Boolean", boolID)
			activity("ValidHeaderFieldName", "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldName", boolID, 1)
			activity("ValidHeaderFieldValue", "gooo://golang.org/x/net/http/httpguts/ValidHeaderFieldValue", boolID, 1)
		case app:
			entity("StrongValidatorFacts", factsID)
			activity("Report", factsID+"/report", factsID, 1)
		}
		exports := array(p["exports"])
		need(len(exports) == len(expected), "fixed starting public ABI export count")
		used := map[string]bool{}
		for _, v := range exports {
			e := object(v)
			n := text(e["name"])
			need(!used[n] && expected[n] != nil, "ABI export name")
			used[n] = true
			same(e, normalized(expected[n]), "fixed public activity/entity ABI: "+n)
		}
	}
	order := array(im["init_order"])
	need(len(order) == len(wanted), "init roster")
	seen = map[string]bool{}
	for _, v := range order {
		p := text(v)
		_, ok := wanted[p]
		need(ok && !seen[p], "init package occurrence")
		if p == app {
			need(seen[core] && seen[header], "dependencies initialize before Report")
		}
		seen[p] = true
	}
	return lower
}
func records(b batch, plan map[string]any) {
	same(plan["schema"], "gooo/body-composition-plan/v1", "plan schema")
	seen := map[string]bool{}
	if !empty(plan["record_types"]) {
		for _, v := range array(plan["record_types"]) {
			rec := object(v)
			id := text(rec["id"])
			need(!seen[id], "record duplicate")
			seen[id] = true
			names, types, name := scanFields, []string{"string", "string"}, "ETagScanResult"
			if id == factsID {
				need(b.Package == app, "unexpected facts record")
				names, types, name = factsFields, factsTypes, "StrongValidatorFacts"
			} else {
				same(id, scanID, "unexpected record ABI")
			}
			same(rec["name"], name, "record name")
			fields := array(rec["fields"])
			need(len(fields) == len(names), "record field count")
			for i, v := range fields {
				f := object(v)
				same(f["name"], names[i], "ordered record field name")
				same(f["id"], id+"/"+names[i], "record field ID")
				same(f["type_id"], "urn:gooo:type:"+types[i], "record field type")
			}
		}
	}
	if b.Kind == "scanner" {
		need(seen[scanID], "scan record required")
	}
	if b.Kind == "facts" {
		need(seen[scanID] && seen[factsID], "Report and scan records required")
	}
}
func typedRecord(actual any, id string, fields []string, types []string) map[string]any {
	m := object(actual)
	need(len(m) == len(fields), "exact actual record field count")
	for i, n := range fields {
		v, ok := m[n]
		need(ok, "missing actual record field: "+n)
		switch types[i] {
		case "string":
			text(v)
		case "boolean":
			_, ok := v.(bool)
			need(ok, "Boolean required: "+n)
		case "integer":
			integer(v)
		}
	}
	return m
}
func processOK(v any) {
	p := object(v)
	for _, k := range []string{"started", "completed"} {
		same(p[k], true, "native process "+k)
	}
	for _, k := range []string{"canceled", "timed_out", "output_truncated"} {
		same(p[k], false, "native process "+k)
	}
	need(integer(p["exit_code"]) == 0 && integer(p["diagnostics_bytes"]) == 0, "native process exit/diagnostics")
}

func inspect(r *runner, b batch, raw []byte, rows []inputRow, want []any, nativeStdout []byte, sources map[string][]byte, manifestPath string, manifestRaw, casesRaw []byte, mutation bool) {
	receipt := object(decode(raw))
	same(receipt["schema"], "gooo/workspace-body-execution-receipt/v1", "receipt schema")
	need(receipt["replayed_from_sha256"] == nil, "fresh receipt cannot have saved replay parent")
	same(receipt["manifest"], manifestPath, "actual manifest path")
	same(receipt["manifest_digest"], digest(manifestRaw), "raw manifest binding")
	same(receipt["cases_digest"], digest(casesRaw), "fresh Go --cases binding")
	result := object(receipt["result"])
	need(result["replay"] == nil, "fresh execute required")
	program := object(result["program"])
	lower := image(b, program, sources)
	c := object(result["composition"])
	same(c["schema"], "gooo/body-composition/v1", "composition schema")
	same(c["stage"], "COMPLETE", "composition completes")
	plan := object(c["plan"])
	records(b, plan)
	rootFound := false
	for _, v := range array(plan["activities"]) {
		p := object(v)
		if p["id"] == b.ID {
			need(!rootFound, "one selected plan activity")
			rootFound = true
			same(p["name"], lower, "plan entry binding")
			same(p["input_entity_id"], textID, "plan Text input")
			output := scanID
			if b.Kind == "strong_match" {
				output = boolID
			}
			if b.Kind == "facts" {
				output = factsID
			}
			same(p["output_entity_id"], output, "plan output ABI")
		}
	}
	need(rootFound, "entry in plan")
	lowered := text(program["lowered_gooo_source"])
	sh := digest([]byte(lowered))
	same(result["execution_source_sha256"], sh, "lowered execution source")
	same(c["gooo_source"], lowered, "composition lowered source")
	rt := object(result["runtime"])
	same(rt["schema"], "gooo/body-composition-runtime/v1", "runtime schema")
	need(rt["failure"] == nil, "runtime failure cannot establish semantic mismatch")
	for _, k := range []string{"original_source_sha256", "selected_source_sha256"} {
		same(c[k], sh, "composition source: "+k)
		same(rt[k], sh, "runtime source: "+k)
	}
	for field, source := range map[string]string{"generated_sha256": "source", "driver_sha256": "driver_source"} {
		h := digest([]byte(text(c[source])))
		same(c[field], h, "actual generated/driver source hash")
		same(rt[field], h, "runtime generated/driver binding")
	}
	same(rt["typed_plan_sha256"], plan["typed_plan_sha256"], "runtime plan binding")
	same(rt["producer_source_sha"], revision, "runtime compiler producer")
	same(object(rt["producer_module"])["path"], "github.com/kimjooyoon/meta-ontology-go", "runtime producer module")
	same(rt["go_tool_selection"], "explicit", "explicit Go selection")
	same(rt["go_tool_sha256"], r.goSHA, "actual Go binary SHA")
	same(rt["go_version"], r.goVersion, "actual Go version")
	need(integer(rt["model_calls"]) == 0, "zero model calls")
	processOK(rt["toolchain"])
	processOK(rt["build"])
	runs := array(rt["runs"])
	need(len(runs) == 2, "two fresh internal native runs")
	for _, v := range runs {
		processOK(v)
	}
	traces := array(rt["traces"])
	need(len(traces) == len(rows), "all fixed batch traces present")
	witness := false
	mismatches := 0
	for i, v := range traces {
		tr := object(v)
		need(integer(tr["case_index"]) == int64(i), "ordered original case index")
		deliveries := array(tr["deliveries"])
		need(len(deliveries) == 1, "one entry delivery per row")
		d := object(deliveries[0])
		same(d["activity_id"], b.ID, "delivery activity ID")
		if len(rows[i].Inputs) == 1 {
			same(d["input"], rows[i].Inputs[0], "unchanged original Text input")
			need(empty(d["inputs"]) && empty(d["input_fields"]), "single original input must not become fields/tuple")
		} else {
			need(d["input"] == nil && empty(d["input_fields"]), "two-Text scalar alias forbidden")
			ins := array(d["inputs"])
			need(len(ins) == 2, "two ordered original inputs")
			for j, v := range ins {
				x := object(v)
				keys(x, "port", "entity_id", "value")
				same(x["port"], "input"+strconv.Itoa(j), "ordered input port")
				same(x["entity_id"], textID, "input Text entity")
				same(x["value"], rows[i].Inputs[j], "original strong operand")
			}
		}
		same(d["expected"], want[i], "receipt expectation must equal fresh independent Go value")
		actual := d["actual"]
		if b.Kind == "strong_match" {
			_, ok := actual.(bool)
			need(ok, "strong Boolean output")
			need(empty(d["actual_fields"]), "Boolean cannot have record fields")
		} else {
			id, names, types := scanID, scanFields, []string{"string", "string"}
			if b.Kind == "facts" {
				id, names, types = factsID, factsFields, factsTypes
			}
			record := typedRecord(actual, id, names, types)
			fields := array(d["actual_fields"])
			need(len(fields) == len(names), "typed actual_fields count")
			for j, v := range fields {
				x := object(v)
				keys(x, "id", "name", "value")
				same(x["name"], names[j], "actual field order")
				same(x["id"], id+"/"+names[j], "actual field ID")
				same(x["value"], record[names[j]], "actual field value/type mirror")
			}
		}
		equal := reflect.DeepEqual(actual, want[i])
		same(d["passed"], equal, "passed flag must reflect actual comparison")
		if !equal {
			mismatches++
			need(mutation && b.Activity == "Report" && i == 6 && rows[i].ID == "CRLF-trim-does-not-sanitize-header" && rows[i].Inputs[0] == "\r\n\"x\"\t\r\n", "unexpected actual mismatch at "+rows[i].ID)
			mutated := object(normalized(want[i]))
			same(mutated["single_strong"], false, "CRLF Go single_strong must be false")
			mutated["single_strong"] = true
			same(actual, mutated, "mutation witness must change only CRLF single_strong false to true")
			witness = true
		}
	}
	if mutation {
		need(witness && mismatches == 1, "typed CRLF mismatch required; process failure alone is insufficient")
		need(integer(rt["finite_total"]) == int64(len(rows)) && integer(rt["finite_passed"]) == int64(len(rows)-1), "mutation finite counters")
		need(receipt["decision"] != "PASS", "actual mutant mismatch cannot be PASS")
	} else {
		same(receipt["decision"], "PASS", "fresh receipt PASS")
		same(rt["stage"], "COMPLETE", "runtime completes")
		need(integer(rt["finite_total"]) == int64(len(rows)) && integer(rt["finite_passed"]) == int64(len(rows)), "all fresh native cases pass")
		same(rt["projection_replayed"], true, "internal projection repeat")
		same(rt["runtime_replayed"], true, "internal second fresh execution")
		for _, v := range runs {
			same(object(v)["stdout_sha256"], digest(nativeStdout), "independent Go ordered native stdout matrix")
		}
	}
}

func execute(r *runner, index int, b batch, workspace string, rows []inputRow, sources map[string][]byte, reportRaw []byte, mutation bool, reuseCases string) string {
	label := fmt.Sprintf("execute-%02d-%s", index, b.Activity)
	if mutation {
		label = "mutation-Report-first19"
	}
	manifestRaw := pretty(manifest(b))
	manifestName := b.Activity + ".workspace.json"
	if b.Package == app {
		manifestRaw = reportRaw
		manifestName = reportFile
	}
	manifestPath := filepath.Join(workspace, manifestName)
	if _, err := os.Lstat(manifestPath); errors.Is(err, os.ErrNotExist) {
		fresh(manifestPath, manifestRaw)
	} else {
		must(err)
		same(readFile(manifestPath, 1<<20), manifestRaw, "existing generated manifest differs")
	}
	caseRaw, want, nativeStdout := cases(b, rows, sources)
	casePath := filepath.Join(r.out, fmt.Sprintf("cases-%02d.json", index))
	if mutation {
		casePath = reuseCases
		same(readFile(casePath, 30000), caseRaw, "mutation must use identical fresh Go --cases bytes")
	} else {
		fresh(casePath, caseRaw)
	}
	p := r.run(label, r.compiler, workspace, "package", "execute", "--json", "--cases", casePath, "--go", r.goTool, manifestPath)
	if !mutation {
		success(p, label)
	}
	inspect(r, b, p.Raw, rows, want, nativeStdout, sources, manifestPath, manifestRaw, caseRaw, mutation)
	same(readFile(casePath, 30000), caseRaw, "cases unchanged after native command")
	same(readFile(manifestPath, 1<<20), manifestRaw, "manifest unchanged after native command")
	for name, raw := range sources {
		same(readFile(filepath.Join(workspace, name), 10<<20), raw, "workspace source unchanged after native command")
	}
	return casePath
}

func work() (code int) {
	compiler := flag.String("compiler", "", "absolute existing clean205896a Gooo compiler")
	goTool := flag.String("go", "", "absolute existing Go1.27.3 binary")
	out := flag.String("out", "", "new absolute output directory; existing paths refused")
	mutation := flag.Bool("mutation-check", false, "after247 pass, require CRLF mismatch in Report first19 only")
	flag.Parse()
	status := map[string]any{"schema": "gooo/portable-etag-check/v1", "decision": "FAIL", "baseline_rows": 0, "baseline_execute_commands": 0, "mutation_checked": false, "scope": "finite fresh native checks; no saved replay, inner canonical validation, full If-Match or authority/effects"}
	snapshots := map[string][]byte{}
	created := false
	var root string
	defer func() {
		if problem := recover(); problem != nil {
			status["first_failure"] = fmt.Sprint(problem)
			code = 1
		}
		unchanged := true
		for path, want := range snapshots {
			func() {
				defer func() {
					if problem := recover(); problem != nil {
						if _, exists := status["first_failure"]; !exists {
							status["first_failure"] = fmt.Sprint(problem)
						}
						status["decision"] = "FAIL"
						unchanged = false
						code = 1
					}
				}()
				same(readFile(path, 128<<20), want, "source/tool bytes changed during check: "+path)
			}()
		}
		status["source_tool_unchanged_after"] = unchanged
		if created {
			func() {
				defer func() {
					if problem := recover(); problem != nil {
						code = 1
						status["decision"] = "FAIL"
						saveFailure := "cannot retain summary.json: " + fmt.Sprint(problem)
						if first, exists := status["first_failure"]; exists {
							fmt.Fprintln(os.Stderr, "ETag check first failure:", first)
						} else {
							status["first_failure"] = saveFailure
						}
						fmt.Fprintln(os.Stderr, "ETag evidence write failure:", saveFailure)
					}
				}()
				fresh(filepath.Join(*out, "summary.json"), pretty(status))
			}()
		}
		if code != 0 {
			fmt.Fprintln(os.Stderr, "ETag check failed; inspect diagnostics and any retained evidence in", *out)
		} else {
			fmt.Println("247 original rows matched fresh Go expectations in7 native commands; evidence:", *out)
		}
	}()
	need(flag.NArg() == 0, "unexpected positional arguments")
	cwd, err := os.Getwd()
	must(err)
	root, err = filepath.EvalSymlinks(cwd)
	must(err)
	need(root == cwd, "run from real project directory")
	need(filepath.IsAbs(*out) && filepath.Clean(*out) == *out, "--out must be an absolute new directory")
	parent, err := filepath.EvalSymlinks(filepath.Dir(*out))
	must(err)
	need(parent == filepath.Dir(*out), "out parent must exist without symlinks")
	need(*out != root && !strings.HasPrefix(root, *out+string(os.PathSeparator)), "out cannot contain project")
	newDir(*out)
	created = true
	for _, name := range files {
		path := filepath.Join(root, name)
		snapshots[path] = readFile(path, 10<<20)
	}
	for _, path := range []string{*compiler, *goTool} {
		need(filepath.IsAbs(path), "compiler and Go must be explicit absolute existing paths")
		snapshots[path] = readFile(path, 128<<20)
		st, err := os.Stat(path)
		must(err)
		need(st.Mode()&0111 != 0, "tool must be executable")
	}
	sources := map[string][]byte{}
	hashes := map[string]string{}
	for _, name := range []string{coreFile, headerFile, appFile} {
		raw := snapshots[filepath.Join(root, name)]
		need(len(raw) > 0 && utf8.Valid(raw), "current source must be nonempty validUTF8")
		sources[name] = raw
		hashes[name] = digest(raw)
	}
	status["current_source_sha256"] = hashes
	status["compiler_sha256"] = digest(snapshots[*compiler])
	status["go_sha256"] = digest(snapshots[*goTool])
	status["compiler_revision"] = revision
	status["corpus_sha256"] = corpusSHA
	status["report_manifest_sha256"] = digest(snapshots[filepath.Join(root, reportFile)])
	rows := corpus(snapshots[filepath.Join(root, "corpus.json")])
	reportRaw := snapshots[filepath.Join(root, reportFile)]
	same(decode(reportRaw), normalized(manifest(batches[5])), "fixed Report manifest source/entry roster")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()
	r := &runner{ctx: ctx, out: *out, root: root, compiler: *compiler, goTool: *goTool, goSHA: digest(snapshots[*goTool])}
	preflight(r, snapshots)
	workspace := filepath.Join(*out, "workspace")
	copyWorkspace(workspace, sources)
	paths := []string{}
	for i, b := range batches {
		paths = append(paths, execute(r, i, b, workspace, rows[b.Kind][b.Start:b.End], sources, reportRaw, false, ""))
		status["baseline_execute_commands"] = i + 1
		status["baseline_rows"] = status["baseline_rows"].(int) + b.End - b.Start
	}
	need(status["baseline_execute_commands"] == 7 && status["baseline_rows"] == 247, "all247 in exactly7 native calls")
	if *mutation {
		old := []byte("let single_strong = header_valid && present && complete && strong")
		need(bytes.Count(sources[appFile], old) == 1, "optional mutation expects original single_strong expression; baseline may still be valid after other rewrites")
		mutated := map[string][]byte{}
		for n, v := range sources {
			mutated[n] = bytes.Clone(v)
		}
		mutated[appFile] = bytes.Replace(sources[appFile], old, []byte("let single_strong = present && complete && strong"), 1)
		mutantWorkspace := filepath.Join(*out, "mutation-workspace")
		copyWorkspace(mutantWorkspace, mutated)
		execute(r, 5, batches[5], mutantWorkspace, rows["facts"][:19], mutated, reportRaw, true, paths[5])
		status["mutation_checked"] = true
		status["mutation_rows"] = 19
		status["mutation_execute_commands"] = 1
		status["mutation_witness"] = "CRLF-trim-does-not-sanitize-header: single_strong false→true"
	}
	status["decision"] = "PASS"
	status["total_direct_commands"] = r.commands
	status["go_version"] = r.goVersion
	status["verified_returned_scalars"] = 727
	return 0
}
func main() { os.Exit(work()) }
