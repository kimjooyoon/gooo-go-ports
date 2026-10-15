package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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
		pathCleanPackage + ":" + pathCleanActivity:               {"ports/path/clean.gooo", "gooo://go/path/Clean", []string{textEntity}},
		pathAliasPackage + ":" + pathAliasActivity:               {"examples/path-alias-facts/main.gooo", "gooo://go-ports/examples/path-alias-facts/report/v1", []string{textEntity, textEntity}},
		decimalPackage + ":" + decimalActivity:                   {"ports/strconv/decimal.gooo", "gooo://go/strconv/parse-int-decimal64/v1", []string{textEntity}},
		payloadAdmissionPackage + ":" + payloadAdmissionActivity: {"examples/payload-admission/main.gooo", "gooo://go-ports/examples/payload-admission/report/v1", []string{textEntity, i, i, i, "gooo://std/boolean", "gooo://std/boolean", "gooo://std/boolean"}},
		literalCutPackage + ":Cut":                               {"ports/strings/cut.gooo", "gooo://go/strings/Cut", []string{textEntity, textEntity}},
		literalCutPackage + ":CutPrefix":                         {"ports/strings/cut.gooo", "gooo://go/strings/CutPrefix", []string{textEntity, textEntity}},
		literalCutPackage + ":CutSuffix":                         {"ports/strings/cut.gooo", "gooo://go/strings/CutSuffix", []string{textEntity, textEntity}},
		literalCutConsumerPackage + ":Report":                    {"examples/literal-cut-report/main.gooo", "gooo://go-ports/examples/literal-cut-report/report", []string{textEntity, textEntity}},
		literalTextPackage + ":Index":                            {"ports/strings/literal.gooo", "gooo://go/strings/Index", []string{textEntity, textEntity}},
		literalTextPackage + ":Count":                            {"ports/strings/literal.gooo", "gooo://go/strings/Count", []string{textEntity, textEntity}},
		literalTextConsumerPackage + ":Report":                   {"examples/literal-text-report/main.gooo", "gooo://go-ports/examples/literal-text-report/report", []string{textEntity, textEntity}},
		uuidParsePackage + ":ParseCanonical":                     {"ports/google/uuid/text/parse_canonical.gooo", "gooo://github.com/google/uuid/text/parse-canonical", []string{textEntity}},
		uuidPackage + ":Validate":                                {"ports/google/uuid/validate.gooo", "gooo://github.com/google/uuid/Validate", []string{textEntity}},
		uuidConsumerPackage + ":Inspect":                         {"examples/uuid-format-inspector/main.gooo", "gooo://go-ports/examples/uuid-format-inspector/inspect", []string{textEntity}},
		stringBytePackage + ":IndexByte":                         {"ports/strings/search.gooo", "gooo://go/strings/IndexByte", []string{textEntity, i}},
		stringByteConsumerPackage + ":Search":                    {"examples/string-byte-search/main.gooo", "gooo://go-ports/examples/string-byte-search/search", []string{textEntity, i}},
		"go/image/color:NRGBAToRGBA":                             {"ports/image/nrgba.gooo", "gooo://go/image/color/NRGBAToRGBA", []string{nrgbaEntity}},
		"app/alpha-matte:MattePixel":                             {"examples/alpha-matte/main.gooo", "gooo://go-ports/examples/alpha-matte/matte-pixel", []string{nrgbaEntity, nrgbaRGBEntity}},
		"go/unicode:To":                                          {"ports/unicode/case_mapping.gooo", "gooo://go/unicode/To", []string{i, i}},
		"go/unicode:ToUpper":                                     {"ports/unicode/case_mapping.gooo", "gooo://go/unicode/ToUpper", []string{i}},
		"go/unicode:ToLower":                                     {"ports/unicode/case_mapping.gooo", "gooo://go/unicode/ToLower", []string{i}},
		"go/unicode:ToTitle":                                     {"ports/unicode/case_mapping.gooo", "gooo://go/unicode/ToTitle", []string{i}},
		"app/case-inspector:Describe":                            {"examples/case-inspector/main.gooo", "gooo://go-ports/examples/case-inspector/describe", []string{i}},
		fixedPackage + ":Int26_6Floor":                           {"ports/image/fixed.gooo", "gooo://golang.org/x/image/math/fixed/Int26_6/Floor", []string{i}},
		fixedPackage + ":Int26_6Ceil":                            {"ports/image/fixed.gooo", "gooo://golang.org/x/image/math/fixed/Int26_6/Ceil", []string{i}},
		"example/fixed-pixel-bounds:PixelBounds":                 {"examples/fixed-pixel-bounds/main.gooo", "gooo://example/fixed-pixel-bounds/pixel-bounds", []string{rectangleEntity, rectangleEntity}},
		"go/image:RectangleCanon":                                {"ports/image/canon.gooo", "gooo://go/image/rectangle-canon", []string{rectangleEntity}},
		"example/crop-planner:DragCropPlan":                      {"examples/crop-planner/drag_crop.gooo", "gooo://example/crop-planner/drag-crop-plan", []string{rectangleEntity, rectangleEntity}},
		"go/image/color:RGBToYCbCr":                              {"ports/image/ycbcr.gooo", "gooo://go/image/color/RGBToYCbCr", []string{i, i, i}},
		"go/image/color:YCbCrToRGB":                              {"ports/image/ycbcr.gooo", "gooo://go/image/color/YCbCrToRGB", []string{i, i, i}},
		"app/ycbcr-inspector:InspectRGB":                         {"examples/ycbcr-inspector/main.gooo", "gooo://go-ports/examples/ycbcr-inspector/inspect-rgb", []string{i, i, i}},
		"go/unicode:SimpleFold":                                  {"ports/unicode/simple_fold.gooo", "gooo://go/unicode/SimpleFold", []string{i}},
		"app/simple-fold:Compare":                                {"examples/simple-fold/main.gooo", "gooo://go-ports/examples/simple-fold/compare", []string{i, i}},
		"go/image:RectangleEmpty":                                {"ports/image/geometry.gooo", "gooo://go/image/rectangle-empty", []string{rectangleEntity}},
		"go/image:RectangleOverlaps":                             {"ports/image/geometry.gooo", "gooo://go/image/rectangle-overlaps", []string{rectangleEntity, rectangleEntity}},
		"go/image:RectangleIntersect":                            {"ports/image/geometry.gooo", "gooo://go/image/rectangle-intersect", []string{rectangleEntity, rectangleEntity}},
		"go/image:RectangleUnion":                                {"ports/image/geometry.gooo", "gooo://go/image/rectangle-union", []string{rectangleEntity, rectangleEntity}},
		"go/image:RectangleIn":                                   {"ports/image/geometry.gooo", "gooo://go/image/rectangle-in", []string{rectangleEntity, rectangleEntity}},
		"example/crop-planner:CropPlan":                          {"examples/crop-planner/crop_planner.gooo", "gooo://example/crop-planner/crop-plan", []string{rectangleEntity, rectangleEntity}},
		"example/fits-crop:FitsCrop":                             {"examples/fits-crop/main.gooo", "gooo://example/fits-crop/fits-crop", []string{rectangleEntity, rectangleEntity}},
		"go/unicode:IsLetter":                                    {"ports/unicode/classification.gooo", "gooo://go/unicode/IsLetter", []string{i}},
		"go/unicode:IsDigit":                                     {"ports/unicode/classification.gooo", "gooo://go/unicode/IsDigit", []string{i}},
		"app/identifier-rune:Classify":                           {"examples/identifier-rune/main.gooo", "gooo://go-ports/examples/identifier-rune/classify", []string{i}},
		"go/unicode:IsSpace":                                     {"ports/unicode/space.gooo", "gooo://go/unicode/IsSpace", []string{i}},
		"app/whitespace-inspector:Inspect":                       {"examples/whitespace-inspector/main.gooo", "gooo://go-ports/examples/whitespace-inspector/inspect", []string{i}},
		"go/time:DurationRound":                                  {"ports/time/duration.gooo", "gooo://go/time/Duration/Round", []string{i, i}},
		"go/time:DurationTruncate":                               {"ports/time/duration.gooo", "gooo://go/time/Duration/Truncate", []string{i, i}},
		"app/duration-quantizer:Inspect":                         {"examples/duration-quantizer/main.gooo", "gooo://go-ports/examples/duration-quantizer/inspect", []string{i, i}},
		"go/net/http:StatusText":                                 {"ports/net/http.gooo", "gooo://go/net/http/status-text", []string{i}},
		"go/unicode/utf8:ValidRune":                              {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/ValidRune", []string{i}},
		"go/unicode/utf8:RuneLen":                                {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/RuneLen", []string{i}},
		"go/unicode/utf8:RuneStart":                              {"ports/unicode/utf8.gooo", "gooo://go/unicode/utf8/RuneStart", []string{i}},
		"go/unicode/utf16:IsSurrogate":                           {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/IsSurrogate", []string{i}},
		"go/unicode/utf16:RuneLen":                               {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/RuneLen", []string{i}},
		"go/unicode/utf16:EncodeRune":                            {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/EncodeRune", []string{i}},
		"go/unicode/utf16:DecodeRune":                            {"ports/unicode/utf16.gooo", "gooo://go/unicode/utf16/DecodeRune", []string{i, i}},
		"go/math/bits:Len8":                                      {"ports/math/bits.gooo", "gooo://go/math/bits/Len8", []string{i}},
		"go/math/bits:LeadingZeros8":                             {"ports/math/bits.gooo", "gooo://go/math/bits/LeadingZeros8", []string{i}},
		"go/math/bits:TrailingZeros8":                            {"ports/math/bits.gooo", "gooo://go/math/bits/TrailingZeros8", []string{i}},
		"go/math/bits:OnesCount8":                                {"ports/math/bits.gooo", "gooo://go/math/bits/OnesCount8", []string{i}},
		"go/math/bits:RotateLeft8":                               {"ports/math/bits.gooo", "gooo://go/math/bits/RotateLeft8", []string{i, i}},
		"app/byte-rotation:Inspect":                              {"examples/byte-rotation/main.gooo", "gooo://go-ports/examples/byte-rotation/inspect", []string{i, i}},
		"go/math/bits:Reverse8":                                  {"ports/math/bits.gooo", "gooo://go/math/bits/Reverse8", []string{i}},
		"go/image/color:RGBToCMYK":                               {"ports/image/color.gooo", "gooo://go/image/color/RGBToCMYK", []string{i, i, i}},
		"go/image/color:CMYKToRGB":                               {"ports/image/color.gooo", "gooo://go/image/color/CMYKToRGB", []string{i, i, i, i}},
		"app/response:Describe":                                  {"examples/response.gooo", "gooo://go-ports/examples/response/describe", []string{i}},
		"app/byte-summary:ByteSummary":                           {"examples/byte-summary/main.gooo", "gooo://go-ports/examples/byte-summary/summarize", []string{i}},
		"app/unicode-inspector:InspectScalar":                    {"examples/unicode-inspector/main.gooo", "gooo://go-ports/examples/unicode-inspector/inspect-scalar", []string{i}},
		"app/unicode-inspector:InspectPair":                      {"examples/unicode-inspector/main.gooo", "gooo://go-ports/examples/unicode-inspector/inspect-pair", []string{i, i}},
		"app/color-inspector:InspectRGB":                         {"examples/color-inspector/main.gooo", "gooo://go-ports/examples/color-inspector/inspect-rgb", []string{i, i, i}},
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

// The decimal/parser composition has a deliberately concrete source and record
// ABI. This supplements the legacy envelope only for these two new entries.
// It does not interpret Go, execute the policy, or replace Go oracle outputs.
type decimalPayloadBody struct {
	Name, Output, ID, Body string
	Inputs                 []string
}

var decimalPayloadDeclaration = regexp.MustCompile(`(?m)^activity[\t ]+([A-Za-z_][A-Za-z_0-9]*)\(([^\n)]*)\)[\t ]*->[\t ]*([A-Za-z_][A-Za-z_0-9]*)[\t ]+id[\t ]+"([^"\n]+)"[\t ]+computes[\t ]+`)

func decimalPayloadBodies(raw []byte) (map[string]decimalPayloadBody, error) {
	result := map[string]decimalPayloadBody{}
	for _, match := range decimalPayloadDeclaration.FindAllSubmatchIndex(raw, -1) {
		name := string(raw[match[2]:match[3]])
		literal, err := strconv.QuotedPrefix(string(raw[match[1]:]))
		if err != nil || result[name].Name != "" {
			return nil, fmt.Errorf("decimal/payload activity literal or duplicate")
		}
		body, err := strconv.Unquote(literal)
		if err != nil || body == "" {
			return nil, fmt.Errorf("decimal/payload empty activity body")
		}
		parameters := strings.Split(string(raw[match[4]:match[5]]), ",")
		for i := range parameters {
			parameters[i] = strings.TrimSpace(parameters[i])
		}
		result[name] = decimalPayloadBody{name, string(raw[match[6]:match[7]]), string(raw[match[8]:match[9]]), body, parameters}
	}
	return result, nil
}

func decimalPayloadDerivedID(namespace, name string) string {
	var result strings.Builder
	var previous rune
	for _, current := range name {
		if unicode.IsUpper(current) && unicode.IsLower(previous) {
			result.WriteByte('-')
		}
		result.WriteRune(unicode.ToLower(current))
		previous = current
	}
	return strings.ReplaceAll(namespace, "_", "-") + "://activity/" + result.String()
}

func decimalPayloadSame(got, want any) bool {
	raw, err := json.Marshal(want)
	if err != nil {
		return false
	}
	expected, err := decode(raw)
	return err == nil && reflect.DeepEqual(got, expected)
}

func decimalPayloadReceiptObject(node map[string]any, keys ...string) (map[string]any, error) {
	for _, key := range keys {
		value, ok := node[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("decimal/payload missing object %s", key)
		}
		node = value
	}
	return node, nil
}

func decimalPayloadRecords(consumer bool) []any {
	definitions := []struct {
		name  string
		s     suite
		kinds []string
	}{
		{"DecimalInt64Result", suite{Package: decimalPackage, Activity: decimalActivity}, []string{"string", "integer", "boolean", "integer"}},
	}
	if consumer {
		definitions = append([]struct {
			name  string
			s     suite
			kinds []string
		}{{"PayloadAdmissionFacts", suite{Package: payloadAdmissionPackage, Activity: payloadAdmissionActivity}, []string{"string", "integer", "integer", "integer", "boolean", "boolean", "boolean", "integer", "integer", "boolean", "boolean", "boolean", "boolean", "integer"}}}, definitions...)
	}
	records := make([]any, 0, len(definitions))
	for _, definition := range definitions {
		contract := whitespaceResultContract(definition.s)
		fields := make([]any, 0, len(contract.Fields))
		for i, field := range contract.Fields {
			fields = append(fields, map[string]any{"name": field.Name, "id": field.ID, "type_id": "urn:gooo:type:" + definition.kinds[i], "go_name": "GoooField" + strings.TrimPrefix(contentDigest([]byte(field.ID)), "sha256:")})
		}
		records = append(records, map[string]any{"name": definition.name, "id": contract.Entity, "go_name": "GoooRecord" + strings.TrimPrefix(contentDigest([]byte(contract.Entity)), "sha256:"), "fields": fields})
	}
	return records
}

func inspectDecimalPayloadABI(raw []byte, s suite, identity expectedIdentity) error {
	consumer := s.Package == payloadAdmissionPackage
	parserSuite := suite{Package: decimalPackage, Activity: decimalActivity}
	parserContract, _ := contractFor(parserSuite)
	contract, _ := contractFor(s)
	files := []string{parserContract.Source}
	if consumer {
		files = append(files, contract.Source)
	}
	if len(identity.SourceBytes) != len(files) || len(identity.Sources) != len(files) {
		return fmt.Errorf("decimal/payload frozen source byte roster")
	}
	bodies := map[string]decimalPayloadBody{}
	for _, name := range files {
		source, ok := identity.SourceBytes[name]
		encoded, err := json.Marshal(string(source))
		if !ok || err != nil || contentDigest(encoded) != identity.Sources[name].Digest {
			return fmt.Errorf("decimal/payload source bytes differ from snapshot")
		}
		declared, err := decimalPayloadBodies(source)
		activity := decimalActivity
		if name != parserContract.Source {
			activity = payloadAdmissionActivity
		}
		if err != nil || len(declared) != 1 || declared[activity].Name != activity {
			return fmt.Errorf("decimal/payload source declaration roster")
		}
		bodies[activity] = declared[activity]
	}
	parserBody := bodies[decimalActivity]
	if parserBody.ID != parserContract.ID || parserBody.Output != "DecimalInt64Result" || !reflect.DeepEqual(parserBody.Inputs, []string{"Text"}) {
		return fmt.Errorf("decimal parser source signature")
	}
	lowered := "GoooPackage0ActivityParseIntDecimal64"
	inputNames, goTypes, outputName := []string{"Text"}, []string{"string"}, "DecimalInt64Result"
	if consumer {
		lowered = "GoooPackage1ActivityReport"
		inputNames = []string{"Text", "Integer", "Integer", "Integer", "Boolean", "Boolean", "Boolean"}
		goTypes = []string{"string", "int64", "int64", "int64", "bool", "bool", "bool"}
		outputName = "PayloadAdmissionFacts"
		body := bodies[payloadAdmissionActivity]
		if body.ID != contract.ID || body.Output != outputName || !reflect.DeepEqual(body.Inputs, inputNames) {
			return fmt.Errorf("payload source signature")
		}
	}
	node, err := decode(raw)
	if err != nil {
		return err
	}
	top, ok := node.(map[string]any)
	if !ok {
		return fmt.Errorf("decimal/payload receipt object")
	}
	r, err := decimalPayloadReceiptObject(top, "result")
	if err != nil {
		return err
	}
	p, err := decimalPayloadReceiptObject(r, "program")
	if err != nil {
		return err
	}
	c, err := decimalPayloadReceiptObject(r, "composition")
	if err != nil {
		return err
	}
	runtime, err := decimalPayloadReceiptObject(r, "runtime")
	if err != nil {
		return err
	}
	image, err := decimalPayloadReceiptObject(p, "workspace_image")
	if err != nil {
		return err
	}
	initOrder := []string{decimalPackage}
	if consumer {
		initOrder = append(initOrder, payloadAdmissionPackage)
	}
	imageEntry := map[string]any{"package_path": s.Package, "source": contract.Source, "activity": s.Activity, "inputs": inputNames, "output": outputName}
	if image["schema"] != "gooo/package-runtime-image/v1" || !decimalPayloadSame(image["init_order"], initOrder) || !decimalPayloadSame(image["entry"], imageEntry) {
		return fmt.Errorf("decimal/payload workspace entry/initialization")
	}
	root := map[string]any{"package_path": s.Package, "activity": s.Activity, "lowered_name": lowered}
	if !decimalPayloadSame(p["entry"], root) || !decimalPayloadSame(p["activities"], []any{root}) || p["entity_aliases"] != nil {
		return fmt.Errorf("decimal/payload lowered entry")
	}
	text, ok := p["lowered_gooo_source"].(string)
	actualBodies, bodyErr := decimalPayloadBodies([]byte(text))
	expectedBodies := map[string]string{"GoooPackage0ActivityParseIntDecimal64": parserBody.Body}
	rootBody := parserBody.Body
	if consumer {
		body := bodies[payloadAdmissionActivity].Body
		const token = "decimal.ParseIntDecimal64"
		at := strings.Index(body, token+"(input0)")
		if at < 0 || strings.Count(body, token) != 1 {
			return fmt.Errorf("payload source parser call")
		}
		rootBody = strings.Replace(body, token, "GoooPackage0ActivityParseIntDecimal64", 1)
		expectedBodies[lowered] = rootBody
		identities := []any{}
		for _, value := range []struct{ pkg, name, lowered, source, namespace, body string }{
			{payloadAdmissionPackage, payloadAdmissionActivity, lowered, contract.Source, "gooo_ports_examples_payload_admission", body},
			{decimalPackage, decimalActivity, "GoooPackage0ActivityParseIntDecimal64", parserContract.Source, "gooo_ports_strconv", parserBody.Body},
		} {
			identities = append(identities, map[string]any{"activity": map[string]any{"package_path": value.pkg, "activity": value.name, "lowered_name": value.lowered}, "activity_id": decimalPayloadDerivedID(value.namespace, value.name), "lowered_activity_id": decimalPayloadDerivedID("gooo_workspace", value.lowered), "source": value.source, "source_body_sha256": contentDigest([]byte(value.body))})
		}
		want := map[string]any{"schema": "gooo/workspace-pure-calls/v1", "activities": identities, "sites": []any{map[string]any{"caller": s.Package + ":" + s.Activity, "callee": decimalPackage + ":" + decimalActivity, "surface": "computes", "start_byte": at, "end_byte": at + len(token)}}}
		if !decimalPayloadSame(p["pure_calls"], want) {
			return fmt.Errorf("payload source/callee pure-call binding")
		}
	} else if p["pure_calls"] != nil {
		return fmt.Errorf("decimal parser unexpected pure calls")
	}
	if !ok || bodyErr != nil || len(actualBodies) != len(expectedBodies) {
		return fmt.Errorf("decimal/payload lowered body roster")
	}
	for name, body := range expectedBodies {
		if actualBodies[name].Body != body {
			return fmt.Errorf("decimal/payload changed lowered body")
		}
	}
	sourceSHA := contentDigest([]byte(text))
	if c["schema"] != "gooo/body-composition/v1" || c["stage"] != "COMPLETE" || c["gooo_source"] != text || c["original_source_sha256"] != sourceSHA || c["selected_source_sha256"] != sourceSHA || r["execution_source_sha256"] != sourceSHA {
		return fmt.Errorf("decimal/payload selected source binding")
	}
	plan, err := decimalPayloadReceiptObject(c, "plan")
	if err != nil {
		return err
	}
	activity := map[string]any{"name": lowered, "id": contract.ID, "input_type": "Text", "output_type": outputName, "input_entity_id": textEntity, "output_entity_id": whitespaceResultContract(s).Entity, "go_function": "GoooComposedActivity0", "input_from": -1, "assembling": false}
	if consumer {
		ports := []any{}
		for i, name := range inputNames {
			ports = append(ports, map[string]any{"port": fmt.Sprintf("input%d", i), "type": name, "entity_id": contract.InputEntities[i], "from": -1})
		}
		activity["inputs"] = ports
		if plan["entry_activity"] != lowered {
			return fmt.Errorf("payload typed plan entry")
		}
	} else if plan["entry_activity"] != nil {
		return fmt.Errorf("decimal unexpected plan entry selection")
	}
	records := decimalPayloadRecords(consumer)
	if plan["schema"] != "gooo/body-composition-plan/v1" || !decimalPayloadSame(plan["activities"], []any{activity}) || !decimalPayloadSame(plan["record_types"], records) || !decimalPayloadSame(plan["edges"], []any{}) || plan["preparations"] != nil {
		return fmt.Errorf("decimal/payload typed record/port plan")
	}
	steps, ok := c["steps"].([]any)
	if !ok || len(steps) != 1 {
		return fmt.Errorf("decimal/payload generation step")
	}
	step, ok := steps[0].(map[string]any)
	if !ok || step["input_source_sha256"] != sourceSHA {
		return fmt.Errorf("decimal/payload generation source")
	}
	generation, err := decimalPayloadReceiptObject(step, "generation")
	if err != nil {
		return err
	}
	g, err := decimalPayloadReceiptObject(generation, "report")
	if err != nil {
		return err
	}
	generated, ok := generation["source"].(string)
	inputType := goTypes[0]
	if consumer {
		inputType = "(" + strings.Join(goTypes, ",") + ")"
	}
	if !ok || generated == "" || g["schema"] != "gooo/body-codegen-report/v3" || g["decision"] != "PASS" || g["activity"] != lowered || g["activity_id"] != contract.ID || g["compiler_source_sha"] != identity.CompilerRevision || g["source_digest"] != sourceSHA || g["program_digest"] != contentDigest([]byte(rootBody)) || g["generated_digest"] != contentDigest([]byte(generated)) || g["replay_digest"] != g["generated_digest"] || g["input_type"] != inputType || g["output_type"] != outputName || !decimalPayloadSame(g["record_types"], records) || g["typecheck_passed"] != true || g["deterministic_replay"] != true || !decimalPayloadSame(g["repository_writes"], 0) {
		return fmt.Errorf("decimal/payload generated source/record ABI")
	}
	if consumer {
		parameters := []any{}
		for i, kind := range goTypes {
			parameters = append(parameters, map[string]any{"name": fmt.Sprintf("input%d", i), "type": kind})
		}
		closure := map[string]any{"schema": "gooo/pure-activity-call-closure/v1", "activities": []any{map[string]any{"name": "GoooPackage0ActivityParseIntDecimal64", "activity_id": parserContract.ID, "program_sha256": contentDigest([]byte(parserBody.Body))}}, "edges": []any{map[string]any{"caller_id": contract.ID, "callee_id": parserContract.ID, "ordinal": 0}}, "max_calls_per_invocation": 1}
		if !decimalPayloadSame(g["input_parameters"], parameters) || !decimalPayloadSame(g["call_closure"], closure) {
			return fmt.Errorf("payload compiled callee/parameter ABI")
		}
	} else if g["input_parameters"] != nil || g["call_closure"] != nil {
		return fmt.Errorf("decimal unexpected parameter/callee metadata")
	}
	driver, ok := c["driver_source"].(string)
	if !ok || driver == "" || c["driver_sha256"] != contentDigest([]byte(driver)) || runtime["driver_sha256"] != c["driver_sha256"] || runtime["typed_plan_sha256"] != plan["typed_plan_sha256"] || runtime["original_source_sha256"] != sourceSHA || runtime["selected_source_sha256"] != sourceSHA {
		return fmt.Errorf("decimal/payload runtime source/driver/plan binding")
	}
	planSHA, ok := plan["typed_plan_sha256"].(string)
	if !ok || !validDigest(planSHA) {
		return fmt.Errorf("decimal/payload typed plan digest")
	}
	if consumer {
		image, err := decimalPayloadReceiptObject(p, "workspace_image")
		if err != nil {
			return err
		}
		packages, ok := image["packages"].([]any)
		if !ok {
			return fmt.Errorf("payload package ABI")
		}
		found := 0
		for _, entry := range packages {
			pkg, ok := entry.(map[string]any)
			if !ok {
				return fmt.Errorf("payload package ABI")
			}
			if pkg["path"] != decimalPackage {
				continue
			}
			exports, ok := pkg["exports"].([]any)
			if !ok {
				return fmt.Errorf("payload callee exports")
			}
			for _, entry := range exports {
				export, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("payload callee export ABI")
				}
				if export["name"] == decimalActivity || export["id"] == parserContract.ID {
					found++
					if export["name"] != decimalActivity || export["kind"] != "activity" || export["id"] != parserContract.ID || !decimalPayloadSame(export["input_types"], []string{textEntity}) || export["output_type"] != decimalResultEntity {
						return fmt.Errorf("payload source callee export ABI")
					}
				}
			}
		}
		if found != 1 {
			return fmt.Errorf("payload callee export count")
		}
	}
	return nil
}
