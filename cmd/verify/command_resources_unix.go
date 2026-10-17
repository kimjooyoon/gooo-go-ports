//go:build linux || darwin

package main

import "syscall"

// Go's waited ProcessState contains the kernel's struct rusage. Only Linux and
// Darwin are interpreted here; their ru_maxrss units intentionally differ.
func nativeCommandResourcesForUsage(goos string, usage any) nativeCommandResources {
	if goos != "linux" && goos != "darwin" {
		return unavailableNativeCommandResources(goos, "unsupported-goos")
	}
	rusage, ok := usage.(*syscall.Rusage)
	if usage == nil || ok && rusage == nil {
		return unavailableNativeCommandResources(goos, "sysusage-unavailable")
	}
	if !ok {
		return unavailableNativeCommandResources(goos, "sysusage-type-unsupported")
	}
	return nativeCommandResourcesFromValues(goos, int64(rusage.Utime.Sec), int64(rusage.Utime.Usec), int64(rusage.Stime.Sec), int64(rusage.Stime.Usec), int64(rusage.Maxrss))
}
