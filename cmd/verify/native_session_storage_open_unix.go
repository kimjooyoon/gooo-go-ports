//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"syscall"
)

func openNativeSessionEvidenceFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("cannot wrap native session evidence descriptor")
	}
	return file, nil
}
