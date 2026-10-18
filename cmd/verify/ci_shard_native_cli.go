package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

type ciShardNativeOptions struct {
	Requested, CompilerRequested    bool
	ExecuteOut, VerifyOut, Compiler string
	Evidence                        ciShardPaths
}

func (n ciShardNativeOptions) validate(s ciShardOptions, conflicts bool) error {
	if conflicts || s.Context == "" || s.Plan == "" || s.Count != 0 || s.PlanOut != "" || s.SelectOut != "" || s.ReduceOut != "" {
		return fmt.Errorf("explicit native shard mode requires context/plan and no full or metadata output flags")
	}
	if (n.ExecuteOut == "") == (n.VerifyOut == "") {
		return fmt.Errorf("choose exactly one nonempty native shard execute/verify output")
	}
	if n.ExecuteOut != "" {
		if s.Index < 0 || s.Index >= maxCIShards || n.Compiler == "" || len(s.Partials) != 0 || len(n.Evidence) != 0 {
			return fmt.Errorf("native shard execution requires one index/compiler and no partials/evidence")
		}
	} else if s.Index != -1 || len(s.Partials) < 1 || len(s.Partials) > maxCIShards || len(n.Evidence) != len(s.Partials) || n.CompilerRequested {
		return fmt.Errorf("native shard verification requires all partials/evidence and no index/compiler")
	}
	return nil
}

// Receipt inspection proves finite file/value bindings. It does not authenticate
// a GitHub job or prove that the claimed run ID belongs to that job. Keep that
// boundary explicit even after every original batch has been inspected.
type ciShardNativeVerification struct {
	Schema                string     `json:"schema"`
	State                 string     `json:"state"`
	NativeAcceptance      bool       `json:"native_acceptance"`
	RawReceiptsInspected  bool       `json:"raw_receipts_inspected"`
	ProducerAuthenticated bool       `json:"producer_authenticated"`
	PlanSHA256            string     `json:"plan_sha256"`
	Run                   ciShardRun `json:"run"`
	Cases                 int64      `json:"cases"`
	Batches               int64      `json:"batches"`
	Partials              []string   `json:"partial_raw_sha256"`
	Scope                 string     `json:"scope"`
}

// RawMessage reports may be reindented by the enclosing partial writer.
// Compare their decoded values, while preserving exact types and all fields.
func equalCIShardSuiteRecords(a, b ciShardSuiteRecord) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	lv, err := decode(left)
	if err != nil {
		return false
	}
	rv, err := decode(right)
	return err == nil && reflect.DeepEqual(lv, rv)
}

func runCIShardNative(n ciShardNativeOptions, options ciShardOptions, repository string) error {
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	var run ciShardRun
	raw, err := readCIShardFile(options.Context, maxCIShardJSONBytes)
	if err != nil {
		return err
	}
	if err := decodeCIShardJSON(raw, &run); err != nil {
		return err
	}
	if !ciShardRunValid(run) {
		return fmt.Errorf("invalid native shard context")
	}
	var supplied ciShardPlan
	raw, err = readCIShardFile(options.Plan, maxCIShardJSONBytes)
	if err != nil {
		return err
	}
	if err := decodeCIShardJSON(raw, &supplied); err != nil {
		return err
	}
	plan, err := currentCIShardPlan(root, run, len(supplied.Shards))
	if err != nil {
		return err
	}
	if err := validateCIShardPlan(supplied, plan); err != nil {
		return err
	}
	if n.ExecuteOut != "" && options.Index >= len(plan.Shards) {
		return fmt.Errorf("native shard index outside current plan")
	}
	inventory, err := loadCIShardNativeInventory(root, plan)
	if err != nil {
		return err
	}
	if err := validateCIShardCheckout(root, plan); err != nil {
		return err
	}
	if n.ExecuteOut != "" {
		out, err := filepath.Abs(n.ExecuteOut)
		if err != nil {
			return err
		}
		compiler, err := filepath.Abs(n.Compiler)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		partial, executionErr := executeCIShardNative(ctx, compiler, out, plan, options.Index, inventory)
		// Even a failed native run may provide joined, source-bound prefix evidence.
		// Never replace a producer failure with a successful metadata write.
		if partial.Schema != "" {
			if err := writeCurrentCIShardJSON(root, plan, filepath.Join(out, "partial.json"), partial); err != nil {
				if executionErr != nil {
					return fmt.Errorf("%v; preserve partial: %w", executionErr, err)
				}
				return err
			}
		}
		return executionErr
	}
	partials := make([]ciShardPartial, 0, len(options.Partials))
	digests := []string{}
	evidence := map[int]string{}
	seenFiles := map[string]bool{}
	seenDirectories := map[string]bool{}
	for i, name := range options.Partials {
		path, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		dir, err := filepath.Abs(n.Evidence[i])
		if err != nil {
			return err
		}
		if seenFiles[path] || seenDirectories[dir] {
			return fmt.Errorf("duplicate native shard input/evidence path")
		}
		seenFiles[path], seenDirectories[dir] = true, true
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("regular native shard evidence directory required")
		}
		raw, err := readCIShardFile(path, maxCIShardJSONBytes)
		if err != nil {
			return err
		}
		var partial ciShardPartial
		if err := decodeCIShardJSON(raw, &partial); err != nil {
			return err
		}
		if _, ok := evidence[partial.Index]; ok {
			return fmt.Errorf("duplicate native shard index")
		}
		partials = append(partials, partial)
		digests = append(digests, contentDigest(raw))
		evidence[partial.Index] = dir
		reader, err := newCIShardEvidenceReader(dir)
		if err != nil {
			return err
		}
		identity, err := reader.read("compiler.json", MaxContractBytes)
		if err != nil {
			return err
		}
		if err := validateCIShardCompilerIdentity(identity, inventory.Lock); err != nil {
			return err
		}
		if err := reader.finish(); err != nil {
			return err
		}
	}
	// This validates every joined shard and original index before any native
	// evidence path is selected. Its success still carries no native acceptance.
	reduced, err := reduceCIShardPartials(plan, partials)
	if err != nil {
		return err
	}
	for _, partial := range partials {
		for _, reported := range partial.Suites {
			observed, err := inspectCIShardSuiteEvidence(plan, partial.Index, reported.OriginalIndex, inventory, evidence[partial.Index], reported)
			if err != nil {
				return fmt.Errorf("shard%d original%d: %w", partial.Index, reported.OriginalIndex, err)
			}
			if !equalCIShardSuiteRecords(observed, reported) {
				return fmt.Errorf("actual shard evidence differs from partial metadata")
			}
		}
	}
	sort.Strings(digests)
	result := ciShardNativeVerification{Schema: "gooo/native-ci-shard-native-verification/v1", State: "NATIVE_RECEIPTS_CHECKED_PRODUCER_UNAUTHENTICATED", NativeAcceptance: false, RawReceiptsInspected: true, ProducerAuthenticated: false, PlanSHA256: plan.PlanSHA256, Run: run, Cases: reduced.Cases, Batches: reduced.Batches, Partials: digests, Scope: "All current original case bytes, manifests, source copies and native/replay receipts independently inspected. Trusted GitHub run/job/artifact authentication and original required-gate integration remain separate; no native acceptance or speedup claim."}
	return writeCurrentCIShardJSON(root, plan, n.VerifyOut, result)
}
