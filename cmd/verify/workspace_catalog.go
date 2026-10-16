package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const verificationCatalogSchema = "gooo/go-ports-verification-catalog/v1"

// This catalog is a source inventory, never a compiler workspace manifest.
type verificationCatalog struct {
	Schema   string          `json:"schema"`
	Packages []sourcePackage `json:"packages"`
}

type frozenVerificationCatalog struct {
	Catalog verificationCatalog
	Raw     []byte
	Sources map[string][]byte
	Hashes  map[string]string
}

type suiteWorkspace struct {
	Manifest struct {
		Schema string `json:"schema"`
		Entry  struct {
			Package  string `json:"package_path"`
			Activity string `json:"activity"`
		} `json:"entry"`
		Packages []sourcePackage `json:"packages"`
	}
	Sources    map[string][]byte
	Names      []string
	Identities map[string]sourceIdentity
}

func freezeVerificationCatalog(path string) (frozenVerificationCatalog, error) {
	var frozen frozenVerificationCatalog
	raw, err := os.ReadFile(path)
	if err != nil {
		return frozen, err
	}
	if err := json.Unmarshal(raw, &frozen.Catalog); err != nil {
		return frozen, err
	}
	if frozen.Catalog.Schema != verificationCatalogSchema || len(frozen.Catalog.Packages) == 0 {
		return frozen, fmt.Errorf("invalid verification source catalog schema or roster")
	}
	frozen.Raw, frozen.Sources, frozen.Hashes = raw, map[string][]byte{}, map[string]string{}
	for _, pkg := range frozen.Catalog.Packages {
		if !(strings.HasPrefix(pkg.Path, "go/") || strings.HasPrefix(pkg.Path, "gooo/") || strings.HasPrefix(pkg.Path, "app/") || strings.HasPrefix(pkg.Path, "example/")) {
			return frozen, fmt.Errorf("unsupported verification package %q", pkg.Path)
		}
		for _, name := range pkg.Sources {
			if !filepath.IsLocal(name) || filepath.Clean(name) != name {
				return frozen, fmt.Errorf("verification source must be a clean relative path: %q", name)
			}
			if _, exists := frozen.Sources[name]; exists {
				return frozen, fmt.Errorf("duplicate verification source %q", name)
			}
			source, err := os.ReadFile(filepath.Join(filepath.Dir(path), name))
			if err != nil {
				return frozen, err
			}
			frozen.Sources[name] = source
			frozen.Hashes[name] = strings.TrimPrefix(contentDigest(source), "sha256:")
		}
	}
	if _, err := snapshotSourceIdentities(frozen.Catalog.Packages, frozen.Sources); err != nil {
		return frozen, err
	}
	return frozen, nil
}

var catalogImport = regexp.MustCompile(`(?m)^[\t ]*import[\t ]+(?:[A-Za-z_][A-Za-z_0-9]*[\t ]+)?"([^"\r\n]+)"`)

// The prior suites retain their original go/* packages and only their own
// consumer. Validation suites select the original UUID/quote sources and their
// own consumer. ParseCanonical adds only its own Text source to those sources.
// Literal Text suites select only search+literal and their own optional Report
// consumer; literal.gooo is excluded from every prior suite. This preserves all
// prior source rosters and keeps unrelated records out of the pinned compiler's
// 16-record ABI. All bytes come from one snapshot.
func (f frozenVerificationCatalog) workspaceForSuite(s suite) (suiteWorkspace, error) {
	if isHeaderAdditionSuite(s) {
		return f.headerAdditionWorkspace(s)
	}
	if isHeaderSuite(s) {
		return f.headerWorkspace(s)
	}
	if isPathAliasSuite(s) {
		return f.pathAliasWorkspace(s)
	}
	if isDecimalPayloadSuite(s) {
		return f.decimalPayloadWorkspace(s)
	}
	w := suiteWorkspace{Sources: map[string][]byte{}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	contract, err := contractFor(s)
	if err != nil {
		return w, err
	}
	selected := map[string]bool{}
	for _, pkg := range f.Catalog.Packages {
		// The decimal and path additions must not add packages or records to
		// earlier workspaces. Their suites select concrete closures above.
		if pkg.Path == decimalPackage || pkg.Path == pathCleanPackage || pkg.Path == headerPortPackage {
			continue
		}
		// The bounded CEL and AgentMesh rules have separate verifiers and importing
		// workspaces. They must not alter any legacy suite's frozen source roster.
		if pkg.Path == "go/cel.dev/cel-go/codelab" || pkg.Path == "go/google/agentmesh/api" {
			continue
		}
		if (isLiteralTextSuite(s) || isLiteralCutSuite(s)) && pkg.Path != "go/strings" && pkg.Path != s.Package {
			continue
		}
		isUUIDPackage := pkg.Path == uuidPackage || pkg.Path == uuidPackage+"/internal/quote" || pkg.Path == uuidParsePackage
		if strings.HasPrefix(pkg.Path, "go/") && isUUIDPackage != isUUIDSuite(s) {
			continue
		}
		if pkg.Path == uuidParsePackage && !isUUIDParseSuite(s) {
			continue
		}
		if !strings.HasPrefix(pkg.Path, "go/") && pkg.Path != s.Package {
			continue
		}
		names := make([]string, 0, len(pkg.Sources))
		for _, name := range pkg.Sources {
			if name == "ports/strings/cut.gooo" && !isLiteralCutSuite(s) {
				continue
			}
			if name == "ports/strings/literal.gooo" && !isLiteralTextSuite(s) && !isLiteralCutSuite(s) {
				continue
			}
			names = append(names, name)
		}
		if isLiteralTextSuite(s) {
			want := []string{"ports/strings/search.gooo", "ports/strings/literal.gooo"}
			if pkg.Path == "app/literal-text-report" {
				want = []string{"examples/literal-text-report/main.gooo"}
			}
			if !equalSourceNames(names, want) {
				return w, fmt.Errorf("literal Text workspace has unexpected sources for %s", pkg.Path)
			}
		}
		if isLiteralCutSuite(s) {
			want := []string{"ports/strings/search.gooo", "ports/strings/literal.gooo", "ports/strings/cut.gooo"}
			if pkg.Path == literalCutConsumerPackage {
				want = []string{"examples/literal-cut-report/main.gooo"}
			}
			if !equalSourceNames(names, want) {
				return w, fmt.Errorf("literal Cut workspace has unexpected sources for %s", pkg.Path)
			}
		}
		selected[pkg.Path] = true
		w.Manifest.Packages = append(w.Manifest.Packages, sourcePackage{Path: pkg.Path, Sources: names})
		for _, name := range names {
			raw, exists := f.Sources[name]
			if !exists || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("missing or altered frozen source %s", name)
			}
			w.Sources[name] = bytes.Clone(raw)
			w.Names = append(w.Names, name)
		}
	}
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	if err != nil {
		return w, err
	}
	if !selected[s.Package] || w.Identities[contract.Source].Package != s.Package {
		return w, fmt.Errorf("verification catalog lacks trusted entry source for %s:%s", s.Package, s.Activity)
	}
	for name, source := range w.Sources {
		for _, match := range catalogImport.FindAllSubmatch(source, -1) {
			if !selected[string(match[1])] {
				return w, fmt.Errorf("selected source %s imports absent package %s", name, match[1])
			}
		}
	}
	return w, nil
}

func (f frozenVerificationCatalog) decimalPayloadWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	packages := []sourcePackage{{Path: decimalPackage, Sources: []string{"ports/strconv/decimal.gooo"}}}
	if s.Package == payloadAdmissionPackage {
		packages = append(packages, sourcePackage{Path: payloadAdmissionPackage, Sources: []string{"examples/payload-admission/main.gooo"}})
	}
	for _, want := range packages {
		found := 0
		for _, candidate := range f.Catalog.Packages {
			if candidate.Path != want.Path {
				continue
			}
			found++
			if !equalSourceNames(candidate.Sources, want.Sources) {
				return w, fmt.Errorf("decimal/payload workspace source closure for %s", want.Path)
			}
		}
		if found != 1 {
			return w, fmt.Errorf("decimal/payload workspace package count for %s", want.Path)
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("missing or altered frozen decimal/payload source %s", name)
			}
			w.Sources[name] = bytes.Clone(raw)
			w.Names = append(w.Names, name)
		}
	}
	var err error
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	if err != nil {
		return w, err
	}
	for name, raw := range w.Sources {
		imports := catalogImport.FindAllSubmatch(raw, -1)
		want := 0
		if name == "examples/payload-admission/main.gooo" {
			want = 1
		}
		if len(imports) != want || want == 1 && string(imports[0][1]) != decimalPackage {
			return w, fmt.Errorf("unexpected decimal/payload import closure in %s", name)
		}
	}
	return w, nil
}

func (f frozenVerificationCatalog) pathAliasWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	packages := []sourcePackage{{Path: pathCleanPackage, Sources: []string{"ports/path/path.gooo", "ports/path/clean.gooo"}}}
	if s.Package == pathAliasPackage {
		packages = append(packages, sourcePackage{Path: pathAliasPackage, Sources: []string{"examples/path-alias-facts/main.gooo"}})
	}
	for _, want := range packages {
		found := 0
		for _, candidate := range f.Catalog.Packages {
			if candidate.Path != want.Path {
				continue
			}
			found++
			if !equalSourceNames(candidate.Sources, want.Sources) {
				return w, fmt.Errorf("path Clean/alias workspace source closure for %s", want.Path)
			}
		}
		if found != 1 {
			return w, fmt.Errorf("path Clean/alias workspace package count for %s", want.Path)
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("missing or altered frozen path Clean/alias source %s", name)
			}
			w.Sources[name] = bytes.Clone(raw)
			w.Names = append(w.Names, name)
		}
	}
	var err error
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	if err != nil {
		return w, err
	}
	for name, raw := range w.Sources {
		imports := catalogImport.FindAllSubmatch(raw, -1)
		want := 0
		if name == "examples/path-alias-facts/main.gooo" {
			want = 1
		}
		if len(imports) != want || want == 1 && string(imports[0][1]) != pathCleanPackage {
			return w, fmt.Errorf("unexpected path Clean/alias import closure in %s", name)
		}
	}
	return w, nil
}

func equalSourceNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
