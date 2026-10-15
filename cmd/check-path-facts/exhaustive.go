package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	stdpath "path"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"text/template"
	"time"
)

//go:embed exhaustive_test.go.txt
var pathExhaustiveTemplate string

const pathCorpusSize = 9841
const pathRowsCap = 8 << 20
const pathSourceHash = "sha256:0a5024c364387eeea9ab5fe0b285f0d62355e8cd181cdbddfdfa227f6916d94c"
const pathCorpusProfile = "gooo/path-facts-generated-native9841/v1"

type pathRowsEvidence struct {
	Rows                int    `json:"rows"`
	ScalarComparisons   int    `json:"scalar_comparisons"`
	Bytes               int    `json:"bytes"`
	SHA256              string `json:"sha256"`
	OrderedInputsSHA256 string `json:"ordered_inputs_sha256"`
	OutputSHA256        string `json:"output_sha256"`
	OracleSHA256        string `json:"oracle_sha256"`
}

type pathHarnessConfig struct {
	ABI                                                                               pathGeneratedABI
	Entry, Activity, Generated, Execution, Replay, Manifest, Inputs, Compiler, GoTool string
}

func pathCorpusInputs() []string {
	// Independent of the harness's base-3 numeric generation.
	all := []string{""}
	level := []string{""}
	for length := 1; length <= 8; length++ {
		next := make([]string, 0, len(level)*3)
		for _, prefix := range level {
			for _, letter := range []string{"a", ".", "/"} {
				next = append(next, prefix+letter)
			}
		}
		all = append(all, next...)
		level = next
	}
	return all
}

func pathOracle(text string) report {
	ext := stdpath.Ext(text)
	return report{Text: text, Ext: ext, IsAbs: stdpath.IsAbs(text), TextByteLength: int64(len(text)), ExtByteLength: int64(len(ext)), HasExtension: len(ext) > 0}
}

func pathTypedValue(value any, want report) (any, error) {
	switch entryName {
	case "ext":
		s, ok := value.(string)
		if !ok || s != want.Ext {
			return nil, fmt.Errorf("extension type/value differs")
		}
		return s, nil
	case "isabs":
		b, ok := value.(bool)
		if !ok || b != want.IsAbs {
			return nil, fmt.Errorf("absolute type/value differs")
		}
		return b, nil
	case "report":
		m, ok := value.(map[string]any)
		if !ok || len(m) != 6 {
			return nil, fmt.Errorf("report must have six present fields")
		}
		values := []any{want.Text, want.Ext, want.IsAbs, want.TextByteLength, want.ExtByteLength, want.HasExtension}
		for i, name := range pathFieldNames {
			got, exists := m[name]
			if !exists || !equal(got, values[i]) {
				return nil, fmt.Errorf("report field %s type/value differs", name)
			}
		}
		return want, nil
	default:
		return nil, fmt.Errorf("unknown path entry")
	}
}

func pathHashLine(h io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = h.Write(append(raw, '\n'))
	return err
}

// Full typed row validation is mandatory even after a successful child/report.
// All counters and digests are derived from rows, never copied from its claim.
func inspectPathRows(raw []byte) (pathRowsEvidence, error) {
	var zero pathRowsEvidence
	if len(raw) == 0 || len(raw) > pathRowsCap || raw[len(raw)-1] != '\n' {
		return zero, fmt.Errorf("native rows size or terminal LF differs")
	}
	inputHash, outputHash, oracleHash := sha256.New(), sha256.New(), sha256.New()
	_, _ = io.WriteString(inputHash, "gooo/path-facts-ordered-input/v1\n")
	for _, h := range []io.Writer{outputHash, oracleHash} {
		_, _ = io.WriteString(h, "gooo/path-facts-native-values/v1\n"+activityID+"\n")
	}
	inputs := pathCorpusInputs()
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 65536)
	i := 0
	for scanner.Scan() {
		if i >= len(inputs) {
			return zero, fmt.Errorf("extra native corpus row")
		}
		line := scanner.Bytes()
		decoded, err := decode(line)
		if err != nil {
			return zero, err
		}
		row, ok := decoded.(map[string]any)
		if !ok || len(row) != 4 || !isInt(row["index"], i) || row["input"] != inputs[i] {
			return zero, fmt.Errorf("native corpus row %d index/input/roster differs", i)
		}
		want := pathOracle(inputs[i])
		actual, err := pathTypedValue(row["actual"], want)
		if err != nil {
			return zero, fmt.Errorf("row %d actual: %w", i, err)
		}
		oracle, err := pathTypedValue(row["oracle"], want)
		if err != nil {
			return zero, fmt.Errorf("row %d original Go oracle: %w", i, err)
		}
		canonical, err := json.Marshal(struct {
			Index  int    `json:"index"`
			Input  string `json:"input"`
			Actual any    `json:"actual"`
			Oracle any    `json:"oracle"`
		}{i, inputs[i], actual, oracle})
		if err != nil || !bytes.Equal(line, canonical) {
			return zero, fmt.Errorf("row %d must retain exact canonical typed JSON", i)
		}
		if err := pathHashLine(inputHash, inputs[i]); err != nil {
			return zero, err
		}
		if err := pathHashLine(outputHash, actual); err != nil {
			return zero, err
		}
		if err := pathHashLine(oracleHash, oracle); err != nil {
			return zero, err
		}
		i++
	}
	if err := scanner.Err(); err != nil {
		return zero, err
	}
	if i != pathCorpusSize {
		return zero, fmt.Errorf("native corpus omitted rows: %d", i)
	}
	comparisons := i
	if entryName == "report" {
		comparisons *= 6
	}
	e := pathRowsEvidence{Rows: i, ScalarComparisons: comparisons, Bytes: len(raw), SHA256: digest(raw), OrderedInputsSHA256: fmt.Sprintf("sha256:%x", inputHash.Sum(nil)), OutputSHA256: fmt.Sprintf("sha256:%x", outputHash.Sum(nil)), OracleSHA256: fmt.Sprintf("sha256:%x", oracleHash.Sum(nil))}
	if e.OutputSHA256 != e.OracleSHA256 {
		return zero, fmt.Errorf("native/oracle typed digests differ")
	}
	return e, nil
}

func makePathHarness(abi pathGeneratedABI, expected string, executionRaw, replayRaw []byte, id identity) ([]byte, error) {
	t, err := template.New("path-native").Parse(pathExhaustiveTemplate)
	if err != nil {
		return nil, err
	}
	config := pathHarnessConfig{abi, entryName, activityID, expected, digest(executionRaw), digest(replayRaw), id.Manifest, id.Inputs, compilerRevision, id.GoTool}
	var out bytes.Buffer
	err = t.Execute(&out, config)
	return out.Bytes(), err
}

func pathExclusive(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func pathFileBinding(path string, cap int64) (map[string]any, error) {
	s, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !s.Mode().IsRegular() || s.Size() > cap {
		return nil, fmt.Errorf("invalid evidence file %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "bytes": len(raw), "sha256": digest(raw)}, nil
}

func pathCheckFile(path, expected string, cap int64) error {
	ref, err := pathFileBinding(path, cap)
	if err != nil {
		return err
	}
	if ref["sha256"] != expected {
		return fmt.Errorf("source/tool changed: %s", path)
	}
	return nil
}

type pathLimitedWriter struct {
	mu         sync.Mutex
	file       *os.File
	count, cap int
	exceeded   bool
}

func (w *pathLimitedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) > w.cap-w.count {
		w.exceeded = true
		return 0, fmt.Errorf("native process output cap exceeded")
	}
	n, err := w.file.Write(p)
	w.count += n
	return n, err
}
func (w *pathLimitedWriter) overflow() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.exceeded }

func pathWorkBound(root, evidence string) error {
	var bytes int64
	for _, base := range []string{root, evidence} {
		if err := filepath.Walk(base, func(name string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in owned native work/evidence")
			}
			if info.Mode().IsRegular() {
				bytes += info.Size()
			}
			if bytes > 384<<20 {
				return fmt.Errorf("native owned work exceeds384MiB")
			}
			return nil
		}); err != nil {
			return err
		}
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(root, &fs); err != nil {
		return err
	}
	if uint64(fs.Bavail)*uint64(fs.Bsize) < 5<<30 {
		return fmt.Errorf("native requires5GiB free storage")
	}
	return nil
}

func pathNativeEnvironment(tmp, cache, gopath, rows, reportPath, root string) []string {
	// Closed environment: no inherited GOFLAGS, GOTOOLCHAIN, module proxies,
	// GOENV, GOWORK, credentials, Laya providers or user build caches.
	env := []string{"PATH=" + filepath.Join(runtime.GOROOT(), "bin"), "TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
		"GOROOT=" + runtime.GOROOT(), "GOCACHE=" + cache, "GOTMPDIR=" + tmp, "GOPATH=" + gopath, "GOMODCACHE=" + filepath.Join(gopath, "pkg/mod"),
		"GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0", "GOFLAGS=", "GOTELEMETRY=off",
		"GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "LANG=C", "LC_ALL=C",
		"GOOO_PATH_ROWS=" + rows, "GOOO_PATH_REPORT=" + reportPath, "GOOO_PATH_ROOT=" + root}
	if runtime.GOARCH == "amd64" {
		env = append(env, "GOAMD64=v1")
	}
	return env
}

func runPathNativeTest(ctx context.Context, goTool, native, tmp, cache, gopath, rowsPath, childReport, root, work, outputDir, testPattern string) (string, error) {
	stdoutPath, stderrPath := filepath.Join(outputDir, "native-stdout.txt"), filepath.Join(outputDir, "native-stderr.txt")
	out, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer out.Close()
	errs, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer errs.Close()
	stdout, stderr := &pathLimitedWriter{file: out, cap: 8 << 20}, &pathLimitedWriter{file: errs, cap: 1 << 20}
	childCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	args := []string{"test", "-count=1", "-timeout=90s", "-run", testPattern, "-v", "."}
	cmd := exec.CommandContext(childCtx, goTool, args...)
	cmd.Dir = native
	cmd.Env = pathNativeEnvironment(tmp, cache, gopath, rowsPath, childReport, root)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 3 * time.Second
	// Inherit the outer checker's owned process group. The outer runner kills and
	// reaps the entire group before cleaning work, including failure descendants.
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var runErr, errorBound error
	waiting := true
	for waiting {
		select {
		case runErr = <-done:
			waiting = false
		case <-ticker.C:
			if stdout.overflow() || stderr.overflow() {
				errorBound = fmt.Errorf("native output cap exceeded")
				cancel()
			}
			if err := pathWorkBound(work, outputDir); err != nil {
				errorBound = err
				cancel()
			}
		}
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if err := errs.Close(); err != nil {
		return "", err
	}
	process := map[string]any{"schema": "gooo/path-generated-native-process/v1", "argv": append([]string{goTool}, args...), "cwd": native, "started": true, "process_reaped": true, "exit_code": cmd.ProcessState.ExitCode(), "context_error": nil, "elapsed_seconds": time.Since(started).Seconds(), "timeout_seconds": 120, "test_timeout_seconds": 90, "automatic_retries": 0, "ownership": "inherited outer checker process group; outer runner records final group cleanup"}
	if childCtx.Err() != nil {
		process["context_error"] = childCtx.Err().Error()
	}
	if errorBound != nil {
		process["storage_or_output_error"] = errorBound.Error()
	}
	for name, path := range map[string]string{"stdout": stdoutPath, "stderr": stderrPath} {
		cap := int64(8 << 20)
		if name == "stderr" {
			cap = 1 << 20
		}
		ref, e := pathFileBinding(path, cap)
		if e != nil {
			return "", e
		}
		process[name] = ref
	}
	processRaw, err := json.MarshalIndent(process, "", "  ")
	if err != nil {
		return "", err
	}
	processRaw = append(processRaw, '\n')
	processPath := filepath.Join(outputDir, "native-process.json")
	if err := pathExclusive(processPath, processRaw); err != nil {
		return "", err
	}
	if runErr != nil || childCtx.Err() != nil || errorBound != nil || stdout.overflow() || stderr.overflow() {
		return "", fmt.Errorf("native child failed; work retained for outer group cleanup: run=%v context=%v bound=%v", runErr, childCtx.Err(), errorBound)
	}
	return processPath, nil
}

func checkGeneratedCorpus(ctx context.Context, root, outputDir string, executionRaw, replayRaw []byte, expectedGenerated string, id identity) (map[string]any, error) {
	if runtime.Version() != goVersion || (runtime.GOOS != "linux" && runtime.GOOS != "darwin") {
		return nil, fmt.Errorf("native corpus requires pinned Go1.27.2 on linux/darwin")
	}
	source, abi, err := inspectPathGenerated(executionRaw, replayRaw, expectedGenerated, id)
	if err != nil {
		return nil, err
	}
	goTool := filepath.Join(runtime.GOROOT(), "bin/go")
	goSource := filepath.Join(runtime.GOROOT(), "src/path/path.go")
	if err := pathCheckFile(goTool, id.GoTool, 100<<20); err != nil {
		return nil, err
	}
	if err := pathCheckFile(goSource, pathSourceHash, 1<<20); err != nil {
		return nil, err
	}
	harness, err := makePathHarness(abi, expectedGenerated, executionRaw, replayRaw, id)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(outputDir))
	if err != nil || parent != filepath.Dir(outputDir) {
		return nil, fmt.Errorf("evidence parent must exist without symlinks")
	}
	if err := os.Mkdir(outputDir, 0700); err != nil {
		return nil, fmt.Errorf("native corpus needs new evidence directory: %w", err)
	}
	native := filepath.Join(outputDir, "native")
	if err := os.Mkdir(native, 0700); err != nil {
		return nil, err
	}
	for name, content := range map[string][]byte{"generated.go": []byte(source), "exhaustive_test.go": harness, "go.mod": []byte("module gooo-path-native\n\ngo 1.27.2\n")} {
		if err := pathExclusive(filepath.Join(native, name), content); err != nil {
			return nil, err
		}
	}
	// TMPDIR is supplied by the outer owned runner. No global fallback is used.
	tmpParent := os.Getenv("TMPDIR")
	if tmpParent == "" || !filepath.IsAbs(tmpParent) {
		return nil, fmt.Errorf("require absolute owned TMPDIR")
	}
	realTmp, err := filepath.EvalSymlinks(tmpParent)
	if err != nil || realTmp != filepath.Clean(tmpParent) {
		return nil, fmt.Errorf("owned TMPDIR must have no symlink components")
	}
	work, err := os.MkdirTemp(tmpParent, "gooo-path-native-")
	if err != nil {
		return nil, err
	}
	ownedInfo, err := os.Lstat(work)
	if err != nil {
		return nil, err
	}
	cache, tmp, gopath := filepath.Join(work, "cache"), filepath.Join(work, "tmp"), filepath.Join(work, "gopath")
	for _, dir := range []string{cache, tmp, gopath} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}
	if err := pathWorkBound(work, outputDir); err != nil {
		return nil, err
	}
	rowsPath, childReport := filepath.Join(outputDir, "rows.jsonl"), filepath.Join(outputDir, "native-report.json")
	processPath, err := runPathNativeTest(ctx, goTool, native, tmp, cache, gopath, rowsPath, childReport, root, work, outputDir, "^TestPathGeneratedCorpus$")
	if err != nil {
		return nil, err
	}
	for path, want := range map[string]string{filepath.Join(native, "generated.go"): expectedGenerated, filepath.Join(native, "exhaustive_test.go"): digest(harness), goTool: id.GoTool, goSource: pathSourceHash} {
		cap := int64(100 << 20)
		if err := pathCheckFile(path, want, cap); err != nil {
			return nil, err
		}
	}
	ref, err := pathFileBinding(rowsPath, pathRowsCap)
	if err != nil {
		return nil, err
	}
	rows, err := os.ReadFile(rowsPath)
	if err != nil {
		return nil, err
	}
	evidence, err := inspectPathRows(rows)
	if err != nil {
		return nil, err
	}
	childRaw, err := os.ReadFile(childReport)
	if err != nil || len(childRaw) > 65536 {
		return nil, fmt.Errorf("missing/oversized child report: %v", err)
	}
	childDecoded, err := decode(childRaw)
	if err != nil {
		return nil, err
	}
	want := pathExpectedChildReport(expectedGenerated, executionRaw, replayRaw, id, evidence)
	want["harness_sha256"] = digest(harness)
	if err := inspectPathChildReport(childDecoded, want); err != nil {
		return nil, err
	}
	if err := pathWorkBound(work, outputDir); err != nil {
		return nil, err
	}
	current, err := os.Lstat(work)
	if err != nil || !os.SameFile(ownedInfo, current) || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("owned native work changed; cleanup refused")
	}
	stat, ok := current.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("owned work inode identity unavailable")
	}
	result := map[string]any{"schema": "gooo/path-facts-generated-native-checked/v1", "decision": "FINITE_GENERATED_NATIVE9841_ORIGINAL_GO_PASS", "profile": pathCorpusProfile, "entry": entryName, "activity_id": activityID, "input_count": pathCorpusSize, "generated_native_outputs": pathCorpusSize, "scalar_comparisons": evidence.ScalarComparisons, "mismatches": 0, "full_cli_transport_outputs": 0, "full_saved_replay_outputs": 0, "execution_sha256": digest(executionRaw), "replay_sha256": digest(replayRaw), "generated_sha256": expectedGenerated, "harness_sha256": digest(harness), "manifest_digest": id.Manifest, "inputs_digest": id.Inputs, "source_bindings": id.Sources, "compiler_revision": compilerRevision, "go_version": goVersion, "go_tool_sha256": id.GoTool, "go_path_source_sha256": pathSourceHash, "rows": evidence, "rows_file": ref, "owned_native_work": map[string]any{"path": work, "device": stat.Dev, "inode": stat.Ino}, "owned_native_work_removed": false, "cleanup_owner": "outer runner after checker and descendant process group is gone", "child_executable_digest_scope": "self-reported by fresh Go test child; format checked, not independently rehashed by parent", "scope": "All ordered a,dot,slash strings of lengths0..8 evaluated by freshly generated native bodies; full9841 CLI transport and individual saved replay counts remain0. Fresh manual24 CLI/replay checked separately. No full Text domain or filesystem authorization claim."}
	result["comparison_count"], result["scalar_comparison_count"] = pathCorpusSize, evidence.ScalarComparisons
	result["rows_sha256"], result["rows_file_bytes"], result["rows_path"] = evidence.SHA256, evidence.Bytes, rowsPath
	for key, path := range map[string]string{"generated_file": filepath.Join(native, "generated.go"), "harness_file": filepath.Join(native, "exhaustive_test.go"), "process_file": processPath, "native_report_file": childReport, "go_path_source_file": goSource} {
		ref, err := pathFileBinding(path, 1<<20)
		if err != nil {
			return nil, err
		}
		result[key] = ref
	}
	toolRef, err := pathFileBinding(goTool, 100<<20)
	if err != nil {
		return nil, err
	}
	result["go_tool_file"] = toolRef
	return result, nil
}

func inspectPathChildReport(decoded any, expected map[string]any) error {
	got, ok := decoded.(map[string]any)
	if !ok || !validDigest(got["executable_sha256"]) {
		return fmt.Errorf("child executable observation missing")
	}
	want := map[string]any{}
	for k, v := range expected {
		want[k] = v
	}
	want["executable_sha256"] = got["executable_sha256"]
	if !equal(decoded, want) {
		return fmt.Errorf("native report fields, identities or row-derived counters differ")
	}
	return nil
}

func pathExpectedChildReport(generated string, executionRaw, replayRaw []byte, id identity, e pathRowsEvidence) map[string]any {
	return map[string]any{"schema": "gooo/path-facts-generated-native-report/v1", "profile": pathCorpusProfile, "entry": entryName, "activity_id": activityID, "alphabet": []string{"a", ".", "/"}, "length_min": 0, "length_max": 8, "input_count": pathCorpusSize, "generated_native_outputs": pathCorpusSize, "scalar_comparisons": e.ScalarComparisons, "mismatches": 0, "full_cli_transport_outputs": 0, "full_saved_replay_outputs": 0, "compiler_revision": compilerRevision, "go_version": goVersion, "generated_sha256": generated, "execution_sha256": digest(executionRaw), "replay_sha256": digest(replayRaw), "manifest_digest": id.Manifest, "inputs_digest": id.Inputs, "go_tool_sha256": id.GoTool, "go_path_source_sha256": pathSourceHash, "rows": e}
}
