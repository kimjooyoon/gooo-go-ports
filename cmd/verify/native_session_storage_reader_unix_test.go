//go:build linux || darwin

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A deterministic replacement at the exact lstat/open seam. There is no
// producer process, native execution or probabilistic filesystem race loop.
func nativeSessionFIFOOpen(t *testing.T, openedFD *int) func(string) (*os.File, error) {
	t.Helper()
	return func(path string) (*os.File, error) {
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		if err := syscall.Mkfifo(path, 0600); err != nil {
			return nil, err
		}
		file, err := openNativeSessionEvidenceFile(path)
		if file != nil {
			*openedFD = int(file.Fd())
		}
		return file, err
	}
}

func nativeSessionExpectPromptFIFORefusal(t *testing.T, path string, run func() error, openedFD *int) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("replacement FIFO was accepted as an original regular file")
		}
	case <-time.After(2 * time.Second):
		// A regression must not leave the test itself with a blocked reader.
		// Opening both ends nonblocking releases any pending blocking open.
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if err == nil {
			_ = syscall.Close(fd)
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("FIFO replacement left an unjoined test reader")
		}
		t.Fatal("post-lstat FIFO replacement blocked the evidence opener")
	}
	if *openedFD >= 0 {
		var info syscall.Stat_t
		if err := syscall.Fstat(*openedFD, &info); !errors.Is(err, syscall.EBADF) {
			t.Fatal("refused FIFO descriptor was not closed", err)
		}
	}
}

func TestNativeSessionSecureReadRejectsPostLstatFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json.gz")
	if err := os.WriteFile(path, []byte("original regular file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fd := -1
	opener := nativeSessionFIFOOpen(t, &fd)
	nativeSessionExpectPromptFIFORefusal(t, path, func() error {
		_, err := readNativeSessionEvidenceFileWithOpen(path, 4096, opener)
		return err
	}, &fd)
}

func TestNativeSessionSecureSnapshotReadAndFinishRejectFIFO(t *testing.T) {
	for _, phase := range []string{"read", "finish"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "execution.json")
			if err := os.WriteFile(path, []byte("{\"original\":true}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			reader, err := newNativeSessionEvidenceReader(root)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "finish" {
				if _, err := reader.read("execution.json", 4096); err != nil {
					t.Fatal(err)
				}
			}
			fd := -1
			opener := nativeSessionFIFOOpen(t, &fd)
			reader.readFile = func(path string, limit int64) ([]byte, error) {
				return readNativeSessionEvidenceFileWithOpen(path, limit, opener)
			}
			nativeSessionExpectPromptFIFORefusal(t, path, func() error {
				if phase == "finish" {
					return reader.finish()
				}
				_, err := reader.read("execution.json", 4096)
				return err
			}, &fd)
		})
	}
}

func TestNativeSessionSecureReadClosesEveryOpenedDescriptor(t *testing.T) {
	for _, mode := range []string{"success", "different-inode", "read-error", "opener-error", "pathname-replaced-after-open", "symlink-after-lstat"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path, other := filepath.Join(root, "receipt.json"), filepath.Join(root, "other.json")
			raw := []byte(" {\"value\":1e0} \n")
			for _, name := range []string{path, other} {
				if err := os.WriteFile(name, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			fd := -1
			sentinel := errors.New("synthetic partial opener error")
			opener := func(name string) (*os.File, error) {
				if mode == "symlink-after-lstat" {
					if err := os.Remove(name); err != nil {
						return nil, err
					}
					if err := os.Symlink(other, name); err != nil {
						return nil, err
					}
				}
				selected := name
				if mode == "different-inode" {
					selected = other
				}
				var file *os.File
				var err error
				if mode == "read-error" {
					file, err = os.OpenFile(selected, os.O_WRONLY, 0)
				} else {
					file, err = openNativeSessionEvidenceFile(selected)
				}
				if file == nil {
					return nil, err
				}
				fd = int(file.Fd())
				if mode != "read-error" {
					flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
					if errno != 0 || flags&syscall.FD_CLOEXEC == 0 {
						return file, errors.New("evidence descriptor lacked close-on-exec")
					}
				}
				if mode == "pathname-replaced-after-open" {
					if err := os.Remove(name); err != nil {
						return file, err
					}
					if err := os.WriteFile(name, raw, 0600); err != nil {
						return file, err
					}
				}
				if mode == "opener-error" {
					return file, sentinel
				}
				return file, err
			}
			got, err := readNativeSessionEvidenceFileWithOpen(path, 4096, opener)
			if mode == "success" {
				if err != nil || !bytes.Equal(got, raw) {
					t.Fatal("regular file bytes changed", err)
				}
			} else if err == nil {
				t.Fatal("changed/error file accepted")
			}
			if mode == "opener-error" && !errors.Is(err, sentinel) {
				t.Fatal("original opener error lost", err)
			}
			if fd >= 0 {
				var info syscall.Stat_t
				if err := syscall.Fstat(fd, &info); !errors.Is(err, syscall.EBADF) {
					t.Fatal("opened descriptor leaked", err)
				}
			}
		})
	}
}

func TestNativeSessionSecureReadRegularAndSnapshotRegression(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "receipt.json")
	raw := []byte(" {\"value\":1e0,\"text\":\"界\"} \n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readNativeSessionEvidenceFile(path, int64(len(raw)))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("regular reader changed raw bytes", err)
	}
	if _, err := readNativeSessionEvidenceFile(path, int64(len(raw)-1)); err == nil {
		t.Fatal("regular size bound bypassed")
	}
	ref, err := writeNativeSessionStoredEvidence(filepath.Join(root, "frame.jsonl"), raw, nativeSessionEvidenceGzip)
	if err != nil {
		t.Fatal(err)
	}
	got, err = readNativeSessionStoredEvidence(ref)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("gzip readback/reference regression", err)
	}
	reader, err := newNativeSessionEvidenceReader(root)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Unix(1700000000, 0)
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.read("receipt.json", int64(len(raw))); err != nil {
		t.Fatal(err)
	}
	if err := reader.finish(); err != nil {
		t.Fatal("stable snapshot refused", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(raw, []byte("1e0"), []byte("2e0"), 1)
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(info, after) || info.Size() != after.Size() || info.Mode() != after.Mode() || !info.ModTime().Equal(after.ModTime()) {
		t.Fatal("hash-drift control must retain inode, length, mode and mtime", err)
	}
	if err := reader.finish(); err == nil {
		t.Fatal("same-inode/size/mtime byte drift bypassed pinned SHA")
	}
	parent := filepath.Join(root, "nested")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "receipt.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	directories, err := newNativeSessionEvidenceReader(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directories.read("nested/receipt.json", int64(len(raw))); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, parent+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "receipt.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := directories.finish(); err == nil {
		t.Fatal("original directory inode pin weakened")
	}
}
