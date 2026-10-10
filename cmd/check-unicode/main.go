// Check the unchanged generated importing consumer against the pinned Go oracle.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const manifestPath = "identifier-rune.workspace.json"
const entryKey = "app/identifier-rune:Classify"
const entryID = "gooo://go-ports/examples/identifier-rune/classify"
const replyID = "gooo://go-ports/examples/identifier-rune/classification"
const letterID = "gooo://go/unicode/IsLetter"
const digitID = "gooo://go/unicode/IsDigit"

type lockfile struct {
	GoVersion        string            `json:"go_version"`
	CompilerRevision string            `json:"compiler_revision"`
	CompilerVersion  string            `json:"compiler_version"`
	Sources          map[string]string `json:"source_sha256"`
}
type buildIdentity struct {
	Schema   string `json:"schema"`
	Revision string `json:"compiler_source_sha"`
	Version  string `json:"version"`
	Go       string `json:"go_version"`
	Native   string `json:"native_go_required"`
	Modified string `json:"vcs_modified"`
	Status   string `json:"source_status"`
}
type sourcePackage struct {
	Path    string   `json:"path"`
	Sources []string `json:"sources"`
}
type workspace struct {
	Schema string `json:"schema"`
	Entry  struct {
		Package  string `json:"package_path"`
		Activity string `json:"activity"`
	} `json:"entry"`
	Packages []sourcePackage `json:"packages"`
}
type classification struct {
	InRuneDomain   bool `json:"in_rune_domain"`
	CanStart       bool `json:"can_start"`
	CanContinue    bool `json:"can_continue"`
	IsDecimalDigit bool `json:"is_decimal_digit"`
}
type finiteCase struct {
	Inputs   map[string]int64          `json:"inputs"`
	Expected map[string]classification `json:"expected"`
}
type caseDocument struct {
	Schema string       `json:"schema"`
	Cases  []finiteCase `json:"cases"`
}
type sourceBinding struct{ Package, Digest string }
type evidenceBinding struct {
	Lock                        lockfile
	ManifestDigest, CasesDigest string
	Sources                     map[string]sourceBinding
	Packages                    map[string]bool
	Cases                       []finiteCase
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	out := flag.String("out", "", "fresh evidence directory beneath ignored out/")
	compiler := flag.String("compiler", "./.gooo", "pinned compiler executable")
	flag.Parse()
	if *out == "" || flag.NArg() != 0 {
		return fmt.Errorf("provide --out for a fresh directory beneath out/")
	}
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	outRoot, err := filepath.Abs("out")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(outRoot, dir)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("--out must be beneath ignored out/")
	}
	var lock lockfile
	if err := readJSON("upstream.json", &lock); err != nil {
		return err
	}
	if runtime.Version() != lock.GoVersion || lock.GoVersion != "go1.27.2" || unicode.Version != "17.0.0" {
		return fmt.Errorf("require pinned Go 1.27.2 / Unicode 17.0.0 oracle")
	}
	for _, name := range []string{"src/unicode/graphic.go", "src/unicode/letter.go", "src/unicode/digit.go", "src/unicode/tables.go"} {
		raw, err := os.ReadFile(filepath.Join(runtime.GOROOT(), name))
		if err != nil {
			return err
		}
		if lock.Sources[name] == "" || digest(raw) != lock.Sources[name] {
			return fmt.Errorf("oracle source hash differs: %s", name)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return fmt.Errorf("fresh output directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	raw, err := command(ctx, filepath.Join(dir, "compiler.json"), "", nil, *compiler, "version", "--build", "--json")
	if err != nil {
		return err
	}
	if err := checkCompiler(raw, lock); err != nil {
		return err
	}
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var ws workspace
	if err := json.Unmarshal(manifest, &ws); err != nil {
		return err
	}
	if ws.Schema != "gooo/package-workspace-manifest/v1" || ws.Entry.Package != "app/identifier-rune" || ws.Entry.Activity != "Classify" {
		return fmt.Errorf("unexpected manifest entry")
	}
	work := filepath.Join(dir, "workspace")
	bind := evidenceBinding{Lock: lock, ManifestDigest: sha(manifest), Sources: map[string]sourceBinding{}, Packages: map[string]bool{}}
	sourceHashes := map[string]string{}
	for _, p := range ws.Packages {
		if p.Path == "" || bind.Packages[p.Path] || len(p.Sources) == 0 {
			return fmt.Errorf("invalid manifest package roster")
		}
		bind.Packages[p.Path] = true
		for _, name := range p.Sources {
			if !filepath.IsLocal(name) {
				return fmt.Errorf("non-local source %q", name)
			}
			if _, exists := bind.Sources[name]; exists {
				return fmt.Errorf("duplicate manifest source %s", name)
			}
			b, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(string(b))
			if err != nil {
				return err
			}
			bind.Sources[name] = sourceBinding{p.Path, sha(encoded)} // Package images hash the JSON string.
			sourceHashes[name] = digest(b)
			if err := write(filepath.Join(work, name), b); err != nil {
				return err
			}
		}
	}
	if err := write(filepath.Join(work, manifestPath), manifest); err != nil {
		return err
	}
	cases, err := finiteCases()
	if err != nil {
		return err
	}
	caseRaw, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		return err
	}
	caseRaw = append(caseRaw, '\n')
	bind.Cases, bind.CasesDigest = cases.Cases, sha(caseRaw)
	casePath, workspacePath := filepath.Join(work, "cases.json"), filepath.Join(work, manifestPath)
	if err := write(casePath, caseRaw); err != nil {
		return err
	}
	executionPath := filepath.Join(dir, "execution.json")
	raw, err = command(ctx, executionPath, "", nil, *compiler, "package", "execute", "--json", "--cases", casePath, workspacePath)
	if err != nil {
		return err
	}
	generated, names, err := inspectReceipt(raw, bind, "")
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	receiptDigest := sha(raw)
	replay, err := command(ctx, filepath.Join(dir, "replay.json"), "", nil, *compiler, "package", "replay", "--json", "--receipt", executionPath, "--cases", casePath, workspacePath)
	if err != nil {
		return err
	}
	replayed, replayNames, err := inspectReceipt(replay, bind, receiptDigest)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	if generated != replayed || names != replayNames {
		return fmt.Errorf("saved replay generated program differs")
	}
	native := filepath.Join(dir, "native")
	if err := write(filepath.Join(native, "go.mod"), []byte("module gooo-unicode-check\n\ngo 1.27.2\n")); err != nil {
		return err
	}
	// Preserve every byte, including the package clause and generated helper bodies.
	if err := write(filepath.Join(native, "generated.go"), []byte(generated)); err != nil {
		return err
	}
	harness, err := makeHarness(names)
	if err != nil {
		return err
	}
	if err := write(filepath.Join(native, "exhaustive_test.go"), harness); err != nil {
		return err
	}
	reportPath := filepath.Join(dir, "native-exhaustive.json")
	if _, err := command(ctx, filepath.Join(dir, "native-test.txt"), native, []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "CHECK_UNICODE_REPORT=" + reportPath}, filepath.Join(runtime.GOROOT(), "bin/go"), "test", "-count=1", "-run", "^TestExhaustiveUnicode$", "-v", "."); err != nil {
		return err
	}
	var exhaustive exhaustiveReport
	if err := readJSON(reportPath, &exhaustive); err != nil {
		return err
	}
	if err := validateExhaustive(exhaustive); err != nil {
		return err
	}
	// Detect accidental changes to the copied program after execution as well.
	onDisk, err := os.ReadFile(filepath.Join(native, "generated.go"))
	if err != nil {
		return err
	}
	if !bytes.Equal(onDisk, []byte(generated)) {
		return fmt.Errorf("generated source changed")
	}
	result := map[string]any{"schema": "gooo/unicode-exhaustive-check/v1", "go_version": runtime.Version(), "unicode_version": unicode.Version, "compiler_revision": lock.CompilerRevision, "source_sha256": sourceHashes, "manifest_sha256": strings.TrimPrefix(bind.ManifestDigest, "sha256:"), "cases_sha256": strings.TrimPrefix(bind.CasesDigest, "sha256:"), "generated_sha256": digest(onDisk), "execution_sha256": strings.TrimPrefix(receiptDigest, "sha256:"), "replay_sha256": digest(replay), "harness_sha256": digest(harness), "finite_receipt_cases": len(cases.Cases), "finite_saved_replay_equal": true, "native": exhaustive, "scope": "Finite package execution and saved replay are separate from the exhaustive native harness. The harness invokes the unmodified generated IsLetter/IsDigit helpers and importing Classify entry for every integer 0..0x10FFFF, including surrogates. Other int32 and outside-int32 Integer observations are finite. No whole-string identifier, normalization, security, whole-library, all-int64, or performance claim."}
	if err := save(filepath.Join(dir, "summary.json"), result); err != nil {
		return err
	}
	fmt.Printf("%d finite receipt cases and identical saved replay; %d Unicode inputs, %d port comparisons, %d consumer field comparisons; no mismatches\n", len(cases.Cases), exhaustive.UnicodeInputs, exhaustive.PortComparisons, exhaustive.ConsumerComparisons)
	return nil
}

func checkCompiler(raw []byte, lock lockfile) error {
	var b buildIdentity
	if err := json.Unmarshal(raw, &b); err != nil {
		return err
	}
	if b.Schema != "gooo/build-identity/v1" || b.Revision != lock.CompilerRevision || b.Version != lock.CompilerVersion || b.Go != lock.GoVersion || b.Native != lock.GoVersion || b.Modified != "false" || b.Status != "CLEAN_VCS" {
		return fmt.Errorf("compiler identity differs from upstream.json")
	}
	return nil
}
func finiteCases() (caseDocument, error) {
	var inputs struct {
		Schema string             `json:"schema"`
		Inputs []map[string]int64 `json:"inputs"`
	}
	if err := readJSON("examples/identifier-rune/inputs.json", &inputs); err != nil {
		return caseDocument{}, err
	}
	if inputs.Schema != "gooo/body-composition-inputs/v1" || len(inputs.Inputs) == 0 {
		return caseDocument{}, fmt.Errorf("invalid finite input fixture")
	}
	doc := caseDocument{Schema: "gooo/body-composition-cases/v1"}
	for _, row := range inputs.Inputs {
		x, ok := row[entryKey]
		if !ok || len(row) != 1 {
			return doc, fmt.Errorf("finite input entry mismatch")
		}
		doc.Cases = append(doc.Cases, finiteCase{row, map[string]classification{entryKey: oracle(x)}})
	}
	return doc, nil
}
func oracle(x int64) classification {
	if x < -2147483648 || x > 2147483647 {
		return classification{}
	}
	digit := unicode.IsDigit(rune(x))
	start := x == 95 || unicode.IsLetter(rune(x))
	return classification{true, start, start || digit, digit}
}
func command(ctx context.Context, log, dir string, env []string, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=")
	cmd.Env = append(cmd.Env, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if e := write(log, stdout.Bytes()); e != nil {
		return nil, e
	}
	if stderr.Len() > 0 {
		if e := write(log+".stderr", stderr.Bytes()); e != nil {
			return nil, e
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", log, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func sha(raw []byte) string    { return "sha256:" + digest(raw) }
func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
func write(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}
func save(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return write(path, append(raw, '\n'))
}
