package main

import "fmt"

const rectangleEntity = "gooo://go/image/rectangle"
const cropPlanEntity = "gooo://example/crop-planner/result"

func rectangleFieldContracts(entity string) []recordFieldContract {
	return []recordFieldContract{{"min_x", entity + "/min-x"}, {"min_y", entity + "/min-y"},
		{"max_x", entity + "/max-x"}, {"max_y", entity + "/max-y"}}
}

func geometryResultContract(s suite) resultContract {
	switch s.Package + ":" + s.Activity {
	case "go/image:RectangleEmpty", "go/image:RectangleOverlaps", "go/image:RectangleIn", "example/fits-crop:FitsCrop":
		return resultContract{Entity: "gooo://std/boolean"}
	case "go/image:RectangleIntersect", "go/image:RectangleUnion", "go/image:RectangleCanon":
		return resultContract{Entity: rectangleEntity, Fields: rectangleFieldContracts(rectangleEntity)}
	case "example/crop-planner:CropPlan", "example/crop-planner:DragCropPlan":
		return resultContract{Entity: cropPlanEntity, Fields: append(rectangleFieldContracts(cropPlanEntity), recordFieldContract{"has_pixels", cropPlanEntity + "/has-pixels"})}
	}
	return resultContract{}
}

func inspectRectangleInputFields(entity string, fields []recordFieldValue, expected any) error {
	if entity != rectangleEntity {
		return nil
	}
	r, err := readRectangle(expected)
	if err != nil {
		return err
	}
	contract := rectangleFieldContracts(rectangleEntity)
	if len(fields) != len(contract) {
		return fmt.Errorf("Rectangle input field roster")
	}
	for i, field := range fields {
		if field.Name != contract[i].Name || field.ID != contract[i].ID || !equalValue(field.Value, r[i]) {
			return fmt.Errorf("Rectangle input field identity, order or exact value")
		}
	}
	return nil
}
