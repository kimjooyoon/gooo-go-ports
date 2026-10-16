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

// Only this new importing suite selects these five whole packages. Existing
// selected workspaces and their case-wire policies remain unchanged.
func headerAdditionPackages() []sourcePackage {
	return []sourcePackage{{Path: headerPortPackage, Sources: []string{headerPortSource}}, {Path: headerReportPackage, Sources: []string{headerReportSource}}, {Path: decimalPackage, Sources: []string{"ports/strconv/decimal.gooo"}}, {Path: payloadAdmissionPackage, Sources: []string{"examples/payload-admission/main.gooo"}}, {Path: headerAdditionPackage, Sources: []string{headerAdditionSource}}}
}
func (f frozenVerificationCatalog) headerAdditionWorkspace(s suite) (suiteWorkspace, error) {
	w := suiteWorkspace{Sources: map[string][]byte{}}
	if !isHeaderAdditionSuite(s) {
		return w, fmt.Errorf("header addition exact entry required")
	}
	w.Manifest.Schema = "gooo/package-workspace-manifest/v1"
	w.Manifest.Entry.Package, w.Manifest.Entry.Activity = s.Package, s.Activity
	imports := map[string][]string{headerPortSource: nil, headerReportSource: {headerPortPackage}, "ports/strconv/decimal.gooo": nil, "examples/payload-admission/main.gooo": {decimalPackage}, headerAdditionSource: {headerReportPackage, payloadAdmissionPackage}}
	for _, want := range headerAdditionPackages() {
		count := 0
		for _, p := range f.Catalog.Packages {
			if p.Path == want.Path {
				count++
				if !equalSourceNames(p.Sources, want.Sources) {
					return w, fmt.Errorf("addition whole-package source roster")
				}
			}
		}
		if count != 1 {
			return w, fmt.Errorf("addition missing/duplicate package")
		}
		w.Manifest.Packages = append(w.Manifest.Packages, want)
		for _, name := range want.Sources {
			raw, ok := f.Sources[name]
			if !ok || strings.TrimPrefix(contentDigest(raw), "sha256:") != f.Hashes[name] {
				return w, fmt.Errorf("addition frozen source bytes")
			}
			actual := catalogImport.FindAllSubmatch(raw, -1)
			if len(actual) != len(imports[name]) {
				return w, fmt.Errorf("addition exact import count")
			}
			for i, p := range imports[name] {
				if string(actual[i][1]) != p {
					return w, fmt.Errorf("addition exact import order")
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
func headerAdditionBatchPolicy(s suite, w suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if !isHeaderAdditionSuite(s) || lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.2" || len(s.Cases) != 50 {
		return nativeCaseBatchPolicy{}, fmt.Errorf("addition requires exact50 and pinned compiler839/Go1.27.2")
	}
	if w.Manifest.Schema != "gooo/package-workspace-manifest/v1" || w.Manifest.Entry.Package != s.Package || w.Manifest.Entry.Activity != s.Activity || !reflect.DeepEqual(w.Manifest.Packages, headerAdditionPackages()) || len(w.Names) != 5 || len(w.Sources) != 5 {
		return nativeCaseBatchPolicy{}, fmt.Errorf("addition exact selected closure")
	}
	seen, total := map[string]bool{}, 0
	for _, name := range w.Names {
		raw, ok := w.Sources[name]
		if !filepath.IsLocal(name) || seen[name] || !ok || len(raw) == 0 || len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("addition source bound")
		}
		seen[name] = true
		total += len(raw)
	}
	lower := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	return nativeCaseBatchPolicy{headerAddition: true, sourceBytes: total, loweredKeyBytesUpperBound: lower}, nil
}
func (p nativeCaseBatchPolicy) prepareHeaderAddition(s suite, start int) (nativePreparedCaseBatch, error) {
	if !isHeaderAdditionSuite(s) || len(s.Cases) != 50 || start < 0 || start >= 50 || start%8 != 0 || p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes || p.loweredKeyBytesUpperBound != len("GoooPackage")+len(strconv.Itoa(p.sourceBytes))+len("Activity")+len(s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("addition fixed-eight batch identity")
	}
	end := min(start+8, len(s.Cases))
	batch := s
	batch.Cases = s.Cases[start:end]
	for _, c := range batch.Cases {
		if err := validateHeaderAdditionInputs(c.Inputs); err != nil {
			return nativePreparedCaseBatch{}, err
		}
	}
	doc := caseDocument(batch)
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nativePreparedCaseBatch{}, err
	}
	compact, err := json.Marshal(doc)
	if err != nil {
		return nativePreparedCaseBatch{}, err
	}
	// Four qualified input keys and one expected key per original row. The
	// pinned source-byte bound limits ordinal digits; field keys do not change.
	growth := max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity))
	if len(raw)+1 > 30000 || len(compact)+5*(end-start)*growth > 32768 {
		return nativePreparedCaseBatch{}, fmt.Errorf("addition fixed batch exceeds actual Go JSON budget; no trimming/repartition")
	}
	return nativePreparedCaseBatch{End: end, Raw: append(raw, '\n')}, nil
}
