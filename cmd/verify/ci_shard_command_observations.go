package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"
)

const ciShardCommandObservationsSchema = "gooo/native-shard-command-observations/v1"

// These records describe actual wait callbacks, including incomplete tails.
// NOT_OBSERVED is never converted to a claim that accounting was unavailable.
type ciShardCommandObservationBatch struct {
	Index            int                         `json:"index"`
	Namespace        string                      `json:"namespace"`
	VerifiedPrefix   bool                        `json:"verified_prefix"`
	CommandResources nativeBatchCommandResources `json:"command_resources"`
}

type ciShardCommandObservationSuite struct {
	OriginalIndex int                              `json:"original_index"`
	State         string                           `json:"state"`
	Batches       []ciShardCommandObservationBatch `json:"batches"`
}

type ciShardCommandObservationDocument struct {
	Schema                 string                           `json:"schema"`
	PlanSHA256             string                           `json:"plan_sha256"`
	Run                    ciShardRun                       `json:"run"`
	Index                  int                              `json:"index"`
	Namespace              string                           `json:"namespace"`
	Sources                map[string]ciShardSource         `json:"sources"`
	GOOS                   string                           `json:"goos"`
	CommandResourcesSchema string                           `json:"command_resources_schema"`
	Joined                 bool                             `json:"joined"`
	Complete               bool                             `json:"complete"`
	State                  string                           `json:"state"`
	Failure                string                           `json:"failure"`
	NativeAcceptance       bool                             `json:"native_acceptance"`
	ProducerAuthenticated  bool                             `json:"producer_authenticated"`
	Suites                 []ciShardCommandObservationSuite `json:"suites"`
	Scope                  string                           `json:"scope"`
}

type ciShardCommandObservations struct {
	mu       sync.Mutex
	plan     ciShardPlan
	shard    int
	closed   bool
	failures []error
	suites   map[int]*ciShardCommandObservationSuite
}

func newCIShardCommandObservations(plan ciShardPlan, shard int) (*ciShardCommandObservations, error) {
	if err := validateCIShardPlanShape(plan); err != nil {
		return nil, err
	}
	if shard < 0 || shard >= len(plan.Shards) {
		return nil, fmt.Errorf("command observations shard outside plan")
	}
	// Detach caller-owned source maps, assignments and nested workspace slices.
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	var frozen ciShardPlan
	if err := json.Unmarshal(raw, &frozen); err != nil {
		return nil, err
	}
	c := &ciShardCommandObservations{plan: frozen, shard: shard, suites: map[int]*ciShardCommandObservationSuite{}}
	for _, original := range frozen.Shards[shard].OriginalIndices {
		s := &ciShardCommandObservationSuite{OriginalIndex: original, State: "NOT_STARTED", Batches: []ciShardCommandObservationBatch{}}
		for batch := 0; int64(batch) < frozen.Suites[original].Batches; batch++ {
			s.Batches = append(s.Batches, ciShardCommandObservationBatch{Index: batch, Namespace: ciShardBatchNamespace(shard, original, batch),
				CommandResources: nativeBatchCommandResources{Execute: unobservedNativeCommandResources(), Replay: unobservedNativeCommandResources()}})
		}
		c.suites[original] = s
	}
	return c, nil
}

func validateCIShardCommandResources(r nativeCommandResources, goos string) error {
	if r.GOOS != goos || r.Scope != "waited-command-accounting" {
		return fmt.Errorf("command accounting GOOS/scope differs")
	}
	if r.Status == "NOT_AVAILABLE" {
		if strings.TrimSpace(r.Reason) == "" || len(r.Reason) > 4096 || !utf8.ValidString(r.Reason) || r.UserCPUNS != nil || r.SystemCPUNS != nil || r.MaxRSSRaw != nil || r.MaxRSSBytes != nil || r.MaxRSSUnit != nil {
			return fmt.Errorf("unavailable command accounting requires reason and null metrics")
		}
		return nil
	}
	if r.Status != "OBSERVED" || r.Reason != "" || r.UserCPUNS == nil || r.SystemCPUNS == nil || r.MaxRSSRaw == nil || r.MaxRSSBytes == nil || r.MaxRSSUnit == nil || *r.UserCPUNS < 0 || *r.SystemCPUNS < 0 || *r.MaxRSSRaw < 0 || *r.MaxRSSBytes < 0 {
		return fmt.Errorf("command accounting requires an actual observed/unavailable callback")
	}
	unit, scale := "", int64(1)
	switch goos {
	case "linux":
		unit, scale = "KiB", 1024
	case "darwin":
		unit = "bytes"
	default:
		return fmt.Errorf("observed command accounting has unsupported GOOS")
	}
	if *r.MaxRSSUnit != unit || *r.MaxRSSRaw > math.MaxInt64/scale || *r.MaxRSSBytes != *r.MaxRSSRaw*scale {
		return fmt.Errorf("command accounting RSS unit/conversion differs")
	}
	return nil
}

func cloneCIShardCommandResources(r nativeCommandResources) nativeCommandResources {
	copyInt := func(p *int64) *int64 {
		if p == nil {
			return nil
		}
		v := *p
		return &v
	}
	r.UserCPUNS, r.SystemCPUNS = copyInt(r.UserCPUNS), copyInt(r.SystemCPUNS)
	r.MaxRSSRaw, r.MaxRSSBytes = copyInt(r.MaxRSSRaw), copyInt(r.MaxRSSBytes)
	if r.MaxRSSUnit != nil {
		value := *r.MaxRSSUnit
		r.MaxRSSUnit = &value
	}
	return r
}

func (c *ciShardCommandObservations) observer(original int) func(int, string, nativeCommandResources) {
	return func(batch int, phase string, resources nativeCommandResources) {
		c.mu.Lock()
		defer c.mu.Unlock()
		fail := func(message string) {
			c.failures = append(c.failures, fmt.Errorf("suite%d batch%d %s: %s", original, batch, phase, message))
		}
		if c.closed {
			fail("callback after command observations closed")
			return
		}
		s, ok := c.suites[original]
		if !ok || batch < 0 || batch >= len(s.Batches) || (phase != "execute" && phase != "replay") {
			fail("foreign original/batch/phase")
			return
		}
		if err := validateCIShardCommandResources(resources, runtime.GOOS); err != nil {
			fail(err.Error())
			return
		}
		if phase == "replay" && s.Batches[batch].CommandResources.Execute.Status == "NOT_OBSERVED" {
			fail("replay wait callback precedes execute")
			return
		}
		target := &s.Batches[batch].CommandResources.Execute
		if phase == "replay" {
			target = &s.Batches[batch].CommandResources.Replay
		}
		if target.Status != "NOT_OBSERVED" {
			fail("duplicate wait callback")
			return
		}
		*target = cloneCIShardCommandResources(resources)
	}
}

// The caller must join every callback before finishing. Even failure documents
// retain actual observations and all unobserved tails; only complete successful
// partials can yield a complete accounting document, never native acceptance.
func (c *ciShardCommandObservations) finish(partial ciShardPartial) (ciShardCommandObservationDocument, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fail := func(message string) { c.failures = append(c.failures, errors.New(message)) }
	if c.closed {
		fail("command observations already finished")
	}
	c.closed = true
	a := c.plan.Shards[c.shard]
	doc := ciShardCommandObservationDocument{Schema: ciShardCommandObservationsSchema, PlanSHA256: c.plan.PlanSHA256, Run: c.plan.Run, Index: c.shard, Namespace: a.Namespace,
		Sources: c.plan.Sources, GOOS: runtime.GOOS, CommandResourcesSchema: nativeCommandResourcesSchema, State: "INCOMPLETE", Suites: []ciShardCommandObservationSuite{},
		Scope: "Actual execute/replay waited-command accounting only. Joined callbacks and verified prefixes are checked locally; no authenticated job, full native acceptance, descendant census, simultaneous memory bound or speedup claim."}
	valid := partial.Schema == ciShardPartialSchema && partial.PlanSHA256 == c.plan.PlanSHA256 && partial.Run == c.plan.Run && partial.Index == c.shard && partial.Namespace == a.Namespace && reflect.DeepEqual(partial.Sources, c.plan.Sources) && len(partial.Suites) == len(a.OriginalIndices)
	if !valid {
		fail("command observations partial identity/source/roster differs")
	}
	doc.Joined = valid && partial.Joined
	if !doc.Joined {
		fail("command observations require all callbacks joined")
	}
	complete := valid && partial.Joined && partial.State == "SUCCEEDED"
	for local, original := range a.OriginalIndices {
		s := c.suites[original]
		if valid {
			record := partial.Suites[local]
			if record.OriginalIndex != original {
				fail("command observations original order differs")
				complete = false
			} else if err := validateCIShardSuite(c.plan, c.shard, record); err != nil {
				fail("command observations invalid suite: " + err.Error())
				complete = false
			} else {
				s.State = record.State
				if record.State != "VERIFIED" {
					complete = false
				}
				for _, prefix := range record.Batches {
					batch := &s.Batches[prefix.Index]
					batch.VerifiedPrefix = true
					if batch.CommandResources.Execute.Status == "NOT_OBSERVED" || batch.CommandResources.Replay.Status == "NOT_OBSERVED" {
						fail(fmt.Sprintf("suite%d batch%d verified prefix lacks execute/replay wait coverage", original, prefix.Index))
					}
				}
				if record.State == "NOT_STARTED" {
					for _, batch := range s.Batches {
						if batch.CommandResources.Execute.Status != "NOT_OBSERVED" || batch.CommandResources.Replay.Status != "NOT_OBSERVED" {
							fail("not-started suite has command observations")
						}
					}
				}
			}
		}
		doc.Suites = append(doc.Suites, *s)
	}
	if !complete {
		fail("native shard command observations remain incomplete")
	}
	doc.Complete = complete && len(c.failures) == 0
	if doc.Complete {
		doc.State = "WAIT_ACCOUNTING_COMPLETE_NATIVE_UNACCEPTED"
	}
	joined := errors.Join(c.failures...)
	if joined != nil {
		doc.Failure = ciShardExecutionDiagnostic(joined)
	}
	// Do not expose the collector's retained maps, slices or metric pointers.
	raw, err := json.Marshal(doc)
	if err != nil {
		return ciShardCommandObservationDocument{}, errors.Join(joined, err)
	}
	var detached ciShardCommandObservationDocument
	if err := json.Unmarshal(raw, &detached); err != nil {
		return ciShardCommandObservationDocument{}, errors.Join(joined, err)
	}
	return detached, joined
}

// Decode the raw document with decodeCIShardJSON before this typed validator:
// JSON duplicate keys and missing/unknown fields must not disappear in decoding.
// Incomplete documents are retained diagnostics, never complete coverage proofs.
func validateCIShardCommandObservationDocument(plan ciShardPlan, partial ciShardPartial, doc ciShardCommandObservationDocument) error {
	c, err := newCIShardCommandObservations(plan, partial.Index)
	if err != nil {
		return err
	}
	assignment := plan.Shards[partial.Index]
	if doc.Schema != ciShardCommandObservationsSchema || doc.PlanSHA256 != plan.PlanSHA256 || doc.Run != plan.Run || doc.Index != partial.Index || doc.Namespace != assignment.Namespace || doc.GOOS != runtime.GOOS || doc.CommandResourcesSchema != nativeCommandResourcesSchema || !reflect.DeepEqual(doc.Sources, plan.Sources) || len(doc.Suites) != len(assignment.OriginalIndices) {
		return fmt.Errorf("command observation document plan/run/source/shard/GOOS differs")
	}
	for local, original := range assignment.OriginalIndices {
		s := doc.Suites[local]
		if s.OriginalIndex != original || int64(len(s.Batches)) != plan.Suites[original].Batches {
			return fmt.Errorf("command observation document missing/duplicate/foreign suite or batch")
		}
		for index, batch := range s.Batches {
			if batch.Index != index || batch.Namespace != ciShardBatchNamespace(partial.Index, original, index) {
				return fmt.Errorf("command observation document duplicate/foreign batch namespace")
			}
			for _, phase := range []string{"execute", "replay"} {
				r := batch.CommandResources.Execute
				if phase == "replay" {
					r = batch.CommandResources.Replay
				}
				if r.Status == "NOT_OBSERVED" {
					if !reflect.DeepEqual(r, unobservedNativeCommandResources()) {
						return fmt.Errorf("unobserved command metrics were fabricated")
					}
					continue
				}
				c.observer(original)(index, phase, r)
			}
		}
	}
	expected, err := c.finish(partial)
	if err != nil {
		return fmt.Errorf("command observation document lacks complete actual coverage: %w", err)
	}
	if !reflect.DeepEqual(doc, expected) {
		return fmt.Errorf("command observation document state/prefix/accounting fields differ")
	}
	return nil
}
