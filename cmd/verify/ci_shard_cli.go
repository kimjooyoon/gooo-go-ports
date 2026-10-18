package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type ciShardPaths []string

func (p *ciShardPaths) String() string { return fmt.Sprint([]string(*p)) }
func (p *ciShardPaths) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty CI shard partial path")
	}
	*p = append(*p, value)
	return nil
}

type ciShardOptions struct {
	Requested                                    bool
	PlanOut, SelectOut, ReduceOut, Plan, Context string
	Count, Index                                 int
	Partials                                     ciShardPaths
}

func (o ciShardOptions) enabled() bool {
	return o.Requested || o.PlanOut != "" || o.SelectOut != "" || o.ReduceOut != "" || o.Plan != "" || o.Context != "" || o.Count != 0 || o.Index != -1 || len(o.Partials) != 0
}

// Run before reading upstream/compiler/corpus or creating an output parent.
func (o ciShardOptions) validate(conflicts bool) error {
	if !o.enabled() {
		return nil
	}
	outputs := 0
	for _, path := range []string{o.PlanOut, o.SelectOut, o.ReduceOut} {
		if path != "" {
			outputs++
		}
	}
	if conflicts || outputs != 1 || o.Context == "" {
		return fmt.Errorf("CI shard metadata mode requires one output and --ci-shard-context; full execution flags cannot be combined")
	}
	switch {
	case o.PlanOut != "":
		if o.Count < 1 || o.Count > maxCIShards || o.Index != -1 || o.Plan != "" || len(o.Partials) != 0 {
			return fmt.Errorf("CI shard planning requires count1..16 and no selection/reduction inputs")
		}
	case o.SelectOut != "":
		if o.Count != 0 || o.Index < 0 || o.Index >= maxCIShards || o.Plan == "" || len(o.Partials) != 0 {
			return fmt.Errorf("CI shard selection requires plan/index and no partials/count")
		}
	default:
		if o.Count != 0 || o.Index != -1 || o.Plan == "" || len(o.Partials) == 0 || len(o.Partials) > maxCIShards {
			return fmt.Errorf("CI shard reduction requires plan and all partials, without count/index")
		}
	}
	return nil
}

func readCIShardFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("bounded regular CI shard file required: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("CI shard file changed before read")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || len(raw) > int(limit) || int64(len(raw)) != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, fmt.Errorf("CI shard file changed during read")
	}
	return raw, nil
}

func writeNewCIShardJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > maxCIShardJSONBytes {
		return fmt.Errorf("CI shard output exceeds bound")
	}
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

func readCIShardGit(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}

func validateCIShardGitState(root string, run ciShardRun) error {
	commit, err := readCIShardGit(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(commit)) != run.TestedCommit {
		return fmt.Errorf("CI shard tested commit differs from checkout")
	}
	tree, err := readCIShardGit(root, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil || strings.TrimSpace(string(tree)) != run.TestedTree {
		return fmt.Errorf("CI shard tested tree differs from checkout")
	}
	status, err := readCIShardGit(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return fmt.Errorf("CI shard metadata requires a clean tested checkout")
	}
	return nil
}

type ciShardGitSource struct {
	Mode, Blob string
}

// Git object IDs bind source bytes to the immutable tested tree, in addition to
// the SHA-256 metadata digest. SHA-1 here is Git's pinned object identity format.
func ciShardGitBlob(raw []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(raw), 0)
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}

func validateCIShardGitSource(raw []byte, mode string, expected ciShardGitSource) error {
	if mode != expected.Mode || ciShardGitBlob(raw) != expected.Blob {
		return fmt.Errorf("CI shard source bytes/mode differ from tested Git tree")
	}
	return nil
}

func decodeCIShardGitSources(raw []byte, modes map[string]string) (map[string]ciShardGitSource, error) {
	if len(raw) == 0 || raw[len(raw)-1] != 0 {
		return nil, fmt.Errorf("CI shard tested tree source roster is incomplete")
	}
	sources := map[string]ciShardGitSource{}
	for _, row := range bytes.Split(raw[:len(raw)-1], []byte{0}) {
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid CI shard tested tree row")
		}
		header, name := strings.Split(string(parts[0]), " "), string(parts[1])
		if len(header) != 3 || header[0] != modes[name] || header[1] != "blob" || !commitPattern.MatchString(header[2]) || sources[name].Blob != "" {
			return nil, fmt.Errorf("CI shard tested tree source mode/identity differs: %s", name)
		}
		sources[name] = ciShardGitSource{Mode: header[0], Blob: header[2]}
	}
	if len(sources) != len(modes) {
		return nil, fmt.Errorf("CI shard tested tree source set differs")
	}
	return sources, nil
}

func validateCIShardSourceSnapshot(root string, sources map[string]ciShardSource) error {
	for name, source := range sources {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := nativeVerificationCheckSourceMode(path, source.Mode); err != nil {
			return fmt.Errorf("CI shard source mode changed: %s: %w", name, err)
		}
		raw, err := readCIShardFile(path, 64<<20)
		if err != nil || int64(len(raw)) != source.Bytes || contentDigest(raw) != source.SHA256 {
			return fmt.Errorf("CI shard source snapshot changed: %s", name)
		}
	}
	return nil
}

func validateCIShardCheckout(root string, plan ciShardPlan) error {
	if err := validateCIShardGitState(root, plan.Run); err != nil {
		return err
	}
	if err := validateCIShardSourceSnapshot(root, plan.Sources); err != nil {
		return err
	}
	return validateCIShardGitState(root, plan.Run)
}

func writeCurrentCIShardJSON(root string, plan ciShardPlan, path string, value any) error {
	if err := validateCIShardCheckout(root, plan); err != nil {
		return err
	}
	return writeNewCIShardJSON(path, value)
}

func currentCIShardPlan(root string, run ciShardRun, count int) (ciShardPlan, error) {
	var zero ciShardPlan
	// Run/attempt/repository are trusted caller inputs, not self-authenticating
	// claims. Bind their tested source locally before deriving any plan anchors.
	if err := validateCIShardGitState(root, run); err != nil {
		return zero, err
	}
	c, err := nativeExpectedVerificationContract()
	if err != nil {
		return zero, err
	}
	ledger, err := readCIShardFile(filepath.Join(root, nativeVerificationContractPath), MaxContractBytes)
	if err != nil {
		return zero, err
	}
	if err = nativeVerificationCheckDiskLedger(ledger); err != nil {
		return zero, err
	}
	var lock upstream
	if err = readJSON(filepath.Join(root, "upstream.json"), &lock); err != nil {
		return zero, err
	}
	if run.CompilerRevision != lock.CompilerRevision || run.GoVersion != lock.GoVersion {
		return zero, fmt.Errorf("CI shard context compiler/Go differs from current pins")
	}
	modes, err := ciShardExpectedSourceModes(c)
	if err != nil {
		return zero, err
	}
	names := make([]string, 0, len(modes))
	for name := range modes {
		names = append(names, name)
	}
	sort.Strings(names)
	args := append([]string{"ls-tree", "-r", "-z", "--full-tree", run.TestedTree, "--"}, names...)
	treeRaw, err := readCIShardGit(root, args...)
	if err != nil {
		return zero, err
	}
	treeSources, err := decodeCIShardGitSources(treeRaw, modes)
	if err != nil {
		return zero, err
	}
	sources := map[string]ciShardSource{}
	for name, mode := range modes {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err = nativeVerificationCheckSourceMode(path, mode); err != nil {
			return zero, err
		}
		raw, err := readCIShardFile(path, 64<<20)
		if err != nil {
			return zero, err
		}
		if err := validateCIShardGitSource(raw, mode, treeSources[name]); err != nil {
			return zero, fmt.Errorf("%s: %w", name, err)
		}
		sources[name] = ciShardSource{Mode: mode, Bytes: int64(len(raw)), SHA256: contentDigest(raw)}
	}
	catalog, err := freezeVerificationCatalog(filepath.Join(root, "verification-catalog.json"))
	if err != nil {
		return zero, err
	}
	if contentDigest(catalog.Raw) != sources["verification-catalog.json"].SHA256 {
		return zero, fmt.Errorf("CI shard catalog differs from tested source snapshot")
	}
	workspaces := make([]ciShardWorkspace, len(c.Suites))
	for i, entry := range c.Suites {
		s := suite{Package: entry.Package, Activity: entry.Activity}
		w, err := nativeWorkspaceProfile(c.Profiles.Workspace).workspaceForSuite(catalog, s)
		if err != nil {
			return zero, err
		}
		if !reflect.DeepEqual(w.Names, entry.SourcePaths) {
			return zero, fmt.Errorf("CI shard workspace roster differs at suite%d", i)
		}
		raw, err := json.MarshalIndent(w.Manifest, "", "  ")
		if err != nil {
			return zero, err
		}
		raw = append(raw, '\n')
		contract, err := contractFor(s)
		if err != nil {
			return zero, err
		}
		anchor := ciShardWorkspace{OriginalIndex: i, ManifestSHA256: contentDigest(raw), EntryProducer: contract.ID, Sources: []ciShardSourceAnchor{}}
		for _, name := range w.Names {
			if !bytes.Equal(w.Sources[name], catalog.Sources[name]) || contentDigest(w.Sources[name]) != sources[name].SHA256 {
				return zero, fmt.Errorf("CI shard catalog/source snapshot differs")
			}
			anchor.Sources = append(anchor.Sources, ciShardSourceAnchor{Name: name, RawSHA256: sources[name].SHA256, Identity: w.Identities[name]})
		}
		anchor.SourceSetSHA256, err = canonicalContractDigest(anchor.Sources)
		if err != nil {
			return zero, err
		}
		workspaces[i] = anchor
	}
	plan, err := newCIShardPlan(c, contentDigest(ledger), run, count, sources, workspaces)
	if err != nil {
		return zero, err
	}
	if err := validateCIShardCheckout(root, plan); err != nil {
		return zero, err
	}
	return plan, nil
}

// Metadata-only CLI: does not invoke a compiler, execute native work, inspect a
// raw native receipt or create an old full-run summary/timing COMPLETE footer.
func runCIShardMetadata(o ciShardOptions, root string) error {
	var run ciShardRun
	raw, err := readCIShardFile(o.Context, maxCIShardJSONBytes)
	if err != nil {
		return err
	}
	if err = decodeCIShardJSON(raw, &run); err != nil {
		return err
	}
	if !ciShardRunValid(run) {
		return fmt.Errorf("invalid trusted CI shard context")
	}
	if o.PlanOut != "" {
		plan, err := currentCIShardPlan(root, run, o.Count)
		if err != nil {
			return err
		}
		return writeCurrentCIShardJSON(root, plan, o.PlanOut, plan)
	}
	var plan ciShardPlan
	raw, err = readCIShardFile(o.Plan, maxCIShardJSONBytes)
	if err != nil {
		return err
	}
	if err = decodeCIShardJSON(raw, &plan); err != nil {
		return err
	}
	expected, err := currentCIShardPlan(root, run, len(plan.Shards))
	if err != nil {
		return err
	}
	if err = validateCIShardPlan(plan, expected); err != nil {
		return err
	}
	if o.SelectOut != "" {
		if o.Index >= len(plan.Shards) {
			return fmt.Errorf("CI shard index outside plan")
		}
		selection := struct {
			Schema           string            `json:"schema"`
			PlanSHA256       string            `json:"plan_sha256"`
			Run              ciShardRun        `json:"run"`
			Shard            ciShardAssignment `json:"shard"`
			NativeAcceptance bool              `json:"native_acceptance"`
		}{"gooo/native-ci-shard-selection/v1", plan.PlanSHA256, run, plan.Shards[o.Index], false}
		return writeCurrentCIShardJSON(root, expected, o.SelectOut, selection)
	}
	partials := []ciShardPartial{}
	paths := append([]string(nil), o.Partials...)
	sort.Strings(paths)
	for i, path := range paths {
		if i > 0 && path == paths[i-1] {
			return fmt.Errorf("repeated CI shard partial file")
		}
		var part ciShardPartial
		raw, err = readCIShardFile(path, maxCIShardJSONBytes)
		if err != nil {
			return err
		}
		if err = decodeCIShardJSON(raw, &part); err != nil {
			return err
		}
		partials = append(partials, part)
	}
	reduction, err := reduceCIShardPartials(expected, partials)
	if err != nil {
		return err
	}
	return writeCurrentCIShardJSON(root, expected, o.ReduceOut, reduction)
}
