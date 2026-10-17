package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func requireNativeCommandResourcesUnavailable(t *testing.T, got nativeCommandResources, reason string) {
	t.Helper()
	if got.Status != "NOT_AVAILABLE" || got.Reason != reason || got.Scope != "waited-command-accounting" || got.UserCPUNS != nil || got.SystemCPUNS != nil || got.MaxRSSRaw != nil || got.MaxRSSUnit != nil || got.MaxRSSBytes != nil {
		t.Fatalf("unavailable accounting must not invent zero counters: %+v", got)
	}
}

func TestNativeCommandResourcesUnitConversionAndOverflow(t *testing.T) {
	for _, test := range []struct {
		goos, unit string
		raw, bytes int64
	}{{"linux", "KiB", 17, 17 * 1024}, {"darwin", "bytes", 17, 17}, {"linux", "KiB", 0, 0}, {"darwin", "bytes", math.MaxInt64, math.MaxInt64}} {
		got := nativeCommandResourcesFromValues(test.goos, 2, 3, 4, 5, test.raw)
		if got.Status != "OBSERVED" || got.Reason != "" || got.GOOS != test.goos || got.Scope != "waited-command-accounting" || got.UserCPUNS == nil || *got.UserCPUNS != 2_000_003_000 || got.SystemCPUNS == nil || *got.SystemCPUNS != 4_000_005_000 || got.MaxRSSRaw == nil || *got.MaxRSSRaw != test.raw || got.MaxRSSUnit == nil || *got.MaxRSSUnit != test.unit || got.MaxRSSBytes == nil || *got.MaxRSSBytes != test.bytes {
			t.Fatalf("incorrect OS-specific accounting conversion: %+v", got)
		}
	}
	for _, test := range []struct {
		name, goos, reason                              string
		userSec, userMicro, systemSec, systemMicro, rss int64
	}{
		{"negative user", "linux", "negative-accounting-value", -1, 0, 0, 0, 1},
		{"negative user fraction", "linux", "negative-accounting-value", 0, -1, 0, 0, 1},
		{"negative system", "linux", "negative-accounting-value", 0, 0, -1, 0, 1},
		{"negative system fraction", "darwin", "negative-accounting-value", 0, 0, 0, -1, 1},
		{"negative RSS", "darwin", "negative-accounting-value", 0, 0, 0, 0, -1},
		{"invalid user fraction", "linux", "invalid-cpu-microseconds", 0, 1_000_000, 0, 0, 1},
		{"invalid system fraction", "darwin", "invalid-cpu-microseconds", 0, 0, 0, 1_000_000, 1},
		{"overflow user", "linux", "cpu-nanoseconds-overflow", math.MaxInt64 / 1_000_000_000, 999_999, 0, 0, 1},
		{"overflow system", "darwin", "cpu-nanoseconds-overflow", 0, 0, math.MaxInt64, 0, 1},
		{"overflow RSS", "linux", "maxrss-bytes-overflow", 0, 0, 0, 0, math.MaxInt64/1024 + 1},
		{"unknown units", "windows", "unsupported-goos", 0, 0, 0, 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := nativeCommandResourcesFromValues(test.goos, test.userSec, test.userMicro, test.systemSec, test.systemMicro, test.rss)
			requireNativeCommandResourcesUnavailable(t, got, test.reason)
		})
	}
	const maxMicroAlignedNS = int64(math.MaxInt64 / 1000 * 1000)
	got, reason := nativeCommandCPUNanoseconds(maxMicroAlignedNS/1_000_000_000, maxMicroAlignedNS%1_000_000_000/1000)
	if reason != "" || got != maxMicroAlignedNS {
		t.Fatal("last representable microsecond-aligned CPU value must remain valid", got, reason)
	}
}

func TestNativeCommandResourcesUnavailableJSONAndDisabledContext(t *testing.T) {
	got := unavailableNativeCommandResources("test-os", "not-observed")
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"user_cpu_ns", "system_cpu_ns", "maxrss_raw", "maxrss_unit", "maxrss_bytes"} {
		if counter, ok := value[key]; !ok || counter != nil {
			t.Fatal("unavailable counter must be present as null", key, string(raw))
		}
	}
	observed, err := json.Marshal(nativeCommandResourcesFromValues("linux", 0, 0, 0, 0, 0))
	if err != nil || bytes.Contains(observed, []byte(`"reason"`)) {
		t.Fatal("observed zero is real accounting and has no unavailable reason", string(observed), err)
	}
	ctx := context.Background()
	if withNativeCommandResources(ctx, nil) != ctx {
		t.Fatal("nil callback must retain the original context")
	}
	emitNativeCommandResources(ctx, nil)
	calls := 0
	emitNativeCommandResources(withNativeCommandResources(ctx, func(r nativeCommandResources) {
		calls++
		requireNativeCommandResourcesUnavailable(t, r, "process-state-unavailable")
	}), nil)
	if calls != 1 {
		t.Fatal("one completed command must deliver at most one optional observation", calls)
	}
}

func nativeResourceHelperCommand(t *testing.T, mode string) (string, []string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return executable, []string{"-test.run=^TestNativeCommandResourcesHelperProcess$", "--", "native-command-resources-helper-v1", mode}
}

// The test binary is the only helper executable. It does not invoke the Gooo
// compiler, Go builds, external commands, network services or another wait.
func TestNativeCommandResourcesHelperProcess(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "--" {
			marker = i
			break
		}
	}
	if marker < 0 || len(os.Args) < marker+3 || os.Args[marker+1] != "native-command-resources-helper-v1" {
		return
	}
	switch os.Args[marker+2] {
	case "success", "failure":
		if len(os.Args) != marker+3 {
			os.Exit(99)
		}
		fmt.Fprint(os.Stdout, "resource-stdout\x00\n")
		fmt.Fprint(os.Stderr, "resource-stderr\n")
		if os.Args[marker+2] == "failure" {
			os.Exit(17)
		}
		os.Exit(0)
	case "cancel":
		if len(os.Args) != marker+4 || os.Args[marker+3] == "" {
			os.Exit(99)
		}
		if _, err := fmt.Fprint(os.Stdout, "resource-ready\n"); err != nil {
			os.Exit(98)
		}
		ready := os.Args[marker+3]
		if err := os.WriteFile(ready+".tmp", []byte("READY\n"), 0600); err != nil {
			os.Exit(98)
		}
		if err := os.Rename(ready+".tmp", ready); err != nil {
			os.Exit(98)
		}
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(99)
	}
}

func checkObservedNativeCommandResources(t *testing.T, got nativeCommandResources) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		requireNativeCommandResourcesUnavailable(t, got, "unsupported-goos")
		return
	}
	if got.Status != "OBSERVED" || got.Reason != "" || got.GOOS != runtime.GOOS || got.Scope != "waited-command-accounting" || got.UserCPUNS == nil || got.SystemCPUNS == nil || got.MaxRSSRaw == nil || got.MaxRSSBytes == nil || got.MaxRSSUnit == nil || *got.UserCPUNS < 0 || *got.SystemCPUNS < 0 || *got.MaxRSSRaw < 0 || *got.MaxRSSBytes < 0 {
		t.Fatalf("real waited command must supply supported nonnegative accounting: %+v", got)
	}
	multiplier, unit := int64(1), "bytes"
	if runtime.GOOS == "linux" {
		multiplier, unit = 1024, "KiB"
	}
	if *got.MaxRSSUnit != unit || *got.MaxRSSBytes != *got.MaxRSSRaw*multiplier {
		t.Fatal("real process RSS unit conversion differs", got)
	}
}

func TestNativeCommandResourcesExecutePreservesDisabledOutputsAndExit(t *testing.T) {
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			program, args := nativeResourceHelperCommand(t, mode)
			root := t.TempDir()
			var priorRaw []byte
			var priorStdout, priorStderr []byte
			for _, enabled := range []bool{false, true} {
				path := filepath.Join(root, fmt.Sprintf("%t.json", enabled))
				ctx := context.Background()
				calls := 0
				if enabled {
					ctx = withNativeCommandResources(ctx, func(got nativeCommandResources) {
						calls++
						// Observation occurs just after Run, before the original save.
						if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
							t.Fatal("observer changed original execute/save order", err)
						}
						checkObservedNativeCommandResources(t, got)
					})
				}
				raw, err := execute(ctx, program, path, args...)
				if mode == "success" && err != nil || mode == "failure" && err == nil {
					t.Fatal("existing command success/failure changed", err)
				}
				if mode == "failure" {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != 17 || raw != nil || !strings.HasSuffix(err.Error(), ": resource-stderr\n") {
						t.Fatal("existing exit error, returned bytes or stderr changed", raw, err)
					}
				}
				stdout, stdoutErr := os.ReadFile(path)
				stderr, stderrErr := os.ReadFile(path + ".stderr")
				if stdoutErr != nil || stderrErr != nil || !bytes.Equal(stdout, []byte("resource-stdout\x00\n")) || !bytes.Equal(stderr, []byte("resource-stderr\n")) {
					t.Fatal("existing exact stdout/stderr files changed", stdoutErr, stderrErr)
				}
				if !enabled {
					priorRaw, priorStdout, priorStderr = raw, stdout, stderr
				} else if calls != 1 || !bytes.Equal(raw, priorRaw) || !bytes.Equal(stdout, priorStdout) || !bytes.Equal(stderr, priorStderr) {
					t.Fatal("enabled observation differs from the disabled execution contract", calls)
				}
			}
		})
	}
}

func TestNativeCommandResourcesExecuteStartFailureAndPreCanceled(t *testing.T) {
	for _, mode := range []string{"missing-executable", "pre-canceled"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "receipt.json")
			program, args := nativeResourceHelperCommand(t, "success")
			ctx := context.Background()
			if mode == "missing-executable" {
				program = filepath.Join(t.TempDir(), "does-not-exist")
			} else {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			calls := 0
			ctx = withNativeCommandResources(ctx, func(got nativeCommandResources) {
				calls++
				requireNativeCommandResourcesUnavailable(t, got, "process-state-unavailable")
			})
			raw, err := execute(ctx, program, path, args...)
			if err == nil || raw != nil || calls != 1 {
				t.Fatal("failed start must retain its error without inventing process accounting", raw, err, calls)
			}
			if mode == "pre-canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation error lost", err)
			}
			stdout, readErr := os.ReadFile(path)
			if readErr != nil || len(stdout) != 0 {
				t.Fatal("original failed-start empty stdout save changed", readErr)
			}
			if _, err := os.Stat(path + ".stderr"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed start invented a stderr file", err)
			}
		})
	}
}

func TestNativeCommandResourcesExecuteCancellationAfterStart(t *testing.T) {
	program, args := nativeResourceHelperCommand(t, "cancel")
	root := t.TempDir()
	ready := filepath.Join(root, "helper-ready")
	args = append(args, ready)
	ctx, cancel := context.WithCancel(context.Background())
	observed := make(chan nativeCommandResources, 2)
	ctx = withNativeCommandResources(ctx, func(got nativeCommandResources) {
		// Never fail the test from execute's goroutine or block its cleanup.
		// A second observation remains detectable after joining the command.
		select {
		case observed <- got:
		default:
		}
	})
	path := filepath.Join(root, "canceled.json")
	type executionResult struct {
		raw []byte
		err error
	}
	completed := make(chan executionResult, 1)
	joined := false
	go func() {
		raw, err := execute(ctx, program, path, args...)
		completed <- executionResult{raw, err}
	}()
	defer func() {
		cancel()
		if !joined {
			// Every failure path still waits for CommandContext to kill and
			// reap this sole helper; no child or execution goroutine is left.
			<-completed
		}
	}()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(10 * time.Millisecond) // Readiness polling is test-only.
	defer poll.Stop()
	for waiting := true; waiting; {
		select {
		case result := <-completed:
			joined = true
			t.Fatal("helper exited before the readiness handshake", result.err)
		case <-deadline.C:
			t.Fatal("helper did not reach the readiness handshake within 10 seconds")
		case <-poll.C:
			marker, err := os.ReadFile(ready)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil || !bytes.Equal(marker, []byte("READY\n")) {
				t.Fatal("invalid helper readiness handshake", string(marker), err)
			}
			waiting = false
		}
	}
	cancel() // The helper has actually started and written its original stdout.
	result := <-completed
	joined = true
	stdout, readErr := os.ReadFile(path)
	if result.err == nil || result.raw != nil || len(observed) != 1 || !errors.Is(ctx.Err(), context.Canceled) || readErr != nil || !bytes.Equal(stdout, []byte("resource-ready\n")) {
		t.Fatal("started helper cancellation/output/wait contract changed", result.raw, result.err, len(observed), readErr, string(stdout))
	}
	checkObservedNativeCommandResources(t, <-observed)
}

func TestNativeCommandResourcesExecuteRetainsOutputWriteErrorPriority(t *testing.T) {
	program, args := nativeResourceHelperCommand(t, "failure")
	path := t.TempDir() // The original stdout save must fail on this directory.
	calls := 0
	ctx := withNativeCommandResources(context.Background(), func(got nativeCommandResources) {
		calls++
		checkObservedNativeCommandResources(t, got)
	})
	raw, err := execute(ctx, program, path, args...)
	var fileError *os.PathError
	var exitError *exec.ExitError
	if raw != nil || !errors.As(err, &fileError) || fileError.Path != path || errors.As(err, &exitError) || calls != 1 {
		t.Fatal("optional observation replaced the original output-write error priority", raw, err, calls)
	}
}
