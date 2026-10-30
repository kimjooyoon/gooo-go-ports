//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const nativeSessionProfileGoVersion = "go1.27.3"
const nativeSessionProfileProbeLimit = 64 << 10

// This constructor is an explicit experimental profile, never a replacement
// for upstream.json. Each suite callback owns the returned client and must
// Close it before returning. Its caller selects the original legacy workspace;
// the verifier also refuses compact cases and per-request wait accounting.
func newExperimentalNativeSessionVerifier(ctx context.Context, compiler, goTool, root string, lock upstream, cache *nativeBuildCache) (nativeSuiteVerifier, *nativeSessionClient, error) {
	var empty nativeSuiteVerifier
	if ctx == nil || !filepath.IsAbs(compiler) || !filepath.IsAbs(goTool) || !filepath.IsAbs(root) {
		return empty, nil, fmt.Errorf("session profile requires context and explicit absolute compiler, Go and evidence paths")
	}
	if lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != nativeSessionProfileGoVersion || runtime.Version() != lock.GoVersion {
		return empty, nil, fmt.Errorf("session profile keeps the original205 oracle lock and requires Go1.27.3")
	}
	owner, err := os.Lstat(root)
	if err != nil || !owner.IsDir() || owner.Mode()&os.ModeSymlink != 0 {
		return empty, nil, fmt.Errorf("session profile requires an existing caller-owned evidence directory")
	}
	profile := filepath.Join(root, "session-profile")
	if err := os.Mkdir(profile, 0700); err != nil {
		return empty, nil, fmt.Errorf("session profile requires fresh probe evidence: %w", err)
	}
	raw, err := nativeSessionProfileProbe(ctx, profile, "compiler", compiler, "version", "--build", "--json")
	if err != nil {
		return empty, nil, err
	}
	if err := validateNativeSessionCompilerIdentity(raw); err != nil {
		return empty, nil, err
	}
	raw, err = nativeSessionProfileProbe(ctx, profile, "go-version", goTool, "version")
	if err != nil {
		return empty, nil, err
	}
	if !bytes.HasPrefix(raw, []byte("go version "+nativeSessionProfileGoVersion+" ")) || bytes.Count(bytes.TrimSpace(raw), []byte{'\n'}) != 0 {
		return empty, nil, fmt.Errorf("session profile requires the explicit Go1.27.3 tool")
	}
	client, err := newNativeSessionClient(ctx, compiler, goTool, root)
	if err != nil {
		return empty, nil, err
	}
	v := newNativeSuiteVerifier(ctx, compiler, root, lock, cache)
	v.compilerRevisionOverride = nativeSessionCompilerRevision
	v.inspectExecution = inspectSessionWithRuntime
	v.execute = client.Execute
	return v, client, nil
}

func validateNativeSessionCompilerIdentity(raw []byte) error {
	if len(raw) == 0 || len(raw) > nativeSessionProfileProbeLimit {
		return fmt.Errorf("bounded session compiler build identity required")
	}
	value, err := decode(raw)
	if err != nil {
		return fmt.Errorf("session compiler build identity: %w", err)
	}
	build, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("session compiler build identity must be an object")
	}
	for key, expected := range map[string]string{
		"schema": "gooo/build-identity/v1", "compiler_source_sha": nativeSessionCompilerRevision,
		"vcs_revision": nativeSessionCompilerRevision, "source_status": "CLEAN_VCS", "vcs_modified": "false",
		"go_version": nativeSessionProfileGoVersion, "native_go_required": nativeSessionProfileGoVersion,
	} {
		if build[key] != expected {
			return fmt.Errorf("session compiler build identity differs: %s", key)
		}
	}
	// Module version/tag context is descriptive; immutable source is the gate.
	return nil
}

type nativeSessionProfileProbeBuffer struct {
	bytes.Buffer
	cancel context.CancelFunc
	limit  bool
}

func (b *nativeSessionProfileProbeBuffer) Write(raw []byte) (int, error) {
	remaining := nativeSessionProfileProbeLimit - b.Len()
	if len(raw) > remaining {
		_, _ = b.Buffer.Write(raw[:remaining])
		b.limit = true
		b.cancel()
		return remaining, fmt.Errorf("identity probe output exceeds64 KiB")
	}
	return b.Buffer.Write(raw)
}

// Identity commands have separate finite output/time bounds. Exact consumed
// stdout/stderr and process status are saved before their contents are checked.
func nativeSessionProfileProbe(parent context.Context, evidence, name, command string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	stdout := &nativeSessionProfileProbeBuffer{cancel: cancel}
	stderr := &nativeSessionProfileProbeBuffer{cancel: cancel}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOTOOLCHAIN=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local")
	started := time.Now()
	runErr := cmd.Run()
	groupRemaining := false
	if cmd.Process != nil {
		groupRemaining = syscall.Kill(-cmd.Process.Pid, 0) == nil
		if groupRemaining {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			runErr = errors.Join(runErr, fmt.Errorf("identity probe left an owned process group; killed"))
		}
	}
	exit := any(nil)
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	record := map[string]any{"argv": append([]string{command}, args...), "exit_code": exit,
		"wall_ns": time.Since(started).Nanoseconds(), "output_limit_exceeded": stdout.limit || stderr.limit,
		"stdout_sha256": contentDigest(stdout.Bytes()), "stderr_sha256": contentDigest(stderr.Bytes()),
		"owned_process_group_remaining_after_wait": groupRemaining, "timeout_or_cancel": ctx.Err() != nil}
	if runErr != nil {
		record["error"] = runErr.Error()
	}
	for _, output := range []struct {
		suffix string
		raw    []byte
	}{{".stdout", stdout.Bytes()}, {".stderr", stderr.Bytes()}} {
		if err := nativeSessionProfileWriteNew(filepath.Join(evidence, name+output.suffix), output.raw); err != nil {
			return nil, errors.Join(runErr, err)
		}
	}
	if err := nativeSessionProfileWriteJSON(filepath.Join(evidence, name+".json"), record); err != nil {
		return nil, errors.Join(runErr, err)
	}
	if runErr != nil || stdout.limit || stderr.limit || ctx.Err() != nil {
		return nil, errors.Join(runErr, ctx.Err(), fmt.Errorf("bounded session profile identity probe failed: %s", name))
	}
	return stdout.Bytes(), nil
}

func nativeSessionProfileWriteNew(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	return errors.Join(writeErr, file.Close())
}

func nativeSessionProfileWriteJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return nativeSessionProfileWriteNew(path, append(raw, '\n'))
}
