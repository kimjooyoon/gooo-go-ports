// Native shard evidence reuses the full verifier's independent Go oracles.
// This reader proves local raw-evidence consistency, never producer/job authority.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

func loadCIShardNativeInventory(root string, plan ciShardPlan) (ciShardNativeInventory, error) {
	var zero ciShardNativeInventory
	absolute, err := filepath.Abs(root)
	if err != nil {
		return zero, err
	}
	cwd, err := os.Getwd()
	if err != nil || filepath.Clean(cwd) != filepath.Clean(absolute) {
		return zero, fmt.Errorf("native shard inventory requires its repository as the current directory")
	}
	expected, err := currentCIShardPlan(root, plan.Run, len(plan.Shards))
	if err != nil {
		return zero, err
	}
	if err := validateCIShardPlan(plan, expected); err != nil {
		return zero, err
	}
	var lock upstream
	if err := readJSON(filepath.Join(root, "upstream.json"), &lock); err != nil {
		return zero, err
	}
	caseProfile := nativeCaseProfile(plan.Profiles.Case)
	workspaceProfile := nativeWorkspaceProfile(plan.Profiles.Workspace)
	if err := caseProfile.validateCompiler(lock); err != nil {
		return zero, err
	}
	if err := workspaceProfile.validateCompiler(lock); err != nil {
		return zero, err
	}
	if runtime.Version() != lock.GoVersion || lock.CompilerRevision != plan.Run.CompilerRevision || lock.GoVersion != plan.Run.GoVersion {
		return zero, fmt.Errorf("native shard actual oracle toolchain or compiler pin differs")
	}
	if strconv.IntSize != 64 || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return zero, fmt.Errorf("native shard oracle requires a supported 64-bit target")
	}
	for path, want := range lock.SourceSHA {
		got, err := digestFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil {
			return zero, err
		}
		if got != want {
			return zero, fmt.Errorf("native shard Go source differs: %s", path)
		}
	}
	if err := verifyExternalOracles(root, lock.ExternalSources); err != nil {
		return zero, err
	}
	suites, err := corpus()
	if err != nil {
		return zero, err
	}
	inspector, err := unicodeInspectorCorpus(suites)
	if err != nil {
		return zero, err
	}
	suites = append(suites, inspector.Suites...)
	colors, err := colorCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, colors.Suites...)
	rotations, err := rotationCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, rotations.Suites...)
	durations, err := durationCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, durations.Suites...)
	whitespace, err := whitespaceCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, whitespace.Suites...)
	classification, err := classificationCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, classification.Suites...)
	geometry, err := geometryCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, geometry.Suites...)
	folding, err := simpleFoldCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, folding.Suites...)
	ycbcr, err := ycbcrCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, ycbcr.Suites...)
	canonical, err := canonCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, canonical.Suites...)
	fixedBounds, err := fixedCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, fixedBounds.Suites...)
	mapping, err := caseMappingCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, mapping.Suites...)
	nrgba, err := nrgbaCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, nrgba.Suites...)
	byteSearch, err := stringByteCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, byteSearch.Suites...)
	uuids, err := uuidCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, uuids.Suites...)
	uuidParse, err := uuidParseCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, uuidParse.Suite)
	containment, err := containmentCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, containment.Suites...)
	literalText, err := literalTextCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, literalText.Suites...)
	literalCut, err := literalCutCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, literalCut.Suites...)
	decimalPayload, err := decimalPayloadCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, decimalPayload.Suites...)
	pathAlias, err := pathAliasCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, pathAlias...)
	if err := verifyHTTPGutsRuntimeDependencies(); err != nil {
		return zero, err
	}
	headers, err := headerCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, headers...)
	addition, err := headerAdditionCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, addition...)
	canonicalHeaderKeys, err := canonicalHeaderCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, canonicalHeaderKeys...)
	etags, err := etagCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, etags...)
	ifMatches, err := ifMatchCorpus(root)
	if err != nil {
		return zero, err
	}
	suites = append(suites, ifMatches...)
	lastIndex, err := lastIndexCorpus()
	if err != nil {
		return zero, err
	}
	suites = append(suites, lastIndex.Suites...)

	catalog, err := freezeVerificationCatalog(filepath.Join(root, "verification-catalog.json"))
	if err != nil {
		return zero, err
	}
	if contentDigest(catalog.Raw) != plan.Sources["verification-catalog.json"].SHA256 {
		return zero, fmt.Errorf("native shard catalog source differs")
	}
	if len(suites) != len(plan.Suites) {
		return zero, fmt.Errorf("native shard original suite population differs")
	}
	workspaces := make([]suiteWorkspace, len(suites))
	var cases, batches int64
	for index, s := range suites {
		wanted := plan.Suites[index]
		if wanted.Index != int64(index) || s.Package != wanted.Package || s.Activity != wanted.Activity || int64(len(s.Cases)) != wanted.Cases {
			return zero, fmt.Errorf("native shard original Go corpus differs at suite %d", index)
		}
		selected, err := workspaceProfile.workspaceForSuite(catalog, s)
		if err != nil {
			return zero, err
		}
		if err := validateCIShardNativeWorkspace(plan, index, s, selected); err != nil {
			return zero, err
		}
		policy, err := caseProfile.policy(s, selected, lock)
		if err != nil {
			return zero, err
		}
		count := int64(0)
		for start := 0; start < len(s.Cases); {
			prepared, err := policy.prepare(s, start)
			if err != nil {
				return zero, err
			}
			if prepared.End <= start || prepared.End > len(s.Cases) {
				return zero, fmt.Errorf("native shard original partition is invalid")
			}
			start = prepared.End
			count++
		}
		if count != wanted.Batches {
			return zero, fmt.Errorf("native shard original batch population differs at suite %d", index)
		}
		cases += int64(len(s.Cases))
		batches += count
		workspaces[index] = selected
	}
	if cases != plan.Cases || batches != plan.Batches {
		return zero, fmt.Errorf("native shard full Go corpus totals differ")
	}
	if err := validateCIShardCheckout(root, plan); err != nil {
		return zero, err
	}
	return ciShardNativeInventory{Suites: suites, Lock: lock, Catalog: catalog, Workspaces: workspaces}, nil
}

func validateCIShardNativeWorkspace(plan ciShardPlan, index int, s suite, selected suiteWorkspace) error {
	if index < 0 || index >= len(plan.Workspaces) || index >= len(plan.Suites) {
		return fmt.Errorf("native shard workspace index differs")
	}
	wanted := plan.Workspaces[index]
	if selected.Manifest.Schema != "gooo/package-workspace-manifest/v1" || selected.Manifest.Entry.Package != s.Package || selected.Manifest.Entry.Activity != s.Activity || len(selected.Names) != len(selected.Sources) || len(selected.Names) != len(selected.Identities) {
		return fmt.Errorf("native shard selected workspace shape or entry differs")
	}
	if !reflect.DeepEqual(selected.Names, plan.Suites[index].SourcePaths) {
		return fmt.Errorf("native shard selected source roster differs")
	}
	raw, err := json.MarshalIndent(selected.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if contentDigest(append(raw, '\n')) != wanted.ManifestSHA256 {
		return fmt.Errorf("native shard manifest bytes differ")
	}
	contract, err := contractFor(s)
	if err != nil || contract.ID != wanted.EntryProducer {
		return fmt.Errorf("native shard original entry producer differs")
	}
	anchors := []ciShardSourceAnchor{}
	for _, name := range selected.Names {
		source, ok := plan.Sources[name]
		if !ok || contentDigest(selected.Sources[name]) != source.SHA256 {
			return fmt.Errorf("native shard selected source bytes differ: %s", name)
		}
		anchors = append(anchors, ciShardSourceAnchor{Name: name, RawSHA256: source.SHA256, Identity: selected.Identities[name]})
	}
	digest, err := canonicalContractDigest(anchors)
	if err != nil || digest != wanted.SourceSetSHA256 || !reflect.DeepEqual(anchors, wanted.Sources) {
		return fmt.Errorf("native shard selected source identity differs")
	}
	return nil
}

type ciShardEvidenceFile struct {
	info   os.FileInfo
	digest string
}
type ciShardEvidenceReader struct {
	root  string
	dirs  map[string]os.FileInfo
	files map[string]ciShardEvidenceFile
	order []string
}

func newCIShardEvidenceReader(root string) (*ciShardEvidenceReader, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("native shard evidence requires a regular root directory")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	return &ciShardEvidenceReader{root: resolved, dirs: map[string]os.FileInfo{"": info}, files: map[string]ciShardEvidenceFile{}}, nil
}

func (r *ciShardEvidenceReader) directory(name string) (string, error) {
	if name != "" && (!filepath.IsLocal(name) || filepath.Clean(name) != name) {
		return "", fmt.Errorf("native shard evidence path escapes namespace")
	}
	path := r.root
	parts := []string{""}
	if name != "" {
		parts = append(parts, strings.Split(name, string(filepath.Separator))...)
	}
	relative := ""
	for i, part := range parts {
		if i > 0 {
			relative = filepath.Join(relative, part)
			path = filepath.Join(r.root, relative)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", fmt.Errorf("native shard evidence directory is missing: %s: %w", relative, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("native shard evidence directory is not a regular directory: %s", relative)
		}
		if prior, ok := r.dirs[relative]; ok && !os.SameFile(prior, info) {
			return "", fmt.Errorf("native shard evidence directory changed: %s", relative)
		}
		r.dirs[relative] = info
	}
	return path, nil
}

func (r *ciShardEvidenceReader) read(name string, limit int64) ([]byte, error) {
	if !filepath.IsLocal(name) || filepath.Clean(name) != name {
		return nil, fmt.Errorf("native shard evidence file escapes namespace")
	}
	parent := filepath.Dir(name)
	if parent == "." {
		parent = ""
	}
	dir, err := r.directory(parent)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, filepath.Base(name))
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	raw, err := readCIShardFile(path, limit)
	if err != nil {
		return nil, err
	}
	if _, err = r.directory(parent); err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("native shard evidence file changed")
	}
	digest := contentDigest(raw)
	if prior, ok := r.files[name]; ok && (!os.SameFile(prior.info, after) || prior.digest != digest || prior.info.Mode() != after.Mode()) {
		return nil, fmt.Errorf("native shard evidence changed after inspection: %s", name)
	}
	if _, exists := r.files[name]; !exists {
		r.order = append(r.order, name)
	}
	r.files[name] = ciShardEvidenceFile{info: after, digest: digest}
	return raw, nil
}

func (r *ciShardEvidenceReader) finish() error {
	return r.finishFrom(0)
}

// Recheck each completed batch locally, then the whole snapshot once at the
// suite boundary. Re-reading every preceding receipt per batch is quadratic.
func (r *ciShardEvidenceReader) finishFrom(start int) error {
	for _, name := range r.order[start:] {
		if _, err := r.read(name, 64<<20); err != nil {
			return err
		}
	}
	return nil
}

func ciShardNativeCaseBytes(s suite, prepared nativePreparedCaseBatch) ([]byte, error) {
	if prepared.Raw != nil {
		return prepared.Raw, nil
	}
	raw, err := json.MarshalIndent(caseDocument(s), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func ciShardNativeErrorMessage(err error) string {
	message := err.Error()
	if len(message) <= 4096 {
		return strings.ToValidUTF8(message, "?")
	}
	return strings.ToValidUTF8(message[:4000], "?") + "\n[diagnosis shortened; original raw evidence and error remain retained]"
}

// Every returned batch was inspected against independently recomputed Go values.
// Failure returns only the completed contiguous prefix, never the bad batch.
func inspectCIShardSuiteEvidence(plan ciShardPlan, shard, original int, inventory ciShardNativeInventory, evidenceRoot string, reported ciShardSuiteRecord) (record ciShardSuiteRecord, returned error) {
	record = ciShardSuiteRecord{OriginalIndex: original, Package: reported.Package, Activity: reported.Activity, State: reported.State, Error: reported.Error, Batches: []ciShardBatch{}, Exhaustive: []ciShardExhaustive{}}
	fail := func(err error) (ciShardSuiteRecord, error) {
		record.State = "FAILED"
		record.Error = ciShardNativeErrorMessage(err)
		return record, err
	}
	if err := validateCIShardPlanShape(plan); err != nil {
		return fail(err)
	}
	if shard < 0 || shard >= len(plan.Shards) || original < 0 || original >= len(plan.Suites) || len(inventory.Suites) != len(plan.Suites) || len(inventory.Workspaces) != len(plan.Suites) {
		return fail(fmt.Errorf("native shard evidence inventory/index differs"))
	}
	assigned := false
	for _, index := range plan.Shards[shard].OriginalIndices {
		if index == original {
			assigned = true
		}
	}
	if !assigned {
		return fail(fmt.Errorf("native shard evidence suite belongs to another namespace"))
	}
	s, selected := inventory.Suites[original], inventory.Workspaces[original]
	if inventory.Lock.CompilerRevision != plan.Run.CompilerRevision || inventory.Lock.GoVersion != plan.Run.GoVersion {
		return fail(fmt.Errorf("native shard inventory producer/toolchain differs from the plan"))
	}
	if reported.OriginalIndex != original || reported.Package != s.Package || reported.Activity != s.Activity || s.Package != plan.Suites[original].Package || s.Activity != plan.Suites[original].Activity || int64(len(s.Cases)) != plan.Suites[original].Cases {
		return fail(fmt.Errorf("native shard evidence suite identity differs"))
	}
	if reported.State != "VERIFIED" && reported.State != "FAILED" && reported.State != "CANCELED" && reported.State != "NOT_STARTED" {
		return fail(fmt.Errorf("native shard evidence reported state differs"))
	}
	if err := validateCIShardNativeWorkspace(plan, original, s, selected); err != nil {
		return fail(err)
	}
	reader, err := newCIShardEvidenceReader(evidenceRoot)
	if err != nil {
		return fail(err)
	}
	suiteRel := filepath.Join(ciShardNamespace(shard), fmt.Sprintf("suite-%04d", original))
	dir, err := reader.directory(suiteRel)
	if err != nil {
		if reported.State == "NOT_STARTED" && os.IsNotExist(err) {
			return record, nil
		}
		return fail(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fail(err)
	}
	observed := map[int]bool{}
	exhaustiveName := ciShardNativeExhaustiveDirectory(s)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "batch-") {
			index, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "batch-"))
			if err != nil || index < 0 || int64(index) >= plan.Suites[original].Batches || entry.Name() != fmt.Sprintf("batch-%06d", index) || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return fail(fmt.Errorf("native shard batch namespace differs"))
			}
			observed[index] = true
		} else if entry.IsDir() && entry.Name() != exhaustiveName {
			return fail(fmt.Errorf("foreign native shard evidence directory"))
		}
	}
	if reported.State == "NOT_STARTED" {
		if len(observed) != 0 {
			return fail(fmt.Errorf("not-started native shard contains batches"))
		}
		return record, nil
	}
	profile := nativeCaseProfile(plan.Profiles.Case)
	policy, err := profile.policy(s, selected, inventory.Lock)
	if err != nil {
		return fail(err)
	}
	manifest, err := json.MarshalIndent(selected.Manifest, "", "  ")
	if err != nil {
		return fail(err)
	}
	manifest = append(manifest, '\n')
	start, index := 0, 0
	var firstReplay []byte
	var firstGenerated string
	for start < len(s.Cases) {
		fileStart := len(reader.order)
		prepared, err := policy.prepare(s, start)
		if err != nil {
			return fail(err)
		}
		if prepared.End <= start || prepared.End > len(s.Cases) {
			return fail(fmt.Errorf("native shard partition differs"))
		}
		if !observed[index] {
			for later := range observed {
				if later > index {
					return fail(fmt.Errorf("native shard batch gap or overlap"))
				}
			}
			if reported.State == "VERIFIED" {
				return fail(fmt.Errorf("verified native shard batch is missing"))
			}
			break
		}
		batch := s
		batch.Cases = s.Cases[start:prepared.End]
		expectedCases, err := ciShardNativeCaseBytes(batch, prepared)
		if err != nil {
			return fail(err)
		}
		relative := filepath.Join(suiteRel, fmt.Sprintf("batch-%06d", index))
		cases, err := reader.read(filepath.Join(relative, "cases.json"), 32<<20)
		if err != nil || !bytes.Equal(cases, expectedCases) {
			return fail(fmt.Errorf("native shard exact original case bytes differ: %v", err))
		}
		workspace, err := reader.read(filepath.Join(relative, "gooo.workspace.json"), 16<<20)
		if err != nil || !bytes.Equal(workspace, manifest) {
			return fail(fmt.Errorf("native shard exact manifest bytes differ: %v", err))
		}
		for _, name := range selected.Names {
			copied, err := reader.read(filepath.Join(relative, filepath.FromSlash(name)), 64<<20)
			if err != nil || !bytes.Equal(copied, selected.Sources[name]) {
				return fail(fmt.Errorf("native shard copied source differs: %s: %v", name, err))
			}
		}
		identity := expectedIdentity{CompilerRevision: inventory.Lock.CompilerRevision, GoVersion: inventory.Lock.GoVersion, ManifestDigest: contentDigest(manifest), CasesDigest: contentDigest(cases), Sources: selected.Identities, SourceBytes: selected.Sources}
		_, execution, err := reader.receipt(filepath.Join(relative, "execution.json"))
		if err != nil {
			return fail(err)
		}
		checkedExecution, err := inspect(execution, batch, false, identity)
		if err != nil {
			return fail(err)
		}
		_, replay, err := reader.receipt(filepath.Join(relative, "replay.json"))
		if err != nil {
			return fail(err)
		}
		checkedReplay, err := inspect(replay, batch, true, identity)
		if err != nil {
			return fail(err)
		}
		if err := inspectReplayBinding(checkedExecution, checkedReplay, contentDigest(execution)); err != nil {
			return fail(err)
		}
		if err := reader.finishFrom(fileStart); err != nil {
			return fail(err)
		}
		anchor := plan.Workspaces[original]
		record.Batches = append(record.Batches, ciShardBatch{Index: index, Start: int64(start), End: int64(prepared.End), Namespace: ciShardBatchNamespace(shard, original, index), CasesSHA256: contentDigest(cases), ManifestSHA256: contentDigest(manifest), SourceSetSHA256: anchor.SourceSetSHA256, EntryProducer: anchor.EntryProducer, GeneratedSHA256: checkedExecution.Generated, ExecutionSHA256: contentDigest(execution), ReplaySHA256: contentDigest(replay), ReplayedFrom: checkedReplay.ReplayedFrom})
		if index == 0 {
			firstReplay = replay
			firstGenerated = checkedExecution.Generated
		}
		start = prepared.End
		index++
	}
	if len(observed) != len(record.Batches) {
		return fail(fmt.Errorf("native shard unfinished or extra batch remains"))
	}
	if reported.State == "VERIFIED" || exhaustiveName != "" && firstReplay != nil {
		if exhaustiveName != "" {
			report, err := inspectCIShardNativeExhaustive(reader, filepath.Join(suiteRel, exhaustiveName), s, firstReplay, firstGenerated)
			if err != nil {
				return fail(err)
			}
			record.Exhaustive = append(record.Exhaustive, report)
		}
	}
	if err := reader.finish(); err != nil {
		return fail(err)
	}
	if len(reported.Batches) > 0 && !reflect.DeepEqual(reported.Batches, record.Batches) {
		return fail(fmt.Errorf("reported native shard batches differ from raw files"))
	}
	if len(reported.Exhaustive) > 0 {
		if len(reported.Exhaustive) != len(record.Exhaustive) {
			return fail(fmt.Errorf("reported exhaustive roster differs from raw files"))
		}
		for index, metadata := range reported.Exhaustive {
			digest, err := ciShardExhaustiveDigest(metadata.Report)
			if err != nil || metadata.Kind != record.Exhaustive[index].Kind || metadata.SHA256 != digest || digest != record.Exhaustive[index].SHA256 {
				return fail(fmt.Errorf("reported exhaustive metadata differs from raw files"))
			}
		}
	}
	if err := validateCIShardSuite(plan, shard, record); err != nil {
		return fail(err)
	}
	return record, nil
}

func ciShardNativeExhaustiveDirectory(s suite) string {
	kind := ciShardExhaustiveKind(ContractSuite{Package: s.Package, Activity: s.Activity})
	switch {
	case kind == "simple-fold":
		return "simple-fold-exhaustive"
	case strings.HasPrefix(kind, "ycbcr/"):
		return "ycbcr-exhaustive-" + s.Activity
	case strings.HasPrefix(kind, "case-mapping/"):
		return "case-mapping-exhaustive-" + s.Activity
	}
	return ""
}

func inspectCIShardNativeExhaustive(reader *ciShardEvidenceReader, dir string, s suite, replay []byte, generated string) (ciShardExhaustive, error) {
	var zero ciShardExhaustive
	kind := ciShardExhaustiveKind(ContractSuite{Package: s.Package, Activity: s.Activity})
	var source string
	var harness []byte
	var expected any
	var module, test string
	var err error
	switch {
	case kind == "simple-fold":
		var function string
		source, function, err = inspectSimpleFoldGenerated(replay, generated)
		if err == nil {
			harness, err = makeSimpleFoldExhaustiveHarness(function, generated)
		}
		expected = expectedSimpleFoldExhaustive(generated)
		module = "gooo-simple-fold-exhaustive"
		test = "TestSimpleFoldExhaustive"
	case strings.HasPrefix(kind, "ycbcr/"):
		var abi ycbcrGeneratedABI
		source, abi, err = inspectYCbCrGenerated(replay, generated, s.Activity)
		if err == nil {
			harness, err = makeYCbCrExhaustiveHarness(abi, generated, s.Activity)
		}
		if err == nil {
			expected, err = expectedYCbCrExhaustive(generated, s.Activity)
		}
		module = "gooo-ycbcr-exhaustive"
		test = "TestYCbCrExhaustive"
	case strings.HasPrefix(kind, "case-mapping/"):
		var function string
		source, function, err = inspectCaseMappingGenerated(replay, generated, s.Activity)
		if err == nil {
			harness, err = makeCaseMappingExhaustiveHarness(function, generated, s.Activity)
		}
		if err == nil {
			expected, err = expectedCaseMappingExhaustive(generated, s.Activity)
		}
		module = "gooo-case-mapping-exhaustive"
		test = "TestCaseMappingExhaustive"
	default:
		return zero, fmt.Errorf("unplanned native exhaustive report")
	}
	if err != nil {
		return zero, err
	}
	for name, wanted := range map[string][]byte{"native/generated.go": []byte(source), "native/exhaustive_test.go": harness, "native/go.mod": []byte("module " + module + "\n\ngo 1.27.3\n")} {
		got, err := reader.read(filepath.Join(dir, filepath.FromSlash(name)), 64<<20)
		if err != nil || !bytes.Equal(got, wanted) {
			return zero, fmt.Errorf("native exhaustive generated source/harness/module differs: %s: %v", name, err)
		}
	}
	log, err := reader.read(filepath.Join(dir, "native-test.txt"), 1<<20)
	if err != nil || !strings.Contains(string(log), "--- PASS: "+test+" ") || !strings.Contains(string(log), "\nPASS\n") || strings.Contains(string(log), "--- FAIL:") || strings.Contains(string(log), "panic:") {
		return zero, fmt.Errorf("native exhaustive diagnostic is incomplete or failed: %v", err)
	}
	raw, err := reader.read(filepath.Join(dir, "exhaustive.json"), 1<<20)
	if err != nil {
		return zero, err
	}
	actual, err := decode(raw)
	if err != nil {
		return zero, err
	}
	wantedRaw, err := json.Marshal(expected)
	if err != nil {
		return zero, err
	}
	wanted, err := decode(wantedRaw)
	if err != nil || !reflect.DeepEqual(actual, wanted) {
		return zero, fmt.Errorf("actual native exhaustive contract differs")
	}
	// Canonical embedded report bytes survive the partial's outer JSON transport.
	canonical, err := json.Marshal(actual)
	if err != nil {
		return zero, err
	}
	digest, err := ciShardExhaustiveDigest(canonical)
	if err != nil {
		return zero, err
	}
	return ciShardExhaustive{Kind: kind, Report: canonical, SHA256: digest}, nil
}
