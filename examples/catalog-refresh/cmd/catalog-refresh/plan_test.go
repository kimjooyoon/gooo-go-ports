// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	recorded "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
)

func putPlanFile(t *testing.T, dir, name string, raw []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func savedFilePlan(t *testing.T, previous []byte, state string, pages []refresh.SnapshotPage) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	putPlanFile(t, dir, "previous.bin", previous)
	refs := make([]map[string]string, 0, len(pages))
	for i, p := range pages {
		// Names intentionally differ from the calling process's directory.
		request, response := string(rune('a'+i))+".request", string(rune('a'+i))+".response"
		putPlanFile(t, dir, request, []byte(p.RequestJSON))
		putPlanFile(t, dir, response, []byte(p.ResponseJSON))
		refs = append(refs, map[string]string{"request_file": request, "response_file": response})
	}
	raw, err := json.Marshal(map[string]any{"previous_catalog_file": "previous.bin", "collector_state": state, "pages": refs})
	if err != nil {
		t.Fatal(err)
	}
	return putPlanFile(t, dir, "plan.json", raw), raw
}

func runSavedFilePlan(t *testing.T, path string) report {
	t.Helper()
	var out, diagnostics bytes.Buffer
	if code := runCommand([]string{"--plan", path}, brokenReader{}, &out, &diagnostics); code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, diagnostics.String(), out.String())
	}
	var result report
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFilePlanPreservesPagesAndBindsBothInputs(t *testing.T) {
	first := page("10", "", "next", `[{"name":"δοκιμή","inputSchema":{"type":"object"}}]`)
	first.RequestJSON = "\r\n  " + first.RequestJSON + "\r\n"
	first.ResponseJSON = strings.Replace(first.ResponseJSON, "δοκιμή", `\u03b4οκιμή`, 1) + " \r\n"
	pages := []refresh.SnapshotPage{first, page("11", "next", "", "[]")}
	previous := []byte{0xff, 0, '\r', '\n', 0x80}
	path, plan := savedFilePlan(t, previous, "SUCCESS", pages)
	assembled, hash, err := assembleFilePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	planSum := sha256.Sum256(plan)
	if hash != "sha256:"+hex.EncodeToString(planSum[:]) {
		t.Fatal("plan digest does not bind exact bytes")
	}
	input, err := decodeInput(assembled)
	if err != nil || !bytes.Equal(input.previous, previous) || input.collector != "SUCCESS" || len(input.pages) != len(pages) {
		t.Fatalf("assembled input differs: %v %+v", err, input)
	}
	for i := range pages {
		if input.pages[i] != pages[i] {
			t.Fatal("frame bytes or ordering changed")
		}
	}
	result := runSavedFilePlan(t, path)
	direct := execute(t, assembled)
	if result.Receipt.State != "APPLIED" || result.FilePlanSHA256 != hash || direct.FilePlanSHA256 != "" || result.InputSHA256 != direct.InputSHA256 || !bytes.Equal(result.StoredCatalog.Bytes, direct.StoredCatalog.Bytes) {
		t.Fatal("plan route does not match validated stdin semantics")
	}
	if raw, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, plan) {
		t.Fatal("plan file changed")
	}
}

func TestFilePlanCollectionStatesAndEmptyPages(t *testing.T) {
	previous := []byte{0xff, 0x00, 0x80, '\n'}
	for _, state := range []string{"SUCCESS", "ERROR", "TIMEOUT", "CANCELLED"} {
		t.Run(state, func(t *testing.T) {
			path, _ := savedFilePlan(t, previous, state, []refresh.SnapshotPage{page("1", "", "pending", "[]")})
			r := runSavedFilePlan(t, path)
			if r.Receipt.State != "RETAINED" || !bytes.Equal(r.StoredCatalog.Bytes, previous) || r.CollectorState != state || r.CollectorStateBasis != "CALLER_SUPPLIED" {
				t.Fatal("incomplete collection lost previous bytes or collector state")
			}
		})
	}
	path, _ := savedFilePlan(t, previous, "SUCCESS", nil)
	if r := runSavedFilePlan(t, path); r.Receipt.State != "RETAINED" || !bytes.Equal(r.StoredCatalog.Bytes, previous) {
		t.Fatal("missing pages erased previous bytes")
	}
	path, _ = savedFilePlan(t, previous, "SUCCESS", []refresh.SnapshotPage{page("2", "", "", "[]")})
	if r := runSavedFilePlan(t, path); r.Receipt.State != "APPLIED" || bytes.Equal(r.StoredCatalog.Bytes, previous) {
		t.Fatal("explicit terminal empty page did not replace catalog")
	}
}

func invalidFilePlan(t *testing.T, path string) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	if code := runCommand([]string{"--plan", path}, brokenReader{}, &out, &diagnostics); code != 1 || out.Len() != 0 || !json.Valid(diagnostics.Bytes()) {
		t.Fatalf("invalid plan yielded code=%d stdout=%s stderr=%s", code, out.String(), diagnostics.String())
	}
}

func TestFilePlanStrictEnvelopeAndPathStrings(t *testing.T) {
	path, raw := savedFilePlan(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")})
	valid := string(raw)
	cases := map[string]string{
		"trailing":            valid + `{}`,
		"unknown":             strings.Replace(valid, `{`, `{"extra":1,`, 1),
		"duplicate":           strings.Replace(valid, `{`, `{"collector_state":"ERROR",`, 1),
		"escaped duplicate":   strings.Replace(valid, `{`, `{"collector_\u0073tate":"ERROR",`, 1),
		"wrong case":          strings.Replace(valid, `"pages"`, `"Pages"`, 1),
		"null previous":       strings.Replace(valid, `"previous.bin"`, `null`, 1),
		"empty path":          strings.Replace(valid, `"previous.bin"`, `""`, 1),
		"nul path":            strings.Replace(valid, `"previous.bin"`, `"previous\u0000.bin"`, 1),
		"high surrogate path": strings.Replace(valid, `"previous.bin"`, `"\ud800"`, 1),
		"low surrogate path":  strings.Replace(valid, `"previous.bin"`, `"\udc00"`, 1),
		"unknown collector":   strings.Replace(valid, `"SUCCESS"`, `"UNKNOWN"`, 1),
		"missing collector":   strings.Replace(valid, `"collector_state":"SUCCESS",`, ``, 1),
		"null pages":          `{"previous_catalog_file":"previous.bin","collector_state":"SUCCESS","pages":null}`,
		"not page object":     `{"previous_catalog_file":"previous.bin","collector_state":"SUCCESS","pages":[null]}`,
		"inline mixing":       strings.Replace(valid, `"request_file"`, `"request_json"`, 1),
		"nested duplicate":    strings.Replace(valid, `"request_file":`, `"response_file":"a.response","request_file":`, 1),
		"nested extra":        strings.Replace(valid, `"request_file":`, `"extra":0,"request_file":`, 1),
		"invalid utf8":        valid + string([]byte{0xff}),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			putPlanFile(t, filepath.Dir(path), "plan.json", []byte(text))
			invalidFilePlan(t, path)
		})
	}
	for _, raw := range []string{`"\ud83d\ude00"`, `"literal\\ud800"`, `"dir\/name"`} {
		if _, err := planString([]byte(raw)); err != nil {
			t.Fatalf("valid JSON string refused %s: %v", raw, err)
		}
	}
}

func TestFilePlanLimitsAndMalformedFrames(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "frame size", "plan size", "assembled size", "invalid frame utf8", "inner duplicate", "inner surrogate", "too many pages"} {
		t.Run(kind, func(t *testing.T) {
			pages := []refresh.SnapshotPage{page("1", "", "", "[]")}
			if kind == "too many pages" {
				pages = make([]refresh.SnapshotPage, 17)
			}
			path, _ := savedFilePlan(t, nil, "SUCCESS", pages)
			dir := filepath.Dir(path)
			switch kind {
			case "missing":
				if err := os.Remove(filepath.Join(dir, "a.request")); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(filepath.Join(dir, "a.request")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, "a.request"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "frame size":
				putPlanFile(t, dir, "a.request", bytes.Repeat([]byte(" "), recorded.MaxFrameBytes+1))
			case "plan size":
				putPlanFile(t, dir, "plan.json", bytes.Repeat([]byte(" "), recorded.MaxInputBytes+1))
			case "assembled size":
				putPlanFile(t, dir, "previous.bin", bytes.Repeat([]byte{0xff}, recorded.MaxInputBytes))
			case "invalid frame utf8":
				putPlanFile(t, dir, "a.response", []byte{0xff})
			case "inner duplicate":
				putPlanFile(t, dir, "a.response", []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[],"tools":[]}}`))
			case "inner surrogate":
				putPlanFile(t, dir, "a.response", []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"\ud800","inputSchema":{}}]}}`))
			}
			invalidFilePlan(t, path)
		})
	}
}

func TestFilePlanAbsolutePathAndRegularSymlink(t *testing.T) {
	path, raw := savedFilePlan(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")})
	dir := filepath.Dir(path)
	absolute, err := json.Marshal(filepath.Join(dir, "previous.bin"))
	if err != nil {
		t.Fatal(err)
	}
	putPlanFile(t, dir, "plan.json", bytes.Replace(raw, []byte(`"previous.bin"`), absolute, 1))
	if r := runSavedFilePlan(t, path); r.Receipt.State != "APPLIED" {
		t.Fatal("absolute path failed")
	}
	if err := os.Symlink(filepath.Join(dir, "a.response"), filepath.Join(dir, "response-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	putPlanFile(t, dir, "plan.json", bytes.Replace(raw, []byte(`"a.response"`), []byte(`"response-link"`), 1))
	runSavedFilePlan(t, path)
}

func TestFilePlanEscapedUnicodeNamesSelectExactFile(t *testing.T) {
	path, raw := savedFilePlan(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")})
	dir := filepath.Dir(path)
	request, err := os.ReadFile(filepath.Join(dir, "a.request"))
	if err != nil {
		t.Fatal(err)
	}
	putPlanFile(t, dir, "😀.request", request)
	putPlanFile(t, dir, "plan.json", bytes.Replace(raw, []byte(`"a.request"`), []byte(`"\ud83d\ude00.request"`), 1))
	if r := runSavedFilePlan(t, path); r.Receipt.State != "APPLIED" {
		t.Fatal("surrogate-pair path did not select the supplied Unicode file")
	}
}

func TestFilePlanFIFORejectedWithoutWaiting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX named FIFO control")
	}
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo unavailable")
	}
	path, _ := savedFilePlan(t, nil, "SUCCESS", []refresh.SnapshotPage{page("1", "", "", "[]")})
	fifo := filepath.Join(filepath.Dir(path), "a.response")
	if err := os.Remove(fifo); err != nil {
		t.Fatal(err)
	}
	if raw, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, raw)
	}
	done := make(chan int, 1)
	go func() {
		var out, diagnostics bytes.Buffer
		code := runCommand([]string{"--plan", path}, brokenReader{}, &out, &diagnostics)
		if out.Len() != 0 || !json.Valid(diagnostics.Bytes()) {
			done <- -1
			return
		}
		done <- code
	}()
	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("FIFO returned %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FIFO read blocked")
	}
}

func TestFilePlanArgumentsDoNotReadStdin(t *testing.T) {
	for _, args := range [][]string{{"--plan"}, {"--plan", ""}, {"--plan", "x", "extra"}, {"--help", "--plan", "x"}} {
		var out, diagnostics bytes.Buffer
		if code := runCommand(args, brokenReader{}, &out, &diagnostics); code != 2 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "UNSUPPORTED_ARGUMENTS") {
			t.Fatalf("invalid plan args: code=%d out=%s stderr=%s", code, out.String(), diagnostics.String())
		}
	}
}
