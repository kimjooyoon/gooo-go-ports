package main

import (
	"bytes"
	"fmt"
	"strings"
)

// Only the new entries select these files. In particular, adding LastIndex to
// the catalog never enlarges any pre-existing literal/UUID/other workspace.
func (f frozenVerificationCatalog) lastIndexWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	if !isLastIndexSuite(s) {
		return w, fmt.Errorf("unknown LastIndex workspace entry")
	}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	stringsSources := []string{"ports/strings/search.gooo", lastIndexSource}
	if s.Package == lastIndexConsumerPackage {
		stringsSources = []string{"ports/strings/search.gooo", "ports/strings/literal.gooo", lastIndexSource}
	}
	packages := []sourcePackage{{Path: literalTextPackage, Sources: stringsSources}}
	if s.Package == lastIndexConsumerPackage {
		packages = append(packages, sourcePackage{Path: lastIndexConsumerPackage, Sources: []string{lastIndexConsumerSource}})
	}
	for _, want := range packages {
		found := 0
		for _, candidate := range f.Catalog.Packages {
			if candidate.Path != want.Path {
				continue
			}
			found++
			if want.Path == lastIndexConsumerPackage && !equalSourceNames(candidate.Sources, want.Sources) {
				return w, fmt.Errorf("LastIndex consumer catalog source roster")
			}
			for _, name := range want.Sources {
				count := 0
				for _, available := range candidate.Sources {
					if available == name {
						count++
					}
				}
				if count != 1 {
					return w, fmt.Errorf("LastIndex missing or duplicate catalog source %s", name)
				}
			}
		}
		if found != 1 {
			return w, fmt.Errorf("LastIndex catalog package count for %s", want.Path)
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("missing or altered frozen LastIndex source %s", name)
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
		if name == lastIndexConsumerSource {
			want = 1
		}
		if len(imports) != want || want == 1 && string(imports[0][1]) != literalTextPackage {
			return w, fmt.Errorf("unexpected LastIndex import closure in %s", name)
		}
	}
	return w, nil
}
