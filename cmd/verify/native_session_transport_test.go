//go:build linux || darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A local test process speaks only the documented JSONL shape. It never loads
// a Gooo compiler, invokes a Go oracle, or runs generated/native artifact code.
func init() {
	if os.Getenv("GOOO_SESSION_TRANSPORT_TEST_CHILD") == "1" {
		os.Exit(nativeSessionTransportStub())
	}
}
func nativeSessionTransportStub() int {
	mode := os.Getenv("GOOO_SESSION_TRANSPORT_TEST_MODE")
	if mode == "exit-idle" {
		return 0
	}
	if mode == "stderr-bound" {
		_, _ = os.Stderr.Write(bytes.Repeat([]byte("e"), nativeSessionMaxStderr+1))
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), nativeSessionMaxRequest)
	for scanner.Scan() {
		var request struct {
			Schema, ID, Operation string
			Arguments             []string
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			return 4
		}
		if request.Operation == "close" {
			if mode == "close-hang" {
				for {
					time.Sleep(time.Second)
				}
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"schema": nativeSessionResponseSchema, "id": request.ID, "exit_code": 0, "closed": true})
			return 0
		}
		if mode == "hang" {
			for {
				time.Sleep(time.Second)
			}
		}
		if mode == "early-eof" {
			return 0
		}
		if mode == "parent-gone-child" {
			child := exec.Command("/bin/sh", "-c", "sleep 60")
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			if child.Start() != nil {
				return 5
			}
			// Parent exits without joining; transport still owns the child's PGID.
			return 0
		}
		if mode == "oversized" {
			_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), nativeSessionMaxFrame+1))
			_, _ = os.Stdout.Write([]byte("\n"))
			return 0
		}
		if mode == "partial" {
			_, _ = os.Stdout.Write([]byte(`{"schema":"gooo/workspace-session-response/v1"`))
			return 0
		}
		if mode == "duplicate" {
			fmt.Fprintf(os.Stdout, "{\"schema\":%q,\"schema\":%q,\"id\":%q,\"exit_code\":0}\n", nativeSessionResponseSchema, nativeSessionResponseSchema, request.ID)
			continue
		}
		if mode == "nested-duplicate" {
			fmt.Fprintf(os.Stdout, "{\"schema\":%q,\"id\":%q,\"exit_code\":0,\"receipt\":{\"schema\":%q,\"x\":1,\"x\":2}}\n", nativeSessionResponseSchema, request.ID, nativeSessionReceiptSchema)
			continue
		}
		if mode == "raw" {
			fmt.Fprintf(os.Stdout, "{\"schema\":%q,\"id\":%q,\"exit_code\":0,\"receipt\":{ \"schema\" : %q, \"decision\" : \"PROGRESS\", \"n\" : 1e0 }}\n", nativeSessionResponseSchema, request.ID, nativeSessionReceiptSchema)
			continue
		}
		response := map[string]any{"schema": nativeSessionResponseSchema, "id": request.ID, "exit_code": 0, "receipt": map[string]any{"schema": nativeSessionReceiptSchema, "decision": "PROGRESS", "stub_arguments": request.Arguments}}
		switch mode {
		case "wrong-schema":
			response["schema"] = "wrong"
		case "wrong-id":
			response["id"] = "another-request"
		case "missing-exit":
			delete(response, "exit_code")
		case "bool-exit":
			response["exit_code"] = false
		case "error":
			response["error"] = "local stub refusal"
		case "nonzero":
			response["exit_code"] = 2
		case "missing-receipt":
			delete(response, "receipt")
		case "null-receipt":
			response["receipt"] = nil
		case "receipt-error":
			response["receipt"] = map[string]any{"schema": nativeSessionReceiptSchema, "error": "stub inner error"}
		case "unexpected-close":
			response["closed"] = true
		}
		if json.NewEncoder(os.Stdout).Encode(response) != nil {
			return 6
		}
	}
	return 0
}
func nativeSessionShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func nativeSessionFixture(t *testing.T, mode string) (*nativeSessionClient, string, string, string) {
	return nativeSessionFixtureWithStorage(t, mode, nativeSessionEvidencePlain)
}

func nativeSessionFixtureWithStorage(t *testing.T, mode string, storage nativeSessionEvidenceStorage) (*nativeSessionClient, string, string, string) {
	t.Helper()
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	compiler := filepath.Join(root, "stub compiler")
	script := "#!/bin/sh\nGORACE=atexit_sleep_ms=0 GOOO_SESSION_TRANSPORT_TEST_CHILD=1 GOOO_SESSION_TRANSPORT_TEST_MODE=" + nativeSessionShellQuote(mode) + " exec " + nativeSessionShellQuote(binary) + " \"$@\"\n"
	if err = os.WriteFile(compiler, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "identity-probe.json"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	batch := filepath.Join(root, "batch with spaces")
	if err = os.Mkdir(batch, 0700); err != nil {
		t.Fatal(err)
	}
	workspace, cases := filepath.Join(batch, "workspace.json"), filepath.Join(batch, "cases.json")
	for _, path := range []string{workspace, cases} {
		if err = os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var c *nativeSessionClient
	if storage == nativeSessionEvidencePlain {
		c, err = newNativeSessionClient(context.Background(), compiler, binary, root)
	} else {
		c, err = newNativeSessionClientWithStorage(context.Background(), compiler, binary, root, storage)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, batch, workspace, cases
}
func nativeSessionAssertJoined(t *testing.T, c *nativeSessionClient) {
	t.Helper()
	s := c.Summary()
	for _, key := range []string{"closed", "waited", "streams_joined"} {
		if s[key] != true {
			t.Fatalf("%s=%v; summary=%v", key, s[key], s)
		}
	}
	if s["process_group_remaining"] != false || len(s["owned_entries_after_close"].([]string)) != 0 {
		t.Fatalf("owned resources survived: %v", s)
	}
}
func TestNativeSessionTransportRawReceiptAndArgv(t *testing.T) {
	c, batch, workspace, cases := nativeSessionFixture(t, "raw")
	execution := filepath.Join(batch, "execution.json")
	raw, err := c.Execute(context.Background(), c.compiler, execution, "package", "execute", "--json", "--cases", cases, workspace)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{ "schema" : "gooo/workspace-body-execution-receipt/v1", "decision" : "PROGRESS", "n" : 1e0 }` + "\n")
	saved, _ := os.ReadFile(execution)
	if !bytes.Equal(raw, want) || !bytes.Equal(saved, want) {
		t.Fatalf("receipt bytes re-marshaled: %q", raw)
	}
	replay := filepath.Join(batch, "replay.json")
	if _, err = c.Execute(context.Background(), c.compiler, replay, "package", "replay", "--json", "--receipt", execution, "--cases", cases, "--go", c.goTool, workspace); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	nativeSessionAssertJoined(t, c)
	observations := c.Summary()["requests"].([]nativeSessionRequestObservation)
	if len(observations) != 3 || !observations[0].GoInjected || observations[1].GoInjected || !observations[0].JSONFlagRemoved {
		t.Fatalf("argument provenance differs: %+v", observations)
	}
	if observations[0].OriginalArgv[2] != "execute" || strings.Contains(strings.Join(observations[0].SessionArguments, " "), "--json") {
		t.Fatal("CLI prefix/json flag leaked into session arguments")
	}
	requestRaw, err := os.ReadFile(observations[1].Request.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(requestRaw, []byte(execution)) {
		t.Fatal("saved replay request lost exact parent path")
	}
	if _, err = os.Stat(filepath.Join(c.root, "identity-probe.json")); err != nil {
		t.Fatal("caller identity evidence changed")
	}
	if err = c.Close(); err != nil {
		t.Fatal("Close is not idempotent", err)
	}
}
func TestNativeSessionTransportRejectsLocalArgumentsAndOverwrite(t *testing.T) {
	c, batch, workspace, cases := nativeSessionFixture(t, "valid")
	existing := filepath.Join(batch, "existing.json")
	_ = os.WriteFile(existing, []byte("original"), 0600)
	base := []string{"package", "execute", "--json", "--cases", cases, workspace}
	tests := map[string][]string{
		"unknown operation":  {"package", "run", "--json", "--cases", cases, workspace},
		"duplicate json":     {"package", "execute", "--json", "--json", "--cases", cases, workspace},
		"duplicate cases":    {"package", "execute", "--json", "--cases", cases, "--cases", cases, workspace},
		"missing json":       {"package", "execute", "--cases", cases, workspace},
		"unknown flag":       {"package", "execute", "--json", "--timing", "--cases", cases, workspace},
		"wrong Go":           {"package", "execute", "--json", "--cases", cases, "--go", "/another/go", workspace},
		"missing parent":     {"package", "replay", "--json", "--cases", cases, workspace},
		"extra parent":       {"package", "execute", "--json", "--receipt", cases, "--cases", cases, workspace},
		"relative workspace": {"package", "execute", "--json", "--cases", cases, "workspace.json"},
		"extra positional":   {"package", "execute", "--json", "extra", "--cases", cases, workspace},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Execute(context.Background(), c.compiler, filepath.Join(batch, "out-"+strings.ReplaceAll(name, " ", "-")+".json"), args...); err == nil {
				t.Fatal("accepted invalid native verifier argv")
			}
		})
	}
	if _, err := c.Execute(context.Background(), c.compiler, existing, base...); err == nil {
		t.Fatal("overwrote existing receipt")
	}
	if b, _ := os.ReadFile(existing); string(b) != "original" {
		t.Fatal("existing batch evidence changed")
	}
	link := filepath.Join(batch, "linked.json")
	if err := os.Symlink(cases, link); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Execute(context.Background(), c.compiler, filepath.Join(batch, "out-link.json"), "package", "execute", "--json", "--cases", link, workspace); err == nil {
		t.Fatal("accepted symlink input")
	}
	if _, err := c.Execute(context.Background(), "/wrong/compiler", filepath.Join(batch, "out-compiler.json"), base...); err == nil {
		t.Fatal("accepted mismatched compiler")
	}
	if _, err := c.Execute(context.Background(), c.compiler, filepath.Join(t.TempDir(), "outside.json"), base...); err == nil {
		t.Fatal("receipt escaped caller root")
	}
	if len(c.Summary()["requests"].([]nativeSessionRequestObservation)) != 0 {
		t.Fatal("invalid calls reached wire")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	nativeSessionAssertJoined(t, c)
}
func TestNativeSessionTransportRefusesBadFrames(t *testing.T) {
	for _, mode := range []string{"wrong-schema", "wrong-id", "missing-exit", "bool-exit", "error", "nonzero", "missing-receipt", "null-receipt", "receipt-error", "unexpected-close", "duplicate", "nested-duplicate", "early-eof", "partial"} {
		t.Run(mode, func(t *testing.T) {
			c, batch, workspace, cases := nativeSessionFixture(t, mode)
			path := filepath.Join(batch, "execution.json")
			if _, err := c.Execute(context.Background(), c.compiler, path, "package", "execute", "--json", "--cases", cases, workspace); err == nil {
				t.Fatal("accepted invalid frame")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("invalid frame acquired a saved success receipt")
			}
			if err := c.Close(); err == nil {
				t.Fatal("broken protocol acquired successful Close")
			}
			nativeSessionAssertJoined(t, c)
		})
	}
}
func TestNativeSessionTransportCancellationAndBounds(t *testing.T) {
	for _, mode := range []string{"hang", "oversized", "stderr-bound", "parent-gone-child"} {
		t.Run(mode, func(t *testing.T) {
			c, batch, workspace, cases := nativeSessionFixture(t, mode)
			limit := 200 * time.Millisecond
			if mode == "parent-gone-child" {
				limit = 3 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), limit)
			defer cancel()
			if _, err := c.Execute(ctx, c.compiler, filepath.Join(batch, "execution.json"), "package", "execute", "--json", "--cases", cases, workspace); err == nil {
				t.Fatal("unbounded/broken stub succeeded")
			}
			_ = c.Close()
			nativeSessionAssertJoined(t, c)
			if mode == "parent-gone-child" && c.Summary()["exit_code"] != 0 {
				t.Fatal("direct parent had not naturally exited before cleanup", c.Summary())
			}
			if b, _ := os.ReadFile(filepath.Join(c.evidence, "session.stderr")); len(b) > nativeSessionMaxStderr {
				t.Fatal("stderr cap exceeded")
			}
			for _, item := range c.Summary()["frames"].([]nativeSessionFileReference) {
				if item.Bytes > nativeSessionMaxFrame {
					t.Fatal("response capture cap exceeded")
				}
			}
		})
	}
}
func TestNativeSessionTransportSerializesAndClosesIdleCancellation(t *testing.T) {
	c, batch, workspace, cases := nativeSessionFixture(t, "valid")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := c.Execute(context.Background(), c.compiler, filepath.Join(batch, fmt.Sprintf("execution-%d.json", index)), "package", "execute", "--json", "--cases", cases, workspace)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	nativeSessionAssertJoined(t, c)
	observations := c.Summary()["requests"].([]nativeSessionRequestObservation)
	if len(observations) != 3 || observations[0].ID == observations[1].ID {
		t.Fatal("single-inflight IDs differ")
	}
	ctx, cancel := context.WithCancel(context.Background())
	root := t.TempDir()
	idle, err := newNativeSessionClient(ctx, c.compiler, c.goTool, root)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-idle.done:
	case <-time.After(5 * time.Second):
		t.Fatal("idle parent cancellation did not join")
	}
	if err = idle.Close(); err == nil {
		t.Fatal("parent cancellation lost failure")
	}
	nativeSessionAssertJoined(t, idle)
}
func TestNativeSessionTransportConstructorOwnership(t *testing.T) {
	c, _, _, _ := nativeSessionFixture(t, "valid")
	if _, err := newNativeSessionClient(context.Background(), c.compiler, c.goTool, c.root); err == nil {
		t.Fatal("reused transport evidence directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newNativeSessionClient(ctx, c.compiler, c.goTool, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled constructor=%v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeSessionTransportCloseBudgetAndCPUAvailability(t *testing.T) {
	t.Run("availability", func(t *testing.T) {
		c, _, _, _ := nativeSessionFixture(t, "valid")
		before := c.Summary()
		if before["waited_process_user_ns"] != nil || before["waited_process_system_ns"] != nil || before["waited_process_resources_available"] != false {
			t.Fatal("unwaited resources appeared measured", before)
		}
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		after := c.Summary()
		user, ok := after["waited_process_user_ns"].(int64)
		system, other := after["waited_process_system_ns"].(int64)
		if !ok || !other || user < 0 || system < 0 || after["waited_process_resources_available"] != true {
			t.Fatal("actual waited resources unavailable", after)
		}
		nativeSessionAssertJoined(t, c)
	})
	t.Run("shared_close_deadline", func(t *testing.T) {
		c, _, _, _ := nativeSessionFixture(t, "close-hang")
		started := time.Now()
		if err := c.Close(); err == nil {
			t.Fatal("hanging close succeeded")
		}
		elapsed := time.Since(started)
		budget := nativeSessionCloseLimit + nativeSessionTermLimit + nativeSessionKillLimit + nativeSessionJoinLimit
		if elapsed > budget+time.Second {
			t.Fatalf("close exceeded declared handshake+cleanup budget: %v > %v", elapsed, budget)
		}
		nativeSessionAssertJoined(t, c)
	})
}

func TestNativeSessionTransportCanceledOwnerCloseRefusal(t *testing.T) {
	c, _, _, _ := nativeSessionFixture(t, "exit-idle")
	select {
	case <-c.readerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("idle stub did not exit")
	}
	// Model the owner's context becoming canceled before its failure callback.
	// The already-dead direct parent still has a successful actual exit status.
	c.cancel()
	if err := c.Close(); err == nil {
		t.Fatal("canceled owner without explicit close became success")
	}
	nativeSessionAssertJoined(t, c)
	if c.Summary()["exit_code"] != 0 || c.Summary()["close_response_received"] != false {
		t.Fatal("control did not retain successful parent exit without close", c.Summary())
	}
}
