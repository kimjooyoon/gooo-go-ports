package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The zero value retains the original writer, bounds and batch partitions.
// A compact profile is a new wire/evidence identity, never an old PASS receipt.
type nativeCaseProfile string

const uuidCompactCaseProfile nativeCaseProfile = "uuid-compact-v1"
const uuidCompactCompilerRevision = "83928fa4538245abe33b08549da47f8cbdc50ed2"
const uuidCompactMaxSourceBytes = 10 << 20
const uuidCompactMaxRows = 128
const uuidCompactMaxRawBytes = 30000
const uuidCompactTranslatedBytes = 32 << 10

func parseNativeCaseProfile(value string, browser bool) (nativeCaseProfile, error) {
	var profile nativeCaseProfile
	switch value {
	case "", "legacy":
	case string(uuidCompactCaseProfile):
		profile = uuidCompactCaseProfile
	default:
		return "", fmt.Errorf("unknown --case-profile %q; require legacy or uuid-compact-v1", value)
	}
	if browser && profile != "" {
		return "", fmt.Errorf("--case-profile uuid-compact-v1 cannot be used with --browser-fixtures")
	}
	return profile, nil
}

func (p nativeCaseProfile) validate() error {
	if p != "" && p != uuidCompactCaseProfile {
		return fmt.Errorf("unsupported native case profile %q", p)
	}
	return nil
}

func (p nativeCaseProfile) validateCompiler(lock upstream) error {
	if err := p.validate(); err != nil {
		return err
	}
	if p != "" && (lock.CompilerRevision != uuidCompactCompilerRevision || lock.GoVersion != "go1.27.2") {
		return fmt.Errorf("uuid-compact-v1 requires pinned compiler839 and Go1.27.2 for its source-derived lowering bound")
	}
	return nil
}

func uuidCompactTarget(s suite) bool {
	switch s.Package + ":" + s.Activity {
	case "go/github.com/google/uuid:Validate",
		"app/uuid-format-inspector:Inspect",
		"go/github.com/google/uuid/text:ParseCanonical":
		return true
	}
	return false
}

type nativePreparedCaseBatch struct {
	End int
	// nil means delegate to the original save(caseDocument) without reserializing.
	// Otherwise these exact sized bytes, including LF, are the sole case writer.
	Raw []byte
}

type nativeCaseBatchPolicy struct {
	compact bool
	// With a nonexpanding lowered key the translated compact struct cannot be
	// larger than our compact map encoding without its final LF.
	sourceBytes, loweredKeyBytesUpperBound int
}

func (p nativeCaseProfile) policy(s suite, selected suiteWorkspace, lock upstream) (nativeCaseBatchPolicy, error) {
	if err := p.validateCompiler(lock); err != nil {
		return nativeCaseBatchPolicy{}, err
	}
	if p == "" || !uuidCompactTarget(s) {
		return nativeCaseBatchPolicy{}, nil
	}
	if len(s.Cases) == 0 {
		return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 requires nonempty UUID cases")
	}
	// No input coercion, trimming, replacement, oracle fallback or deduplication.
	for index, c := range s.Cases {
		if _, err := readUUIDInput(c.Inputs); err != nil {
			return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 case%d: %w", index, err)
		}
		if _, ok := c.Expected.(map[string]any); !ok {
			return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 case%d requires original expected record", index)
		}
	}
	if selected.Manifest.Schema != "gooo/package-workspace-manifest/v1" ||
		selected.Manifest.Entry.Package != s.Package || selected.Manifest.Entry.Activity != s.Activity ||
		len(selected.Names) == 0 || len(selected.Sources) != len(selected.Names) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 requires the selected exact source closure and entry")
	}
	seen, total := map[string]bool{}, 0
	for _, name := range selected.Names {
		raw, ok := selected.Sources[name]
		if !filepath.IsLocal(name) || seen[name] || !ok || len(raw) == 0 ||
			len(raw) > uuidCompactMaxSourceBytes-total {
			return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 source closure is missing, repeated or exceeds10MiB")
		}
		seen[name] = true
		total += len(raw)
	}
	// Pinned839 workspace lowering uses GoooPackage<ordinal>Activity<name>.
	// The number of parsed declarations cannot exceed the selected original
	// source-byte sum. That bounds ordinal decimal digits without guessing an
	// actual LoweredName. These three fixed ASCII activity keys are unary.
	lowerMax := len("GoooPackage") + len(strconv.Itoa(total)) + len("Activity") + len(s.Activity)
	if total < 1 || lowerMax > len(s.Package+":"+s.Activity) {
		return nativeCaseBatchPolicy{}, fmt.Errorf("uuid-compact-v1 cannot prove nonexpanding lowered activity keys")
	}
	return nativeCaseBatchPolicy{compact: true, sourceBytes: total, loweredKeyBytesUpperBound: lowerMax}, nil
}

func (p nativeCaseBatchPolicy) prepare(s suite, start int) (nativePreparedCaseBatch, error) {
	if !p.compact {
		end, err := nativeBatchEnd(s, start, nativeBatchSize(s))
		return nativePreparedCaseBatch{End: end}, err
	}
	if !uuidCompactTarget(s) || start < 0 || start >= len(s.Cases) ||
		p.sourceBytes < 1 || p.sourceBytes > uuidCompactMaxSourceBytes ||
		p.loweredKeyBytesUpperBound > len(s.Package+":"+s.Activity) {
		return nativePreparedCaseBatch{}, fmt.Errorf("invalid uuid-compact-v1 batch or lowering bound")
	}
	low, high, best := start+1, min(start+uuidCompactMaxRows, len(s.Cases)), start
	var selected []byte
	for low <= high {
		end := low + (high-low)/2
		batch := s
		batch.Cases = s.Cases[start:end]
		raw, err := json.Marshal(caseDocument(batch))
		if err != nil {
			return nativePreparedCaseBatch{}, fmt.Errorf("serialize uuid-compact-v1: %w", err)
		}
		raw = append(raw, '\n')
		if len(raw) <= uuidCompactMaxRawBytes && len(raw)-1 <= uuidCompactTranslatedBytes {
			best, selected, low = end, raw, end+1
		} else {
			high = end - 1
		}
	}
	if best == start {
		return nativePreparedCaseBatch{}, fmt.Errorf("one original UUID case exceeds uuid-compact-v1 wire budget")
	}
	return nativePreparedCaseBatch{End: best, Raw: selected}, nil
}

func writeNativeCaseBatch(path string, batch suite, prepared nativePreparedCaseBatch) error {
	if prepared.Raw == nil {
		return save(path, caseDocument(batch))
	}
	// No second JSON encoding after sizing. The existing digestFile/inspect and
	// saved replay below bind this exact file for both native invocations.
	return os.WriteFile(path, prepared.Raw, 0644)
}

func (p nativeCaseProfile) identity() map[string]any {
	return map[string]any{
		"schema": "gooo/native-case-profile/v1", "name": string(p),
		"compiler_revision": uuidCompactCompilerRevision, "go_version": "go1.27.2",
		"entries":      []string{"go/github.com/google/uuid:Validate", "app/uuid-format-inspector:Inspect", "go/github.com/google/uuid/text:ParseCanonical"},
		"serializer":   "Go1.27.2 json.Marshal(caseDocument(batch)) plus exactly one LF",
		"maximum_rows": uuidCompactMaxRows, "raw_bytes_including_LF": uuidCompactMaxRawBytes,
		"translated_compact_engine_bytes": uuidCompactTranslatedBytes, "maximum_selected_source_bytes": uuidCompactMaxSourceBytes,
		"lowering_guard": "Pinned839 source-derived nonexpansion upper bound; not measured translated bytes or an actual LoweredName.",
		"scope":          "Only UUID3 wire bytes/partitions change; original ordered cases, independent expected values, exact workspace and fresh execution/saved replay remain required. No legacy batch total, native-capacity acceptance, full-profile compatibility or speedup is inherited.",
	}
}

func (p nativeCaseProfile) decorateSummary(summary map[string]any) {
	if p == "" {
		return
	}
	summary["case_profile"] = p.identity()
	// Correct historical prefix-boundary claims only in the explicit new wire
	// profile. Legacy output retains every existing field/string/absence.
	replacements := map[string][2]string{
		"scope": {
			"The prior 56 suites, 108,835 cases, source rosters and 1,416 batch boundaries are unchanged.",
			"All original case values/order/source rosters are retained. Only UUID3 use uuid-compact-v1 wire bytes and newly observed batch boundaries; no legacy batch total is inherited.",
		},
		"uuid_parse_scope": {
			"All prior 56 suites, 108835 cases, 1416 batch boundaries and source rosters remain unchanged.",
			"All original inputs/expectations/source rosters remain unchanged; UUID3 boundaries use uuid-compact-v1 and are not inherited from the legacy prefix.",
		},
		"containment_scope": {
			"Original 57 suites, 130702 case documents and 1741 batch boundaries remain unchanged.",
			"Original case documents and geometry semantics remain unchanged; UUID3 boundaries belong to uuid-compact-v1 rather than the historical legacy prefix.",
		},
	}
	for name, pair := range replacements {
		if value, ok := summary[name].(string); ok {
			summary[name] = strings.ReplaceAll(value, pair[0], pair[1])
		}
	}
}

func (p nativeCaseProfile) decorateTimingHeader(header map[string]any) {
	if p == "" {
		return
	}
	header["case_profile"] = p.identity()
	header["boundary_scope"] = "Actual contiguous batch coverage is observed. UUID3 use uuid-compact-v1 compact wire bytes and newly selected bounds; no original UUID batch boundaries or legacy2337 total are claimed preserved. Non-UUID suites retain nativeBatchEnd/save behavior. The recorder does not recompute expected partitions."
}
