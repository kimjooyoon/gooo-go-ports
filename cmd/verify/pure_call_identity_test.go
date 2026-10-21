package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestPureCallIdentityProfilesAreSourcePinned(t *testing.T) {
	for _, revision := range []string{"", "205896a", strings.Repeat("0", 40), authoredPureCallCompilerRevision + "-modified"} {
		if _, err := selectPureCallIdentityProfile(revision); err == nil {
			t.Fatal("unknown compiler profile accepted", revision)
		}
	}
	current, err := selectPureCallIdentityProfile(authoredPureCallCompilerRevision)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := selectPureCallIdentityProfile(legacyPureCallCompilerRevision)
	if err != nil {
		t.Fatal(err)
	}
	const authored = "gooo://go/path/Clean"
	if current.original("gooo_ports_path", "Clean", authored) != authored || current.lowered("GoooPackage0ActivityClean", authored) != authored {
		t.Fatal("source205 authored identity lost")
	}
	if legacy.original("gooo_ports_path", "Clean", authored) != "gooo-ports-path://activity/clean" || legacy.lowered("GoooPackage0ActivityClean", authored) != "gooo-workspace://activity/gooo-package0activity-clean" {
		t.Fatal("source839 historical identity changed")
	}
}

// These are synthetic consistency fixtures, never executed or presented as
// native evidence. Original839 fixtures remain unchanged; this creates a new
// source205 view whose independent authored IDs are parsed from frozen source.
func TestCurrentPureCallProfilesRejectHistoricalAndNeighborIDs(t *testing.T) {
	paths, err := pathAliasCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	decimals, err := decimalPayloadCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		make func() (map[string]any, suite, expectedIdentity)
	}{
		{"path", func() (map[string]any, suite, expectedIdentity) { return pathAliasReceiptFixture(t, paths[0], 0) }},
		{"headers", func() (map[string]any, suite, expectedIdentity) {
			return headerReceiptFixture(t, suite{Package: headerReportPackage, Activity: headerReportActivity})
		}},
		{"payload", func() (map[string]any, suite, expectedIdentity) {
			for _, s := range decimals.Suites {
				if s.Package == payloadAdmissionPackage {
					return decimalPayloadReceiptFixture(t, s, 0)
				}
			}
			t.Fatal("payload suite absent")
			return nil, suite{}, expectedIdentity{}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, s, id := c.make()
			legacyRaw := mustJSON(t, base)
			if _, err := inspect(legacyRaw, s, false, id); err != nil {
				t.Fatal("unchanged historical839 fixture", err)
			}
			id.CompilerRevision = authoredPureCallCompilerRevision
			result := base["result"].(map[string]any)
			rt := result["runtime"].(map[string]any)
			rt["producer_source_sha"] = id.CompilerRevision
			report := result["composition"].(map[string]any)["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)["report"].(map[string]any)
			report["compiler_source_sha"] = id.CompilerRevision
			calls := result["program"].(map[string]any)["pure_calls"].(map[string]any)
			participants := calls["activities"].([]any)
			old := [][2]any{}
			for _, v := range participants {
				item := v.(map[string]any)
				ref := item["activity"].(map[string]any)
				name := ref["activity"].(string)
				source := item["source"].(string)
				bodies, err := decimalPayloadBodies(id.SourceBytes[source])
				if err != nil || bodies[name].ID == "" {
					t.Fatal("independent source declaration", err, name)
				}
				old = append(old, [2]any{item["activity_id"], item["lowered_activity_id"]})
				item["activity_id"] = bodies[name].ID
				item["lowered_activity_id"] = bodies[name].ID
			}
			if _, err := inspect(mustJSON(t, base), s, false, id); err != nil {
				t.Fatal("new source205 consistency fixture", err)
			}
			original, err := inspect(mustJSON(t, base), s, false, id)
			if err != nil {
				t.Fatal(err)
			}
			replayed := pathAliasTestClone(t, base)
			replayed["replayed_from_sha256"] = contentDigest(mustJSON(t, base))
			replayed["result"].(map[string]any)["replay"] = map[string]any{"model_calls": 0}
			replay, err := inspect(mustJSON(t, replayed), s, true, id)
			if err != nil {
				t.Fatal("source205 synthetic saved replay", err)
			}
			if err := inspectReplayBinding(original, replay, contentDigest(mustJSON(t, base))); err != nil {
				t.Fatal(err)
			}
			if err := inspectReplayBinding(original, replay, contentDigest([]byte("foreign original raw receipt"))); err == nil {
				t.Fatal("source205 foreign raw replay parent accepted")
			}

			for i := range participants {
				for k, field := range []string{"activity_id", "lowered_activity_id"} {
					changed := pathAliasTestClone(t, base)
					a := changed["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)["activities"].([]any)
					a[i].(map[string]any)[field] = old[i][k]
					if _, err := inspect(mustJSON(t, changed), s, false, id); err == nil {
						t.Fatal("historical namespace alias accepted under205", i, field)
					}
					changed = pathAliasTestClone(t, base)
					a = changed["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)["activities"].([]any)
					a[i].(map[string]any)[field] = a[(i+1)%len(a)].(map[string]any)[field]
					if _, err := inspect(mustJSON(t, changed), s, false, id); err == nil {
						t.Fatal("authored neighbor identity accepted", i, field)
					}
				}
			}
			for _, field := range []string{"source_body_sha256", "source"} {
				changed := pathAliasTestClone(t, base)
				a := changed["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)["activities"].([]any)
				a[0].(map[string]any)[field] = "foreign"
				if _, err := inspect(mustJSON(t, changed), s, false, id); err == nil {
					t.Fatal("source/body binding lost", field)
				}
			}
			changed := pathAliasTestClone(t, base)
			pc := changed["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)
			pc["sites"].([]any)[0].(map[string]any)["start_byte"] = json.Number("-1")
			if _, err := inspect(mustJSON(t, changed), s, false, id); err == nil {
				t.Fatal("original source call span lost")
			}
			unknown := id
			unknown.CompilerRevision = strings.Repeat("f", 40)
			changed = pathAliasTestClone(t, base)
			r := changed["result"].(map[string]any)
			r["runtime"].(map[string]any)["producer_source_sha"] = unknown.CompilerRevision
			r["composition"].(map[string]any)["steps"].([]any)[0].(map[string]any)["generation"].(map[string]any)["report"].(map[string]any)["compiler_source_sha"] = unknown.CompilerRevision
			if _, err := inspect(mustJSON(t, changed), s, false, unknown); err == nil {
				t.Fatal("unknown compiler identity accepted")
			}
			if _, err := inspect(legacyRaw, s, false, expectedIdentity{CompilerRevision: legacyPureCallCompilerRevision, GoVersion: id.GoVersion, ManifestDigest: id.ManifestDigest, CasesDigest: id.CasesDigest, Sources: id.Sources, SourceBytes: id.SourceBytes}); err != nil {
				t.Fatal("historical bytes no longer accepted", err)
			}
		})
	}
}

func TestCurrentCanonicalAndAdditionPureCallsPreserveAllOtherFields(t *testing.T) {
	current, err := selectPureCallIdentityProfile(authoredPureCallCompilerRevision)
	if err != nil {
		t.Fatal(err)
	}
	type participant struct{ namespace, name, lowered, id string }
	for _, which := range []string{"canonical", "addition"} {
		t.Run(which, func(t *testing.T) {
			var before, after map[string]any
			known := map[string]participant{}
			if which == "canonical" {
				s := canonicalHeaderTestSuites()[1]
				defs, err := canonicalHeaderDefinitions(s, canonicalHeaderTestIdentity(t, s))
				if err != nil {
					t.Fatal(err)
				}
				before, err = canonicalHeaderPureCalls(defs, true)
				if err != nil {
					t.Fatal(err)
				}
				after, err = canonicalHeaderPureCallsWithIdentity(defs, true, current)
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range defs {
					known[d.Package+":"+d.Declaration.Name] = participant{d.Namespace, d.Declaration.Name, d.Lowered, d.Declaration.ID}
				}
			} else {
				defs, err := headerAdditionDefinitions(headerAdditionTestIdentity(t))
				if err != nil {
					t.Fatal(err)
				}
				before, err = headerAdditionPureCalls(defs)
				if err != nil {
					t.Fatal(err)
				}
				after, err = headerAdditionPureCallsWithIdentity(defs, current)
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range defs {
					known[d.Package+":"+d.Declaration.Name] = participant{d.Namespace, d.Declaration.Name, d.Lowered, d.Declaration.ID}
				}
			}
			unchanged := pathAliasTestClone(t, after)
			a := unchanged["activities"].([]any)
			for i, v := range a {
				item := v.(map[string]any)
				ref := item["activity"].(map[string]any)
				d := known[ref["package_path"].(string)+":"+ref["activity"].(string)]
				if d.id == "" || item["activity_id"] != d.id || item["lowered_activity_id"] != d.id {
					t.Fatal("authored identity mismatch")
				}
				for _, field := range []string{"activity_id", "lowered_activity_id"} {
					wrong := pathAliasTestClone(t, after)
					w := wrong["activities"].([]any)
					alias := decimalPayloadDerivedID(d.namespace, d.name)
					if field == "lowered_activity_id" {
						alias = decimalPayloadDerivedID("gooo_workspace", d.lowered)
					}
					w[i].(map[string]any)[field] = alias
					if decimalPayloadSame(wrong, after) {
						t.Fatal("historical alias matched exact205 expectation", field)
					}
					w[i].(map[string]any)[field] = w[(i+1)%len(w)].(map[string]any)[field]
					if decimalPayloadSame(wrong, after) {
						t.Fatal("neighbor authored ID matched205 expectation", field)
					}
				}
				item["activity_id"] = decimalPayloadDerivedID(d.namespace, d.name)
				item["lowered_activity_id"] = decimalPayloadDerivedID("gooo_workspace", d.lowered)
			}
			if !decimalPayloadSame(unchanged, before) {
				t.Fatal("nonidentity source body/site/participant order changed")
			}
		})
	}
}

// Deliberately incomplete ABI documents isolate the remaining call guards.
// Good metadata must advance to the named later source-binding refusal; these
// are neither complete receipts nor native execution evidence.
func TestCurrentETagCanonicalAdditionProductionCallGuards(t *testing.T) {
	current, err := selectPureCallIdentityProfile(authoredPureCallCompilerRevision)
	if err != nil {
		t.Fatal(err)
	}
	for _, which := range []string{"etag", "canonical", "addition"} {
		t.Run(which, func(t *testing.T) {
			var s suite
			var id expectedIdentity
			var text string
			var calls map[string]any
			var image map[string]any
			var rootPkg, rootName, rootLower, rootSource, rootOutput string
			var rootInputs []string
			type decl struct{ namespace, name, lowered, authored string }
			known := map[string]decl{}
			var check func([]byte, suite, expectedIdentity) error
			var afterCalls, pureError string
			if which == "etag" {
				s = etagTestSuites()[2]
				id = etagTestIdentity(t, s)
				var defs map[string]etagDefinition
				text, defs = etagTestLowered(t, s, false)
				root := defs[s.Package+":"+s.Activity]
				rootPkg, rootName, rootLower, rootSource, rootOutput, rootInputs = root.Package, root.Declaration.Name, root.Lowered, root.Source, root.Declaration.Output, root.Declaration.Inputs
				image = etagTestImage(t, s, defs)
				activities, sites := []any{}, []any{}
				keys := []string{}
				for key := range defs {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					d := defs[key]
					known[key] = decl{d.Namespace, d.Declaration.Name, d.Lowered, d.Declaration.ID}
					activities = append(activities, map[string]any{"activity": map[string]any{"package_path": d.Package, "activity": d.Declaration.Name, "lowered_name": d.Lowered}, "activity_id": d.Declaration.ID, "lowered_activity_id": d.Declaration.ID, "source": d.Source, "source_body_sha256": contentDigest([]byte(d.Declaration.Body))})
				}
				for _, key := range etagCallees(s) {
					token := etagToken(key)
					at := strings.Index(root.Declaration.Body, token+"(")
					sites = append(sites, map[string]any{"caller": s.Package + ":" + s.Activity, "callee": key, "surface": "computes", "start_byte": at, "end_byte": at + len(token)})
				}
				calls = map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": activities, "sites": sites}
				check = inspectETagABI
				afterCalls = "etag lowered raw source binding"
				pureError = "etag pure exact body/ID"
			} else {
				var body strings.Builder
				body.WriteString("package gooo_workspace\nnamespace gooo_workspace\n")
				var initOrder []string
				if which == "canonical" {
					s = canonicalHeaderTestSuites()[1]
					id = canonicalHeaderTestIdentity(t, s)
					defs, err := canonicalHeaderDefinitions(s, id)
					if err != nil {
						t.Fatal(err)
					}
					root := defs[len(defs)-1]
					rootPkg, rootName, rootLower, rootSource, rootOutput, rootInputs = root.Package, root.Declaration.Name, root.Lowered, root.Source, root.Declaration.Output, root.Declaration.Inputs
					for i, d := range defs {
						known[d.Package+":"+d.Declaration.Name] = decl{d.Namespace, d.Declaration.Name, d.Lowered, d.Declaration.ID}
						fmt.Fprintf(&body, "activity %s(%s) -> %s id %q computes %s\n", d.Lowered, strings.Join(d.Declaration.Inputs, ", "), d.Declaration.Output, d.Declaration.ID, strconv.Quote(canonicalHeaderLowerBody(i, defs, true)))
					}
					for _, pkg := range canonicalHeaderPackages(s) {
						initOrder = append(initOrder, pkg.Path)
					}
					calls, err = canonicalHeaderPureCallsWithIdentity(defs, true, current)
					if err != nil {
						t.Fatal(err)
					}
					check = inspectCanonicalHeaderABI
					afterCalls = "canonical header selected original source binding"
					pureError = "canonical header source pure participants/call sites"
				} else {
					s = suite{Package: headerAdditionPackage, Activity: headerAdditionActivity}
					id = headerAdditionTestIdentity(t)
					defs, err := headerAdditionDefinitions(id)
					if err != nil {
						t.Fatal(err)
					}
					root := defs[5]
					rootPkg, rootName, rootLower, rootSource, rootOutput, rootInputs = root.Package, root.Declaration.Name, root.Lowered, root.Source, root.Declaration.Output, root.Declaration.Inputs
					for i, d := range defs {
						known[d.Package+":"+d.Declaration.Name] = decl{d.Namespace, d.Declaration.Name, d.Lowered, d.Declaration.ID}
						fmt.Fprintf(&body, "activity %s(%s) -> %s id %q computes %s\n", d.Lowered, strings.Join(d.Declaration.Inputs, ", "), d.Declaration.Output, d.Declaration.ID, strconv.Quote(headerAdditionLowerBody(i, defs)))
					}
					initOrder = []string{headerPortPackage, headerReportPackage, decimalPackage, payloadAdmissionPackage, headerAdditionPackage}
					calls, err = headerAdditionPureCallsWithIdentity(defs, current)
					if err != nil {
						t.Fatal(err)
					}
					check = inspectHeaderAdditionABI
					afterCalls = "addition selected source binding"
					pureError = "addition six pure identities/five source call sites"
				}
				names := []string{}
				for name := range id.SourceBytes {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					for _, record := range headerAdditionRecordPattern.FindAll(id.SourceBytes[name], -1) {
						body.Write(record)
						body.WriteByte('\n')
					}
				}
				text = body.String()
				image = map[string]any{"schema": "gooo/package-runtime-image/v1", "init_order": initOrder, "entry": map[string]any{"package_path": rootPkg, "activity": rootName, "source": rootSource, "inputs": rootInputs, "output": rootOutput}}
			}
			entry := map[string]any{"package_path": rootPkg, "activity": rootName, "lowered_name": rootLower}
			base := pathAliasTestClone(t, map[string]any{"result": map[string]any{"schema": "gooo/workspace-body-execution/v1", "program": map[string]any{"schema": "gooo/workspace-body-program/v1", "entry": entry, "activities": []any{entry}, "workspace_image": image, "lowered_gooo_source": text, "pure_calls": calls}, "composition": map[string]any{}, "runtime": map[string]any{}}})
			if err := check(mustJSON(t, base), s, id); err == nil || err.Error() != afterCalls {
				t.Fatal("good source205 call metadata must advance to explicit later refusal", err)
			}
			as := base["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)["activities"].([]any)
			for i, v := range as {
				a := v.(map[string]any)
				ref := a["activity"].(map[string]any)
				d := known[ref["package_path"].(string)+":"+ref["activity"].(string)]
				if d.authored == "" || a["activity_id"] != d.authored || a["lowered_activity_id"] != d.authored {
					t.Fatal("source-parsed authored ID missing")
				}
				for _, field := range []string{"activity_id", "lowered_activity_id"} {
					for _, neighbor := range []bool{false, true} {
						changed := pathAliasTestClone(t, base)
						rows := changed["result"].(map[string]any)["program"].(map[string]any)["pure_calls"].(map[string]any)["activities"].([]any)
						alias := decimalPayloadDerivedID(d.namespace, d.name)
						if field == "lowered_activity_id" {
							alias = decimalPayloadDerivedID("gooo_workspace", d.lowered)
						}
						if neighbor {
							alias = rows[(i+1)%len(rows)].(map[string]any)[field].(string)
						}
						rows[i].(map[string]any)[field] = alias
						if err := check(mustJSON(t, changed), s, id); err == nil || err.Error() != pureError {
							t.Fatal("wrong identity did not fail at exact production call guard", i, field, neighbor, err)
						}
					}
				}
			}
			unknown := id
			unknown.CompilerRevision = strings.Repeat("d", 40)
			if err := check(mustJSON(t, base), s, unknown); err == nil || !strings.Contains(err.Error(), "unsupported pure-call compiler identity") {
				t.Fatal("unknown compiler profile bypassed call guard", err)
			}
		})
	}
}
