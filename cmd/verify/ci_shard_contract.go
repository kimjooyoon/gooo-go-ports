package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const (
	ciShardPlanSchema      = "gooo/native-ci-shard-plan/v1"
	ciShardPartialSchema   = "gooo/native-ci-shard-partial/v1"
	ciShardReductionSchema = "gooo/native-ci-shard-reduction/v1"
	maxCIShards            = 16
	maxCIShardJSONBytes    = 16 << 20
)

// These additions bind the new reader separately; the existing full-mode ledger
// and its historical guards are not rewritten to introduce an optional mode.
var ciShardReaderSources = []string{
	"cmd/verify/ci_shard_contract.go", "cmd/verify/ci_shard_cli.go", "cmd/verify/ci_shard_contract_test.go",
	"cmd/verify/ci_shard_native_types.go", "cmd/verify/ci_shard_execute.go", "cmd/verify/ci_shard_execute_test.go",
	"cmd/verify/ci_shard_native_evidence.go", "cmd/verify/ci_shard_native_evidence_test.go",
	"cmd/verify/ci_shard_native_cli.go", "cmd/verify/ci_shard_native_cli_test.go",
	"cmd/verify/ci_shard_native_smoke_test.go",
}
var ciShardRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var ciShardDecimalPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

type ciShardRun struct {
	Repository       string `json:"repository"`
	RunID            string `json:"run_id"`
	Attempt          int    `json:"attempt"`
	TestedCommit     string `json:"tested_commit"`
	TestedTree       string `json:"tested_tree"`
	CompilerRevision string `json:"compiler_revision"`
	GoVersion        string `json:"go_version"`
	Workers          int    `json:"workers"`
}

type ciShardSource struct {
	Mode   string `json:"mode"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type ciShardSourceAnchor struct {
	Name      string         `json:"name"`
	RawSHA256 string         `json:"raw_sha256"`
	Identity  sourceIdentity `json:"identity"`
}

type ciShardWorkspace struct {
	OriginalIndex   int                   `json:"original_index"`
	ManifestSHA256  string                `json:"manifest_sha256"`
	SourceSetSHA256 string                `json:"source_set_sha256"`
	EntryProducer   string                `json:"entry_producer"`
	Sources         []ciShardSourceAnchor `json:"sources"`
}

type ciShardAssignment struct {
	Index           int    `json:"index"`
	Namespace       string `json:"namespace"`
	OriginalIndices []int  `json:"original_indices"`
}

type ciShardPlan struct {
	Schema         string                   `json:"schema"`
	PlanSHA256     string                   `json:"plan_sha256"`
	ContractSHA256 string                   `json:"contract_sha256"`
	Run            ciShardRun               `json:"run"`
	Profiles       ContractProfiles         `json:"profiles"`
	Sources        map[string]ciShardSource `json:"sources"`
	Suites         []ContractSuite          `json:"suites"`
	Workspaces     []ciShardWorkspace       `json:"workspaces"`
	Shards         []ciShardAssignment      `json:"shards"`
	Cases          int64                    `json:"cases"`
	Batches        int64                    `json:"batches"`
}

type ciShardBatch struct {
	Index           int    `json:"index"`
	Start           int64  `json:"start"`
	End             int64  `json:"end"`
	Namespace       string `json:"namespace"`
	CasesSHA256     string `json:"cases_sha256"`
	ManifestSHA256  string `json:"manifest_sha256"`
	SourceSetSHA256 string `json:"source_set_sha256"`
	EntryProducer   string `json:"entry_producer"`
	GeneratedSHA256 string `json:"generated_sha256"`
	ExecutionSHA256 string `json:"execution_sha256"`
	ReplaySHA256    string `json:"replay_sha256"`
	ReplayedFrom    string `json:"replayed_from"`
}

type ciShardExhaustive struct {
	Kind   string          `json:"kind"`
	Report json.RawMessage `json:"report"`
	SHA256 string          `json:"sha256"`
}

type ciShardSuiteRecord struct {
	OriginalIndex int                 `json:"original_index"`
	Package       string              `json:"package"`
	Activity      string              `json:"activity"`
	State         string              `json:"state"`
	Error         string              `json:"error"`
	Batches       []ciShardBatch      `json:"batches"`
	Exhaustive    []ciShardExhaustive `json:"exhaustive"`
}

type ciShardPartial struct {
	Schema     string                   `json:"schema"`
	PlanSHA256 string                   `json:"plan_sha256"`
	Run        ciShardRun               `json:"run"`
	Index      int                      `json:"index"`
	Namespace  string                   `json:"namespace"`
	Sources    map[string]ciShardSource `json:"sources"`
	Joined     bool                     `json:"joined"`
	State      string                   `json:"state"`
	Suites     []ciShardSuiteRecord     `json:"suites"`
}

// This is intentionally not the existing differential summary or timing footer.
// Metadata hashes do not replace inspect, inspectReplayBinding, fresh execution,
// or the existing full-summary checks. No first-stage reducer can assert PASS.
type ciShardReduction struct {
	Schema               string               `json:"schema"`
	State                string               `json:"state"`
	NativeAcceptance     bool                 `json:"native_acceptance"`
	RawReceiptsInspected bool                 `json:"raw_receipts_inspected"`
	PlanSHA256           string               `json:"plan_sha256"`
	Run                  ciShardRun           `json:"run"`
	Cases                int64                `json:"cases"`
	Batches              int64                `json:"batches"`
	Suites               []ciShardSuiteRecord `json:"suites"`
}

type ciShardFailure struct {
	OriginalIndex int
	Message       string
}

func (e *ciShardFailure) Error() string {
	return fmt.Sprintf("CI shard suite %d: %s", e.OriginalIndex, e.Message)
}

func ciShardNamespace(index int) string { return fmt.Sprintf("native-ci-shard-%03d", index) }
func ciShardBatchNamespace(shard, suite, batch int) string {
	return fmt.Sprintf("%s/suite-%04d/batch-%06d", ciShardNamespace(shard), suite, batch)
}

func ciShardRunValid(r ciShardRun) bool {
	return ciShardRepositoryPattern.MatchString(r.Repository) && !strings.Contains(r.Repository, "..") &&
		ciShardDecimalPattern.MatchString(r.RunID) && r.Attempt > 0 && r.Attempt <= 1000000 &&
		commitPattern.MatchString(r.TestedCommit) && commitPattern.MatchString(r.TestedTree) &&
		commitPattern.MatchString(r.CompilerRevision) && r.GoVersion == "go1.27.2" && r.Workers == 3
}

func ciShardExpectedSourceModes(c *NativeVerificationContract) (map[string]string, error) {
	m := map[string]string{}
	for _, s := range c.Sources {
		if !safeContractPath(s.Path) || m[s.Path] != "" || (s.Mode != "100644" && s.Mode != "100755") {
			return nil, fmt.Errorf("invalid shard source roster")
		}
		m[s.Path] = s.Mode
	}
	for _, name := range ciShardReaderSources {
		if m[name] != "" {
			return nil, fmt.Errorf("repeated shard reader source")
		}
		m[name] = "100644"
	}
	return m, nil
}

func ciShardPlanDigest(p ciShardPlan) (string, error) {
	p.PlanSHA256 = ""
	return canonicalContractDigest(p)
}

// Partition full, independently loaded ledger entries. The algorithm is fixed:
// largest batch count first; tie by cases/index; place in the least-batch shard.
// The plan is workload metadata, never an execution result or timing forecast.
func newCIShardPlan(c *NativeVerificationContract, contractSHA string, run ciShardRun, count int,
	sources map[string]ciShardSource, workspaces []ciShardWorkspace) (ciShardPlan, error) {
	var zero ciShardPlan
	if c == nil || !ciShardRunValid(run) || !digestPattern.MatchString(contractSHA) || count < 1 || count > maxCIShards || count > len(c.Suites) || len(workspaces) != len(c.Suites) || len(c.Suites) == 0 {
		return zero, fmt.Errorf("invalid CI shard plan inputs")
	}
	modes, err := ciShardExpectedSourceModes(c)
	if err != nil || len(modes) != len(sources) {
		return zero, fmt.Errorf("CI shard source set differs: %v", err)
	}
	for name, mode := range modes {
		s, found := sources[name]
		if !found || s.Mode != mode || s.Bytes < 1 || !digestPattern.MatchString(s.SHA256) {
			return zero, fmt.Errorf("CI shard source binding differs: %s", name)
		}
	}
	p := ciShardPlan{Schema: ciShardPlanSchema, ContractSHA256: contractSHA, Run: run, Profiles: c.Profiles, Sources: sources, Suites: c.Suites, Workspaces: workspaces}
	seenEntries := map[string]bool{}
	for i, s := range c.Suites {
		entry := s.Package + ":" + s.Activity
		if s.Index != int64(i) || s.Package == "" || s.Activity == "" || seenEntries[entry] || s.Cases < 1 || s.Batches < 1 || s.Batches > s.Cases || len(s.SourcePaths) == 0 {
			return zero, fmt.Errorf("invalid original shard suite %d", i)
		}
		seenEntries[entry] = true
		p.Cases, err = checkedContractAdd(p.Cases, s.Cases)
		if err != nil {
			return zero, err
		}
		p.Batches, err = checkedContractAdd(p.Batches, s.Batches)
		if err != nil {
			return zero, err
		}
		w := workspaces[i]
		if w.OriginalIndex != i || !digestPattern.MatchString(w.ManifestSHA256) || !strings.HasPrefix(w.EntryProducer, "gooo://") || len(w.Sources) != len(s.SourcePaths) {
			return zero, fmt.Errorf("workspace anchor differs at original suite %d", i)
		}
		seenSources := map[string]bool{}
		for j, name := range s.SourcePaths {
			a := w.Sources[j]
			source, found := sources[name]
			if !found || seenSources[name] || a.Name != name || a.RawSHA256 != source.SHA256 || a.Identity.Package == "" || !digestPattern.MatchString(a.Identity.Digest) {
				return zero, fmt.Errorf("source anchor differs at original suite %d", i)
			}
			seenSources[name] = true
		}
		anchorSHA, err := canonicalContractDigest(w.Sources)
		if err != nil || anchorSHA != w.SourceSetSHA256 {
			return zero, fmt.Errorf("source anchor hash differs at original suite %d", i)
		}
	}
	if p.Cases != c.TotalCases || p.Batches != c.TotalBatches {
		return zero, fmt.Errorf("CI shard ledger totals differ")
	}
	for i := 0; i < count; i++ {
		p.Shards = append(p.Shards, ciShardAssignment{Index: i, Namespace: ciShardNamespace(i), OriginalIndices: []int{}})
	}
	indices := make([]int, len(c.Suites))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		a, b := c.Suites[indices[i]], c.Suites[indices[j]]
		if a.Batches != b.Batches {
			return a.Batches > b.Batches
		}
		if a.Cases != b.Cases {
			return a.Cases > b.Cases
		}
		return a.Index < b.Index
	})
	loads := make([]int64, count)
	for _, i := range indices {
		least := 0
		for j := 1; j < count; j++ {
			if loads[j] < loads[least] {
				least = j
			}
		}
		p.Shards[least].OriginalIndices = append(p.Shards[least].OriginalIndices, i)
		loads[least], err = checkedContractAdd(loads[least], c.Suites[i].Batches)
		if err != nil {
			return zero, err
		}
	}
	for i := range p.Shards {
		sort.Ints(p.Shards[i].OriginalIndices)
	}
	p.PlanSHA256, err = ciShardPlanDigest(p)
	if err != nil {
		return zero, err
	}
	// Detach maps/slices from caller mutation after hashing.
	raw, err := json.Marshal(p)
	if err != nil {
		return zero, err
	}
	var detached ciShardPlan
	if err = json.Unmarshal(raw, &detached); err != nil {
		return zero, err
	}
	return detached, nil
}

func validateCIShardPlan(p, expected ciShardPlan) error {
	digest, err := ciShardPlanDigest(p)
	if err != nil || p.Schema != ciShardPlanSchema || digest != p.PlanSHA256 || !reflect.DeepEqual(p, expected) {
		return fmt.Errorf("CI shard plan differs from independent current-source plan")
	}
	return nil
}

func validateCIShardPlanShape(p ciShardPlan) error {
	readers := map[string]bool{}
	for _, name := range ciShardReaderSources {
		readers[name] = true
	}
	c := &NativeVerificationContract{Profiles: p.Profiles, Suites: p.Suites, TotalCases: p.Cases, TotalBatches: p.Batches}
	names := []string{}
	for name := range p.Sources {
		if !readers[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		c.Sources = append(c.Sources, ContractSource{Path: name, Mode: p.Sources[name].Mode})
	}
	expected, err := newCIShardPlan(c, p.ContractSHA256, p.Run, len(p.Shards), p.Sources, p.Workspaces)
	if err != nil {
		return err
	}
	return validateCIShardPlan(p, expected)
}

func ciShardExhaustiveKind(s ContractSuite) string {
	switch s.Package + ":" + s.Activity {
	case "go/unicode:SimpleFold":
		return "simple-fold"
	case "go/image/color:RGBToYCbCr", "go/image/color:YCbCrToRGB":
		return "ycbcr/" + s.Activity
	case "go/unicode:To", "go/unicode:ToUpper", "go/unicode:ToLower", "go/unicode:ToTitle":
		return "case-mapping/" + s.Activity
	}
	return ""
}

// The enclosing partial writer may indent RawMessage values. Bind the decoded
// report independently of lexical whitespace; its actual artifact is read and
// compared separately by the native evidence reducer.
func ciShardExhaustiveDigest(raw json.RawMessage) (string, error) {
	if err := decimalPayloadJSON(raw); err != nil {
		return "", err
	}
	value, err := decode(raw)
	if err != nil {
		return "", err
	}
	return canonicalContractDigest(value)
}

func validateCIShardExhaustive(s ContractSuite, batches []ciShardBatch, reports []ciShardExhaustive, required bool) error {
	kind := ciShardExhaustiveKind(s)
	if len(reports) > 1 || (kind == "" && len(reports) != 0) || (required && kind != "" && len(reports) != 1) {
		return fmt.Errorf("exhaustive report roster differs")
	}
	if len(reports) == 0 {
		return nil
	}
	r := reports[0]
	digest, err := ciShardExhaustiveDigest(r.Report)
	if err != nil || len(batches) == 0 || r.Kind != kind || digest != r.SHA256 {
		return fmt.Errorf("exhaustive report binding differs")
	}
	generated := batches[0].GeneratedSHA256
	var expected any
	switch {
	case kind == "simple-fold":
		expected = expectedSimpleFoldExhaustive(generated)
	case strings.HasPrefix(kind, "ycbcr/"):
		v, err := expectedYCbCrExhaustive(generated, s.Activity)
		if err != nil {
			return err
		}
		expected = v
	default:
		v, err := expectedCaseMappingExhaustive(generated, s.Activity)
		if err != nil {
			return err
		}
		expected = v
	}
	got, err := decode(r.Report)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	want, err := decode(raw)
	if err != nil || !reflect.DeepEqual(got, want) {
		return fmt.Errorf("exhaustive report fields differ from original contract")
	}
	return nil
}

func validateCIShardSuite(p ciShardPlan, shard int, r ciShardSuiteRecord) error {
	s := p.Suites[r.OriginalIndex]
	w := p.Workspaces[r.OriginalIndex]
	if r.Package != s.Package || r.Activity != s.Activity || int64(len(r.Batches)) > s.Batches {
		return fmt.Errorf("suite identity or batch population differs")
	}
	cursor := int64(0)
	for i, b := range r.Batches {
		if b.Index != i || b.Start != cursor || b.End <= b.Start || b.End > s.Cases || b.Namespace != ciShardBatchNamespace(shard, r.OriginalIndex, i) ||
			b.ManifestSHA256 != w.ManifestSHA256 || b.SourceSetSHA256 != w.SourceSetSHA256 || b.EntryProducer != w.EntryProducer ||
			!digestPattern.MatchString(b.CasesSHA256) || !digestPattern.MatchString(b.GeneratedSHA256) || !digestPattern.MatchString(b.ExecutionSHA256) || !digestPattern.MatchString(b.ReplaySHA256) || b.ReplayedFrom != b.ExecutionSHA256 {
			return fmt.Errorf("original batch interval/workspace/source/producer/receipt binding differs")
		}
		cursor = b.End
	}
	switch r.State {
	case "VERIFIED":
		if r.Error != "" || cursor != s.Cases || int64(len(r.Batches)) != s.Batches {
			return fmt.Errorf("verified suite incomplete")
		}
	case "FAILED":
		if len(r.Error) < 1 || len(r.Error) > 4096 {
			return fmt.Errorf("failed suite requires bounded diagnosis")
		}
	case "CANCELED", "NOT_STARTED":
		if r.Error != "" || r.State == "NOT_STARTED" && len(r.Batches) != 0 {
			return fmt.Errorf("invalid uncompleted suite state")
		}
	default:
		return fmt.Errorf("unknown suite state")
	}
	return validateCIShardExhaustive(s, r.Batches, r.Exhaustive, r.State == "VERIFIED")
}

// Callers must obtain the expected plan independently from current source and
// wait for all jobs. A failed shard does not authorize cancellation of other
// runners: this explicit mode chooses the lowest original failure after joins.
func reduceCIShardPartials(expected ciShardPlan, partials []ciShardPartial) (ciShardReduction, error) {
	var zero ciShardReduction
	if err := validateCIShardPlanShape(expected); err != nil {
		return zero, fmt.Errorf("invalid independent shard plan: %w", err)
	}
	if len(partials) != len(expected.Shards) || len(partials) == 0 {
		return zero, fmt.Errorf("all planned shards are required")
	}
	seen := map[int]bool{}
	records := make([]ciShardSuiteRecord, len(expected.Suites))
	lowest := len(expected.Suites)
	message := ""
	for _, part := range partials {
		if part.Index < 0 || part.Index >= len(expected.Shards) || seen[part.Index] {
			return zero, fmt.Errorf("missing, repeated or foreign shard index")
		}
		seen[part.Index] = true
		a := expected.Shards[part.Index]
		if part.Schema != ciShardPartialSchema || part.PlanSHA256 != expected.PlanSHA256 || part.Run != expected.Run || part.Namespace != a.Namespace || !reflect.DeepEqual(part.Sources, expected.Sources) || !part.Joined || len(part.Suites) != len(a.OriginalIndices) {
			return zero, fmt.Errorf("shard producer/source/namespace/completion differs")
		}
		localFailure := len(expected.Suites)
		for i, record := range part.Suites {
			if record.OriginalIndex != a.OriginalIndices[i] {
				return zero, fmt.Errorf("missing, duplicate or foreign original suite index")
			}
			if err := validateCIShardSuite(expected, part.Index, record); err != nil {
				return zero, fmt.Errorf("suite %d: %w", record.OriginalIndex, err)
			}
			records[record.OriginalIndex] = record
			if record.State == "FAILED" && record.OriginalIndex < localFailure {
				localFailure = record.OriginalIndex
			}
			if record.State == "FAILED" && record.OriginalIndex < lowest {
				lowest = record.OriginalIndex
				message = record.Error
			}
		}
		switch part.State {
		case "SUCCEEDED":
			for _, record := range part.Suites {
				if record.State != "VERIFIED" {
					return zero, fmt.Errorf("successful shard contains unverified suite")
				}
			}
		case "FAILED":
			if localFailure == len(expected.Suites) {
				return zero, fmt.Errorf("failed shard has no original failure")
			}
			for _, record := range part.Suites {
				if record.OriginalIndex < localFailure && record.State != "VERIFIED" {
					return zero, fmt.Errorf("earlier catalog suite was not completed before failure")
				}
			}
		default:
			return zero, fmt.Errorf("incomplete or unknown shard state")
		}
	}
	if lowest < len(expected.Suites) {
		return zero, &ciShardFailure{lowest, message}
	}
	return ciShardReduction{Schema: ciShardReductionSchema, State: "CONTRACT_COMPLETE_NATIVE_UNCHECKED", NativeAcceptance: false, RawReceiptsInspected: false, PlanSHA256: expected.PlanSHA256, Run: expected.Run, Cases: expected.Cases, Batches: expected.Batches, Suites: records}, nil
}

// Compare decoded shapes after typed decoding too: encoding/json otherwise
// accepts case-insensitive field aliases and absent required zero-valued fields.
func decodeCIShardJSON(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > maxCIShardJSONBytes {
		return fmt.Errorf("bounded nonempty CI shard JSON required")
	}
	if err := decimalPayloadJSON(raw); err != nil {
		return err
	}
	original, err := decode(raw)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return err
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return err
	}
	canonical, err := decode(encoded)
	if err != nil || !reflect.DeepEqual(original, canonical) {
		return fmt.Errorf("CI shard JSON has missing, unknown, aliased or noncanonical integer fields")
	}
	return nil
}
