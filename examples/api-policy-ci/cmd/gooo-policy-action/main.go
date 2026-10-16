// This host transports facts and CI status. The imported generated Gooo owns
// original/candidate decisions, all fifteen difference flags and classification.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
	original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

const revisionVersion = "v0.0.0-20261014171906-e723f9af6dc3"
const originalVersion = "v0.0.0-20261014100709-8cddcc4e65af"

type moduleIdentity struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

func main() {
	status, code := run()
	if code != 0 {
		// Closed labels only. Do not print input values, provider credentials,
		// dependency errors containing caller data, or whole reports to CI logs.
		fmt.Fprintln(os.Stderr, "Gooo policy action:", status)
	}
	os.Exit(code)
}

func compiledModules() ([]moduleIdentity, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.GoVersion != "go1.27.2" {
		return nil, errors.New("compiled toolchain mismatch")
	}
	want := map[string]moduleIdentity{
		"github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision": {Path: "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision", Version: revisionVersion, Sum: "h1:0KwF9VosVIxM3+aodSheEPbv++9eHfVMSB7OErfiwf8="},
		"github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy":      {Path: "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy", Version: originalVersion, Sum: "h1:fvrka7xThFrx1JkbTEp+lZ7hjcUsK4csF/XbpmF4VuM="},
	}
	actual := make(map[string]moduleIdentity)
	for _, dependency := range info.Deps {
		if expected, exists := want[dependency.Path]; exists {
			if dependency.Replace != nil || dependency.Version != expected.Version || dependency.Sum != expected.Sum {
				return nil, errors.New("compiled dependency mismatch")
			}
			actual[dependency.Path] = expected
		}
	}
	if len(actual) != 2 {
		return nil, errors.New("compiled dependency unavailable")
	}
	return []moduleIdentity{actual["github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"], actual["github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"]}, nil
}

func workspaceInput(workspace, name string, limit int64) ([]byte, os.FileInfo, error) {
	if name == "" || filepath.IsAbs(name) || !filepath.IsLocal(name) || filepath.Clean(name) != name || strings.ContainsAny(name, "\x00\r\n") {
		return nil, nil, errors.New("canonical workspace-relative input required")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	parts := strings.Split(name, string(filepath.Separator))
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, errors.New("symlink input refused")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, nil, errors.New("non-directory input component")
		}
	}
	// Root confines resolution to the workspace. NONBLOCK avoids waiting on a
	// FIFO swapped in between inspection and open; f.Stat still requires regular.
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, nil, errors.New("input outside regular-file bound")
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, nil, errors.New("input byte cap")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(len(raw)) != after.Size() {
		return nil, nil, errors.New("input changed during read")
	}
	current, err := root.Stat(name)
	if err != nil || !os.SameFile(after, current) {
		return nil, nil, errors.New("input path changed during read")
	}
	return raw, after, nil
}

func saveNew(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	n, err := file.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closed := file.Close()
	if err != nil {
		return err
	}
	return closed
}

type outputFile struct {
	file *os.File
	path string
	info os.FileInfo
	size int64
}

func within(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && (relative == "." || filepath.IsLocal(relative))
}

func canonicalFile(path string) (os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
		return nil, errors.New("canonical absolute output required")
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("output path component refused")
		}
		if current != path && !info.IsDir() {
			return nil, errors.New("output parent refused")
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("output regular file required")
	}
	return info, nil
}

func singleOwnedFile(info os.FileInfo) bool {
	value, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && value.Nlink == 1 && int(value.Uid) == os.Geteuid()
}

func openOutput(path, temporary, workspace, work string) (*outputFile, error) {
	// Normal runner output is an existing file under RUNNER_TEMP, separate from
	// both caller input storage and this action's fresh result/cache directory.
	if !within(path, temporary) || within(path, workspace) || within(path, work) {
		return nil, errors.New("output location refused")
	}
	before, err := canonicalFile(path)
	if err != nil || !singleOwnedFile(before) {
		return nil, errors.New("output preflight refused")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !singleOwnedFile(opened) || !os.SameFile(before, opened) || before.Size() != opened.Size() {
		_ = file.Close()
		return nil, errors.New("output changed during preflight")
	}
	result := &outputFile{file: file, path: path, info: opened, size: opened.Size()}
	if err := result.check(nil); err != nil {
		_ = file.Close()
		return nil, err
	}
	return result, nil
}

func (output *outputFile) check(forbidden []os.FileInfo) error {
	actual, err := output.file.Stat()
	if err != nil || !singleOwnedFile(actual) || !os.SameFile(output.info, actual) || actual.Size() != output.size {
		return errors.New("output descriptor changed")
	}
	named, err := canonicalFile(output.path)
	if err != nil || !singleOwnedFile(named) || !os.SameFile(actual, named) || named.Size() != output.size {
		return errors.New("output name changed")
	}
	for _, info := range forbidden {
		if info == nil || os.SameFile(actual, info) {
			return errors.New("output aliases protected input or result")
		}
	}
	return nil
}

func (output *outputFile) append(values [][2]string, forbidden []os.FileInfo) error {
	var buffer bytes.Buffer
	for _, pair := range values {
		if strings.ContainsAny(pair[0]+pair[1], "\x00\r\n") {
			return errors.New("output line invalid")
		}
		fmt.Fprintf(&buffer, "%s=%s\n", pair[0], pair[1])
	}
	if buffer.Len() > 4096 {
		return errors.New("output byte cap")
	}
	if err := output.check(forbidden); err != nil {
		return err
	}
	n, err := output.file.Write(buffer.Bytes())
	if err == nil && n != buffer.Len() {
		err = io.ErrShortWrite
	}
	output.size += int64(n)
	if err == nil {
		err = output.file.Sync()
	}
	if err != nil {
		return err
	}
	return output.check(forbidden)
}

func run() (string, int) {
	if runtime.GOOS != "linux" {
		return "UNSUPPORTED_RUNNER", 2
	}
	mode, input := os.Getenv("GOOO_ACTION_MODE"), os.Getenv("GOOO_ACTION_INPUT")
	fail := os.Getenv("GOOO_ACTION_FAIL_NEWLY_DENIED")
	if (mode != "compare" && mode != "replay") || (fail != "true" && fail != "false") {
		return "INPUT_CONFIGURATION_INVALID", 2
	}
	workspace, err := filepath.EvalSymlinks(os.Getenv("GITHUB_WORKSPACE"))
	if err != nil {
		return "WORKSPACE_UNAVAILABLE", 2
	}
	work, err := filepath.EvalSymlinks(os.Getenv("GOOO_ACTION_WORK"))
	if err != nil {
		return "PRIVATE_WORK_UNAVAILABLE", 2
	}
	info, err := os.Stat(work)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return "PRIVATE_WORK_REFUSED", 2
	}
	temporary, err := filepath.EvalSymlinks(os.Getenv("RUNNER_TEMP"))
	if err != nil || !within(work, temporary) || within(work, workspace) || within(workspace, work) {
		return "PRIVATE_WORK_REFUSED", 2
	}
	outputs, err := openOutput(os.Getenv("GITHUB_OUTPUT"), temporary, workspace, work)
	if err != nil {
		return "ACTION_OUTPUT_PREFLIGHT_REFUSED", 2
	}
	defer outputs.file.Close()
	modules, err := compiledModules()
	if err != nil {
		return "COMPILED_SOURCE_IDENTITY_REFUSED", 2
	}
	limit := int64(revision.MaxInputBytes)
	if mode == "replay" {
		limit = int64(revision.MaxReportBytes)
	}
	raw, inputInfo, err := workspaceInput(workspace, input, limit)
	if err != nil {
		return "INPUT_FILE_REFUSED", 2
	}
	if err := outputs.check([]os.FileInfo{inputInfo}); err != nil {
		return "ACTION_OUTPUT_INPUT_ALIAS_REFUSED", 2
	}
	var report revision.ComparisonReport
	if mode == "compare" {
		facts, err := original.DecodeAPI(raw)
		if err != nil {
			return "FACTS_INVALID", 2
		}
		request, err := revision.Baseline(facts)
		if err != nil {
			return "BASELINE_REFUSED", 2
		}
		report, err = revision.Compare(request)
		if err != nil {
			return "COMPARISON_REFUSED", 2
		}
	} else {
		saved, err := revision.DecodeComparisonReport(raw)
		if err != nil {
			return "REPORT_INVALID", 2
		}
		report, err = revision.VerifyComparison(saved)
		if err != nil {
			return "REPLAY_REFUSED", 2
		}
	}
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil || len(output)+1 > revision.MaxReportBytes {
		return "REPORT_ENCODING_REFUSED", 2
	}
	output = append(output, '\n')
	// Check transport and complete saved report through the published verifier.
	decoded, err := revision.DecodeComparisonReport(output)
	if err != nil {
		return "SAVED_TRANSPORT_REFUSED", 2
	}
	if _, err := revision.VerifyComparison(decoded); err != nil {
		return "SAVED_REPLAY_REFUSED", 2
	}
	directory := filepath.Join(work, "result")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return "RESULT_DIRECTORY_EXISTS_OR_REFUSED", 2
	}
	reportPath, receiptPath := filepath.Join(directory, "comparison.json"), filepath.Join(directory, "receipt.json")
	if err := saveNew(reportPath, output); err != nil {
		return "REPORT_SAVE_FAILED", 2
	}
	inputSHA, reportSHA := fmt.Sprintf("%x", sha256.Sum256(raw)), fmt.Sprintf("%x", sha256.Sum256(output))
	receipt := struct {
		Schema       string           `json:"schema"`
		Mode         string           `json:"mode"`
		InputSHA256  string           `json:"input_sha256"`
		InputBytes   int              `json:"input_bytes"`
		ReportSHA256 string           `json:"report_sha256"`
		ReportBytes  int              `json:"report_bytes"`
		Modules      []moduleIdentity `json:"compiled_modules"`
		Go           string           `json:"compiled_go"`
		OS           string           `json:"compiled_os"`
		Arch         string           `json:"compiled_arch"`
		SavedReplay  string           `json:"saved_replay"`
		Scope        string           `json:"scope"`
	}{Schema: "gooo/api-policy-action-observation/v1", Mode: mode, InputSHA256: inputSHA, InputBytes: len(raw), ReportSHA256: reportSHA, ReportBytes: len(output), Modules: modules, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, SavedReplay: "COMPLETE_TYPED_REPORT_RECOMPUTED", Scope: "Supplied facts, two sealed profiles, unsigned consistency. No current authority, remote action, human approval gate or arbitrary policy support."}
	receiptJSON, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil || len(receiptJSON) > 65536 {
		return "RECEIPT_ENCODING_FAILED", 2
	}
	if err := saveNew(receiptPath, append(receiptJSON, '\n')); err != nil {
		return "RECEIPT_SAVE_FAILED", 2
	}
	reportInfo, reportErr := os.Stat(reportPath)
	receiptInfo, receiptErr := os.Stat(receiptPath)
	if reportErr != nil || receiptErr != nil {
		return "RESULT_IDENTITY_UNAVAILABLE", 2
	}
	currentInput, currentInfo, err := workspaceInput(workspace, input, limit)
	if err != nil || !os.SameFile(inputInfo, currentInfo) || !bytes.Equal(raw, currentInput) {
		return "INPUT_CHANGED_BEFORE_OUTPUT", 2
	}
	values := [][2]string{{"classification", report.Summary.Classification}, {"changed-count", strconv.Itoa(report.Summary.ChangedCount)}, {"report-path", reportPath}, {"receipt-path", receiptPath}, {"report-sha256", reportSHA}}
	if err := outputs.append(values, []os.FileInfo{inputInfo, currentInfo, reportInfo, receiptInfo}); err != nil {
		return "ACTION_OUTPUT_SAVE_FAILED", 2
	}
	currentInput, currentInfo, err = workspaceInput(workspace, input, limit)
	if err != nil || !os.SameFile(inputInfo, currentInfo) || !bytes.Equal(raw, currentInput) {
		return "INPUT_CHANGED_AFTER_OUTPUT", 2
	}
	// Only fixed computed labels/count/digest reach stdout; raw facts stay private.
	safe, _ := json.Marshal(struct {
		Classification string `json:"classification"`
		ChangedCount   int    `json:"changed_count"`
		ReportSHA256   string `json:"report_sha256"`
	}{Classification: report.Summary.Classification, ChangedCount: report.Summary.ChangedCount, ReportSHA256: reportSHA})
	fmt.Println(string(safe))
	if fail == "true" && report.Summary.Classification == "NEWLY_DENIED" {
		return "REQUESTED_CI_NEWLY_DENIED_FAILURE", 3
	}
	return "COMPARISON_OR_REPLAY_SAVED", 0
}
