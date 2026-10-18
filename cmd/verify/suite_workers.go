package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxNativeSuiteWorkers = 4

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
		index  int
		result nativeSuiteResult
		err    error
	}
	completed := make(chan completion, workers)
	var joined sync.WaitGroup
	active := map[int]context.CancelFunc{}
	results := make([]nativeSuiteResult, len(suites))
	next, failed := 0, len(suites)
	var failure error
	parentDone := ctx.Done()
	start := func() {
		for next < len(dispatch) && len(active) < workers && ctx.Err() == nil {
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
				completed <- completion{index, result, err}
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if failure != nil {
		s := suites[failed]
		return nil, fmt.Errorf("suite %d %s:%s: %w", failed, s.Package, s.Activity, failure)
	}
	return results, nil
}
