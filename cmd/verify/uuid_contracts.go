package main

const uuidPackage = "go/github.com/google/uuid"
const uuidConsumerPackage = "app/uuid-format-inspector"
const uuidResultEntity = "gooo://github.com/google/uuid/validation-result"
const uuidReportEntity = "gooo://go-ports/examples/uuid-format-inspector/report"

func isUUIDSuite(s suite) bool {
	return (s.Package == uuidPackage && s.Activity == "Validate") ||
		(s.Package == uuidConsumerPackage && s.Activity == "Inspect")
}

// Trusted result IDs and ordered fields come from reviewed declarations,
// independently of the receipt's export and field metadata.
func uuidResultContract(s suite) resultContract {
	entity := ""
	switch s.Package + ":" + s.Activity {
	case uuidPackage + ":Validate":
		entity = uuidResultEntity
	case uuidConsumerPackage + ":Inspect":
		entity = uuidReportEntity
	default:
		return resultContract{}
	}
	fields := []recordFieldContract{{"valid", entity + "/valid"}, {"error", entity + "/error"}, {"invalid_length", entity + "/invalid-length"}}
	if s.Package == uuidConsumerPackage {
		fields = append(fields, recordFieldContract{"byte_length", entity + "/byte-length"})
	}
	return resultContract{Entity: entity, Fields: fields}
}
