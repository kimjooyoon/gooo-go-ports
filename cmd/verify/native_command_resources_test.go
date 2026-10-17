package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func TestNativeCommandResourcesDisabledKeepsParentContext(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	var batch *nativeBatchTiming
	if batch.commandContext(parent, "execute") != parent {
		t.Fatal("disabled timing replaced the command's original parent context")
	}
}

func finishResourceBatch(t *testing.T, recorder *nativeTiming, batch *nativeBatchTiming) {
	t.Helper()
	batch.phase("inspect_replay")
	batch.phase("receipt_binding")
	batch.bind(nativeTimingTestBinding(t, recorder), contentDigest([]byte("resource test execution receipt")))
	batch.finish(nil, nil, true)
}

func TestNativeCommandResourcesContextsBindExecuteReplayAndParallelBatches(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("OS wait accounting is supported on Linux and Darwin")
	}
	a, b := nativeTimingTestSuite("resourceA", 1), nativeTimingTestSuite("resourceB", 1)
	recorder, path := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{a, b})
	var wg sync.WaitGroup
	for i, s := range []suite{a, b} {
		span := recorder.beginSuite(i, s)
		wg.Go(func() {
			batch := recorder.beginBatch(s, 0, 0, 1)
			// Distinct synthetic observations expose cross-suite and cross-phase
			// attribution errors; real OS wait conversion is tested separately.
			observe := func(phase string, value int64) {
				ctx := batch.commandContext(context.Background(), phase)
				callback := ctx.Value(nativeCommandResourcesContextKey{}).(func(nativeCommandResources))
				callback(nativeCommandResourcesFromValues(runtime.GOOS, value, 3, value+10, 5, value+20))
			}
			batch.phase("execute")
			observe("execute", int64(i*100+1))
			batch.phase("inspect_execution")
			batch.phase("replay")
			observe("replay", int64(i*100+2))
			finishResourceBatch(t, recorder, batch)
			span.finish(nativeTimingTestObservation(s, 1), nil, nil)
		})
	}
	wg.Wait()
	if err := recorder.close(nil, nil); err != nil {
		t.Fatal(err)
	}
	rows := nativeTimingTestEvents(t, path)
	if rows[0]["command_resources_schema"] != nativeCommandResourcesSchema || rows[0]["command_resources_scope"] != nativeCommandResourcesScope || len(rows) != 8 {
		t.Fatal("resource extension changed the timing event roster or lost its scope")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if row["kind"] != "batch_end" {
			continue
		}
		key := fmt.Sprint(row["activity"])
		if seen[key] || row["verified"] != true {
			t.Fatal("resource reporting changed original batch verification", row)
		}
		seen[key] = true
		resources := row["command_resources"].(map[string]any)
		if len(resources) != 2 {
			t.Fatal("execute/replay accounting was not independently attached", resources)
		}
		for phaseIndex, phase := range []string{"execute", "replay"} {
			r := resources[phase].(map[string]any)
			index := map[string]int{"resourceA": 0, "resourceB": 1}[key]
			value := int64(index*100 + phaseIndex + 1)
			want := nativeCommandResourcesFromValues(runtime.GOOS, value, 3, value+10, 5, value+20)
			raw, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var expected map[string]any
			if err := json.Unmarshal(raw, &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r, expected) {
				t.Fatal("command accounting crossed its exact suite/phase", key, phase, r, expected)
			}
		}
	}
	if !reflect.DeepEqual(seen, map[string]bool{"resourceA": true, "resourceB": true}) {
		t.Fatal("parallel command accounting crossed batch ownership", seen)
	}
}

func TestNativeCommandResourcesRejectDuplicateWrongPhaseAndLateObservation(t *testing.T) {
	for _, mode := range []string{"duplicate", "wrong-phase", "late"} {
		t.Run(mode, func(t *testing.T) {
			s := nativeTimingTestSuite("invalidResource", 1)
			recorder, _ := timingRecorderForTest(t, nativeTimingTestRoot(t), []suite{s})
			recorder.beginSuite(0, s)
			batch := recorder.beginBatch(s, 0, 0, 1)
			batch.phase("execute")
			ctx := batch.commandContext(context.Background(), "execute")
			emitNativeCommandResources(ctx, nil)
			if mode == "wrong-phase" {
				ctx = batch.commandContext(context.Background(), "replay")
			} else if mode == "late" {
				batch.finish(nil, nil, false)
			}
			emitNativeCommandResources(ctx, nil)
			if recorder.err == nil {
				t.Fatal("invalid accounting did not fail the diagnostic observation")
			}
			if err := recorder.close(nil, nil); err == nil {
				t.Fatal("invalid accounting was accepted as complete timing")
			}
		})
	}
}
