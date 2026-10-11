package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type upstream struct {
	ExternalSources  []externalSource  `json:"external_sources"`
	GoVersion        string            `json:"go_version"`
	CompilerRevision string            `json:"compiler_revision"`
	CompilerVersion  string            `json:"compiler_version"`
	SourceSHA        map[string]string `json:"source_sha256"`
}
type observation struct {
	Package     string `json:"package"`
	Activity    string `json:"activity"`
	Domain      string `json:"domain"`
	Cases       int    `json:"cases"`
	Batches     int    `json:"batches"`
	Passed      int    `json:"passed"`
	ReplayEqual bool   `json:"replay_equal"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	compiler := flag.String("compiler", "./.gooo", "pinned compiler executable")
	out := flag.String("out", "", "new evidence directory (must not exist)")
	browser := flag.String("browser-fixtures", "", "write independent browser oracle fixtures to a new JSON file")
	flag.Parse()
	if *out != "" && *browser != "" {
		return fmt.Errorf("choose either --out or --browser-fixtures")
	}
	var lock upstream
	if err := readJSON("upstream.json", &lock); err != nil {
		return err
	}
	if runtime.Version() != lock.GoVersion {
		return fmt.Errorf("oracle toolchain %s, require %s", runtime.Version(), lock.GoVersion)
	}
	if strconv.IntSize != 64 || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("HTTP, rotation and image.Rectangle int64 compatibility require a 64-bit target")
	}
	for path, want := range lock.SourceSHA {
		got, err := digestFile(filepath.Join(runtime.GOROOT(), path))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("Go upstream source hash differs: %s", path)
		}
	}
	if err := verifyFixedOracle(".", lock.ExternalSources); err != nil {
		return err
	}
	suites, err := corpus()
	if err != nil {
		return err
	}
	if *browser != "" {
		return writeBrowserFixtures(*browser, suites, lock)
	}
	inspector, err := unicodeInspectorCorpus(suites)
	if err != nil {
		return err
	}
	legacyCases := 0
	for _, s := range suites {
		legacyCases += len(s.Cases)
	}
	suites = append(suites, inspector.Suites...)
	colors, err := colorCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, colors.Suites...)
	rotations, err := rotationCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, rotations.Suites...)
	durations, err := durationCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, durations.Suites...)
	whitespace, err := whitespaceCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, whitespace.Suites...)
	classification, err := classificationCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, classification.Suites...)
	geometry, err := geometryCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, geometry.Suites...)
	folding, err := simpleFoldCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, folding.Suites...)
	ycbcr, err := ycbcrCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, ycbcr.Suites...)
	canonical, err := canonCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, canonical.Suites...)
	fixedBounds, err := fixedCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, fixedBounds.Suites...)
	mapping, err := caseMappingCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, mapping.Suites...)
	nrgba, err := nrgbaCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, nrgba.Suites...)
	if *out == "" {
		return fmt.Errorf("provide --out for a new evidence directory")
	}
	root, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(root), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(root, 0755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	raw, err := execute(ctx, *compiler, filepath.Join(root, "compiler.json"), "version", "--build", "--json")
	if err != nil {
		return err
	}
	var build struct {
		Version  string `json:"version"`
		Revision string `json:"compiler_source_sha"`
		Modified string `json:"vcs_modified"`
		Go       string `json:"go_version"`
		Native   string `json:"native_go_required"`
	}
	if err = json.Unmarshal(raw, &build); err != nil {
		return err
	}
	if build.Revision != lock.CompilerRevision || build.Version != lock.CompilerVersion || build.Modified != "false" || build.Go != lock.GoVersion || build.Native != lock.GoVersion {
		return fmt.Errorf("compiler identity differs from upstream.json: %s", raw)
	}
	catalog, err := freezeVerificationCatalog("verification-catalog.json")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "verification-catalog.json"), catalog.Raw, 0644); err != nil {
		return err
	}
	workspaces := map[string]suiteWorkspace{}
	for _, s := range suites {
		workspace, err := catalog.workspaceForSuite(s)
		if err != nil {
			return err
		}
		workspaces[s.Package+":"+s.Activity] = workspace
	}
	cache, err := newNativeBuildCache(root)
	if err != nil {
		return err
	}
	defer func() {
		if err := cache.close(); err != nil {
			fmt.Fprintln(os.Stderr, "native build cache cleanup:", err)
		}
	}()
	reports := []observation{}
	var foldingExhaustive simpleFoldExhaustiveReport
	ycbcrExhaustive := map[string]ycbcrExhaustiveReport{}
	mappingExhaustive := map[string]caseMappingExhaustiveReport{}
	for _, s := range suites {
		selected := workspaces[s.Package+":"+s.Activity]
		r := observation{Package: s.Package, Activity: s.Activity, Domain: s.Domain, Cases: len(s.Cases), ReplayEqual: true}
		batchSize := 128
		if s.Package == fixedPackage {
			batchSize = fixedPortBatchSize
		}
		if s.Package == "example/fixed-pixel-bounds" {
			batchSize = fixedConsumerBatchSize
		}
		if s.Package == "app/unicode-inspector" {
			batchSize = unicodeInspectorBatchSize
		}
		if s.Package == "go/image/color" {
			batchSize = colorPortBatchSize
		}
		if s.Package == "go/image/color" && s.Activity == "NRGBAToRGBA" {
			batchSize = nrgbaPortBatchSize
		}
		if s.Package == "app/alpha-matte" {
			batchSize = nrgbaConsumerBatchSize
		}
		if s.Package == "app/color-inspector" {
			batchSize = colorInspectorBatchSize
		}
		if s.Package == "go/image/color" && (s.Activity == "RGBToYCbCr" || s.Activity == "YCbCrToRGB") {
			batchSize = ycbcrPortBatchSize
		}
		if s.Package == "app/ycbcr-inspector" {
			batchSize = ycbcrInspectorBatchSize
		}
		if s.Package == "go/math/bits" && s.Activity == "RotateLeft8" {
			batchSize = rotationPortBatchSize
		}
		if s.Package == "app/byte-rotation" {
			batchSize = rotationInspectorBatchSize
		}
		if s.Package == "go/time" {
			batchSize = durationPortBatchSize
		}
		if s.Package == "app/duration-quantizer" {
			batchSize = durationQuantizerBatchSize
		}
		if s.Package == "go/unicode" {
			batchSize = whitespacePortBatchSize
		}
		if s.Package == "go/unicode" && (s.Activity == "IsLetter" || s.Activity == "IsDigit") {
			batchSize = classificationPortBatchSize
		}
		if s.Package == "app/whitespace-inspector" {
			batchSize = whitespaceInspectorBatchSize
		}
		if s.Package == "app/identifier-rune" {
			batchSize = classificationConsumerBatchSize
		}
		if s.Package == "app/simple-fold" {
			batchSize = simpleFoldConsumerBatchSize
		}
		if s.Package == "go/unicode" && s.Activity == "SimpleFold" {
			batchSize = simpleFoldPortBatchSize
		}
		if s.Package == "go/unicode" && (s.Activity == "To" || s.Activity == "ToUpper" || s.Activity == "ToLower" || s.Activity == "ToTitle") {
			batchSize = caseMappingPortBatchSize
		}
		if s.Package == "app/case-inspector" {
			batchSize = caseMappingConsumerBatchSize
		}
		if s.Package == "go/image" || s.Package == "example/crop-planner" {
			batchSize = geometryBinaryBatchSize
			if s.Activity == "RectangleEmpty" || s.Activity == "RectangleCanon" {
				batchSize = geometryUnaryBatchSize
			}
		}
		for start := 0; start < len(s.Cases); start += batchSize {
			end := min(start+batchSize, len(s.Cases))
			batch := s
			batch.Cases = s.Cases[start:end]
			dir := filepath.Join(root, strings.ReplaceAll(s.Package, "/", "-")+"-"+s.Activity, fmt.Sprintf("%03d", r.Batches))
			if err = os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			for _, source := range selected.Names {
				b := selected.Sources[source]
				path := filepath.Join(dir, source)
				if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				if err = os.WriteFile(path, b, 0644); err != nil {
					return err
				}
			}
			workspace := filepath.Join(dir, "gooo.workspace.json")
			cases := filepath.Join(dir, "cases.json")
			if err = save(workspace, selected.Manifest); err != nil {
				return err
			}
			if err = save(cases, caseDocument(batch)); err != nil {
				return err
			}
			manifestSHA, err := digestFile(workspace)
			if err != nil {
				return err
			}
			casesSHA, err := digestFile(cases)
			if err != nil {
				return err
			}
			identity := expectedIdentity{CompilerRevision: lock.CompilerRevision, GoVersion: lock.GoVersion, ManifestDigest: "sha256:" + manifestSHA, CasesDigest: "sha256:" + casesSHA, Sources: selected.Identities}
			receipt := filepath.Join(dir, "execution.json")
			raw, err = execute(ctx, *compiler, receipt, "package", "execute", "--json", "--cases", cases, workspace)
			if err != nil {
				return err
			}
			original, err := inspect(raw, batch, false, identity)
			if err != nil {
				return fmt.Errorf("%s: %w", receipt, err)
			}
			replayPath := filepath.Join(dir, "replay.json")
			raw, err = execute(ctx, *compiler, replayPath, "package", "replay", "--json", "--receipt", receipt, "--cases", cases, workspace)
			if err != nil {
				return err
			}
			replay, err := inspect(raw, batch, true, identity)
			if err != nil {
				return fmt.Errorf("%s: %w", replayPath, err)
			}
			h, err := digestFile(receipt)
			if err != nil {
				return err
			}
			if err := inspectReplayBinding(original, replay, "sha256:"+h); err != nil {
				return err
			}
			if s.Package == "go/unicode" && s.Activity == "SimpleFold" && r.Batches == 0 {
				foldingExhaustive, err = checkSimpleFoldExhaustive(ctx, filepath.Join(root, "simple-fold-exhaustive"), raw, original.Generated, cache)
				if err != nil {
					return err
				}
			}
			if s.Package == "go/image/color" && (s.Activity == "RGBToYCbCr" || s.Activity == "YCbCrToRGB") && r.Batches == 0 {
				exhaustive, err := checkYCbCrExhaustive(ctx, filepath.Join(root, "ycbcr-exhaustive-"+s.Activity), raw, original.Generated, s.Activity, cache)
				if err != nil {
					return err
				}
				ycbcrExhaustive[s.Activity] = exhaustive
			}
			if s.Package == "go/unicode" && (s.Activity == "To" || s.Activity == "ToUpper" || s.Activity == "ToLower" || s.Activity == "ToTitle") && r.Batches == 0 {
				exhaustive, err := checkCaseMappingExhaustive(ctx, filepath.Join(root, "case-mapping-exhaustive-"+s.Activity), raw, original.Generated, s.Activity, cache)
				if err != nil {
					return err
				}
				mappingExhaustive[s.Activity] = exhaustive
			}
			r.Batches++
			r.Passed += len(batch.Cases)
		}
		reports = append(reports, r)
		fmt.Printf("%s:%s %d/%d; %d batches; replay identical\n", r.Package, r.Activity, r.Passed, r.Cases, r.Batches)
	}
	total := 0
	for _, r := range reports {
		total += r.Cases
	}
	return save(filepath.Join(root, "summary.json"), map[string]any{
		"schema": "gooo/go-ports-differential/v1", "go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"compiler_revision": lock.CompilerRevision, "source_sha256": catalog.Hashes,
		"verification_catalog_sha256": contentDigest(catalog.Raw),
		"ported_functions":            36, "consumer_programs": 14, "consumer_activities": 16,
		"nrgba_port_reference_cases": nrgba.PortReferenceCases, "nrgba_port_rejection_cases": nrgba.PortRejectionCases,
		"nrgba_consumer_reference_cases": nrgba.ConsumerReferenceCases, "nrgba_consumer_rejection_cases": nrgba.ConsumerRejectionCases,
		"nrgba_scope":                "Finite four-byte NRGBA conversion references use pinned color.NRGBA.RGBA. The importing consumer uses real 1x1 image/draw.Draw, draw.Over, no mask and an opaque RGB destination; optimized and generic wrapper paths must agree for every reference. Out-of-byte rejection is a separate extension checked on original int64 record fields before narrowing. The reported RGBA16 channels are premultiplied; final consumer alpha is implicitly 255. No exhaustive 2^32 or 2^56, full-image rendering, linear-light or color-management claim.",
		"fixed_port_reference_cases": fixedBounds.PortReferenceCases, "fixed_port_rejection_cases": fixedBounds.PortRejectionCases,
		"fixed_consumer_reference_cases": fixedBounds.ConsumerReferenceCases, "fixed_consumer_domain_rejection_cases": fixedBounds.ConsumerDomainRejectionCases, "fixed_consumer_overflow_rejection_cases": fixedBounds.ConsumerOverflowRejectionCases,
		"fixed_oracle_source_sha256": fixedSourceSHA,
		"legacy_cases":               legacyCases, "unicode_reference_cases": inspector.ReferenceCases, "unicode_rejection_cases": inspector.RejectionCases,
		"color_port_reference_cases": colors.PortReferenceCases, "color_port_rejection_cases": colors.PortRejectionCases,
		"color_consumer_reference_cases": colors.ConsumerReferenceCases, "color_consumer_rejection_cases": colors.ConsumerRejectionCases,
		"rotation_port_reference_cases": rotations.PortReferenceCases, "rotation_port_rejection_cases": rotations.PortRejectionCases,
		"rotation_consumer_reference_cases": rotations.ConsumerReferenceCases, "rotation_consumer_rejection_cases": rotations.ConsumerRejectionCases,
		"duration_port_reference_cases": durations.PortReferenceCases, "duration_consumer_reference_cases": durations.ConsumerReferenceCases,
		"whitespace_port_reference_cases": whitespace.PortReferenceCases, "whitespace_port_rejection_cases": whitespace.PortRejectionCases,
		"whitespace_consumer_reference_cases": whitespace.ConsumerReferenceCases, "whitespace_consumer_rejection_cases": whitespace.ConsumerRejectionCases,
		"classification_port_reference_cases": classification.PortReferenceCases, "classification_port_rejection_cases": classification.PortRejectionCases,
		"classification_consumer_reference_cases": classification.ConsumerReferenceCases, "classification_consumer_rejection_cases": classification.ConsumerRejectionCases,
		"geometry_port_reference_cases": geometry.PortReferenceCases, "geometry_consumer_reference_cases": geometry.ConsumerReferenceCases,
		"canon_port_reference_cases": canonical.PortReferenceCases, "drag_crop_consumer_reference_cases": canonical.ConsumerReferenceCases,
		"simple_fold_port_reference_cases": folding.PortReferenceCases, "simple_fold_port_extension_cases": folding.PortExtensionCases,
		"simple_fold_consumer_reference_cases": folding.ConsumerReferenceCases, "simple_fold_consumer_rejection_cases": folding.ConsumerRejectionCases,
		"ycbcr_port_reference_cases": ycbcr.PortReferenceCases, "ycbcr_port_rejection_cases": ycbcr.PortRejectionCases,
		"ycbcr_consumer_reference_cases": ycbcr.ConsumerReferenceCases, "ycbcr_consumer_rejection_cases": ycbcr.ConsumerRejectionCases,
		"case_mapping_port_reference_cases": mapping.PortReferenceCases, "case_mapping_port_extension_cases": mapping.PortExtensionCases,
		"case_mapping_invalid_case_reference_cases": mapping.InvalidCaseReferenceCases, "case_mapping_invalid_case_extension_cases": mapping.InvalidCaseExtensionCases,
		"case_mapping_consumer_reference_cases": mapping.ConsumerReferenceCases, "case_mapping_consumer_extension_cases": mapping.ConsumerExtensionCases,
		"case_mapping_exhaustive":       mappingExhaustive,
		"case_mapping_exhaustive_scope": "The unchanged generated Go from each checked replay is compared with pinned Go for every input 0..0x10FFFF: To for each case 0,1,2 and the three unary wrappers separately (6,684,672 comparisons total). Other signed int32 rune boundaries, outside-int32 identity extensions and invalid case selectors are finite, separately counted observations. Invalid-case counts are subsets of the corresponding reference/extension totals. This does not exhaust signed int32, signed int64 selectors/extensions, the importing consumer, locale-specific, multi-rune or whole-string casing.",
		"simple_fold_exhaustive":        foldingExhaustive,
		"ycbcr_exhaustive":              ycbcrExhaustive,
		"ycbcr_exhaustive_scope":        "For each conversion separately, all 16,777,216 byte triples are compared by calling the unmodified generated Go function from a checked finite replay receipt against the pinned Go standard library. These counts are separate from finite receipt cases and do not prove all signed int64 extension inputs, the importing consumer, all runtime targets, a lossless round trip, or whole-image/JPEG behavior.",
		"cases":                         total, "passed": total, "runs": reports,
		"scope": "Finite differential observations against pinned Go source, plus identical saved replay. The 57,251 prior cases are unchanged. NRGBA adds finite record-input conversion and opaque matte references with original int64 domain rejection, independent color and draw oracles and separate counters. Case mapping adds independent Go references, separately counted outside-int32 extensions, and a three-mapping diagnostic consumer. Invalid To case selectors return U+FFFD before rune-domain handling; valid selectors and unary wrappers preserve outside-int32 inputs as an explicit identity extension. Fixed-point Floor/Ceil add finite comparisons with byte-preserved golang.org/x/image source, including original int32 overflow; outside-int32 rejection is separate. PixelBounds rejects invalid raw coordinates first, returns original empty/reversed selections as empty before quantization, and rejects nonempty max coordinates that overflow Ceil; it clips the rounded rectangle against unnormalized int64 bounds. No 2^32 exhaustive check or font/image rendering claim. YCbCr adds finite Go-reference tuples and separately counted out-of-byte rejection cases; original int64 inputs are checked before conversion. Its importing RGB round-trip consumer reports signed restored-minus-original deltas and does not promise a lossless conversion. SimpleFold preserves the pinned ASCII, special-orbit and case-range rules over original signed int32 runes; its outside-int32 identity extension is counted separately. The importing two-rune consumer compares simple folding classes with a maximum orbit length verified from the complete pinned Unicode graph; it is not whole-string folding or locale-aware matching. The separate simple_fold_exhaustive report covers the unchanged generated SimpleFold function on every input 0..0x10FFFF, plus separately counted finite int32 and int64 extension samples, not every int32 value or rune pair. The original four geometry methods preserve four original signed int64 coordinates in each Rectangle record on 64-bit Go, including empty and noncanonical rectangles; named boundaries, a small grid and fixed samples are finite Go method comparisons, with no coordinate normalization or domain-rejection extension. Both-empty Union preserves the second original record; empty Intersect returns the zero record. RectangleCanon separately compares and swaps inverted axes; its finite corpus includes canonical outputs again to observe idempotence. DragCropPlan canonicalizes only the original second record (drag corners) before intersection; original bounds and the existing CropPlan semantics are unchanged. These activities introduce no coordinate arithmetic or domain extension. CropPlan computes bounds only and does not process image pixels or calculate possibly overflowing widths/areas. Classification adds complete pinned Letter and Digit table semantics over original signed int32 rune inputs, observed here only through finite range boundaries, stride holes, Latin-1 and deterministic samples. A separate check-unicode command exhaustively checks 0..0x10FFFF using the unmodified generated Go; its counts are not included in these receipt batches. Whitespace uses pinned Unicode 17.0.0 White_Space semantics over original signed int32 rune inputs; all 256 Latin-1 values and all 25 true property members are included, with other domains observed finitely. Outside-int32 false/-1 extensions are counted separately from Go-reference observations. Duration ports and their consumer preserve both signed int64 nanosecond inputs, including nonpositive quanta, and add finite Go-reference observations without domain-rejection extensions. Color and rotation port/consumer Go-reference cases and explicit out-of-byte rejection cases are counted separately. RuneStart and the five original unary math/bits functions each cover all 256 byte values exactly once, excluding out-of-domain inputs. RotateLeft8 covers all 2,048 byte/rotation pairs only for x=0..255 and k=0..7; other signed int64 rotations have finite boundary/sample observations with original inputs preserved. No claim of exhaustive equivalence for all rotations or other domains, input-independent execution time, whole-package compatibility, ICC color management, or printed appearance.",
	})
}
func caseDocument(s suite) any {
	rows := []any{}
	key := s.Package + ":" + s.Activity
	for _, c := range s.Cases {
		inputs := map[string]any{}
		for i, v := range c.Inputs {
			k := key
			if len(c.Inputs) > 1 {
				k = fmt.Sprintf("%s.input%d", key, i)
			}
			inputs[k] = v
		}
		rows = append(rows, map[string]any{"inputs": inputs, "expected": map[string]any{key: c.Expected}})
	}
	return map[string]any{"schema": "gooo/body-composition-cases/v1", "cases": rows}
}
func execute(ctx context.Context, compiler, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, compiler, args...)
	cmd.Env = append(os.Environ(), "GOOO_LAYA_URL=", "GOOO_LAYA_API_KEY=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if e := os.WriteFile(path, stdout.Bytes(), 0644); e != nil {
		return nil, e
	}
	if stderr.Len() > 0 {
		if e := os.WriteFile(path+".stderr", stderr.Bytes(), 0644); e != nil {
			return nil, e
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
func save(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}
func digestFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
