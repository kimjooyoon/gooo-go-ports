package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

// The zero value retains the original selector and every legacy fixture.
// A workspace profile is independent from case serialization and partitioning.
type nativeWorkspaceProfile string

const dependencyClosureWorkspaceProfile nativeWorkspaceProfile = "dependency-closure-v1"

func parseNativeWorkspaceProfile(value string, browser bool) (nativeWorkspaceProfile, error) {
	var profile nativeWorkspaceProfile
	switch value {
	case "", "legacy":
	case string(dependencyClosureWorkspaceProfile):
		profile = dependencyClosureWorkspaceProfile
	default:
		return "", fmt.Errorf("unknown --workspace-profile %q; require legacy or dependency-closure-v1", value)
	}
	if browser && profile != "" {
		return "", fmt.Errorf("--workspace-profile dependency-closure-v1 cannot be used with --browser-fixtures")
	}
	return profile, nil
}

func (p nativeWorkspaceProfile) validate() error {
	if p != "" && p != dependencyClosureWorkspaceProfile {
		return fmt.Errorf("unsupported native workspace profile %q", p)
	}
	return nil
}

func (p nativeWorkspaceProfile) validateCompiler(lock upstream) error {
	if err := p.validate(); err != nil {
		return err
	}
	if p != "" && (lock.CompilerRevision != "83928fa4538245abe33b08549da47f8cbdc50ed2" || lock.CompilerVersion != "0.6.32-dev" || lock.GoVersion != "go1.27.2") {
		return fmt.Errorf("dependency-closure-v1 requires pinned compiler839/0.6.32-dev and Go1.27.2")
	}
	return nil
}

func (p nativeWorkspaceProfile) workspaceForSuite(f frozenVerificationCatalog, s suite) (suiteWorkspace, error) {
	if err := p.validate(); err != nil {
		return suiteWorkspace{}, err
	}
	// Always apply the original source exclusions first. In particular, a whole
	// selected package must not re-add literal/Cut/UUID/path/decimal exclusions.
	original, err := f.workspaceForSuite(s)
	if err != nil {
		return original, err
	}
	return p.workspaceForSelected(f, s, original)
}

// Apply the profile to one already-selected legacy workspace. The production
// wrapper always performs the original selector and propagates its error first.
// No source, case, validation result or derived workspace is cached here.
func (p nativeWorkspaceProfile) workspaceForSelected(f frozenVerificationCatalog, s suite, original suiteWorkspace) (suiteWorkspace, error) {
	if err := p.validate(); err != nil {
		return suiteWorkspace{}, err
	}
	if p == "" {
		return original, nil
	}
	if original.Manifest.Schema != "gooo/package-workspace-manifest/v1" || original.Manifest.Entry.Package != s.Package || original.Manifest.Entry.Activity != s.Activity {
		return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 original workspace entry differs")
	}
	packages := map[string]sourcePackage{}
	orderedNames := []string{}
	for _, pkg := range original.Manifest.Packages {
		if pkg.Path == "" || len(pkg.Sources) == 0 || packages[pkg.Path].Path != "" {
			return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 missing or duplicate selected package")
		}
		packages[pkg.Path] = pkg
		orderedNames = append(orderedNames, pkg.Sources...)
	}
	if packages[s.Package].Path == "" || !equalSourceNames(orderedNames, original.Names) {
		return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 selected entry or source order differs")
	}
	identities, err := snapshotSourceIdentities(original.Manifest.Packages, original.Sources)
	if err != nil || !reflect.DeepEqual(identities, original.Identities) {
		return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 original source identities differ: %v", err)
	}
	dependencies := map[string][]string{}
	for _, pkg := range original.Manifest.Packages {
		seenImports := map[string]bool{}
		for _, name := range pkg.Sources {
			raw, ok := original.Sources[name]
			if !ok || len(raw) == 0 || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] || !bytes.Equal(raw, f.Sources[name]) {
				return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 missing or altered selected source %s", name)
			}
			imports, err := workspaceProfileImports(raw)
			if err != nil {
				return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 %s: %w", name, err)
			}
			for _, imported := range imports {
				if packages[imported].Path == "" {
					return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 missing imported package %s", imported)
				}
				if !seenImports[imported] {
					dependencies[pkg.Path] = append(dependencies[pkg.Path], imported)
					seenImports[imported] = true
				}
			}
		}
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(path string) error {
		if state[path] == 1 {
			return fmt.Errorf("dependency-closure-v1 import cycle at %s", path)
		}
		if state[path] == 2 {
			return nil
		}
		state[path] = 1
		for _, imported := range dependencies[path] {
			if err := visit(imported); err != nil {
				return err
			}
		}
		state[path] = 2
		return nil
	}
	if err := visit(s.Package); err != nil {
		return suiteWorkspace{}, err
	}
	reachable := map[string]bool{}
	for path, status := range state {
		reachable[path] = status == 2
	}
	// Do not turn an invalid original source graph into a valid profile by
	// discarding an unrelated cycle before it can be observed.
	for _, pkg := range original.Manifest.Packages {
		if err := visit(pkg.Path); err != nil {
			return suiteWorkspace{}, err
		}
	}
	selected := suiteWorkspace{Sources: map[string][]byte{}}
	selected.Manifest = original.Manifest
	selected.Manifest.Packages = nil
	for _, pkg := range original.Manifest.Packages {
		if !reachable[pkg.Path] {
			continue
		}
		selected.Manifest.Packages = append(selected.Manifest.Packages, sourcePackage{Path: pkg.Path, Sources: append([]string(nil), pkg.Sources...)})
		for _, name := range pkg.Sources {
			selected.Names = append(selected.Names, name)
			selected.Sources[name] = bytes.Clone(original.Sources[name])
		}
	}
	selected.Identities, err = snapshotSourceIdentities(selected.Manifest.Packages, selected.Sources)
	if err != nil {
		return suiteWorkspace{}, err
	}
	contract, err := contractFor(s)
	if err != nil || selected.Identities[contract.Source].Package != s.Package {
		return suiteWorkspace{}, fmt.Errorf("dependency-closure-v1 lacks trusted entry source")
	}
	return selected, nil
}

// Recognize only the current validated catalog's simple one-line import form.
// Unsupported import lines fail closed; this is not a general Gooo parser.
func workspaceProfileImports(raw []byte) ([]string, error) {
	var imports []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "import") {
			continue
		}
		match := catalogImport.FindStringSubmatchIndex(line)
		if match == nil {
			return nil, fmt.Errorf("unsupported workspace import format")
		}
		tail := strings.TrimSpace(line[match[1]:])
		if tail != "" && !strings.HasPrefix(tail, "//") {
			return nil, fmt.Errorf("unsupported workspace import suffix")
		}
		imports = append(imports, line[match[2]:match[3]])
	}
	return imports, nil
}

func (p nativeWorkspaceProfile) identity() map[string]any {
	if p != dependencyClosureWorkspaceProfile {
		return nil
	}
	return map[string]any{
		"schema": "gooo/native-workspace-profile/v1", "name": string(p),
		"basis":             "legacy-selected-workspace",
		"selection":         "entry import closure over whole already-selected packages; original package/source order",
		"compiler_revision": "83928fa4538245abe33b08549da47f8cbdc50ed2", "go_version": "go1.27.2",
	}
}

func (p nativeWorkspaceProfile) decorateSummary(summary map[string]any) {
	if p != dependencyClosureWorkspaceProfile {
		return
	}
	summary["workspace_profile"] = p.identity()
	// These global historical source-roster claims no longer describe a narrowed
	// workspace. Correct only opted-in output, after case-profile decoration.
	for name, replacements := range map[string][][2]string{
		"scope": {
			{"The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged.", "Original case values/order are retained; the explicit workspace profile selects reachable whole already-selected packages."},
			{"All original case values/order/source rosters are retained.", "All original case values/order are retained; the explicit workspace profile selects reachable whole already-selected packages."},
		},
		"uuid_parse_scope": {
			{"All prior 56 suites, 108835 cases, 1416 batch boundaries and source rosters remain unchanged.", "Original inputs/expectations are retained; source rosters belong to the explicit workspace profile."},
			{"All original inputs/expectations/source rosters remain unchanged;", "All original inputs/expectations remain unchanged; source rosters belong to the explicit workspace profile;"},
		},
	} {
		if value, ok := summary[name].(string); ok {
			for _, pair := range replacements {
				value = strings.ReplaceAll(value, pair[0], pair[1])
			}
			summary[name] = value
		}
	}
}

func (p nativeWorkspaceProfile) decorateTimingHeader(header map[string]any) {
	if p == dependencyClosureWorkspaceProfile {
		header["workspace_profile"] = p.identity()
		header["source_set_scope"] = "Each suite binds the exact dependency-closure-v1 source map derived from its original selected workspace. Suite_end stores that map once and each batch binds its SHA-256. The complete frozen catalog and all timing inputs remain hashed and rechecked at close."
	}
}
