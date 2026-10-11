package main

import "fmt"

const nrgbaEntity = "gooo://go/image/color/nrgba"
const nrgbaRGBEntity = "gooo://go/image/color/rgb"
const nrgbaRGBA16Entity = "gooo://go/image/color/rgba16"
const nrgbaMatteEntity = "gooo://go-ports/examples/alpha-matte/result"

func nrgbaFieldContracts(entity string) []recordFieldContract {
	names := nrgbaChannelNames
	if entity == nrgbaRGBEntity {
		names = nrgbaRGBNames
	}
	var result []recordFieldContract
	for _, name := range names {
		result = append(result, recordFieldContract{name, entity + "/" + name})
	}
	return result
}
func nrgbaResultContract(s suite) resultContract {
	switch s.Package + ":" + s.Activity {
	case "go/image/color:NRGBAToRGBA":
		return resultContract{Entity: nrgbaRGBA16Entity, Fields: nrgbaFieldContracts(nrgbaRGBA16Entity)}
	case "app/alpha-matte:MattePixel":
		e := nrgbaMatteEntity
		return resultContract{Entity: e, Fields: []recordFieldContract{{"in_byte_domain", e + "/in-byte-domain"}, {"premultiplied_red", e + "/premultiplied-red"}, {"premultiplied_green", e + "/premultiplied-green"}, {"premultiplied_blue", e + "/premultiplied-blue"}, {"premultiplied_alpha", e + "/premultiplied-alpha"}, {"red", e + "/red"}, {"green", e + "/green"}, {"blue", e + "/blue"}}}
	}
	return resultContract{}
}
func inspectNRGBAInputFields(entity string, fields []recordFieldValue, expected any) error {
	if entity != nrgbaEntity && entity != nrgbaRGBEntity {
		return nil
	}
	names := nrgbaChannelNames
	if entity == nrgbaRGBEntity {
		names = nrgbaRGBNames
	}
	values, err := readNRGBARecord(expected, names)
	if err != nil {
		return err
	}
	contract := nrgbaFieldContracts(entity)
	if len(fields) != len(contract) {
		return fmt.Errorf("color input field roster")
	}
	for i, field := range fields {
		if field.Name != contract[i].Name || field.ID != contract[i].ID || !equalValue(field.Value, values[i]) {
			return fmt.Errorf("color input field identity, order or exact int64 value")
		}
	}
	return nil
}
