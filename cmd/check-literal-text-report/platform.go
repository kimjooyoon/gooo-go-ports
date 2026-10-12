package main

import "runtime"

func runtimePlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }
