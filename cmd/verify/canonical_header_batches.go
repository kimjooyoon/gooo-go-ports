package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

func canonicalHeaderPackages(s suite) []sourcePackage {
	packages := []sourcePackage{{Path: headerPortPackage, Sources: []string{headerPortSource}}}
	if canonicalHeaderConsumer(s) {
		packages = append(packages, sourcePackage{Path: headerReportPackage, Sources: []string{headerReportSource}})
	}
	packages = append(packages, sourcePackage{Path: canonicalHeaderPackage, Sources: []string{canonicalHeaderSource}})
	if canonicalHeaderConsumer(s) {
		packages = append(packages, sourcePackage{Path: canonicalHeaderReportPackage, Sources: []string{canonicalHeaderReportSource}})
	}
	return packages
}
func (f frozenVerificationCatalog) canonicalHeaderWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	if !isCanonicalHeaderSuite(s) {
		return w, fmt.Errorf("canonical header exact entry required")
	}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	imports := map[string][]string{headerPortSource: nil, headerReportSource: {headerPortPackage}, canonicalHeaderSource: {headerPortPackage}, canonicalHeaderReportSource: {canonicalHeaderPackage, headerReportPackage}}
	for _, want := range canonicalHeaderPackages(s) {
		count := 0
		for _, pkg := range f.Catalog.Packages {
			if pkg.Path == want.Path {
				count++
				if !equalSourceNames(pkg.Sources, want.Sources) {
					return w, fmt.Errorf("canonical header whole-package source roster")
				}
			}
		}
		if count != 1 {
			return w, fmt.Errorf("canonical header missing/duplicate package")
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("canonical header frozen source bytes")
			}
			actual := catalogImport.FindAllSubmatch(raw, -1)
			if len(actual) != len(imports[name]) {
				return w, fmt.Errorf("canonical header exact import count")
			}
			for i, path := range imports[name] {
				if string(actual[i][1]) != path {
					return w, fmt.Errorf("canonical header exact import order")
				}
			}
			w.Names = append(w.Names, name)
			w.Sources[name] = bytes.Clone(raw)
		}
	}
	var err error
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	return w, err
}
func canonicalHeaderBatchPolicy(s suite, w suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if !isCanonicalHeaderSuite(s) || len(s.Cases) == 0 || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.3" {
		return nativeCaseBatchPolicy{}, fmt.Errorf("canonical header nonempty original suite and pinned839/Go1.27.3 required")
	}
	if w.Manifest.Schema != "gooo/package-workspace-manifest/v1" || w.Manifest.Entry.Package != s.Package || w.Manifest.Entry.Activity != s.Activity || !reflect.DeepEqual(w.Manifest.Packages, canonicalHeaderPackages(s)) || len(w.Names) != len(w.Manifest.Packages) || len(w.Sources) != len(w.Names) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("canonical header exact selected closure")
	}
	seen, total := map[string]bool{}, 0
	for i, name := range w.Names {
		raw, ok := w.Sources[name]
		if !filepath.IsLocal(name) || w.Manifest.Packages[i].Sources[0] != name || seen[name] || !ok || len(raw) == 0 || len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("canonical header bounded selected source identity")
		}
		seen[name] = true
		total += len(raw)
	}
	lower := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	return nativeCaseBatchPolicy{canonicalHeader: true, sourceBytes: total, loweredKeyBytesUpperBound: lower}, nil
}

// Apply the existing source-bounded pretty/compact Go JSON policy to maximal
// contiguous prefixes. A source-byte bound limits ordinal digits; it does not
// guess an actual lowered name, batch count, or one-row native capacity.
func (p nativeCaseBatchPolicy) prepareCanonicalHeader(s suite, start int) (nativePreparedCaseBatch, error) {
	if !isCanonicalHeaderSuite(s) || start < 0 || start >= len(s.Cases) || p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes || p.loweredKeyBytesUpperBound != len("GoooPackage")+len(strconv.Itoa(p.sourceBytes))+len("Activity")+len(s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("canonical header batch/source lowering bound")
	}
	growth := max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
	low, high, best := start+1, min(start+128, len(s.Cases)), start
	var selected []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		for _, row := range batch.Cases {
			if _, err := readCanonicalHeaderInput(row.Inputs); err != nil {
				return nativePreparedCaseBatch{}, err
			}
		}
		document := caseDocument(batch)
		raw, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		compact, err := json.Marshal(document)
		if err != nil {
			return nativePreparedCaseBatch{}, err
		}
		// Unary input key plus expected key, twice per original occurrence.
		translatedUpper := len(compact) + 2*(end-start)*growth
		if len(raw)+1 <= 30000 && translatedUpper <= 32768 {
			best, selected, low = end, append(raw, '\n'), end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one original canonical header case exceeds wire budget; no omission")
	}
	return nativePreparedCaseBatch{End: best, Raw: selected}, nil
}
