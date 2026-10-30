//go:build linux || darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	nativeSessionRequestSchema  = "gooo/workspace-session-request/v1"
	nativeSessionResponseSchema = "gooo/workspace-session-response/v1"
	nativeSessionReceiptSchema  = "gooo/workspace-body-execution-receipt/v1"
	nativeSessionMaxRequest     = 64 << 10
	// This finite transport bound is separate from the verifier's input/receipt
	// semantics. A larger frame fails; no successful receipt is truncated.
	nativeSessionMaxFrame  = 16 << 20
	nativeSessionMaxStderr = 1 << 20
	// One deadline covers gate acquisition, close response, EOF, stderr drain and Wait.
	nativeSessionCloseLimit = 5 * time.Second
	// Failure cleanup adds at most7s group termination and one shared2s join window.
	nativeSessionTermLimit = 5 * time.Second
	nativeSessionKillLimit = 2 * time.Second
	nativeSessionJoinLimit = 2 * time.Second
)

type nativeSessionFileReference struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type nativeSessionRequestObservation struct {
	ID               string                      `json:"id"`
	Operation        string                      `json:"operation"`
	OriginalArgv     []string                    `json:"original_argv,omitempty"`
	SessionArguments []string                    `json:"session_arguments,omitempty"`
	JSONFlagRemoved  bool                        `json:"json_flag_removed"`
	GoInjected       bool                        `json:"go_injected"`
	Request          nativeSessionFileReference  `json:"request"`
	Frame            *nativeSessionFileReference `json:"frame,omitempty"`
	Receipt          *nativeSessionFileReference `json:"receipt,omitempty"`
	Error            string                      `json:"error,omitempty"`
}

type nativeSessionFrame struct {
	raw       []byte
	reference *nativeSessionFileReference
	err       error
}

type nativeSessionClient struct {
	compiler, goTool, root, evidence, nativeRoot string
	cmd                                          *exec.Cmd
	stdin                                        io.WriteCloser
	stdout, stderr                               io.ReadCloser
	ctx                                          context.Context
	cancel                                       context.CancelFunc
	stopParent                                   func() bool
	gate                                         chan struct{}
	frames                                       chan nativeSessionFrame
	readerDone, stderrDone, waitDone, done       chan struct{}
	waitOnce, finishOnce, closeOnce              sync.Once
	mu                                           sync.Mutex
	sequence                                     uint64
	observations                                 []nativeSessionRequestObservation
	frameReferences                              []nativeSessionFileReference
	firstErr                                     error
	closing, closed, waited, streamsJoined       bool
	closeSent, closeReceived                     bool
	exitCode                                     *int
	waitError                                    string
	userNS, systemNS                             int64
	started, finished                            time.Time
	closeErr                                     error
}

// newNativeSessionClient owns only its new session-transport subtree. The
// caller's evidenceRoot may already contain identity probes or batch files.
// Profile/producer identity and receipt semantic inspection remain caller-owned.
func newNativeSessionClient(ctx context.Context, compiler, goTool, evidenceRoot string) (*nativeSessionClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for label, path := range map[string]string{"compiler": compiler, "Go tool": goTool, "evidence root": evidenceRoot} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("session %s must be a clean absolute path", label)
		}
	}
	info, err := os.Lstat(evidenceRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("session evidence root must be an existing directory")
	}
	for _, path := range []string{compiler, goTool} {
		info, err = os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("session executable is not a regular file: %s", path)
		}
	}
	c := &nativeSessionClient{compiler: compiler, goTool: goTool, root: evidenceRoot, evidence: filepath.Join(evidenceRoot, "session-transport"),
		gate: make(chan struct{}, 1), frames: make(chan nativeSessionFrame, 1), readerDone: make(chan struct{}), stderrDone: make(chan struct{}), waitDone: make(chan struct{}), done: make(chan struct{})}
	c.nativeRoot = filepath.Join(c.evidence, "native-work")
	if err = os.Mkdir(c.evidence, 0700); err != nil {
		return nil, fmt.Errorf("create exclusive session evidence: %w", err)
	}
	for _, path := range []string{filepath.Join(c.evidence, "requests"), filepath.Join(c.evidence, "frames"), c.nativeRoot} {
		if err = os.Mkdir(path, 0700); err != nil {
			return nil, err
		}
	}
	stderrFile, err := os.OpenFile(filepath.Join(c.evidence, "session.stderr"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.cmd = exec.CommandContext(c.ctx, compiler, "package", "session")
	c.cmd.Dir = evidenceRoot
	c.cmd.Env = append(os.Environ(), "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=", "TMPDIR="+c.nativeRoot, "TMP="+c.nativeRoot, "TEMP="+c.nativeRoot)
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.cmd.WaitDelay = 2 * time.Second
	c.cmd.Cancel = func() error {
		if c.cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-c.cmd.Process.Pid, syscall.SIGTERM)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		stderrFile.Close()
		c.cancel()
		return nil, err
	}
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		stderrFile.Close()
		c.stdin.Close()
		c.cancel()
		return nil, err
	}
	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		stderrFile.Close()
		c.stdin.Close()
		c.stdout.Close()
		c.cancel()
		return nil, err
	}
	c.started = time.Now().UTC()
	if err = c.cmd.Start(); err != nil {
		stderrFile.Close()
		c.stdin.Close()
		c.stdout.Close()
		c.stderr.Close()
		c.cancel()
		return nil, fmt.Errorf("start package session: %w", err)
	}
	go c.readFrames()
	go c.readStderr(stderrFile)
	stopParent := context.AfterFunc(ctx, func() { c.fail(fmt.Errorf("session parent canceled: %w", ctx.Err())) })
	c.mu.Lock()
	c.stopParent = stopParent
	c.mu.Unlock()
	if err = c.writeJSON(filepath.Join(c.evidence, "started.json"), map[string]any{"schema": "gooo/native-session-process-start/v1", "pid": c.cmd.Process.Pid, "argv": []string{compiler, "package", "session"}, "cwd": evidenceRoot, "started_at_utc": c.started.Format(time.RFC3339Nano), "compiler": compiler, "configured_go_tool": goTool, "native_work": c.nativeRoot}); err != nil {
		c.fail(err)
		<-c.done
		return nil, err
	}
	return c, nil
}

func nativeSessionReference(path string, raw []byte) nativeSessionFileReference {
	sum := sha256.Sum256(raw)
	return nativeSessionFileReference{path, len(raw), hex.EncodeToString(sum[:])}
}
func nativeSessionWrite(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	return errors.Join(writeErr, f.Close())
}
func (c *nativeSessionClient) writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return nativeSessionWrite(path, append(raw, '\n'))
}
func (c *nativeSessionClient) setError(err error) {
	if err != nil {
		c.mu.Lock()
		if c.firstErr == nil {
			c.firstErr = err
		}
		c.mu.Unlock()
	}
}
func (c *nativeSessionClient) failure() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.firstErr != nil {
		return c.firstErr
	}
	return errors.New("session is closed")
}
func (c *nativeSessionClient) fail(err error) { c.setError(err); c.finish() }
func (c *nativeSessionClient) readFrames() {
	defer close(c.readerDone)
	defer close(c.frames)
	r := bufio.NewReaderSize(c.stdout, 64<<10)
	number := 0
	for {
		var raw []byte
		var err error
		for {
			var piece []byte
			piece, err = r.ReadSlice('\n')
			if len(raw)+len(piece) > nativeSessionMaxFrame {
				left := nativeSessionMaxFrame - len(raw)
				raw = append(raw, piece[:left]...)
				err = fmt.Errorf("session frame exceeds %d bytes; retained bounded prefix", nativeSessionMaxFrame)
				break
			}
			raw = append(raw, piece...)
			if err != bufio.ErrBufferFull {
				break
			}
		}
		frame := nativeSessionFrame{raw: raw, err: err}
		if len(raw) > 0 {
			number++
			path := filepath.Join(c.evidence, "frames", fmt.Sprintf("%06d.jsonl", number))
			if e := nativeSessionWrite(path, raw); e != nil {
				frame.err = errors.Join(err, e)
			} else {
				ref := nativeSessionReference(path, raw)
				frame.reference = &ref
				c.mu.Lock()
				c.frameReferences = append(c.frameReferences, ref)
				c.mu.Unlock()
			}
		}
		if err == nil && (len(raw) == 0 || raw[len(raw)-1] != '\n') {
			frame.err = errors.New("session response lacks LF")
		}
		select {
		case c.frames <- frame:
		case <-c.ctx.Done():
			return
		}
		if frame.err != nil {
			return
		}
	}
}
func (c *nativeSessionClient) readStderr(out *os.File) {
	defer close(c.stderrDone)
	defer out.Close()
	remaining := nativeSessionMaxStderr
	buf := make([]byte, 32<<10)
	for {
		n, err := c.stderr.Read(buf)
		if n > 0 {
			kept := n
			if kept > remaining {
				kept = remaining
			}
			if _, e := out.Write(buf[:kept]); e != nil {
				c.fail(e)
				return
			}
			remaining -= kept
			if kept != n {
				c.fail(fmt.Errorf("session stderr exceeds %d bytes", nativeSessionMaxStderr))
				return
			}
		}
		if err != nil {
			if err != io.EOF && c.ctx.Err() == nil {
				c.fail(fmt.Errorf("read session stderr: %w", err))
			}
			return
		}
	}
}
func nativeSessionGroupExists(pid int) bool { return syscall.Kill(-pid, 0) != syscall.ESRCH }
func nativeSessionStopGroup(pid int) {
	for _, step := range []struct {
		signal syscall.Signal
		limit  time.Duration
	}{{syscall.SIGTERM, nativeSessionTermLimit}, {syscall.SIGKILL, nativeSessionKillLimit}} {
		if !nativeSessionGroupExists(pid) {
			return
		}
		_ = syscall.Kill(-pid, step.signal)
		until := time.Now().Add(step.limit)
		for nativeSessionGroupExists(pid) && time.Now().Before(until) {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
func (c *nativeSessionClient) startWait() {
	c.waitOnce.Do(func() {
		go func() {
			err := c.cmd.Wait()
			c.mu.Lock()
			c.waited = true
			if err != nil {
				c.waitError = err.Error()
			}
			if c.cmd.ProcessState != nil {
				x := c.cmd.ProcessState.ExitCode()
				c.exitCode = &x
				c.userNS = c.cmd.ProcessState.UserTime().Nanoseconds()
				c.systemNS = c.cmd.ProcessState.SystemTime().Nanoseconds()
			}
			c.mu.Unlock()
			close(c.waitDone)
		}()
	})
}
func nativeSessionWaitContext(ctx context.Context, done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
	}
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
func (c *nativeSessionClient) finish() {
	c.finishOnce.Do(func() {
		go func() {
			c.cancel()
			_ = c.stdin.Close()
			_ = c.stdout.Close()
			_ = c.stderr.Close()
			// Wait concurrently with process-group cleanup: a reaped direct parent must
			// not hide surviving owned children, and a zombie parent must not stall it.
			c.startWait()
			nativeSessionStopGroup(c.cmd.Process.Pid)
			joinCtx, joinCancel := context.WithTimeout(context.Background(), nativeSessionJoinLimit)
			waited := nativeSessionWaitContext(joinCtx, c.waitDone)
			readJoined := nativeSessionWaitContext(joinCtx, c.readerDone)
			stderrJoined := nativeSessionWaitContext(joinCtx, c.stderrDone)
			joinCancel()
			joined := waited && readJoined && stderrJoined
			if !joined {
				c.setError(errors.New("session process/pipe join did not complete"))
			}
			entries, err := os.ReadDir(c.nativeRoot)
			if err != nil {
				c.setError(err)
			}
			if len(entries) > 0 {
				c.setError(errors.New("owned native resources remain after session close"))
			}
			if nativeSessionGroupExists(c.cmd.Process.Pid) {
				c.setError(errors.New("owned session process group remains after close"))
			}
			c.mu.Lock()
			c.streamsJoined = joined
			c.closed = true
			c.finished = time.Now().UTC()
			if c.waitError != "" && c.firstErr == nil {
				c.firstErr = errors.New(c.waitError)
			}
			c.mu.Unlock()
			c.mu.Lock()
			stopParent := c.stopParent
			c.mu.Unlock()
			if stopParent != nil {
				stopParent()
			}
			if err = c.writeJSON(filepath.Join(c.evidence, "process.json"), c.Summary()); err != nil {
				c.setError(err)
			}
			close(c.done)
		}()
	})
}

// Summary is a diagnostic owner/lifecycle record. Shared process wait times are
// not request Rusage and it makes no native/artifact success claim.
func (c *nativeSessionClient) Summary() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	observations := append([]nativeSessionRequestObservation(nil), c.observations...)
	frames := append([]nativeSessionFileReference(nil), c.frameReferences...)
	entries := []string{}
	if found, err := os.ReadDir(c.nativeRoot); err == nil {
		for _, item := range found {
			entries = append(entries, item.Name())
		}
	}
	var failure any
	if c.firstErr != nil {
		failure = c.firstErr.Error()
	}
	var exit any
	if c.exitCode != nil {
		exit = *c.exitCode
	}
	var userNS, systemNS, finished any
	if c.waited && c.exitCode != nil {
		userNS, systemNS = c.userNS, c.systemNS
	}
	if !c.finished.IsZero() {
		finished = c.finished.Format(time.RFC3339Nano)
	}
	return map[string]any{"schema": "gooo/native-session-transport-summary/v1", "compiler": c.compiler, "configured_go_tool": c.goTool, "evidence_root": c.evidence, "native_work": c.nativeRoot, "pid": c.cmd.Process.Pid, "argv": []string{c.compiler, "package", "session"}, "closed": c.closed, "waited": c.waited, "streams_joined": c.streamsJoined, "exit_code": exit, "error": failure, "process_group_remaining": nativeSessionGroupExists(c.cmd.Process.Pid), "owned_entries_after_close": entries, "close_request_sent": c.closeSent, "close_response_received": c.closeReceived, "started_at_utc": c.started.Format(time.RFC3339Nano), "finished_at_utc": finished, "requests": observations, "frames": frames, "stderr_path": filepath.Join(c.evidence, "session.stderr"), "waited_process_user_ns": userNS, "waited_process_system_ns": systemNS, "waited_process_resources_available": c.waited && c.exitCode != nil, "close_handshake_limit_ns": nativeSessionCloseLimit.Nanoseconds(), "close_cleanup_limit_ns": (nativeSessionTermLimit + nativeSessionKillLimit + nativeSessionJoinLimit).Nanoseconds(), "close_protocol_cleanup_budget_ns": (nativeSessionCloseLimit + nativeSessionTermLimit + nativeSessionKillLimit + nativeSessionJoinLimit).Nanoseconds(), "resource_scope": "Whole waited session process only; per-request command resources unavailable. No native/build/artifact counters or source identity acceptance."}
}

type nativeSessionArguments struct {
	operation               string
	wire                    []string
	jsonRemoved, goInjected bool
}

func (c *nativeSessionClient) ownedPath(path string, mustExist bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return errors.New("session file path must be clean and absolute")
	}
	relative, err := filepath.Rel(c.root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("session file path escapes caller evidence root")
	}
	current := c.root
	parts := strings.Split(relative, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e != nil {
			if !mustExist && i == len(parts)-1 && os.IsNotExist(e) {
				return nil
			}
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("session file path contains symlink")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return errors.New("session parent path is not a directory")
		}
		if i == len(parts)-1 {
			if !mustExist {
				return errors.New("refuse to overwrite existing receipt")
			}
			if !info.Mode().IsRegular() {
				return errors.New("session input is not a regular file")
			}
		}
	}
	return nil
}
func (c *nativeSessionClient) arguments(compiler, receiptPath string, args []string) (nativeSessionArguments, error) {
	parsed := nativeSessionArguments{}
	if compiler != c.compiler {
		return parsed, errors.New("session compiler differs from configured path")
	}
	if err := c.ownedPath(receiptPath, false); err != nil {
		return parsed, err
	}
	if len(args) < 5 || len(args) > 50 || args[0] != "package" || (args[1] != "execute" && args[1] != "replay") {
		return parsed, errors.New("session accepts only native verifier package execute/replay arguments")
	}
	parsed.operation = args[1]
	seen := map[string]bool{}
	values := map[string]string{}
	manifest := ""
	for i := 2; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			if seen[arg] {
				return parsed, fmt.Errorf("duplicate session flag %s", arg)
			}
			seen[arg] = true
			switch arg {
			case "--json":
				parsed.jsonRemoved = true
			case "--cases", "--go", "--receipt":
				if i+1 >= len(args) {
					return parsed, fmt.Errorf("missing session flag value %s", arg)
				}
				i++
				values[arg] = args[i]
				parsed.wire = append(parsed.wire, arg, args[i])
			default:
				return parsed, fmt.Errorf("unsupported session flag %s", arg)
			}
		} else {
			if i != len(args)-1 || manifest != "" {
				return parsed, errors.New("session workspace must be the final argument")
			}
			manifest = arg
		}
	}
	if !parsed.jsonRemoved || values["--cases"] == "" || manifest == "" {
		return parsed, errors.New("session requires one --json, --cases and final workspace")
	}
	if (parsed.operation == "replay") != (values["--receipt"] != "") {
		return parsed, errors.New("session replay parent flag differs from operation")
	}
	for _, path := range []string{manifest, values["--cases"]} {
		if err := c.ownedPath(path, true); err != nil {
			return parsed, err
		}
		if path == receiptPath {
			return parsed, errors.New("receipt aliases current input")
		}
	}
	if manifest == values["--cases"] {
		return parsed, errors.New("cases and workspace paths alias")
	}
	if parent := values["--receipt"]; parent != "" {
		if err := c.ownedPath(parent, true); err != nil {
			return parsed, err
		}
		if parent == manifest || parent == values["--cases"] {
			return parsed, errors.New("replay parent aliases current input")
		}
	}
	if tool, ok := values["--go"]; ok {
		if tool != c.goTool {
			return parsed, errors.New("session --go differs from configured absolute Go tool")
		}
	} else {
		parsed.wire = append(parsed.wire, "--go", c.goTool)
		parsed.goInjected = true
	}
	parsed.wire = append(parsed.wire, manifest)
	if len(parsed.wire) > 48 {
		return parsed, errors.New("session argument count exceeds48")
	}
	return parsed, nil
}

func nativeSessionCheckJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("session JSON is not valid UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate or malformed session JSON key %v", key)
				}
				seen[name] = true
				if err = value(); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("malformed session JSON object")
			}
			return nil
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("malformed session JSON array")
			}
			return nil
		default:
			return errors.New("unexpected session JSON delimiter")
		}
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing session JSON data")
	}
	return nil
}
func nativeSessionDecodeFrame(raw []byte, id, operation string) ([]byte, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("session frame lacks LF")
	}
	if err := nativeSessionCheckJSON(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for key := range fields {
		switch key {
		case "schema", "id", "exit_code", "receipt", "closed", "error":
		default:
			return nil, fmt.Errorf("unknown session response field %s", key)
		}
	}
	var f struct {
		Schema   string          `json:"schema"`
		ID       string          `json:"id"`
		ExitCode *int            `json:"exit_code"`
		Receipt  json.RawMessage `json:"receipt"`
		Closed   bool            `json:"closed"`
		Error    string          `json:"error"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Schema != nativeSessionResponseSchema || f.ID != id || len(f.ID) > 128 || f.ExitCode == nil {
		return nil, errors.New("session response schema/id/exit field differs")
	}
	if *f.ExitCode != 0 || f.Error != "" {
		return nil, fmt.Errorf("session command failed (exit %d): %s", *f.ExitCode, f.Error)
	}
	if operation == "close" {
		if !f.Closed || len(f.Receipt) > 0 {
			return nil, errors.New("invalid explicit session close response")
		}
		return nil, nil
	}
	if f.Closed || len(f.Receipt) == 0 || bytes.Equal(f.Receipt, []byte("null")) || len(f.Receipt) > nativeSessionMaxFrame {
		return nil, errors.New("session response has no execution receipt")
	}
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal(f.Receipt, &receipt); err != nil {
		return nil, err
	}
	var schema string
	if err := json.Unmarshal(receipt["schema"], &schema); err != nil || schema != nativeSessionReceiptSchema {
		return nil, errors.New("session inner receipt schema differs")
	}
	if e := receipt["error"]; len(e) > 0 && !bytes.Equal(e, []byte("null")) && !bytes.Equal(e, []byte(`""`)) {
		return nil, errors.New("session inner receipt contains an error")
	}
	return append(append([]byte(nil), f.Receipt...), '\n'), nil
}
func (c *nativeSessionClient) exchange(ctx context.Context, operation string, arguments []string, original []string, parsed nativeSessionArguments) ([]byte, int, error) {
	c.mu.Lock()
	c.sequence++
	id := fmt.Sprintf("native-%06d", c.sequence)
	c.mu.Unlock()
	request := struct {
		Schema    string   `json:"schema"`
		ID        string   `json:"id"`
		Operation string   `json:"operation"`
		Arguments []string `json:"arguments,omitempty"`
	}{nativeSessionRequestSchema, id, operation, arguments}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, -1, err
	}
	raw = append(raw, '\n')
	if len(raw) > nativeSessionMaxRequest || len(id) > 128 {
		return nil, -1, errors.New("session request bound exceeded")
	}
	path := filepath.Join(c.evidence, "requests", id+".jsonl")
	if err = nativeSessionWrite(path, raw); err != nil {
		return nil, -1, err
	}
	observation := nativeSessionRequestObservation{ID: id, Operation: operation, OriginalArgv: append([]string(nil), original...), SessionArguments: append([]string(nil), arguments...), JSONFlagRemoved: parsed.jsonRemoved, GoInjected: parsed.goInjected, Request: nativeSessionReference(path, raw)}
	c.mu.Lock()
	index := len(c.observations)
	c.observations = append(c.observations, observation)
	c.mu.Unlock()
	written := make(chan error, 1)
	go func() { _, e := c.stdin.Write(raw); written <- e }()
	select {
	case err = <-written:
	case <-ctx.Done():
		err = ctx.Err()
	case <-c.done:
		err = c.failure()
	}
	if err != nil {
		return nil, index, err
	}
	var frame nativeSessionFrame
	select {
	case f, ok := <-c.frames:
		if !ok {
			return nil, index, errors.New("session stdout closed before response")
		}
		frame = f
	case <-ctx.Done():
		return nil, index, ctx.Err()
	case <-c.done:
		return nil, index, c.failure()
	}
	c.mu.Lock()
	c.observations[index].Frame = frame.reference
	c.mu.Unlock()
	if frame.err != nil {
		return nil, index, fmt.Errorf("read session response: %w", frame.err)
	}
	receipt, err := nativeSessionDecodeFrame(frame.raw, id, operation)
	return receipt, index, err
}

func (c *nativeSessionClient) Execute(ctx context.Context, compiler, receiptPath string, args ...string) ([]byte, error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.failure()
	}
	defer func() { <-c.gate }()
	c.mu.Lock()
	closed := c.closing || c.closed
	c.mu.Unlock()
	if closed || c.ctx.Err() != nil {
		return nil, c.failure()
	}
	parsed, err := c.arguments(compiler, receiptPath, args)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { c.fail(fmt.Errorf("session request canceled: %w", ctx.Err())) })
	defer stop()
	receipt, index, err := c.exchange(ctx, parsed.operation, parsed.wire, append([]string{compiler}, args...), parsed)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = nativeSessionWrite(receiptPath, receipt)
	}
	c.mu.Lock()
	if index >= 0 {
		if err != nil {
			c.observations[index].Error = err.Error()
		} else {
			ref := nativeSessionReference(receiptPath, receipt)
			c.observations[index].Receipt = &ref
		}
	}
	c.mu.Unlock()
	if err != nil {
		c.fail(err)
		<-c.done
		return nil, err
	}
	return receipt, nil
}

func (c *nativeSessionClient) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closing = true
		c.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), nativeSessionCloseLimit)
		defer cancel()
		select {
		case c.gate <- struct{}{}:
			defer func() { <-c.gate }()
		case <-c.done:
			c.closeErr = c.firstFailure()
			return
		case <-ctx.Done():
			c.fail(errors.New("close blocked by active session request"))
			<-c.done
			c.closeErr = c.firstFailure()
			return
		}
		if c.ctx.Err() == nil {
			c.mu.Lock()
			c.closeSent = true
			c.mu.Unlock()
			_, index, err := c.exchange(ctx, "close", nil, nil, nativeSessionArguments{})
			if err == nil {
				c.mu.Lock()
				c.closeReceived = true
				c.mu.Unlock()
				_ = c.stdin.Close()
				select {
				case f, ok := <-c.frames:
					if ok && (len(f.raw) > 0 || f.err != io.EOF) {
						err = errors.New("unexpected session output after close")
					}
				case <-ctx.Done():
					err = ctx.Err()
				}
			}
			if err == nil && !nativeSessionWaitContext(ctx, c.stderrDone) {
				err = errors.New("session stderr did not close")
			}
			if err == nil {
				c.startWait()
				if !nativeSessionWaitContext(ctx, c.waitDone) {
					err = errors.New("session process did not exit after close")
				}
			}
			if err != nil {
				c.mu.Lock()
				if index >= 0 {
					c.observations[index].Error = err.Error()
				}
				c.mu.Unlock()
				c.setError(err)
			}
		} else {
			c.setError(fmt.Errorf("session owner canceled before explicit close: %w", c.ctx.Err()))
		}
		c.finish()
		<-c.done
		c.closeErr = c.firstFailure()
	})
	return c.closeErr
}
func (c *nativeSessionClient) firstFailure() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.firstErr
}
