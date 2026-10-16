package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Keep each core entry self-contained. The importing Report retains the
// original manifest order: ETag core, unchanged httpguts, original consumer.
func etagPackages(s suite) []sourcePackage {
	if isIfMatchSuite(s) {
		return []sourcePackage{{Path: ifMatchPackage, Sources: []string{ifMatchSource}}}
	}
	packages := []sourcePackage{{Path: etagPortPackage, Sources: []string{etagPortSource}}}
	if etagReportConsumer(s) {
		packages = append(packages,
			sourcePackage{Path: headerPortPackage, Sources: []string{headerPortSource}},
			sourcePackage{Path: etagReportPackage, Sources: []string{etagReportSource}})
	}
	return packages
}

func (f frozenVerificationCatalog) etagWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	if !isHTTPConditionalSuite(s) {
		return w, fmt.Errorf("ETag exact entry required")
	}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	imports := map[string][]string{
		etagPortSource: nil, headerPortSource: nil, ifMatchSource: nil,
		etagReportSource: {etagPortPackage, headerPortPackage},
	}
	for _, want := range etagPackages(s) {
		count := 0
		for _, pkg := range f.Catalog.Packages {
			if pkg.Path == want.Path {
				count++
				if !equalSourceNames(pkg.Sources, want.Sources) {
					return w, fmt.Errorf("ETag whole-package source roster differs")
				}
			}
		}
		if count != 1 {
			return w, fmt.Errorf("ETag missing or duplicate package")
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("ETag frozen source bytes differ: %s", name)
			}
			actual, err := workspaceProfileImports(raw)
			if err != nil || !reflect.DeepEqual(actual, imports[name]) {
				return w, fmt.Errorf("ETag exact source import paths/order differ: %s", name)
			}
			w.Names = append(w.Names, name)
			w.Sources[name] = bytes.Clone(raw)
		}
	}
	var err error
	w.Identities, err = snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	return w, err
}

func etagBatchArity(s suite) int {
	if isIfMatchSuite(s) || s.Package == etagPortPackage && s.Activity == etagStrongActivity {
		return 2
	}
	return 1
}

func validateETagBatchInputs(s suite, values []any) error {
	if !isHTTPConditionalSuite(s) || len(values) != etagBatchArity(s) {
		return fmt.Errorf("ETag original Text input arity differs")
	}
	for _, value := range values {
		text, ok := value.(string)
		if !ok || !utf8.ValidString(text) || len(text) > 1024 {
			return fmt.Errorf("ETag each original validUTF8 Text must contain0..1024bytes")
		}
	}
	return nil
}

func etagBatchPolicy(s suite, w suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if !isHTTPConditionalSuite(s) || len(s.Cases) == 0 || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.2" {
		return nativeCaseBatchPolicy{}, fmt.Errorf("ETag nonempty original suite and pinned839/Go1.27.2 required")
	}
	if w.Manifest.Schema != "gooo/package-workspace-manifest/v1" || w.Manifest.Entry.Package != s.Package || w.Manifest.Entry.Activity != s.Activity || !reflect.DeepEqual(w.Manifest.Packages, etagPackages(s)) || len(w.Names) != len(w.Manifest.Packages) || len(w.Sources) != len(w.Names) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("ETag exact selected source closure required")
	}
	seen, total := map[string]bool{}, 0
	for i, name := range w.Names {
		raw, ok := w.Sources[name]
		if !filepath.IsLocal(name) || filepath.Clean(name) != name || w.Manifest.Packages[i].Sources[0] != name || seen[name] || !ok || len(raw) == 0 || len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("ETag bounded selected source identity differs")
		}
		seen[name] = true
		total += len(raw)
	}
	identities, err := snapshotSourceIdentities(w.Manifest.Packages, w.Sources)
	if err != nil || !reflect.DeepEqual(identities, w.Identities) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("ETag selected source identities differ")
	}
	for index, row := range s.Cases {
		if err := validateETagBatchInputs(s, row.Inputs); err != nil {
			return nativeCaseBatchPolicy{}, fmt.Errorf("ETag case%d: %w", index, err)
		}
	}
	lower := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	return nativeCaseBatchPolicy{etag: true, sourceBytes: total, loweredKeyBytesUpperBound: lower}, nil
}

// This bounds the existing Go-encoded case document and its pinned839 lowered
// key growth. It is not evidence of native transport capacity for either the
// two-Text entry or the full finite population. An oversized row is an error.
func (p nativeCaseBatchPolicy) prepareETag(s suite, start int) (nativePreparedCaseBatch, error) {
	if !p.etag || !isHTTPConditionalSuite(s) || start < 0 || start >= len(s.Cases) || p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes || p.loweredKeyBytesUpperBound != len("GoooPackage")+len(strconv.Itoa(p.sourceBytes))+len("Activity")+len(s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("ETag batch/source lowering bound differs")
	}
	growth := max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
	low, high, best := start+1, min(start+uuidCompactMaxRows, len(s.Cases)), start
	var selected []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		for _, row := range batch.Cases {
			if err := validateETagBatchInputs(s, row.Inputs); err != nil {
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
		// Each original occurrence has arity distinct input keys and one
		// expected key. .input0/.input1 suffixes themselves do not expand.
		translatedUpper := len(compact) + (etagBatchArity(s)+1)*(end-start)*growth
		if len(raw)+1 <= uuidCompactMaxRawBytes && translatedUpper <= uuidCompactTranslatedBytes {
			best, selected, low = end, append(raw, '\n'), end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one original ETag case exceeds wire budget; no omission or domain shrink")
	}
	return nativePreparedCaseBatch{End: best, Raw: selected}, nil
}
