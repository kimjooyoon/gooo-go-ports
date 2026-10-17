//go:build !linux && !darwin

package main

// Keep the optional observer buildable without assuming another OS's SysUsage
// representation or memory units. Unavailable counters are explicit JSON nulls.
func nativeCommandResourcesForUsage(goos string, _ any) nativeCommandResources {
	return unavailableNativeCommandResources(goos, "unsupported-goos")
}
