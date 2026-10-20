package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	apiPolicyTestOriginalCompiler = "83928fa4538245abe33b08549da47f8cbdc50ed2"
	apiPolicyTestCurrentCompiler  = "205896a413e5dea92366309c35ae85e542d055f6"
	apiPolicyTestRevision         = "sha256:1904100e57ff22b00f4db1597c4f12ebb3c6893063d6deac69e2eb516452a2ca"
	apiPolicyTestOriginal         = "sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46"
)

// These are source/package validator tests. They neither execute a compiler nor
// substitute fixture data for the separate actual native/Wasm execution gates.
func apiPolicyTestDocuments(t *testing.T) (map[string][]byte, lockfile, string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	documents := map[string][]byte{}
	dir := filepath.Join(root, "examples/api-policy-revision")
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			t.Fatalf("source fixture unexpectedly contains a symlink: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		documents[filepath.ToSlash(rel)] = raw
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "upstream.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock lockfile
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	return documents, lock, root
}

func apiPolicyTestHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func apiPolicyTestTempDir(t *testing.T) string {
	t.Helper()
	// macOS commonly exposes its temp root through /var -> /private/var.
	// Resolve that ambient alias so a refusal test targets only its own symlink.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func apiPolicyTestGeneration(t *testing.T, documents map[string][]byte, name string) map[string]string {
	t.Helper()
	var generation struct {
		Assets map[string]string `json:"assets"`
	}
	if err := json.Unmarshal(documents[name], &generation); err != nil {
		t.Fatal(err)
	}
	return generation.Assets
}

func apiPolicyTestRequiredPaths(t *testing.T, documents map[string][]byte) []string {
	t.Helper()
	paths := []string{
		"examples/api-policy-revision/generation.json",
		"examples/api-policy-revision/profiles/original/generation.json",
	}
	for _, prefix := range []string{"examples/api-policy-revision/", "examples/api-policy-revision/profiles/original/"} {
		for name := range apiPolicyTestGeneration(t, documents, prefix+"generation.json") {
			paths = append(paths, prefix+name)
		}
	}
	// Four packaging assets are outside the original28 bindings but remain part
	// of its immutable33-file archive. Test every one, including both licenses.
	for _, name := range []string{"go.mod", "NOTICE.md", "LICENSE-APACHE-2.0", "oracle/upstream/LICENSE"} {
		paths = append(paths, "examples/api-policy-revision/profiles/original/"+name)
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func TestAPIPolicyBuildBindsCompleteFrozenProfiles(t *testing.T) {
	documents, lock, root := apiPolicyTestDocuments(t)
	read, err := apiPolicyReadDocumentsAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, documents) {
		t.Fatal("physical reader did not capture the complete current module bytes")
	}
	bound, err := apiPolicyCheckDocuments(documents, lock)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []struct {
		name, generation, compiler string
		count                      int
		bindings                   apiPolicyBindings
	}{
		{"examples/api-policy-revision/", apiPolicyTestRevision, apiPolicyTestCurrentCompiler, 38, bound.Revision},
		{"examples/api-policy-revision/profiles/original/", apiPolicyTestOriginal, apiPolicyTestOriginalCompiler, 28, bound.Original},
	} {
		if profile.bindings.CompilerSourceSHA != profile.compiler || profile.bindings.GenerationSHA256 != profile.generation || len(profile.bindings.Assets) != profile.count {
			t.Fatalf("incomplete or rebound profile metadata: %s", profile.name)
		}
		if got := apiPolicyTestHash(documents[profile.name+"generation.json"]); got != profile.generation {
			t.Fatalf("raw generation identity changed: %s", profile.name)
		}
		want := apiPolicyTestGeneration(t, documents, profile.name+"generation.json")
		if !reflect.DeepEqual(profile.bindings.Assets, want) {
			t.Fatalf("not every sealed asset binding was retained: %s", profile.name)
		}
		for name, hash := range profile.bindings.Assets {
			if apiPolicyTestHash(documents[profile.name+name]) != hash {
				t.Fatalf("fixture asset hash differs: %s%s", profile.name, name)
			}
		}
	}
	count := 0
	for name := range documents {
		if strings.HasPrefix(name, "examples/api-policy-revision/profiles/original/") {
			count++
		}
	}
	if count != 33 {
		t.Fatal("frozen original archive must retain exactly33 physical files", count)
	}
}

func TestAPIPolicyBuildRejectsEverySealedOrPackagingByteChangeAndOmission(t *testing.T) {
	documents, lock, _ := apiPolicyTestDocuments(t)
	for _, name := range apiPolicyTestRequiredPaths(t, documents) {
		t.Run(name, func(t *testing.T) {
			if len(documents[name]) == 0 {
				t.Fatal("missing baseline source fixture")
			}
			changed := maps.Clone(documents)
			changed[name] = append(bytes.Clone(documents[name]), '\n')
			if _, err := apiPolicyCheckDocuments(changed, lock); err == nil {
				t.Fatal("accepted a changed sealed/archive byte", name)
			}
			delete(changed, name)
			if _, err := apiPolicyCheckDocuments(changed, lock); err == nil {
				t.Fatal("accepted a missing sealed/archive file", name)
			}
		})
	}
}

func TestAPIPolicyBuildRejectsUnsealedRuntimeAndArchiveAdditions(t *testing.T) {
	documents, lock, _ := apiPolicyTestDocuments(t)
	for _, name := range []string{
		"examples/api-policy-revision/unsealed.go",
		"examples/api-policy-revision/internal/flags/unsealed.go",
		"examples/api-policy-revision/profiles/original/unsealed.go",
		"examples/api-policy-revision/profiles/original/README-extra.md",
		"examples/api-policy-revision/../outside.go",
		"../outside.go",
	} {
		t.Run(name, func(t *testing.T) {
			changed := maps.Clone(documents)
			changed[name] = []byte("package unsealed\n")
			if _, err := apiPolicyCheckDocuments(changed, lock); err == nil {
				t.Fatal("accepted an unbound runtime/archive/escaping path", name)
			}
		})
	}
}

func TestAPIPolicyBuildRejectsWrongToolchainPins(t *testing.T) {
	documents, lock, _ := apiPolicyTestDocuments(t)
	for _, changed := range []lockfile{
		{CompilerRevision: strings.Repeat("0", 40), CompilerVersion: lock.CompilerVersion, GoVersion: lock.GoVersion},
		{CompilerRevision: lock.CompilerRevision, CompilerVersion: "different", GoVersion: lock.GoVersion},
		{CompilerRevision: lock.CompilerRevision, CompilerVersion: lock.CompilerVersion, GoVersion: "go1.27.1"},
	} {
		if _, err := apiPolicyCheckDocuments(documents, changed); err == nil {
			t.Fatal("accepted a different compiler/toolchain pin")
		}
	}
}

func TestAPIPolicySourceJSONAndPathsAreUnambiguous(t *testing.T) {
	for _, raw := range []string{
		`{"schema":"first","schema":"second"}`,
		`{"schema":"first","\u0073chema":"second"}`,
		`{"schema":"first"} {}`,
		`{"schema":"first","extra":true}`,
	} {
		var target struct {
			Schema string `json:"schema"`
		}
		if apiPolicyDecode([]byte(raw), &target) == nil {
			t.Fatal("accepted an ambiguous source JSON record", raw)
		}
	}
	for _, name := range []string{"", ".", "..", "../x", "/absolute", "./a", "a/../b", "a//b", `a\b`} {
		if apiPolicyLocalPath(name) {
			t.Fatal("accepted an escaping or ambiguous package path", name)
		}
	}
	for _, name := range []string{"canonical/main.gooo", "profiles/original/generation.json", "internal/flags/generated.go"} {
		if !apiPolicyLocalPath(name) {
			t.Fatal("refused a canonical local source path", name)
		}
	}
}

func TestAPIPolicyPhysicalReaderRejectsSymlinkAndExtraRuntimeSource(t *testing.T) {
	documents, lock, _ := apiPolicyTestDocuments(t)
	root := apiPolicyTestTempDir(t)
	for name, raw := range documents {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := apiPolicyReadDocumentsAt(root); err != nil {
		t.Fatal("regular temporary source fixture refused", err)
	}
	path := filepath.Join(root, "examples/api-policy-revision/canonical/main.gooo")
	original := bytes.Clone(documents["examples/api-policy-revision/canonical/main.gooo"])
	outside := filepath.Join(apiPolicyTestTempDir(t), "same-bytes.gooo")
	if err := os.WriteFile(outside, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := apiPolicyReadDocumentsAt(root); err == nil {
		t.Fatal("accepted symlink source even though its bytes match")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := apiPolicyReadDocumentsAt(root); err == nil {
		t.Fatal("accepted a source file whose promised0644 mode changed")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(root, "examples/api-policy-revision")
	held := filepath.Join(root, "held-module")
	if err := os.Rename(module, held); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(held, module); err != nil {
		t.Fatal(err)
	}
	if _, err := apiPolicyReadDocumentsAt(root); err == nil {
		t.Fatal("accepted a symlink source-module ancestor")
	}
	if err := os.Remove(module); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(held, module); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(root, "examples/api-policy-revision/profiles/original/extra.go")
	if err := os.WriteFile(extra, []byte("package nativepolicy\n"), 0644); err != nil {
		t.Fatal(err)
	}
	read, err := apiPolicyReadDocumentsAt(root)
	if err == nil {
		if _, err := apiPolicyCheckDocuments(read, lock); err == nil {
			t.Fatal("accepted an extra original archive/runtime file")
		}
	}
}

func TestAPIPolicyNativeCLICanonicalBytesRetainExactlyTheJSONBody(t *testing.T) {
	// A fixture of serialization only. It is not a fabricated native receipt.
	body := []byte("{\"raw\":\"\\u003c\\r\\n\\t\\u0000é😀\",\"count\":0,\"flag\":false}")
	stdout := append(bytes.Clone(body), '\n')
	canonical, err := apiPolicyCanonicalCLI(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, body) || !bytes.Equal(stdout, append(bytes.Clone(body), '\n')) {
		t.Fatal("native CLI framing removal changed JSON bytes or mutated stdout")
	}
	if apiPolicyTestHash(canonical) == apiPolicyTestHash(stdout) {
		t.Fatal("raw CLI stdout and no-LF canonical identities were conflated")
	}
	for _, raw := range [][]byte{
		nil, {}, []byte("{}"), []byte("{}\r\n"), []byte("{}\n\n"),
		[]byte(" {\"x\":1}\n"), []byte("{ \"x\":1}\n"), []byte("{} {}\n"),
		[]byte("{\"x\":1,\"x\":2}\n"), []byte("{\"x\":1,\"\\u0078\":2}\n"),
		[]byte("{\"x\":\"\xff\"}\n"), []byte("{}\x00\n"),
	} {
		if _, err := apiPolicyCanonicalCLI(raw); err == nil {
			t.Fatalf("accepted invalid or noncanonical CLI framing/body: %q", raw)
		}
	}
}

func TestAPIPolicyPublicMetadataRetainsNativeLimitsAndEveryBinding(t *testing.T) {
	documents, lock, _ := apiPolicyTestDocuments(t)
	bound, err := apiPolicyCheckDocuments(documents, lock)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"schema": "gooo/api-policy-browser-bridge-info/v1",
		"scope":  "unsigned_historical_hypothetical_comparison",
		"modes":  []string{"baseline", "compare", "replay"},
		"profiles": map[string]string{
			"original": "cel-api-original-v1", "candidate": "cel-api-readonly-principal-v1",
		},
		"limits": map[string]int{
			"text_utf8_bytes": 65536, "baseline_input_bytes": 4194304, "compare_input_bytes": 4194304,
			"replay_input_bytes": 16777216, "original_proof_bytes": 4194304,
			"native_facts_raw_bytes": 3539232, "output_bytes": 67108864,
		},
		"bindings": map[string]apiPolicyBindings{"original": bound.Original, "revision": bound.Revision},
	}
	if !reflect.DeepEqual(apiPolicyExpectedInfo(bound), want) {
		t.Fatal("public startup metadata changed a native mode/profile/domain/full binding")
	}
}

func TestAPIPolicyPublishedSourceSubsetContainsOnlyBoundSourceAndPackaging(t *testing.T) {
	documents, _, _ := apiPolicyTestDocuments(t)
	for _, profile := range []struct {
		prefix string
		names  []string
	}{
		{"examples/api-policy-revision/", apiPolicyPublishedRevision},
		{"examples/api-policy-revision/profiles/original/", apiPolicyPublishedOriginal},
	} {
		allowed := apiPolicyTestGeneration(t, documents, profile.prefix+"generation.json")
		allowed["generation.json"] = apiPolicyTestHash(documents[profile.prefix+"generation.json"])
		if strings.Contains(profile.prefix, "/profiles/original/") {
			for _, name := range []string{"go.mod", "NOTICE.md", "LICENSE-APACHE-2.0", "oracle/upstream/LICENSE"} {
				allowed[name] = apiPolicyTestHash(documents[profile.prefix+name])
			}
		}
		seen := map[string]bool{}
		if len(profile.names) == 0 {
			t.Fatal("published source subset cannot be empty")
		}
		for _, name := range profile.names {
			if seen[name] || !apiPolicyLocalPath(name) || allowed[name] == "" || len(documents[profile.prefix+name]) == 0 {
				t.Fatal("unbound, duplicate, private or non-local published source", profile.prefix+name)
			}
			seen[name] = true
		}
	}
}

func TestAPIPolicyPublicRecordRefusesPrivatePathsBeforeWriting(t *testing.T) {
	root := apiPolicyTestTempDir(t)
	for _, private := range []string{"/Users/synthetic/private", "/home/synthetic/private", filepath.Join(root, "private-command")} {
		path := filepath.Join(root, "public.json")
		if _, err := apiPolicyPutJSON(path, map[string]any{"argv": []string{private}}, root); err == nil {
			t.Fatal("accepted private command/path data into public JSON")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("privacy refusal left a public file behind", err)
		}
	}
	path := filepath.Join(root, "sanitized.json")
	want := map[string]any{"source": "source/revision/compare.gooo", "exit_code": 0,
		"stdout": map[string]any{"bytes": 3, "sha256": strings.Repeat("0", 64)}}
	raw, err := apiPolicyPutJSON(path, want, root)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, raw) || bytes.Contains(raw, []byte(root)) {
		t.Fatal("sanitized record was not written as its exact returned public bytes", err)
	}
}

func TestAPIPolicyBridgeImportsOnlyTheSealedPublicLibraries(t *testing.T) {
	_, _, root := apiPolicyTestDocuments(t)
	path := filepath.Join(root, "tools/api-policy-browser-bridge.go.txt")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]bool{}
	for _, spec := range parsed.Imports {
		name, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		imports[name] = true
	}
	want := map[string]bool{
		"encoding/json": true, "errors": true, "math": true, "syscall/js": true,
		"github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision": true,
		"github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy":      true,
	}
	if !reflect.DeepEqual(imports, want) {
		t.Fatal("bridge imports changed; no compiler/oracle/effect host belongs in this byte-only bridge", imports)
	}
}
