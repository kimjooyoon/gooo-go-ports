package main

// Preserve the established row limits for all existing suites.
func nativeBatchSize(s suite) int {
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
	return batchSize
}
