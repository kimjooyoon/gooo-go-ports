package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDragCropEvidenceAndOriginalModeRemainDistinct(t *testing.T) {
	raw, lock, manifest, inputs, sources := cropProgramEvidenceFixture(t, dragCropProgram)
	got, err := inspectCropProgramReceipt(raw, lock, manifest, inputs, sources, "", dragCropProgram)
	if err != nil || got == "" {
		t.Fatal("valid drag evidence", err)
	}
	if _, err := inspectCropReceipt(raw, lock, manifest, inputs, sources, ""); err == nil {
		t.Fatal("drag evidence accepted as original mode")
	}
	v, _ := cropDecode(raw)
	e := cropMap(v)
	e["replayed_from_sha256"] = "sha256:" + digest(raw)
	cropMap(e["result"])["replay"] = map[string]any{"model_calls": 0}
	if replayed, err := inspectCropProgramReceipt(testJSON(t, e), lock, manifest, inputs, sources, "sha256:"+digest(raw), dragCropProgram); err != nil || replayed != got {
		t.Fatal("saved drag replay", err)
	}
}

func TestDragCropEvidenceRejectsWrongModeNormalizationAndSources(t *testing.T) {
	raw, lock, manifest, inputs, sources := cropProgramEvidenceFixture(t, dragCropProgram)
	for _, mutation := range []string{"entry", "source omitted", "source swapped", "source digest", "export alias", "old output", "normalized input", "rounded input"} {
		t.Run(mutation, func(t *testing.T) {
			v, _ := cropDecode(raw)
			e := cropMap(v)
			r := cropMap(e["result"])
			p := cropMap(r["program"])
			packages := cropList(cropMap(p["workspace_image"])["packages"])
			var pkg map[string]any
			for _, value := range packages {
				if cropMap(value)["path"] == "go/image" {
					pkg = cropMap(value)
				}
			}
			sourceRows := cropList(pkg["sources"])
			traces := cropList(cropMap(r["runtime"])["traces"])
			delivery := func(i int) map[string]any { return cropMap(cropList(cropMap(traces[i])["deliveries"])[0]) }
			switch mutation {
			case "entry":
				cropMap(p["entry"])["activity"] = "CropPlan"
			case "source omitted":
				pkg["sources"] = sourceRows[:1]
			case "source swapped":
				sourceRows[0], sourceRows[1] = sourceRows[1], sourceRows[0]
			case "source digest":
				cropMap(sourceRows[1])["source_digest"] = "sha256:wrong"
			case "export alias":
				for _, value := range packages {
					q := cropMap(value)
					if q["path"] == "example/crop-planner" {
						cropMap(cropList(q["exports"])[0])["name"] = "CropPlan"
					}
				}
			case "old output":
				want := map[string]any{"min_x": 0, "min_y": 0, "max_x": 0, "max_y": 0, "has_pixels": false}
				delivery(3)["actual"] = want
				delivery(3)["actual_fields"] = cropFields(cropResultID, append(append([]string(nil), cropCoordinates...), "has_pixels"), want)
			case "normalized input":
				record := cropMap(cropList(delivery(3)["inputs"])[1])
				value := map[string]any{"min_x": 100, "min_y": 100, "max_x": 200, "max_y": 200}
				record["value"] = value
				record["fields"] = cropFields(cropRectangleID, cropCoordinates, value)
			case "rounded input":
				record := cropMap(cropList(delivery(11)["inputs"])[1])
				cropMap(record["value"])["max_x"] = json.Number("9007199254740992")
			}
			if _, err := inspectCropProgramReceipt(testJSON(t, e), lock, manifest, inputs, sources, "", dragCropProgram); err == nil {
				t.Fatal("accepted", mutation)
			}
		})
	}
}

func TestDragCropReferenceUsesOriginalGoCanonWithoutNormalizingBounds(t *testing.T) {
	rows, err := cropProgramInputRows(cropTestRead(t, dragCropProgram.inputs), dragCropProgram)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := int64(-1<<63), int64(1<<63-1)
	want := [][4]int64{{100, 100, 200, 200}, {100, 100, 200, 200}, {100, 100, 200, 200}, {100, 100, 200, 200}, {0, 40, 200, 480}, {}, {}, {}, {-8, -30, 5, 4}, {hi - 1, hi - 1, hi, hi}, {lo, lo, lo + 1, lo + 1}, {9007199254740993, -9007199254740992, 9007199254740994, -9007199254740991}}
	for i, row := range rows {
		result := cropProgramOracle(row, dragCropProgram)
		for j, field := range cropCoordinates {
			if result[field] != want[i][j] {
				t.Fatal(i, field, result)
			}
		}
		if result["has_pixels"] != (i < 5 || i > 7) {
			t.Fatal("has_pixels", i, result)
		}
	}
	if cropOracle(rows[3])["has_pixels"] != false {
		t.Fatal("original ordered mode changed")
	}
	t.Chdir("../..")
	var lock lockfile
	if err := readJSON("upstream.json", &lock); err != nil {
		t.Fatal(err)
	}
	a, err := makeCropProgramFixtures(rows, lock, dragCropProgram)
	if err != nil {
		t.Fatal(err)
	}
	b, err := makeCropProgramFixtures(rows, lock, dragCropProgram)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic", err)
	}
	if len(a.Cases) != 88 || len(a.Rejections) != 96 {
		t.Fatal("fixture roster changed", len(a.Cases), len(a.Rejections))
	}
}
