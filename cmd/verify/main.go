package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type upstream struct {
	GoVersion        string            `json:"go_version"`
	CompilerRevision string            `json:"compiler_revision"`
	CompilerVersion  string            `json:"compiler_version"`
	SourceSHA        map[string]string `json:"source_sha256"`
}
type observation struct {
	Package     string `json:"package"`
	Activity    string `json:"activity"`
	Domain      string `json:"domain"`
	Cases       int    `json:"cases"`
	Batches     int    `json:"batches"`
	Passed      int    `json:"passed"`
	ReplayEqual bool   `json:"replay_equal"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	compiler := flag.String("compiler", "./.gooo", "pinned compiler executable")
	out := flag.String("out", "", "new evidence directory (must not exist)")
	browser := flag.String("browser-fixtures", "", "write independent browser oracle fixtures to a new JSON file")
	flag.Parse()
	if *out != "" && *browser != "" {
		return fmt.Errorf("choose either --out or --browser-fixtures")
	}
	var lock upstream
	if err := readJSON("upstream.json", &lock); err != nil {
		return err
	}
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("oracle toolchain %s, require %s", runtime.Version(), lock.GoVersion)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("HTTP int64 compatibility requires a 64-bit target")
	}
	for path, want := range lock.SourceSHA {
		got, err := digestFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("Go upstream source hash differs: %s", path)
		}
	}
	suites, err := corpus()
	if err != nil {
		return err
	}
	if *browser != "" {
		return writeBrowserFixtures(*browser, suites, lock)
	}
	inspector, err := unicodeInspectorCorpus(suites)
	if err != nil {
		return err
	}
	legacyCases := 0
	for _, s := range suites {
		legacyCases += len(s.Cases)
	}
	suites = append(suites, inspector.Suites...)
	colors, err := colorCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, colors.Suites...)
	if *out == "" {
		return fmt.Errorf("provide --out for a new evidence directory")
	}
	root, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(root), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(root, 0755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	raw, err := execute(ctx, *compiler, filepath.Join(root, "compiler.json"), "version", "--build", "--json")
	if err != nil {
		return err
	}
	var build struct {
		Version  string `json:"version"`
		Revision string `json:"compiler_source_sha"`
		Modified string `json:"vcs_modified"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
	}
	if err = json.Unmarshal(raw, &build); err != nil {
		return err
	}
	if build.Revision != lock.CompilerRevision || build.Version != lock.CompilerVersion || build.Modified != "false" || build.Go != lock.GoVersion || build.Native != lock.GoVersion {
		return fmt.Errorf("compiler identity differs from upstream.json: %s", raw)
	}
	var manifest map[string]json.RawMessage
	if err = readJSON("gooo.workspace.json", &manifest); err != nil {
		return err
	}
	var packages []sourcePackage
	if err = json.Unmarshal(manifest["packages"], &packages); err != nil {
		return err
	}
	sourceHashes := map[string]string{}
	sourceBytes := map[string][]byte{}
	var sources []string
	for _, p := range packages {
		for _, source := range p.Sources {
			raw, err := os.ReadFile(source)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			sourceBytes[source] = raw
			sourceHashes[source] = hex.EncodeToString(sum[:])
			sources = append(sources, source)
		}
	}
	sourceIdentities, err := snapshotSourceIdentities(packages, sourceBytes)
	if err != nil {
		return err
	}
	reports := []observation{}
	for _, s := range suites {
		r := observation{Package: s.Package, Activity: s.Activity, Domain: s.Domain, Cases: len(s.Cases), ReplayEqual: true}
		batchSize := 128
		if s.Package == "app/unicode-inspector" {
			batchSize = unicodeInspectorBatchSize
		}
		if s.Package == "go/image/color" {
			batchSize = colorPortBatchSize
		}
		if s.Package == "app/color-inspector" {
			batchSize = colorInspectorBatchSize
		}
		for start := 0; start < len(s.Cases); start += batchSize {
			end := min(start+batchSize, len(s.Cases))
			batch := s
			batch.Cases = s.Cases[start:end]
			dir := filepath.Join(root, strings.ReplaceAll(s.Package, "/", "-")+"-"+s.Activity, fmt.Sprintf("%03d", r.Batches))
			if err = os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			for _, source := range sources {
				b := sourceBytes[source]
				path := filepath.Join(dir, source)
				if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				if err = os.WriteFile(path, b, 0644); err != nil {
					return err
				}
			}
			entry, _ := json.Marshal(map[string]string{"package_path": s.Package, "activity": s.Activity})
			manifest["entry"] = entry
			workspace := filepath.Join(dir, "gooo.workspace.json")
			cases := filepath.Join(dir, "cases.json")
			if err = save(workspace, manifest); err != nil {
				return err
			}
			if err = save(cases, caseDocument(batch)); err != nil {
				return err
			}
			manifestSHA, err := digestFile(workspace)
			if err != nil {
				return err
			}
			casesSHA, err := digestFile(cases)
			if err != nil {
				return err
			}
			identity := expectedIdentity{CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion, ManifestDigest: "sha256:" + manifestSHA, CasesDigest: "sha256:" + casesSHA, Sources: sourceIdentities}
			receipt := filepath.Join(dir, "execution.json")
			raw, err = execute(ctx, *compiler, receipt, "package", "execute", "--json", "--cases", cases, workspace)
			if err != nil {
				return err
			}
			original, err := inspect(raw, batch, false, identity)
			if err != nil {
				return fmt.Errorf("%s: %w", receipt, err)
			}
			replayPath := filepath.Join(dir, "replay.json")
			raw, err = execute(ctx, *compiler, replayPath, "package", "replay", "--json", "--receipt", receipt, "--cases", cases, workspace)
			if err != nil {
				return err
			}
			replay, err := inspect(raw, batch, true, identity)
			if err != nil {
				return fmt.Errorf("%s: %w", replayPath, err)
			}
			h, err := digestFile(receipt)
			if err != nil {
				return err
			}
			if replay.Generated != original.Generated || replay.ReplayedFrom != "sha256:"+h {
				return fmt.Errorf("saved replay is not bound to the same receipt and generated program")
			}
			r.Batches++
			r.Passed += len(batch.Cases)
		}
		reports = append(reports, r)
		fmt.Printf("%s:%s %d/%d; %d batches; replay identical\n", r.Package, r.Activity, r.Passed, r.Cases, r.Batches)
	}
	total := 0
	for _, r := range reports {
		total += r.Cases
	}
	return save(filepath.Join(root, "summary.json"), map[string]any{
		"schema": "gooo/go-ports-differential/v1", "go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"compiler_revision": lock.CompilerRevision, "source_sha256": sourceHashes,
		"ported_functions": 15, "consumer_programs": 4, "consumer_activities": 5,
		"legacy_cases": legacyCases, "unicode_reference_cases": inspector.ReferenceCases, "unicode_rejection_cases": inspector.RejectionCases,
		"color_port_reference_cases": colors.PortReferenceCases, "color_port_rejection_cases": colors.PortRejectionCases,
		"color_consumer_reference_cases": colors.ConsumerReferenceCases, "color_consumer_rejection_cases": colors.ConsumerRejectionCases,
		"cases": total, "passed": total, "runs": reports,
		"scope": "Finite differential observations against pinned Go source, plus identical saved replay. The 3,147 legacy and 346 Unicode inspector cases are unchanged. Color port and consumer Go-reference cases and explicit out-of-byte rejection cases are counted separately. RuneStart and the five math/bits functions each cover all 256 byte values exactly once. Out-of-domain math/bits inputs are excluded; no claim of exhaustive equivalence for other domains, whole-package compatibility, ICC color management, or printed appearance.",
	})
}
func caseDocument(s suite) any {
	rows := []any{}
	key := s.Package + ":" + s.Activity
	for _, c := range s.Cases {
		inputs := map[string]any{}
		for i, v := range c.Inputs {
			k := key
			if len(c.Inputs) > 1 {
				k = fmt.Sprintf("%s.input%d", key, i)
			}
			inputs[k] = v
		}
		rows = append(rows, map[string]any{"inputs": inputs, "expected": map[string]any{key: c.Expected}})
	}
	return map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": rows}
}
func execute(ctx context.Context, compiler, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, compiler, args...)
	cmd.Env = append(os.Environ(), "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if e := os.WriteFile(path, stdout.Bytes(), 0644); e != nil {
		return nil, e
	}
	if stderr.Len() > 0 {
		if e := os.WriteFile(path+".stderr", stderr.Bytes(), 0644); e != nil {
			return nil, e
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
func save(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}
func digestFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
