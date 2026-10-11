package main

import (
	"encoding/json"
	"fmt"
)

type recordFieldValue struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type recordFieldContract struct{ Name, ID string }
type resultContract struct {
	Entity string
	Fields []recordFieldContract
}

// Result expectations are fixed from reviewed declarations, not receipt metadata.
// Existing activities retain their contracts; these new result identities also
// bind the record's semantic field IDs, order and values to independent Go output.
func whitespaceResultContract(s suite) resultContract {
	if geometry := geometryResultContract(s); geometry.Entity != "" {
		return geometry
	}
	switch s.Package + ":" + s.Activity {
	case "go/image/color:RGBToYCbCr":
		const entity = "gooo://go/image/color/ycbcr"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"y", entity + "/y"}, {"cb", entity + "/cb"}, {"cr", entity + "/cr"},
		}}
	case "go/image/color:YCbCrToRGB":
		const entity = "gooo://go/image/color/rgb"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"red", entity + "/red"}, {"green", entity + "/green"}, {"blue", entity + "/blue"},
		}}
	case "app/ycbcr-inspector:InspectRGB":
		const entity = "gooo://go-ports/examples/ycbcr-inspector/inspection"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"in_byte_domain", entity + "/in-byte-domain"}, {"y", entity + "/y"},
			{"cb", entity + "/cb"}, {"cr", entity + "/cr"},
			{"red", entity + "/red"}, {"green", entity + "/green"}, {"blue", entity + "/blue"},
			{"red_delta", entity + "/red-delta"}, {"green_delta", entity + "/green-delta"}, {"blue_delta", entity + "/blue-delta"},
			{"exact", entity + "/exact"},
		}}
	case "go/unicode:SimpleFold":
		return resultContract{Entity: integerEntity}
	case "app/simple-fold:Compare":
		const entity = "gooo://go-ports/examples/simple-fold/comparison"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"in_rune_domain", entity + "/in-rune-domain"}, {"equivalent", entity + "/equivalent"},
			{"left_next", entity + "/left-next"}, {"right_next", entity + "/right-next"},
		}}
	case "go/unicode:IsSpace", "go/unicode:IsLetter", "go/unicode:IsDigit":
		return resultContract{Entity: "gooo://std/boolean"}
	case "app/identifier-rune:Classify":
		const entity = "gooo://go-ports/examples/identifier-rune/classification"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"in_rune_domain", entity + "/in-rune-domain"}, {"can_start", entity + "/can-start"},
			{"can_continue", entity + "/can-continue"}, {"is_decimal_digit", entity + "/is-decimal-digit"},
		}}
	case "app/whitespace-inspector:Inspect":
		const entity = "gooo://go-ports/examples/whitespace-inspector/inspection"
		return resultContract{Entity: entity, Fields: []recordFieldContract{
			{"in_rune_domain", entity + "/in-rune-domain"}, {"valid_scalar", entity + "/valid-scalar"},
			{"is_go_space", entity + "/is-go-space"}, {"is_ascii_space", entity + "/is-ascii-space"},
			{"utf8_bytes", entity + "/utf8-bytes"}, {"utf16_units", entity + "/utf16-units"},
		}}
	}
	return resultContract{}
}

func inspectWhitespaceFields(s suite, fields []recordFieldValue, expected any) error {
	contract := whitespaceResultContract(s)
	if contract.Entity == "" {
		return nil
	}
	if len(fields) != len(contract.Fields) {
		return fmt.Errorf("record field roster")
	}
	if len(fields) == 0 {
		return nil
	}
	want, ok := expected.(map[string]any)
	if !ok || len(want) != len(contract.Fields) {
		return fmt.Errorf("record oracle shape")
	}
	for i, field := range fields {
		bound := contract.Fields[i]
		value, exists := want[bound.Name]
		if !exists || field.ID != bound.ID || field.Name != bound.Name || !equalValue(field.Value, value) {
			return fmt.Errorf("record field identity, order or value")
		}
	}
	return nil
}
