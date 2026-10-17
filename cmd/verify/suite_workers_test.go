package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func workerTestSuites(count int) []suite {
	suites := make([]suite, count)
	for i := range suites {
		suites[i] = suite{Package: "example/worker", Activity: fmt.Sprintf("Suite%02d", i)}
	}
	return suites
}

func workerTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestNativeSuiteWorkersBoundCoverageAndOrder(t *testing.T) {
	for _, workers := range []int{1, 2, 3, maxNativeSuiteWorkers} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			ctx := workerTestContext(t)
			suites := workerTestSuites(59)
			calls := make([]atomic.Int32, len(suites))
			var active, peak atomic.Int32
			started := make(chan int, len(suites))
			release := make(chan struct{})
			finished := make(chan struct{})
			var results []nativeSuiteResult
			var err error
			go func() {
				results, err = runNativeSuites(ctx, suites, workers, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
					calls[index].Add(1)
					current := active.Add(1)
					defer active.Add(-1)
					for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
					}
					started <- index
					select {
					case <-release:
					case <-ctx.Done():
						return nativeSuiteResult{}, ctx.Err()
					}
					return nativeSuiteResult{Observation: observation{Package: s.Package, Activity: s.Activity, Cases: index + 1}}, nil
				})
				close(finished)
			}()
			for range workers {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("workers did not start", ctx.Err())
				}
			}
			select {
			case <-started:
				t.Fatal("started work above the worker bound")
			default:
			}
			close(release)
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("workers did not join", ctx.Err())
			}
			if err != nil || len(results) != len(suites) || active.Load() != 0 || peak.Load() != int32(workers) {
				t.Fatalf("coverage/bound/join failed: results=%d active=%d peak=%d error=%v", len(results), active.Load(), peak.Load(), err)
			}
			for index, result := range results {
				if calls[index].Load() != 1 || result.Observation.Activity != suites[index].Activity || result.Observation.Cases != index+1 {
					t.Fatal("suite skipped, repeated, or reordered", index)
				}
			}
		})
	}
}

func TestNativeSuiteWorkersRetainOrderAfterOutOfOrderCompletion(t *testing.T) {
	ctx := workerTestContext(t)
	first := make(chan struct{})
	second := make(chan struct{})
	finished := make(chan struct{})
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuites(ctx, workerTestSuites(2), 2, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
			if index == 0 {
				select {
				case <-first:
				case <-ctx.Done():
					return nativeSuiteResult{}, ctx.Err()
				}
			} else {
				close(second)
			}
			return nativeSuiteResult{Observation: observation{Activity: s.Activity}}, nil
		})
		close(finished)
	}()
	select {
	case <-second:
	case <-ctx.Done():
		t.Fatal("second suite did not finish")
	}
	close(first)
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("workers did not join")
	}
	if err != nil || results[0].Observation.Activity != "Suite00" || results[1].Observation.Activity != "Suite01" {
		t.Fatal("completion order changed result order", err)
	}
}

func TestNativeSuiteWorkersReturnEarliestFailureAndJoinCancelledWork(t *testing.T) {
	ctx := workerTestContext(t)
	lowError, highError := errors.New("earlier suite failed"), errors.New("later suite failed")
	release := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	started := make(chan int, 5)
	laterJoined := make(chan struct{})
	finished := make(chan struct{})
	var active atomic.Int32
	var results []nativeSuiteResult
	var err error
	go func() {
		results, err = runNativeSuites(ctx, workerTestSuites(5), 4, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
			active.Add(1)
			defer active.Add(-1)
			started <- index
			if index == 3 {
				<-ctx.Done()
				close(laterJoined)
				return nativeSuiteResult{}, ctx.Err()
			}
			if index > 3 {
				return nativeSuiteResult{}, errors.New("new suite started after failure")
			}
			select {
			case <-release[index]:
			case <-ctx.Done():
				return nativeSuiteResult{}, ctx.Err()
			}
			if index == 1 {
				return nativeSuiteResult{}, lowError
			}
			if index == 2 {
				return nativeSuiteResult{}, highError
			}
			return nativeSuiteResult{}, nil
		})
		close(finished)
	}()
	for range 4 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("initial workers did not start")
		}
	}
	close(release[2])
	select {
	case <-laterJoined:
	case <-ctx.Done():
		t.Fatal("later suite was not cancelled")
	}
	close(release[0])
	close(release[1])
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("failure workers did not join")
	}
	if results != nil || !errors.Is(err, lowError) || active.Load() != 0 {
		t.Fatal("did not return the earliest failure after joining", err, active.Load())
	}
	select {
	case index := <-started:
		t.Fatal("started a new suite after failure", index)
	default:
	}
}

func TestNativeSuiteWorkersParentCancellationAndInvalidBounds(t *testing.T) {
	for _, workers := range []int{-1, 0, maxNativeSuiteWorkers + 1} {
		_, err := runNativeSuites(context.Background(), workerTestSuites(1), workers,
			func(context.Context, int, suite) (nativeSuiteResult, error) {
				t.Fatal("invalid worker setting executed work")
				return nativeSuiteResult{}, nil
			})
		if err == nil {
			t.Fatal("accepted invalid worker setting", workers)
		}
	}
	ctx, cancel := context.WithCancel(workerTestContext(t))
	started := make(chan struct{}, 2)
	finished := make(chan error, 1)
	var active atomic.Int32
	go func() {
		_, err := runNativeSuites(ctx, workerTestSuites(3), 2, func(ctx context.Context, _ int, _ suite) (nativeSuiteResult, error) {
			active.Add(1)
			defer active.Add(-1)
			started <- struct{}{}
			<-ctx.Done()
			return nativeSuiteResult{}, ctx.Err()
		})
		finished <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not start")
		}
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) || active.Load() != 0 {
		t.Fatal("parent cancellation returned before joining", err, active.Load())
	}
	select {
	case <-started:
		t.Fatal("started work after parent cancellation")
	default:
	}
}

func TestSuiteEvidenceDirectoryCollisionIsRejected(t *testing.T) {
	suites := []suite{{Package: "go/a-b", Activity: "Check"}, {Package: "go/a/b", Activity: "Check"}}
	if err := validateSuiteEvidenceDirectories(t.TempDir(), suites); err == nil {
		t.Fatal("accepted distinct suites with colliding output paths")
	}
}
