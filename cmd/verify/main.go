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
	workers := flag.Int("workers", 1, "native suites to verify concurrently (1..4; default serial)")
	timingOut := flag.String("timing-out", "", "optional new timing JSONL directly under --out (observation only)")
	caseProfileFlag := flag.String("case-profile", "", "optional case wire profile: legacy, uuid-compact-v1 or uuid-and-literal-report-compact-v1")
	workspaceProfileFlag := flag.String("workspace-profile", "", "optional workspace source profile: legacy or dependency-closure-v1")
	flag.Parse()
	caseProfile, err := parseNativeCaseProfile(*caseProfileFlag, *browser != "")
	if err != nil {
		return err
	}
	workspaceProfile, err := parseNativeWorkspaceProfile(*workspaceProfileFlag, *browser != "")
	if err != nil {
		return err
	}
	if err := validateNativeSuiteWorkers(*workers); err != nil {
		return err
	}
	if *timingOut != "" && (*out == "" || *browser != "") {
		return fmt.Errorf("--timing-out requires --out and cannot be used with --browser-fixtures")
	}
	if *out != "" && *browser != "" {
		return fmt.Errorf("choose either --out or --browser-fixtures")
	}
	var lock upstream
	if err := readJSON("upstream.json", &lock); err != nil {
		return err
	}
	if err := caseProfile.validateCompiler(lock); err != nil {
		return err
	}
	if err := workspaceProfile.validateCompiler(lock); err != nil {
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
	if err := verifyExternalOracles(".", lock.ExternalSources); err != nil {
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
	byteSearch, err := stringByteCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, byteSearch.Suites...)
	uuids, err := uuidCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, uuids.Suites...)
	uuidParse, err := uuidParseCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, uuidParse.Suite)
	containment, err := containmentCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, containment.Suites...)
	literalText, err := literalTextCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, literalText.Suites...)
	literalCut, err := literalCutCorpus()
	if err != nil {
		return err
	}
	suites = append(suites, literalCut.Suites...)
	decimalPayload, err := decimalPayloadCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, decimalPayload.Suites...)
	pathAlias, err := pathAliasCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, pathAlias...)
	if err := verifyHTTPGutsRuntimeDependencies(); err != nil {
		return err
	}
	headers, err := headerCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, headers...)
	addition, err := headerAdditionCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, addition...)
	canonicalHeaderKeys, err := canonicalHeaderCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, canonicalHeaderKeys...)
	etags, err := etagCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, etags...)
	ifMatches, err := ifMatchCorpus(".")
	if err != nil {
		return err
	}
	suites = append(suites, ifMatches...)
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
	if *timingOut != "" {
		if err := validateNativeTimingOutput(*timingOut, root); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
	if err := validateSuiteEvidenceDirectories(root, suites); err != nil {
		return err
	}
	workspaces := make([]suiteWorkspace, len(suites))
	for index, s := range suites {
		workspace, err := workspaceProfile.workspaceForSuite(catalog, s)
		if err != nil {
			return err
		}
		workspaces[index] = workspace
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
	var timing *nativeTiming
	if *timingOut != "" {
		timing, err = newNativeTiming(nativeTimingConfig{
			Output: *timingOut, EvidenceRoot: root, RepositoryRoot: ".",
			Compiler: *compiler, Workers: *workers, Suites: suites,
			Lock: lock, Catalog: catalog, CompilerIdentity: raw, CaseProfile: caseProfile, WorkspaceProfile: workspaceProfile,
		})
		if err != nil {
			return err
		}
	}
	timingClosed := false
	defer func() {
		if timing != nil && !timingClosed {
			// Do not recover a panic or turn an interrupted verifier into a timing PASS.
			timing.abort()
			if err := timing.close(nil, ctx.Err()); err != nil {
				fmt.Fprintln(os.Stderr, "incomplete native timing:", err)
			}
		}
	}()
	reports := []observation{}
	var foldingExhaustive simpleFoldExhaustiveReport
	ycbcrExhaustive := map[string]ycbcrExhaustiveReport{}
	mappingExhaustive := map[string]caseMappingExhaustiveReport{}
	verifier := newNativeSuiteVerifier(ctx, *compiler, root, lock, cache)
	verifier.timing = timing
	verifier.caseProfile = caseProfile
	results, err := runNativeSuites(ctx, suites, *workers, func(ctx context.Context, index int, s suite) (nativeSuiteResult, error) {
		suiteTiming := timing.beginSuite(index, s)
		result, err := verifier.verify(ctx, s, workspaces[index])
		suiteTiming.finish(result, err, ctx.Err())
		if err == nil {
			r := result.Observation
			fmt.Printf("%s:%s %d/%d; %d batches; replay identical\n", r.Package, r.Activity, r.Passed, r.Cases, r.Batches)
		}
		return result, err
	})
	if err != nil {
		// Preserve the original earliest-index verification error as the primary result.
		if timingErr := timing.close(err, ctx.Err()); timingErr != nil {
			fmt.Fprintln(os.Stderr, "native timing:", timingErr)
		}
		timingClosed = true
		return err
	}
	for _, result := range results {
		r := result.Observation
		reports = append(reports, r)
		if result.SimpleFold != nil {
			foldingExhaustive = *result.SimpleFold
		}
		if result.YCbCr != nil {
			ycbcrExhaustive[r.Activity] = *result.YCbCr
		}
		if result.CaseMapping != nil {
			mappingExhaustive[r.Activity] = *result.CaseMapping
		}
	}
	total, passed, portedFunctions, consumerActivities := 0, 0, 0, 0
	consumerPrograms := map[string]bool{}
	for _, r := range reports {
		total += r.Cases
		passed += r.Passed
		if strings.HasPrefix(r.Package, "go/") {
			portedFunctions++
		} else {
			consumerPrograms[r.Package] = true
			consumerActivities++
		}
	}
	summaryDocument := map[string]any{
		"schema": "gooo/go-ports-differential/v1", "go_version": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"native_suite_workers": *workers,
		"compiler_revision":    lock.CompilerRevision, "source_sha256": catalog.Hashes,
		"verification_catalog_sha256": contentDigest(catalog.Raw),
		"ported_functions":            portedFunctions, "consumer_programs": len(consumerPrograms), "consumer_activities": consumerActivities,
		"if_match_reference_cases":                     len(ifMatches[0].Cases),
		"if_match_input_ports":                         2,
		"if_match_result_fields":                       1,
		"if_match_port_source_count":                   1,
		"if_match_conditions":                          map[string]int64{"condNone": 0, "condTrue": 1, "condFalse": 2},
		"if_match_scope":                               ifMatchScope,
		"if_match_provenance_scope":                    ifMatchProvenanceScope,
		"etag_scan_reference_cases":                    len(etags[0].Cases),
		"etag_strong_reference_cases":                  len(etags[1].Cases),
		"etag_report_reference_cases":                  len(etags[2].Cases),
		"etag_scan_unique_inputs":                      190,
		"etag_scan_duplicate_occurrences":              2,
		"etag_report_fields":                           len(etagFactsNames),
		"etag_port_source_count":                       1,
		"etag_report_source_count":                     3,
		"etag_scope":                                   "Bounded decoded Unicode-scalar validUTF8 Text0..1024bytes per operand, including empty/NUL/control/invalidETag grammar. Private pinned Go scanETag/etagStrongMatch are exposed as dedicated Gooo APIs; the original13field Report separates raw header validity from parsed tag facts. All192 scanner occurrences,31 raw pairs and24 facts inputs retained. No arbitrary-byte/unbounded strings, fullIfMatch, currentresource authority, effects or production-safety claim.",
		"etag_provenance_scope":                        "Fresh native and saved replay bind original inputs, exact2/13 ordered typed fields, current1/3source closures, lowered bodies, generated ABI, Report3callee ordinals0/1/2, exact explicit Go binary and raw saved parent. All247 independent Go expectations are separately compared with literal anchors. Inner canonical/typed-plan hashing remains NOT_CHECKED; a label or private firstcase is not public acceptance.",
		"canonical_header_reference_cases":             len(canonicalHeaderKeys[0].Cases),
		"canonical_header_report_reference_cases":      len(canonicalHeaderKeys[1].Cases),
		"canonical_header_raw_input_occurrences":       canonicalHeaderRawCases,
		"canonical_header_unique_keys":                 canonicalHeaderUniqueKeys,
		"canonical_header_within_partition_duplicates": 0,
		"canonical_header_cross_partition_duplicates":  5,
		"canonical_header_report_fields":               len(canonicalHeaderResultNames),
		"canonical_header_port_source_count":           2,
		"canonical_header_report_source_count":         4,
		"canonical_header_scope":                       "Decoded validUTF8 Text0..1024bytes, including empty and invalid header names. Actual Go1.27.2 net/textproto.CanonicalMIMEHeaderKey supplies canonical Text; the importing original8-field Report retains exact original/canonical Text, changed/validity, byte lengths and first invalid offset/octet. All1059 ordered occurrences,1054 unique keys,zero within-partition and five cross-partition duplicates are retained per entry. No arbitrary-byte/unbounded Go string, HTTP parsing, authorization, effect, production or performance claim.",
		"canonical_header_provenance_scope":            "Fresh native execution and saved replay bind original Text, exact typed8 result fields and order, current2/4-source closures, lowered source/record/callee ABI, producer identities and raw outer saved parents. The scalar expectation is the actual pinned Go standard library; Report diagnostics reuse the independent existing header reference. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot receipts are not fresh public acceptance.",
		"header_addition_reference_cases":              len(addition[0].Cases),
		"header_addition_fixed_batch_rows":             8,
		"header_addition_record_types":                 5,
		"header_addition_pure_activities":              6,
		"header_addition_callee_edges":                 5,
		"header_addition_scope":                        "Original deterministic12-field RequestFacts plus three bounded Text inputs return16 typed header-addition decision fields. Actual pinned x/net/httpguts and strconv references supply the50 finite expected results. No per-request human approval is required to compute; host authority/current binding and atomic final apply remain necessary.",
		"header_addition_provenance_scope":             "Fresh native execution and saved replay bind five current sources, six pure activities, five record layouts, five callee edges, exact12 input/16 output fields, generated ABI and raw outer saved parents. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot results are not fresh public acceptance.",
		"header_name_reference_cases":                  len(headers[0].Cases),
		"header_value_reference_cases":                 len(headers[1].Cases),
		"header_facts_reference_cases":                 len(headers[2].Cases),
		"header_provenance_scope":                      "Fresh Gooo execution and saved replay bind original Texts, exact typed12 fields, source images and generated ABI/calls, producers and raw outer receipt parents. Independent inner typed canonical prior_result_sha256 remains NOT_CHECKED; private pilot results are not fresh public acceptance.",
		"header_scope":                                 "Bounded decoded validUTF8 Text<=1024 per field. Actual x/net/httpguts0.59 supplies predicate and prefix/single-byte witness references; original12-field diagnostics retain inputs. Fresh source/typed ABI/value/type/producer/raw outer saved-parent checks; independent inner typed canonical parent digest NOT_CHECKED. No arbitrary-length Go-string,HTTP2,authorization or production safety claim.",
		"path_clean_reference_cases":                   len(pathAlias[0].Cases),
		"path_alias_reference_cases":                   len(pathAlias[1].Cases),
		"path_alias_scope":                             "Decoded valid UTF-8 Text up to1024 bytes, slash-based lexical Go path.Clean. Original11-field Report preserves both original inputs and exposes clean/absolute/change/equality/alias facts. Actual Go path.Clean and path.IsAbs supply independent finite expectations; no URL, router, filesystem, authorization or safety claim.",
		"path_alias_provenance_scope":                  "Fresh native execution and saved replay check original Texts, exact result/field identities and order, selected current source bytes, generated bodies, producer identities and raw outer receipt parents. This generic path does not independently check inner prior_result_sha256 or general callee ABI; private receipts are not fresh public acceptance.",
		"decimal_reference_cases":                      decimalPayload.DecimalReferenceCases,
		"payload_admission_reference_cases":            decimalPayload.PayloadReferenceCases,
		"decimal_payload_scope":                        "Decoded valid UTF-8 Text up to 1024 bytes; exact signed-int64 and Boolean facts. ParseIntDecimal64 compares real Go strconv.ParseInt(input,10,64) values and syntax/range classes, including saturation. Original payload eligibility retains all seven inputs and fourteen fields and is checked against a separate ordered policy. These are finite pure decisions over caller-supplied facts, without HTTP enforcement, current-fact authentication or production safety claims.",
		"decimal_payload_provenance_scope":             "Fresh native execution and saved replay bind original inputs, required output field identities and order, selected source bytes, producers, generated source and outer receipt parents. New entries also check their source-bound lowered bodies and call/typed-plan bindings. Inner prior_result_sha256 is NOT_CHECKED by this generic finite path; private pilot receipts do not substitute for fresh public verification.",
		"uuid_port_reference_cases":                    uuids.PortReferenceCases, "uuid_consumer_reference_cases": uuids.ConsumerReferenceCases,
		"uuid_module_revision": uuidRevision, "uuid_module_h1": uuidModuleH1,
		"uuid_parse_reference_cases":          uuidParse.ReferenceCases,
		"uuid_parse_legacy_reference_inputs":  uuidParse.LegacyReferenceInputs,
		"uuid_parse_anchor_inputs":            uuidParse.ParserAnchorInputs,
		"uuid_parse_added_reference_inputs":   uuidParse.AddedReferenceInputs,
		"uuid_parse_scope":                    "A separate ParseCanonical(Text) API maps original google/uuid v1.6.0 Parse plus returned UUID.String, exact error and IsInvalidLengthError to required parsed/canonical_uuid/error/invalid_length fields. The returned canonical UUID preserves all 16 bytes even when an error occurs, including raw 32-byte failed-byte assignment and standard-form partial returns. Parse accepts 38-byte outer delimiters without Validate's brace gate. Decoded valid UTF-8 Text is bounded to 1024 bytes; original JSON wire bytes are not preserved through decoder replacement. All prior 56 suites, 108835 cases, 1416 batch boundaries and source rosters remain unchanged. Only this new isolated UUID/quote/Text workspace adds finite source-bound comparisons and saved replay; its actual serialized case JSON is bounded to 30000 bytes. No binary UUID method set, ParseBytes, UUID generation, full-module, exhaustive-string-domain, production or performance claim.",
		"uuid_scope":                          "Original google/uuid v1.6.0 Validate accepts four representations and returns exact errors and IsInvalidLengthError classification, mapped to a required result record. References use valid UTF-8 decoded Text of at most 1024 bytes; raw invalid JSON UTF-8/surrogates may be replaced by the core JSON decoder and are not preserved or explicitly rejected. First-nine-byte prefix quoting is computed in Gooo, including truncated runes, using complete pinned Go IsPrint tables. Finite ASCII-position mutations, URN case variants, lengths, Unicode/control prefixes and deterministic samples are not all strings. No Parse, UUID generation, version/variant restriction or whole-library compatibility claim. The importing consumer adds exact UTF-8 byte length. Prior54 suites,65345 cases and882 batch boundaries are preserved; only new UUID suites add serialized-byte bounded batches.",
		"literal_cut_reference_cases":         literalCut.CutReferenceCases,
		"literal_cut_prefix_reference_cases":  literalCut.CutPrefixReferenceCases,
		"literal_cut_suffix_reference_cases":  literalCut.CutSuffixReferenceCases,
		"literal_cut_report_reference_cases":  literalCut.ReportReferenceCases,
		"literal_cut_scope":                   literalCut.Suites[0].Domain,
		"literal_cut_finite_provenance_scope": "The appended Cut/CutPrefix/CutSuffix/Report suites preserve original Text inputs, exact result/field identities, workspace source and producer bindings, generated bytes and outer saved-receipt links through the existing finite verifier. Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED by this finite path. The separate private32 input-only observation belongs to its original pilot source and is not fresh acceptance for the renamed public entry.",
		"literal_index_reference_cases":       literalText.IndexReferenceCases,
		"literal_count_reference_cases":       literalText.CountReferenceCases,
		"literal_report_reference_cases":      literalText.ReportReferenceCases,
		"literal_text_category_candidates":    literalText.Roster.CategoryCandidates,
		"literal_text_category_added":         literalText.Roster.CategoryAdded,
		"literal_text_scope":                  literalText.Suites[0].Domain,
		"literal_finite_provenance_scope":     "Finite Index/Count/Report suites use the generic verifier's exact original input/source/producer/output and saved outer-receipt binding checks. Inner prior_result_sha256, full generation-report provenance and composition/runtime typed-plan equality are NOT_CHECKED by this finite path. The separate required input-only24 Report checker provides those stronger bindings for its independently reviewed pinned profile; it does not replace this full finite corpus.",
		"string_byte_port_reference_cases":    byteSearch.PortReferenceCases, "string_byte_port_extension_cases": byteSearch.PortExtensionCases,
		"string_byte_consumer_reference_cases": byteSearch.ConsumerReferenceCases, "string_byte_consumer_extension_cases": byteSearch.ConsumerExtensionCases,
		"string_byte_scope":          "Six valid UTF-8 Text inputs, including empty, repeated ASCII, embedded NUL, multibyte text and exactly 1024 bytes, are crossed with every byte needle 0..255. The independent Go strings.IndexByte oracle preserves first-match byte offsets, including UTF-8 continuation bytes. Sixteen original out-of-byte int64 needles per activity are separately counted extensions, rejected before narrowing: offset -1 and, for the consumer, found=false, in_byte_domain=false and unchanged byte length. This does not cover arbitrary invalid UTF-8 Go strings, longer native Text, all strings, rune search, full strings-package compatibility or performance.",
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
		"rectangle_in_reference_cases":          containment.PortReferenceCases,
		"fits_crop_reference_cases":             containment.ConsumerReferenceCases,
		"containment_baseline_reference_inputs": containment.BaselineReferenceInputs,
		"containment_added_reference_inputs":    containment.AddedReferenceInputs,
		"containment_scope":                     "RectangleIn independently compares original image.Rectangle.In over raw signed-int64 Rectangle literals on 64-bit Go, including empty/reversed receivers and exact extrema. FitsCrop separately compares !requested.Empty() && requested.In(bounds), rejecting empty requests instead of clipping them. No image.Rect construction, normalization, subtraction, narrowing or coordinate-domain extension is used. Original 57 suites, 130702 case documents and 1741 batch boundaries remain unchanged. Both appended suites use the existing 32-row geometry binary policy and check actual serialized JSON plus final LF within 32KiB. They use finite original geometry inputs plus named containment/edge/empty/extreme/precision pairs; they do not claim all Rectangle pairs, image/GIF processing or production safety.",
		"canon_port_reference_cases":            canonical.PortReferenceCases, "drag_crop_consumer_reference_cases": canonical.ConsumerReferenceCases,
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
		"cases":                         total, "passed": passed, "runs": reports,
		"scope": "Finite differential observations against pinned Go source, plus identical saved replay. The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged. A separate ParseCanonical Text API adds original Google UUID Parse and returned String observations, preserving partial values on errors. Google UUID Validate and its importing consumer add finite original-source references, exact error/classification records and isolated package workspaces. Strings.IndexByte adds independent byte-search references and separately counted original int64 needle extensions; its two Text suites and all UUID suites use a serialized-byte batch bound. NRGBA adds finite record-input conversion and opaque matte references with original int64 domain rejection, independent color and draw oracles and separate counters. Case mapping adds independent Go references, separately counted outside-int32 extensions, and a three-mapping diagnostic consumer. Invalid To case selectors return U+FFFD before rune-domain handling; valid selectors and unary wrappers preserve outside-int32 inputs as an explicit identity extension. Fixed-point Floor/Ceil add finite comparisons with byte-preserved golang.org/x/image source, including original int32 overflow; outside-int32 rejection is separate. PixelBounds rejects invalid raw coordinates first, returns original empty/reversed selections as empty before quantization, and rejects nonempty max coordinates that overflow Ceil; it clips the rounded rectangle against unnormalized int64 bounds. No 2^32 exhaustive check or font/image rendering claim. YCbCr adds finite Go-reference tuples and separately counted out-of-byte rejection cases; original int64 inputs are checked before conversion. Its importing RGB round-trip consumer reports signed restored-minus-original deltas and does not promise a lossless conversion. SimpleFold preserves the pinned ASCII, special-orbit and case-range rules over original signed int32 runes; its outside-int32 identity extension is counted separately. The importing two-rune consumer compares simple folding classes with a maximum orbit length verified from the complete pinned Unicode graph; it is not whole-string folding or locale-aware matching. The separate simple_fold_exhaustive report covers the unchanged generated SimpleFold function on every input 0..0x10FFFF, plus separately counted finite int32 and int64 extension samples, not every int32 value or rune pair. The original four geometry methods preserve four original signed int64 coordinates in each Rectangle record on 64-bit Go, including empty and noncanonical rectangles; named boundaries, a small grid and fixed samples are finite Go method comparisons, with no coordinate normalization or domain-rejection extension. Both-empty Union preserves the second original record; empty Intersect returns the zero record. RectangleCanon separately compares and swaps inverted axes; its finite corpus includes canonical outputs again to observe idempotence. DragCropPlan canonicalizes only the original second record (drag corners) before intersection; original bounds and the existing CropPlan semantics are unchanged. These activities introduce no coordinate arithmetic or domain extension. CropPlan computes bounds only and does not process image pixels or calculate possibly overflowing widths/areas. Classification adds complete pinned Letter and Digit table semantics over original signed int32 rune inputs, observed here only through finite range boundaries, stride holes, Latin-1 and deterministic samples. A separate check-unicode command exhaustively checks 0..0x10FFFF using the unmodified generated Go; its counts are not included in these receipt batches. Whitespace uses pinned Unicode 17.0.0 White_Space semantics over original signed int32 rune inputs; all 256 Latin-1 values and all 25 true property members are included, with other domains observed finitely. Outside-int32 false/-1 extensions are counted separately from Go-reference observations. Duration ports and their consumer preserve both signed int64 nanosecond inputs, including nonpositive quanta, and add finite Go-reference observations without domain-rejection extensions. Color and rotation port/consumer Go-reference cases and explicit out-of-byte rejection cases are counted separately. RuneStart and the five original unary math/bits functions each cover all 256 byte values exactly once, excluding out-of-domain inputs. RotateLeft8 covers all 2,048 byte/rotation pairs only for x=0..255 and k=0..7; other signed int64 rotations have finite boundary/sample observations with original inputs preserved. No claim of exhaustive equivalence for all rotations or other domains, input-independent execution time, whole-package compatibility, ICC color management, or printed appearance.",
	}
	caseProfile.decorateSummary(summaryDocument)
	workspaceProfile.decorateSummary(summaryDocument)
	summaryErr := save(filepath.Join(root, "summary.json"), summaryDocument)
	// Timing never substitutes for the original summary or its required CI predicates.
	timingErr := timing.close(summaryErr, ctx.Err())
	timingClosed = true
	if summaryErr != nil {
		if timingErr != nil {
			fmt.Fprintln(os.Stderr, "native timing:", timingErr)
		}
		return summaryErr
	}
	return timingErr
}
func caseDocument(s suite) any {
	rows := make([]any, 0, len(s.Cases))
	key := s.Package + ":" + s.Activity
	for _, c := range s.Cases {
		inputs := make(map[string]any, len(c.Inputs))
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
