package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxNativeSuiteWorkers = 4
const defaultNativeSuiteTimeout = 30 * time.Minute

func validateNativeSuiteTimeout(timeout time.Duration) error {
	if timeout < time.Second || timeout > 60*time.Minute {
		return fmt.Errorf("--timeout must be between 1s and 60m")
	}
	return nil
}

func addNativeExecutionSummary(summary map[string]any, requested, failFast bool, timeout time.Duration) {
	if !requested {
		return
	}
	selection := "earliest-catalog-index"
	if failFast {
		selection = "first-completed-uncanceled"
	}
	summary["native_fail_fast"] = failFast
	summary["native_timeout_ns"] = int64(timeout)
	summary["native_failure_selection"] = selection
}

type nativeSuiteOrder string

const (
	nativeSuiteOrderCatalog            nativeSuiteOrder = "catalog"
	nativeSuiteOrderLargestCorpusFirst nativeSuiteOrder = "largest-corpus-first"
)

func parseNativeSuiteOrder(value string) (nativeSuiteOrder, error) {
	switch nativeSuiteOrder(value) {
	case "", nativeSuiteOrderCatalog:
		return nativeSuiteOrderCatalog, nil
	case nativeSuiteOrderLargestCorpusFirst:
		return nativeSuiteOrderLargestCorpusFirst, nil
	default:
		return "", fmt.Errorf("unknown native suite order %q", value)
	}
}

// Only dispatch indices change. The suites, Cases and original ledger indices
// remain untouched; reading their lengths does not serialize case documents.
func nativeSuiteDispatchOrder(suites []suite, order nativeSuiteOrder) ([]int, error) {
	if order != nativeSuiteOrderCatalog && order != nativeSuiteOrderLargestCorpusFirst {
		return nil, fmt.Errorf("unknown native suite order %q", order)
	}
	indices := make([]int, len(suites))
	for index := range suites {
		indices[index] = index
	}
	if order == nativeSuiteOrderLargestCorpusFirst {
		sort.Slice(indices, func(i, j int) bool {
			left, right := indices[i], indices[j]
			if len(suites[left].Cases) != len(suites[right].Cases) {
				return len(suites[left].Cases) > len(suites[right].Cases)
			}
			return left < right
		})
	}
	return indices, nil
}

type nativeSuiteResult struct {
	Observation observation
	SimpleFold  *simpleFoldExhaustiveReport
	YCbCr       *ycbcrExhaustiveReport
	CaseMapping *caseMappingExhaustiveReport
}

func validateNativeSuiteWorkers(workers int) error {
	if workers < 1 || workers > maxNativeSuiteWorkers {
		return fmt.Errorf("workers must be between 1 and %d", maxNativeSuiteWorkers)
	}
	return nil
}

func suiteEvidenceDirectory(root string, s suite) string {
	return filepath.Join(root, strings.ReplaceAll(s.Package, "/", "-")+"-"+s.Activity)
}

func validateSuiteEvidenceDirectories(root string, suites []suite) error {
	seen := map[string]int{}
	for index, s := range suites {
		dir := suiteEvidenceDirectory(root, s)
		if prior, exists := seen[dir]; exists {
			return fmt.Errorf("suites %d and %d share evidence directory %s", prior, index, dir)
		}
		seen[dir] = index
	}
	return nil
}

func runNativeSuites(ctx context.Context, suites []suite, workers int,
	verify func(context.Context, int, suite) (nativeSuiteResult, error)) ([]nativeSuiteResult, error) {
	order, err := nativeSuiteDispatchOrder(suites, nativeSuiteOrderCatalog)
	if err != nil {
		return nil, err
	}
	return runNativeSuitesOrdered(ctx, suites, workers, order, verify)
}

// Only this coordinator writes results and owns cancellation functions. After
// a failure it still dispatches every unstarted lower original index, so a
// priority order cannot hide an earlier failure. Higher active indices are
// canceled, and every started goroutine is joined before returning.
func runNativeSuitesOrdered(ctx context.Context, suites []suite, workers int, order []int,
	verify func(context.Context, int, suite) (nativeSuiteResult, error)) ([]nativeSuiteResult, error) {
	return runNativeSuitesOrderedPolicy(ctx, suites, workers, order, false, verify)
}

// Fail-fast is opt-in. Its first uncanceled failure closes dispatch and cancels
// every active suite context, but still joins every started callback. Commands
// retain their existing parent context and finish the active complete batch.
func runNativeSuitesOrderedPolicy(ctx context.Context, suites []suite, workers int, order []int, failFast bool,
	verify func(context.Context, int, suite) (nativeSuiteResult, error)) ([]nativeSuiteResult, error) {
	if err := validateNativeSuiteWorkers(workers); err != nil {
		return nil, err
	}
	if len(order) != len(suites) {
		return nil, fmt.Errorf("native suite dispatch order has %d indices, require %d", len(order), len(suites))
	}
	// Freeze and validate the whole permutation before any callback can run.
	dispatch := append([]int(nil), order...)
	seen := make([]bool, len(suites))
	for position, index := range dispatch {
		if index < 0 || index >= len(suites) || seen[index] {
			return nil, fmt.Errorf("native suite dispatch order has invalid or repeated index %d at position %d", index, position)
		}
		seen[index] = true
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type completion struct {
		index        int
		result       nativeSuiteResult
		err          error
		contextError error
	}
	completed := make(chan completion, workers)
	var joined sync.WaitGroup
	active := map[int]context.CancelFunc{}
	results := make([]nativeSuiteResult, len(suites))
	next, failed := 0, len(suites)
	var failure error
	fastFailureIndex := len(suites)
	var fastFailure error
	// Keep a completed, uncanceled failure for diagnostics if the parent later
	// expires while earlier original indices are still being checked. This does
	// not select the outcome or change dispatch, cancellation, or error priority.
	diagnosticIndex := len(suites)
	var diagnosticFailure error
	parentDone := ctx.Done()
	start := func() {
		for next < len(dispatch) && len(active) < workers && ctx.Err() == nil && fastFailure == nil {
			index := dispatch[next]
			next++
			if index > failed {
				// The earliest failure index can only decrease; this skipped
				// higher index will never become required later.
				continue
			}
			suiteContext, cancel := context.WithCancel(ctx)
			active[index] = cancel
			joined.Add(1)
			go func() {
				defer joined.Done()
				var result nativeSuiteResult
				err := suiteContext.Err()
				if err == nil {
					result, err = verify(suiteContext, index, suites[index])
				}
				completed <- completion{index, result, err, suiteContext.Err()}
			}()
		}
	}
	start()
	for len(active) > 0 {
		select {
		case <-parentDone:
			parentDone = nil
			for _, cancel := range active {
				cancel()
			}
		case done := <-completed:
			active[done.index]()
			delete(active, done.index)
			results[done.index] = done.result
			if done.err != nil && done.contextError == nil && done.index < diagnosticIndex {
				diagnosticIndex, diagnosticFailure = done.index, done.err
			}
			// A parent cancellation already visible at this boundary retains its
			// context outcome. Once selected, a genuine uncanceled failure is not
			// replaced by canceled callbacks or a later deadline while joining.
			if failFast && fastFailure == nil && done.err != nil && done.contextError == nil && ctx.Err() == nil {
				fastFailureIndex, fastFailure = done.index, done.err
				for _, cancel := range active {
					cancel()
				}
			}
			if done.err != nil && done.index < failed {
				failed, failure = done.index, done.err
				for index, cancel := range active {
					if index > failed {
						cancel()
					}
				}
			}
			start()
		}
	}
	joined.Wait()
	if fastFailure != nil {
		s := suites[fastFailureIndex]
		return nil, fmt.Errorf("suite %d %s:%s: %w", fastFailureIndex, s.Package, s.Activity, fastFailure)
	}
	if err := ctx.Err(); err != nil {
		if diagnosticFailure != nil {
			s := suites[diagnosticIndex]
			return nil, fmt.Errorf("%w; previously observed suite %d %s:%s: %v", err, diagnosticIndex, s.Package, s.Activity, diagnosticFailure)
		}
		return nil, err
	}
	if failure != nil {
		s := suites[failed]
		return nil, fmt.Errorf("suite %d %s:%s: %w", failed, s.Package, s.Activity, failure)
	}
	return results, nil
}
