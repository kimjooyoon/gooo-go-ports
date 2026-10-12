package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFixedLiteralDomainRoundingAndOverflow(t *testing.T) {
	for _, row := range [][3]int64{{math.MinInt32, -33554432, -33554432}, {math.MinInt32 + 1, -33554432, -33554431}, {-129, -3, -2}, {-128, -2, -2}, {-97, -2, -1}, {-96, -2, -1}, {-65, -2, -1}, {-64, -1, -1}, {-63, -1, 0}, {-32, -1, 0}, {-1, -1, 0}, {0, 0, 0}, {1, 0, 1}, {32, 0, 1}, {63, 0, 1}, {64, 1, 1}, {65, 1, 2}, {math.MaxInt32 - 64, 33554430, 33554431}, {math.MaxInt32 - 63, 33554431, 33554431}, {math.MaxInt32 - 62, 33554431, -33554432}, {math.MaxInt32, 33554431, -33554432}} {
		for i, ceil := range []bool{false, true} {
			want := map[string]any{"in_fixed_domain": true, "value": row[i+1]}
			if !reflect.DeepEqual(fixedPortOracle(row[0], ceil), want) {
				t.Fatalf("raw=%d ceil=%v", row[0], ceil)
			}
		}
	}
	for n := int64(math.MaxInt32 - 62); n <= math.MaxInt32; n++ {
		if got := fixedPortOracle(n, true).(map[string]any)["value"]; got != int64(-33554432) {
			t.Fatal("original int32 overflow changed", n, got)
		}
	}
	for _, n := range fixedRejectedInputs() {
		for _, ceil := range []bool{false, true} {
			if !reflect.DeepEqual(fixedPortOracle(n, ceil), map[string]any{"in_fixed_domain": false, "value": int64(0)}) {
				t.Fatal("outside raw domain narrowed", n)
			}
		}
	}
}
func TestFixedPixelLiteralOrderAndUnnormalizedBounds(t *testing.T) {
	added, err := fixedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		accepted bool
		r        rectangleCoordinates
		pixels   bool
	}{
		{true, rectangleCoordinates{0, 0, 2, 2}, true}, {true, rectangleCoordinates{-2, -1, 1, 2}, true},
		{true, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false},
		{true, rectangleCoordinates{-10, -10, 10, 10}, true}, {true, rectangleCoordinates{}, false},
		{true, rectangleCoordinates{-33554432, -33554432, 33554431, 33554431}, true},
		{false, rectangleCoordinates{}, false}, {false, rectangleCoordinates{}, false},
		{true, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false}, {false, rectangleCoordinates{}, false}, {true, rectangleCoordinates{}, false},
	}
	for i, p := range fixedPixelNamedPairs() {
		want := rectangleRecord(rows[i].r)
		want["accepted"], want["has_pixels"] = rows[i].accepted, rows[i].pixels
		checkConsumerCase(t, added.Suites[2], []any{rectangleRecord(p[0]), rectangleRecord(p[1])}, want)
	}
}
func TestFixedOriginalInt64ValidationBeforeOracle(t *testing.T) {
	for _, bad := range []any{float64(1), json.Number("1"), "1", nil, int32(1), true} {
		calls := 0
		oracle := func(n int64, ceil bool) any { calls++; return fixedPortOracle(n, ceil) }
		if _, err := fixedPortSuite("Int26_6Floor", [][]any{{int64(0)}, {bad}}, oracle); err == nil || calls != 0 {
			t.Fatal("malformed later input accepted or oracle called", bad, calls)
		}
		for pos := 0; pos < 2; pos++ {
			for _, field := range rectangleNames {
				in := fixedPixelInputs()[:2]
				r := in[1][pos].(map[string]any)
				r[field] = bad
				calls = 0
				_, err := geometrySuite("example/fixed-pixel-bounds", "PixelBounds", 2, in, func(r []image.Rectangle) any { calls++; return fixedPixelOracle(r) })
				if err == nil || calls != 0 {
					t.Fatal("malformed record accepted before oracle", pos, field)
				}
			}
		}
	}
}
func TestFixedCorpusCoverageBudgetAndPreservation(t *testing.T) {
	added, err := fixedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	again, err := fixedCorpus()
	if err != nil || !reflect.DeepEqual(added, again) {
		t.Fatal("corpus nondeterministic", err)
	}
	seen := map[int64]bool{}
	for _, n := range fixedRawInputs() {
		if n < math.MinInt32 || n > math.MaxInt32 || seen[n] {
			t.Fatal("invalid or duplicate reference", n)
		}
		seen[n] = true
	}
	for _, q := range []int64{-33554432, -33554431, -65536, -2, -1, 0, 1, 2, 65536, 33554430, 33554431} {
		for r := int64(0); r < 64; r++ {
			if !seen[q*64+r] {
				t.Fatal("missing residue", q, r)
			}
		}
	}
	batches := 0
	for i, s := range added.Suites {
		size := fixedPortBatchSize
		if i == 2 {
			size = fixedConsumerBatchSize
		}
		for start := 0; start < len(s.Cases); start += size {
			batch := s
			batch.Cases = s.Cases[start:min(start+size, len(s.Cases))]
			raw, err := json.MarshalIndent(caseDocument(batch), "", "  ")
			if err != nil || len(raw)+1 > 32<<10 {
				t.Fatal("native batch bytes", len(raw)+1, err)
			}
			batches++
		}
		t.Logf("%s: %d cases", s.Activity, len(s.Cases))
	}
	legacy := canonLegacyCorpus(t)
	canon, err := canonCorpus()
	if err != nil {
		t.Fatal(err)
	}
	legacy = append(legacy, canon.Suites...)
	count := 0
	for _, s := range legacy {
		count += len(s.Cases)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(mustJSON(t, legacy)))
	if digest != "293ab9e81ef8b07c17d74fa3aec069f228d8fbf0242a1b4e675df1603b024166" {
		t.Fatal("existing 35042-case corpus changed")
	}
	if count != 35042 {
		t.Fatal("legacy count changed", count)
	}
	t.Logf("legacy sha256=%s; fixed counts=%+v; batches=%d", digest, struct{ PortReference, PortRejection, ConsumerReference, ConsumerDomain, ConsumerOverflow int }{added.PortReferenceCases, added.PortRejectionCases, added.ConsumerReferenceCases, added.ConsumerDomainRejectionCases, added.ConsumerOverflowRejectionCases}, batches)
}
func TestFixedOracleBytesAndLicense(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	if err := verifyFixedOracle("../..", externalSourcesForModule(lock.ExternalSources, "golang.org/x/image")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"third_party/x-image/math/fixed/fixed.go", "third_party/x-image/LICENSE"} {
		dir := t.TempDir()
		for _, p := range []string{"third_party/x-image/math/fixed/fixed.go", "third_party/x-image/LICENSE"} {
			raw, err := os.ReadFile(filepath.Join("../..", p))
			if err != nil {
				t.Fatal(err)
			}
			if p == name {
				raw = append(raw, '\n')
			}
			path := filepath.Join(dir, p)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, raw, 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := verifyFixedOracle(dir, externalSourcesForModule(lock.ExternalSources, "golang.org/x/image")); err == nil {
			t.Fatal("accepted changed upstream", name)
		}
	}
}

func TestFixedExternalSourceLockRejectsMissingDuplicateAndAlteredMetadata(t *testing.T) {
	var lock upstream
	if err := readJSON("../../upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	if err := verifyFixedExternalSources(externalSourcesForModule(lock.ExternalSources, "golang.org/x/image")); err != nil {
		t.Fatal(err)
	}
	for _, sources := range [][]externalSource{nil, {}, append(append([]externalSource{}, lock.ExternalSources...), lock.ExternalSources...)} {
		if err := verifyFixedExternalSources(sources); err == nil {
			t.Fatal("accepted missing/duplicate external source roster")
		}
	}
	original := lock.ExternalSources[0]
	for _, field := range []string{"Module", "Repository", "Revision", "Source", "SourceSHA", "OracleCopy", "License", "LicenseSHA", "Target"} {
		for _, value := range []string{"", "wrong"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				mutated := original
				reflect.ValueOf(&mutated).Elem().FieldByName(field).SetString(value)
				if err := verifyFixedOracle("../..", []externalSource{mutated}); err == nil {
					t.Fatal("accepted unbound metadata")
				}
			})
		}
	}
	for _, symbols := range [][]string{nil, {"Int26_6.Floor"}, {"Int26_6.Floor", "Int26_6.Round"}, {"Int26_6.Floor", "Int26_6.Ceil", "Int26_6.Ceil"}} {
		mutated := original
		mutated.Symbols = symbols
		if err := verifyFixedExternalSources([]externalSource{mutated}); err == nil {
			t.Fatal("accepted changed symbol roster")
		}
	}
	for name, raw := range map[string]string{"missing": "{}", "null": `{"external_sources":null}`, "wrong type": `{"external_sources":[{"revision":1}]}`} {
		t.Run(name, func(t *testing.T) {
			var parsed upstream
			err := json.Unmarshal([]byte(raw), &parsed)
			if err == nil && verifyFixedExternalSources(parsed.ExternalSources) == nil {
				t.Fatal("accepted invalid lock")
			}
		})
	}
}
