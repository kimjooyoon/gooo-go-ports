//go:build linux || darwin

package main

import (
	"syscall"
	"testing"
)

func TestNativeCommandResourcesSysUsageRefusesNilAndWrongType(t *testing.T) {
	var typedNil *syscall.Rusage
	for _, usage := range []any{nil, typedNil} {
		requireNativeCommandResourcesUnavailable(t, nativeCommandResourcesForUsage("linux", usage), "sysusage-unavailable")
	}
	for _, usage := range []any{syscall.Rusage{}, 0, "not a usage struct"} {
		requireNativeCommandResourcesUnavailable(t, nativeCommandResourcesForUsage("darwin", usage), "sysusage-type-unsupported")
	}
	requireNativeCommandResourcesUnavailable(t, nativeCommandResourcesForUsage("unknown", &syscall.Rusage{}), "unsupported-goos")
	usage := &syscall.Rusage{Maxrss: 19}
	usage.Utime.Sec, usage.Utime.Usec = 2, 3
	usage.Stime.Sec, usage.Stime.Usec = 4, 5
	for _, goos := range []string{"linux", "darwin"} {
		got := nativeCommandResourcesForUsage(goos, usage)
		if got.Status != "OBSERVED" || got.UserCPUNS == nil || *got.UserCPUNS != 2_000_003_000 || got.SystemCPUNS == nil || *got.SystemCPUNS != 4_000_005_000 || got.MaxRSSRaw == nil || *got.MaxRSSRaw != 19 {
			t.Fatal("real platform usage fields must be converted without another wait", got)
		}
	}
}
