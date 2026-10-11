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
		if !(strings.HasPrefix(pkg.Path, "go/") || strings.HasPrefix(pkg.Path, "app/") || strings.HasPrefix(pkg.Path, "example/")) {
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

// Every suite retains all go/* port packages, but only its own consumer. This
// keeps unrelated consumer records out of the pinned compiler's 16-record ABI.
// All bytes are copied from the initial snapshot, never reread during batches.
func (f frozenVerificationCatalog) workspaceForSuite(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	contract, err := contractFor(s)
	if err != nil {
		return w, err
	}
	selected := map[string]bool{}
	for _, pkg := range f.Catalog.Packages {
		if !strings.HasPrefix(pkg.Path, "go/") && pkg.Path != s.Package {
			continue
		}
		selected[pkg.Path] = true
		w.Manifest.Packages = append(w.Manifest.Packages, sourcePackage{Path: pkg.Path, Sources: append([]string(nil), pkg.Sources...)})
		for _, name := range pkg.Sources {
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
