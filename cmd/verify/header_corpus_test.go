package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHeaderFrozenPopulationAndAllBatchBoundaries(t *testing.T) {
	suites, err := headerCorpus("../..")
	if err != nil {
		t.Fatal(err)
	}
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	total, batches := 0, 0
	for index, s := range suites {
		if len(s.Cases) != []int{232, 232, 476}[index] {
			t.Fatal("corpus count", index)
		}
		w, err := f.workspaceForSuite(s)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if index == 2 {
			want = 2
		}
		if len(w.Names) != want || len(w.Manifest.Packages) != want {
			t.Fatal("exact isolated closure", w.Names)
		}
		for name := range w.Sources {
			if name != headerPortSource && name != headerReportSource {
				t.Fatal("unrelated source", name)
			}
		}
		p, err := nativeCaseProfile(uuidLiteralReportCompactCaseProfile).policy(s, w, caseProfileTestLock())
		if err != nil {
			t.Fatal(err)
		}
		per := 0
		for start := 0; start < len(s.Cases); {
			part, err := p.prepare(s, start)
			if err != nil {
				t.Fatal(err)
			}
			batch := s
			batch.Cases = s.Cases[start:part.End]
			if part.End <= start || part.End-start > 128 || len(part.Raw) > 30000 || !equalValue(part.Raw, caseDocument(batch)) {
				t.Fatal("batch inputs/expectations/order/budget changed")
			}
			compact, err := json.Marshal(caseDocument(batch))
			if err != nil || len(compact)+(len(s.Cases[0].Inputs)+1)*(part.End-start)*max(0, p.loweredKeyBytesUpperBound-len(s.Package+":"+s.Activity)) > 32768 {
				t.Fatal("lowered wire budget", err)
			}
			start = part.End
			per++
		}
		t.Logf("header partition %s:%s cases=%d batches=%d", s.Package, s.Activity, len(s.Cases), per)
		total += len(s.Cases)
		batches += per
	}
	if total != 940 {
		t.Fatal("full940 omitted", total)
	}
	t.Logf("fresh header partitions batches=%d; full suites73 cases171167 batches%d rows%d footer%d", batches, 2153+batches, 2301+batches, 2300+batches)
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	inputs, err := nativeTimingSourceIdentity(nativeTimingConfig{RepositoryRoot: "../..", Catalog: f, Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fresh timing source inputs=%d", len(inputs.Sources))
}

func TestHeaderReferenceByteAndWholeInputWitnesses(t *testing.T) {
	for _, test := range []struct {
		name, value              string
		no, nb, vo, vb           int64
		nv, vv, upper, obs, edge bool
	}{
		{"X-Test", "ok", -1, -1, -1, -1, true, true, true, false, false},
		{"", "", -1, -1, -1, -1, false, true, false, false, false},
		{"x", "a\r\nb", -1, -1, 1, 13, true, false, false, false, false},
		{"a가", "가\r", 1, 234, 3, 13, false, false, false, true, false},
		{strings.Repeat("a", 511) + " X", " \t", 511, 32, -1, -1, false, true, true, false, true},
	} {
		got := headerReference(test.name, test.value)
		for k, want := range map[string]any{"name": test.name, "value": test.value, "name_invalid_offset": test.no, "name_invalid_octet": test.nb, "value_invalid_offset": test.vo, "value_invalid_octet": test.vb, "name_valid": test.nv, "value_valid": test.vv, "name_has_uppercase": test.upper, "value_has_obs_text": test.obs, "value_edge_whitespace": test.edge} {
			if got[k] != want {
				t.Fatalf("%s got%v want%v", k, got[k], want)
			}
		}
	}
}

func TestHeaderPopulationRefusesLateMalformedRowsAndWrongWire(t *testing.T) {
	raw, err := os.ReadFile("../../" + headerCorpusFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"late-type", "late-size", "duplicate", "unknown", "missing"} {
		v, err := decode(raw)
		if err != nil {
			t.Fatal(err)
		}
		top := v.(map[string]any)
		rows := top["pairs"].([]any)
		last := rows[len(rows)-1].(map[string]any)
		switch kind {
		case "late-type":
			last["value"] = false
		case "late-size":
			last["value"] = strings.Repeat("a", 1025)
		case "duplicate":
			rows[len(rows)-1] = rows[0]
		case "unknown":
			last["valueText"] = "x"
		case "missing":
			delete(top, "values")
		}
		altered, _ := json.Marshal(top)
		out, err := headerInputs(altered)
		if err == nil || out.Names != nil || out.Values != nil || out.Pairs != nil {
			t.Fatal("invalid population returned partial/admitted input", kind)
		}
	}
	for _, bad := range [][]byte{[]byte(`{"schema":"x","schema":"x","names":[],"values":[],"pairs":[]}`), append(append([]byte{}, raw...), []byte(`{}`)...), []byte(`{"schema":"gooo/header-native-inputs/v1","names":["\ud800"]}`)} {
		if _, err := headerInputs(bad); err == nil {
			t.Fatal("malformed wire accepted")
		}
	}
	s := suite{Package: headerReportPackage, Activity: headerReportActivity, Cases: []testCase{{Inputs: []any{strings.Repeat("x", 1025), ""}, Expected: map[string]any{}}}}
	f, err := freezeVerificationCatalog("../../verification-catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.workspaceForSuite(s)
	if err != nil {
		t.Fatal(err)
	}
	p, err := headerBatchPolicy(s, w, caseProfileTestLock())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.prepare(s, 0); err == nil {
		t.Fatal("out-of-domain input reached native batch")
	}
}
