package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
)

const integerEntity = "gooo://std/integer"

type activityContract struct {
	Source, ID    string
	InputEntities []string
}

// These expectations come from reviewed repository declarations, never a
// receipt. Keep them separate from the unchanged oracle corpus serialization.
func nativeActivityContracts() map[string]activityContract {
	i := integerEntity
	return map[string]activityContract{
		"go/unicode:IsLetter":                 {"ports/unicode/classification.gooo", "gooo://go/unicode/IsLetter", []string{i}},
		"go/unicode:IsDigit":                  {"ports/unicode/classification.gooo", "gooo://go/unicode/IsDigit", []string{i}},
		"app/identifier-rune:Classify":        {"examples/identifier-rune/main.gooo", "gooo://go-ports/examples/identifier-rune/classify", []string{i}},
		"go/unicode:IsSpace":                  {"ports/unicode/space.gooo", "gooo://go/unicode/IsSpace", []string{i}},
		"app/whitespace-inspector:Inspect":    {"examples/whitespace-inspector/main.gooo", "gooo://go-ports/examples/whitespace-inspector/inspect", []string{i}},
		"go/time:DurationRound":               {"ports/time/duration.gooo", "gooo://go/time/Duration/Round", []string{i, i}},
		"go/time:DurationTruncate":            {"ports/time/duration.gooo", "gooo://go/time/Duration/Truncate", []string{i, i}},
		"app/duration-quantizer:Inspect":      {"examples/duration-quantizer/main.gooo", "gooo://go-ports/examples/duration-quantizer/inspect", []string{i, i}},
		"go/net/http:StatusText":              {"ports/net/http.gooo", "gooo://go/net/http/status-text", []string{i}},
		"go/unicode/utf8:ValidRune":           {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/ValidRune", []string{i}},
		"go/unicode/utf8:RuneLen":             {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/RuneLen", []string{i}},
		"go/unicode/utf8:RuneStart":           {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/RuneStart", []string{i}},
		"go/unicode/utf16:IsSurrogate":        {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/IsSurrogate", []string{i}},
		"go/unicode/utf16:RuneLen":            {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/RuneLen", []string{i}},
		"go/unicode/utf16:EncodeRune":         {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/EncodeRune", []string{i}},
		"go/unicode/utf16:DecodeRune":         {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/DecodeRune", []string{i, i}},
		"go/math/bits:Len8":                   {"ports/math/bits.gooo", "gooo://go/math/bits/Len8", []string{i}},
		"go/math/bits:LeadingZeros8":          {"ports/math/bits.gooo", "gooo://go/math/bits/LeadingZeros8", []string{i}},
		"go/math/bits:TrailingZeros8":         {"ports/math/bits.gooo", "gooo://go/math/bits/TrailingZeros8", []string{i}},
		"go/math/bits:OnesCount8":             {"ports/math/bits.gooo", "gooo://go/math/bits/OnesCount8", []string{i}},
		"go/math/bits:RotateLeft8":            {"ports/math/bits.gooo", "gooo://go/math/bits/RotateLeft8", []string{i, i}},
		"app/byte-rotation:Inspect":           {"examples/byte-rotation/main.gooo", "gooo://go-ports/examples/byte-rotation/inspect", []string{i, i}},
		"go/math/bits:Reverse8":               {"ports/math/bits.gooo", "gooo://go/math/bits/Reverse8", []string{i}},
		"go/image/color:RGBToCMYK":            {"ports/image/color.gooo", "gooo://go/image/color/RGBToCMYK", []string{i, i, i}},
		"go/image/color:CMYKToRGB":            {"ports/image/color.gooo", "gooo://go/image/color/CMYKToRGB", []string{i, i, i, i}},
		"app/response:Describe":               {"examples/response.gooo", "gooo://go-ports/examples/response/describe", []string{i}},
		"app/byte-summary:ByteSummary":        {"examples/byte-summary/main.gooo", "gooo://go-ports/examples/byte-summary/summarize", []string{i}},
		"app/unicode-inspector:InspectScalar": {"examples/unicode-inspector/main.gooo", "gooo://go-ports/examples/unicode-inspector/inspect-scalar", []string{i}},
		"app/unicode-inspector:InspectPair":   {"examples/unicode-inspector/main.gooo", "gooo://go-ports/examples/unicode-inspector/inspect-pair", []string{i, i}},
		"app/color-inspector:InspectRGB":      {"examples/color-inspector/main.gooo", "gooo://go-ports/examples/color-inspector/inspect-rgb", []string{i, i, i}},
	}
}

func contractFor(s suite) (activityContract, error) {
	contract, ok := nativeActivityContracts()[s.Package+":"+s.Activity]
	if !ok || contract.Source == "" || contract.ID == "" || len(contract.InputEntities) == 0 {
		return activityContract{}, fmt.Errorf("missing trusted activity contract for %s:%s", s.Package, s.Activity)
	}
	for _, entity := range contract.InputEntities {
		if entity == "" {
			return activityContract{}, fmt.Errorf("missing trusted input entity")
		}
	}
	return contract, nil
}

type sourcePackage struct {
	Path    string   `json:"path"`
	Sources []string `json:"sources"`
}

type sourceIdentity struct{ Package, Digest string }

func contentDigest(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }

// Match the pinned workspace image's digest of JSON-encoded source TEXT, not
// the raw-file digest used by summary.json. Both use the same frozen bytes.
func snapshotSourceIdentities(packages []sourcePackage, sourceBytes map[string][]byte) (map[string]sourceIdentity, error) {
	result := map[string]sourceIdentity{}
	seenPackages := map[string]bool{}
	for _, pkg := range packages {
		if pkg.Path == "" || seenPackages[pkg.Path] || len(pkg.Sources) == 0 {
			return nil, fmt.Errorf("invalid trusted source package %q", pkg.Path)
		}
		seenPackages[pkg.Path] = true
		for _, name := range pkg.Sources {
			raw, ok := sourceBytes[name]
			if !ok || name == "" || result[name].Package != "" {
				return nil, fmt.Errorf("missing or duplicate frozen source %q", name)
			}
			encoded, err := json.Marshal(string(raw))
			if err != nil {
				return nil, err
			}
			result[name] = sourceIdentity{pkg.Path, contentDigest(encoded)}
		}
	}
	if len(result) == 0 || len(result) != len(sourceBytes) {
		return nil, fmt.Errorf("trusted source roster differs from frozen bytes")
	}
	return result, nil
}

type workspaceExport struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	ID         string   `json:"id"`
	InputTypes []string `json:"input_types"`
	OutputType string   `json:"output_type,omitempty"`
}
type workspaceSource struct {
	Filename string `json:"filename"`
	Digest   string `json:"source_digest"`
}
type workspacePackage struct {
	Path    string            `json:"path"`
	Exports []workspaceExport `json:"exports"`
	Sources []workspaceSource `json:"sources"`
}
type workspaceImage struct {
	Packages []workspacePackage `json:"packages"`
}

func inspectWorkspace(image workspaceImage, s suite, contract activityContract, expected map[string]sourceIdentity) error {
	if len(expected) == 0 || expected[contract.Source].Package != s.Package {
		return fmt.Errorf("missing trusted source for activity contract")
	}
	expectedPackages := map[string]bool{}
	for _, source := range expected {
		if source.Package == "" || !validDigest(source.Digest) {
			return fmt.Errorf("invalid trusted source identity")
		}
		expectedPackages[source.Package] = true
	}
	seenPackages := map[string]bool{}
	seenSources := map[string]sourceIdentity{}
	selected := 0
	for _, pkg := range image.Packages {
		if !expectedPackages[pkg.Path] || seenPackages[pkg.Path] {
			return fmt.Errorf("workspace package roster")
		}
		seenPackages[pkg.Path] = true
		for _, source := range pkg.Sources {
			if _, exists := seenSources[source.Filename]; exists {
				return fmt.Errorf("duplicate workspace source")
			}
			got := sourceIdentity{pkg.Path, source.Digest}
			if want, ok := expected[source.Filename]; !ok || got != want {
				return fmt.Errorf("workspace source binding: %s", source.Filename)
			}
			seenSources[source.Filename] = got
		}
		if pkg.Path != s.Package {
			continue
		}
		for _, export := range pkg.Exports {
			if export.Name != s.Activity && export.ID != contract.ID {
				continue
			}
			selected++
			if export.Name != s.Activity || export.Kind != "activity" || export.ID != contract.ID || !reflect.DeepEqual(export.InputTypes, contract.InputEntities) {
				return fmt.Errorf("selected workspace export contract")
			}
			if result := whitespaceResultContract(s); result.Entity != "" && export.OutputType != result.Entity {
				return fmt.Errorf("selected workspace output type")
			}
		}
	}
	if selected != 1 || !reflect.DeepEqual(seenSources, expected) || !reflect.DeepEqual(seenPackages, expectedPackages) {
		return fmt.Errorf("workspace source roster or selected export count")
	}
	return nil
}
