// Copyright 2026 Gooo Go Ports contributors. BSD-style terms: ../../LICENSE.
package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed core.source.gooo gooo.workspace.json cases-*.json profile.json SOURCE.json LICENSE-MIT LICENSE-OpenFGA captures/*
var bundled embed.FS

type limitedBuffer struct {
	bytes.Buffer
	remaining int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		return 0, fmt.Errorf("output byte bound exceeded")
	}
	b.remaining -= len(p)
	return b.Buffer.Write(p)
}

type limitedFile struct {
	file      *os.File
	remaining int
}

func (b *limitedFile) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		return 0, fmt.Errorf("stderr exceeded1MiB")
	}
	b.remaining -= len(p)
	return b.file.Write(p)
}
func saveRaw(path string, raw []byte) {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		panic(e)
	}
	_, e = f.Write(raw)
	closeErr := f.Close()
	if e != nil {
		panic(e)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}
func save(path string, value any) {
	raw, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		panic(e)
	}
	saveRaw(path, append(raw, '\n'))
}
func groupExists(pid int) bool { return syscall.Kill(-pid, 0) != syscall.ESRCH }
func stopGroup(pid int) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		if !groupExists(pid) {
			return
		}
		_ = syscall.Kill(-pid, sig)
		until := time.Now().Add(5 * time.Second)
		for groupExists(pid) && time.Now().Before(until) {
			time.Sleep(25 * time.Millisecond)
		}
	}
}
func probe(ctx context.Context, argv []string, env []string) ([]byte, []byte) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, errs := &limitedBuffer{remaining: 1 << 20}, &limitedBuffer{remaining: 1 << 20}
	cmd.Stdout = out
	cmd.Stderr = errs
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	if cmd.Process != nil {
		stopGroup(cmd.Process.Pid)
	}
	if err != nil {
		panic(fmt.Errorf("identity probe failed: %w", err))
	}
	return out.Bytes(), errs.Bytes()
}
func frameLine(ctx context.Context, reader *bufio.Reader) ([]byte, error) {
	type answer struct {
		raw []byte
		err error
	}
	ch := make(chan answer, 1)
	go func() {
		var raw []byte
		for {
			piece, e := reader.ReadSlice('\n')
			raw = append(raw, piece...)
			if len(raw) > 8<<20 {
				ch <- answer{raw, fmt.Errorf("frame exceeded8MiB")}
				return
			}
			if e != bufio.ErrBufferFull {
				ch <- answer{raw, e}
				return
			}
		}
	}()
	select {
	case a := <-ch:
		return a.raw, a.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func fileEntries(path string) []string {
	entries, e := os.ReadDir(path)
	if e != nil {
		panic(e)
	}
	result := []string{}
	for _, entry := range entries {
		result = append(result, entry.Name())
	}
	return result
}
func environment(native string) []string {
	env := os.Environ()
	for _, pair := range []string{"GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOPROXY=off", "GOSUMDB=off", "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY="} {
		env = append(env, pair)
	}
	if native != "" {
		env = append(env, "TMPDIR="+native, "TMP="+native, "TEMP="+native)
	}
	return env
}

type options struct{ compiler, goTool, out string }

func run(parentCtx context.Context, o options) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	out, e := filepath.Abs(o.out)
	if e != nil {
		panic(e)
	}
	info, e := os.Lstat(out)
	require(e == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && len(fileEntries(out)) == 0, "--out must be a caller-created empty persistent directory; evidence is never overwritten")
	saveRaw(filepath.Join(out, ".assessment-session-owner"), nil)
	compiler, e := filepath.Abs(o.compiler)
	if e != nil {
		panic(e)
	}
	goTool, e := filepath.Abs(o.goTool)
	if e != nil {
		panic(e)
	}
	identity, identityErr := probe(parentCtx, []string{compiler, "version", "--build", "--json"}, environment(""))
	saveRaw(filepath.Join(out, "compiler-identity.jsonl"), identity)
	saveRaw(filepath.Join(out, "compiler-identity.stderr"), identityErr)
	id := object(parse(identity))
	require(id["compiler_source_sha"] == engineSHA && id["source_status"] == "CLEAN_VCS" && id["vcs_modified"] == "false" && id["go_version"] == "go1.27.3", "Use an explicit clean experimental2767341 compiler built with Go1.27.3. The default205/native17 pins are unchanged and are not this session profile.")
	version, versionErr := probe(parentCtx, []string{goTool, "version"}, environment(""))
	saveRaw(filepath.Join(out, "go-version.txt"), version)
	saveRaw(filepath.Join(out, "go-version.stderr"), versionErr)
	goVersion := string(bytes.TrimSpace(version))
	require(bytes.HasPrefix(version, []byte("go version go1.27.3 ")), "--go must select Go1.27.3")
	profile := filepath.Join(out, "profile")
	if e = os.Mkdir(profile, 0700); e != nil {
		panic(e)
	}
	metadataRaw, e := bundled.ReadFile("SOURCE.json")
	if e != nil {
		panic(e)
	}
	metadata := object(parse(metadataRaw))
	for _, value := range list(metadata["profile_files"]) {
		record := object(value)
		path := text(record["path"])
		require(filepath.IsLocal(path), "profile path differs")
		raw, e := bundled.ReadFile(path)
		if e != nil {
			panic(e)
		}
		require(hash(raw) == record["sha256"] && len(raw) == count(record["bytes"]), "bundled profile differs")
		target := filepath.Join(profile, path)
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			panic(e)
		}
		saveRaw(target, raw)
	}
	saveRaw(filepath.Join(profile, "SOURCE.json"), metadataRaw)
	profileRaw, _ := bundled.ReadFile("profile.json")
	rows := list(object(parse(profileRaw))["rows"])
	require(len(rows) == 4, "four profile result cases required")
	captureRaw, _ := bundled.ReadFile("captures/manifest.json")
	captures := object(object(parse(captureRaw))["captures"])
	native := filepath.Join(out, "native-work")
	if e = os.Mkdir(native, 0700); e != nil {
		panic(e)
	}
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, compiler, "package", "session")
	cmd.Dir = out
	cmd.Env = environment(native)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 5 * time.Second
	stdin, e := cmd.StdinPipe()
	if e != nil {
		panic(e)
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		panic(e)
	}
	stderr, e := os.OpenFile(filepath.Join(out, "session.stderr"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		panic(e)
	}
	cmd.Stderr = &limitedFile{stderr, 1 << 20}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	if e = cmd.Start(); e != nil {
		stderr.Close()
		panic(e)
	}
	pid := cmd.Process.Pid
	started := time.Now().UTC().Format(time.RFC3339Nano)
	waited := false
	cleaned := false
	history := []map[string]any{}
	contexts := []contextRecord{}
	frames := []fileRef{}
	var parent *fileRef
	buildSHA := ""
	suites := map[string]bool{}
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("%v", r)
		}
		cancel()
		stdin.Close()
		stdout.Close()
		if !waited {
			joined := make(chan error, 1)
			go func() { joined <- cmd.Wait() }()
			stopGroup(pid)
			_ = <-joined
			waited = true
		} else if !cleaned {
			stopGroup(pid)
		}
		stderr.Close()
		failure := any(nil)
		if err != nil {
			failure = err.Error()
		}
		exit := -1
		if cmd.ProcessState != nil {
			exit = cmd.ProcessState.ExitCode()
		}
		save(filepath.Join(out, "session-process.json"), map[string]any{"pid": pid, "started_at_utc": started, "exit_code": exit, "waited": waited, "failure": failure, "process_group_remaining": groupExists(pid), "owned_entries_after_close": fileEntries(native), "history": history})
	}()
	save(filepath.Join(out, "session-started.json"), map[string]any{"pid": pid, "started_at_utc": started, "compiler": reference(compiler), "Go": reference(goTool)})
	reader := bufio.NewReaderSize(stdout, 64<<10)
	exchange := func(name, operation string, args []string) (map[string]any, fileRef, fileRef) {
		request := map[string]any{"schema": "gooo/workspace-session-request/v1", "id": name, "operation": operation}
		if len(args) > 0 {
			request["arguments"] = args
		}
		raw, e := json.Marshal(request)
		if e != nil {
			panic(e)
		}
		raw = append(raw, '\n')
		require(len(raw) < 64<<10, "request exceeded64KiB")
		requestPath := filepath.Join(out, name+"-request.jsonl")
		saveRaw(requestPath, raw)
		if _, e = stdin.Write(raw); e != nil {
			panic(e)
		}
		line, e := frameLine(ctx, reader)
		framePath := filepath.Join(out, name+"-response.jsonl")
		if len(line) > 0 {
			saveRaw(framePath, line)
		}
		if e != nil {
			panic(e)
		}
		f := object(parse(line))
		require(f["schema"] == "gooo/workspace-session-response/v1" && f["id"] == name, "session frame identity differs")
		return f, reference(framePath), reference(requestPath)
	}
	for index, value := range rows {
		row := object(value)
		name, operation := text(row["id"]), text(row["operation"])
		args := []string{"--go", goTool, "--cases", filepath.Join(profile, "cases-"+name+".json"), filepath.Join(profile, "gooo.workspace.json")}
		if operation == "replay" {
			require(parent != nil, "first parent missing")
			args = append([]string{"--receipt", parent.Path}, args...)
		}
		frame, frameRef, requestRef := exchange(name, operation, args)
		require(count(frame["exit_code"]) == 0 && frame["error"] == nil, "session refused request")
		receipt := object(frame["receipt"])
		runtime := object(object(receipt["result"])["runtime"])
		artifact := object(runtime["artifact"])
		require(artifact["executable_verified"] == true && artifact["reused"] == (index > 0), "artifact reuse differs")
		originBuild := object(artifact["source_build"])
		require(originBuild["started"] == true && originBuild["completed"] == true && count(originBuild["exit_code"]) == 0, "historical source build incomplete")
		runs := list(runtime["runs"])
		require(len(runs) == 2, "two fresh native runs missing")
		for _, v := range runs {
			r := object(v)
			require(r["started"] == true && r["completed"] == true && count(r["exit_code"]) == 0 && r["canceled"] == false && r["timed_out"] == false && r["output_truncated"] == false, "current native run incomplete")
		}
		build := object(runtime["build"])
		if index == 0 {
			require(build["started"] == true && build["completed"] == true, "initial build missing")
			buildSHA = text(artifact["source_build_sha256"])
			require(len(buildSHA) == 71 && buildSHA[:7] == "sha256:", "source build identity missing")
			var raw map[string]json.RawMessage
			line, _ := os.ReadFile(frameRef.Path)
			if e = json.Unmarshal(line, &raw); e != nil {
				panic(e)
			}
			saveRaw(filepath.Join(out, "first-receipt.json"), append(raw["receipt"], '\n'))
			ref := reference(filepath.Join(out, "first-receipt.json"))
			parent = &ref
		} else {
			require(build["started"] == false && artifact["source_build_sha256"] == buildSHA && receipt["replayed_from_sha256"] == "sha256:"+parent.SHA, "replay/source build differs")
		}
		suite := text(runtime["runtime_suite_sha256"])
		require(len(suite) == 71 && suite[:7] == "sha256:", "runtime suite identity missing")
		require(!suites[suite], "runtime suite reused")
		suites[suite] = true
		dirs, _ := filepath.Glob(filepath.Join(native, "gooo-composition-runtime-*"))
		require(len(dirs) == 1, "owned artifact population differs")
		captureName := text(row["original_capture"])
		capture := object(captures[captureName])
		refs := map[string]fileRef{}
		for _, key := range []string{"stdout", "process", "source_binding"} {
			refs[key] = reference(filepath.Join(profile, "captures", text(object(capture[key])["path"])))
		}
		c := contextRecord{ID: name, Operation: operation, ProfileRoot: profile, Source: reference(filepath.Join(profile, "core.source.gooo")), Manifest: reference(filepath.Join(profile, "gooo.workspace.json")), Input: reference(filepath.Join(profile, "cases-"+name+".json")), Request: requestRef, Go: reference(goTool), GoVersion: goVersion, Capture: captureName, CaptureFiles: refs}
		if index > 0 {
			c.Parent = parent
		}
		save(filepath.Join(out, name+"-context.json"), c)
		contexts = append(contexts, c)
		frames = append(frames, frameRef)
		history = append(history, map[string]any{"id": name, "finite_passed": runtime["finite_passed"], "finite_total": runtime["finite_total"], "receipt_decision": receipt["decision"], "native_runs": 2, "native_build_started": build["started"], "artifact_reused": artifact["reused"], "runtime_suite_sha256": suite, "frame": frameRef, "parent": c.Parent})
	}
	close, _, _ := exchange("finish", "close", nil)
	require(close["closed"] == true && count(close["exit_code"]) == 0 && close["error"] == nil, "explicit close failed")
	stdin.Close()
	tail, tailErr := frameLine(ctx, reader)
	require(len(tail) == 0 && tailErr == io.EOF, "trailing session output or incomplete close")
	e = cmd.Wait()
	waited = true
	require(e == nil, "session process failed")
	stopGroup(pid)
	cleaned = true
	require(!groupExists(pid) && len(fileEntries(native)) == 0, "owned resources survived close")
	require(reader.Buffered() == 0, "trailing session data")
	views := filepath.Join(out, "views")
	if e = os.Mkdir(views, 0700); e != nil {
		panic(e)
	}
	for i, c := range contexts {
		view, e := display(frames[i], c)
		if e != nil {
			panic(e)
		}
		row := object(rows[i])
		require(is(view["program_value"], row["expected_program_value"]), "actual Assess value differs from authored record")
		execution := object(view["assessment_execution"])
		require(execution["receipt_decision"] == row["expected_outer_decision"] && execution["matched_expected_result_cases"] == count(row["expected_finite_passed"]) && execution["result_cases"] == count(row["expected_finite_total"]), "outer finite result differs")
		save(filepath.Join(views, c.ID+".json"), view)
		line, _ := json.Marshal(view)
		require(len(line) <= 32<<10, "view exceeded32KiB")
		fmt.Println(string(line))
	}
	save(filepath.Join(out, "summary.json"), map[string]any{"status": "FOUR_CAPTURED_ASSESSMENTS_CHECKED", "engine_source": engineSHA, "result_cases": 4, "fresh_native_runs": 8, "native_artifact_builds": 1, "artifact_reuses": 3, "original_host_captures": 2, "intent_capture_combinations": 3, "inner_named_nodes": 4, "inner_leaf_nodes": 3, "host_test_reruns": 0, "model_calls": 0, "session_waited": waited, "owned_entries_after_close": fileEntries(native), "session_process_group_remaining": groupExists(pid), "star_replay_parent": parent, "history": history})
	return nil
}
func main() {
	var o options
	flag.StringVar(&o.compiler, "compiler", "", "explicit clean experimental276 compiler binary")
	flag.StringVar(&o.goTool, "go", "", "Go1.27.3 executable for native execution")
	flag.StringVar(&o.out, "out", "", "caller-created empty persistent evidence directory")
	flag.Parse()
	if o.compiler == "" || o.goTool == "" || o.out == "" {
		flag.Usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, o); err != nil {
		fmt.Fprintln(os.Stderr, "assessment-session:", err)
		os.Exit(1)
	}
}
