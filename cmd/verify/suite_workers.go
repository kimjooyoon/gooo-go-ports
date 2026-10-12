package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

const maxNativeSuiteWorkers = 4

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

// Only this coordinator writes results and owns cancellation functions. A
// failed suite cancels later suites, but earlier suites finish so the returned
// failure is the same earliest suite that serial execution would encounter.
// Every started callback is joined before results or an error are returned.
func runNativeSuites(ctx context.Context, suites []suite, workers int,
	verify func(context.Context, int, suite) (nativeSuiteResult, error)) ([]nativeSuiteResult, error) {
	if err := validateNativeSuiteWorkers(workers); err != nil {
		return nil, err
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
		for next < len(suites) && len(active) < workers && failed == len(suites) && ctx.Err() == nil {
			index := next
			next++
			suiteContext, cancel := context.WithCancel(ctx)
			active[index] = cancel
			joined.Add(1)
			go func() {
				defer joined.Done()
				result, err := verify(suiteContext, index, suites[index])
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
