package main

import (
	"context"
	"math"
	"os"
	"runtime"
)

// This is the accounting returned for one waited command. It is neither a
// simultaneous process-tree memory measurement nor a total of per-command RSS.
type nativeCommandResources struct {
	Status      string  `json:"status"`
	Reason      string  `json:"reason,omitempty"`
	GOOS        string  `json:"goos"`
	Scope       string  `json:"scope"`
	UserCPUNS   *int64  `json:"user_cpu_ns"`
	SystemCPUNS *int64  `json:"system_cpu_ns"`
	MaxRSSRaw   *int64  `json:"maxrss_raw"`
	MaxRSSUnit  *string `json:"maxrss_unit"`
	MaxRSSBytes *int64  `json:"maxrss_bytes"`
}

type nativeCommandResourcesContextKey struct{}

func withNativeCommandResources(ctx context.Context, callback func(nativeCommandResources)) context.Context {
	if callback == nil {
		return ctx
	}
	return context.WithValue(ctx, nativeCommandResourcesContextKey{}, callback)
}

func emitNativeCommandResources(ctx context.Context, state *os.ProcessState) {
	callback, _ := ctx.Value(nativeCommandResourcesContextKey{}).(func(nativeCommandResources))
	if callback == nil {
		return // Disabled observation must not inspect ProcessState or SysUsage.
	}
	if state == nil {
		callback(unavailableNativeCommandResources(runtime.GOOS, "process-state-unavailable"))
		return
	}
	callback(nativeCommandResourcesForUsage(runtime.GOOS, state.SysUsage()))
}

func unavailableNativeCommandResources(goos, reason string) nativeCommandResources {
	return nativeCommandResources{Status: "NOT_AVAILABLE", Reason: reason, GOOS: goos, Scope: "waited-command-accounting"}
}

// Convert raw timeval components before multiplying: time.Duration alone can
// overflow into another positive number and must not certify a valid sample.
func nativeCommandCPUNanoseconds(seconds, microseconds int64) (int64, string) {
	if seconds < 0 || microseconds < 0 {
		return 0, "negative-accounting-value"
	}
	if microseconds >= 1_000_000 {
		return 0, "invalid-cpu-microseconds"
	}
	const billion = int64(1_000_000_000)
	fraction := microseconds * 1_000
	if seconds > (math.MaxInt64-fraction)/billion {
		return 0, "cpu-nanoseconds-overflow"
	}
	return seconds*billion + fraction, ""
}

func nativeCommandResourcesFromValues(goos string, userSeconds, userMicroseconds, systemSeconds, systemMicroseconds, maxRSS int64) nativeCommandResources {
	unit, multiplier := "", int64(1)
	switch goos {
	case "linux":
		unit, multiplier = "KiB", 1024
	case "darwin":
		unit = "bytes"
	default:
		return unavailableNativeCommandResources(goos, "unsupported-goos")
	}
	user, reason := nativeCommandCPUNanoseconds(userSeconds, userMicroseconds)
	if reason != "" {
		return unavailableNativeCommandResources(goos, reason)
	}
	system, reason := nativeCommandCPUNanoseconds(systemSeconds, systemMicroseconds)
	if reason != "" {
		return unavailableNativeCommandResources(goos, reason)
	}
	if maxRSS < 0 {
		return unavailableNativeCommandResources(goos, "negative-accounting-value")
	}
	if maxRSS > math.MaxInt64/multiplier {
		return unavailableNativeCommandResources(goos, "maxrss-bytes-overflow")
	}
	bytes := maxRSS * multiplier
	return nativeCommandResources{Status: "OBSERVED", GOOS: goos, Scope: "waited-command-accounting", UserCPUNS: &user, SystemCPUNS: &system, MaxRSSRaw: &maxRSS, MaxRSSUnit: &unit, MaxRSSBytes: &bytes}
}
