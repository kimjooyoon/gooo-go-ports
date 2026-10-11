# Direction-independent drag selection crop planner

This experimental program computes a crop rectangle from two drag corners.
In the [browser crop planner](https://kimjooyoon.github.io/gooo-go-ports/crop-planner.html),
choose **Two-corner selection** to run the compiled `DragCropPlan` activity.
For example, (200,200) to (100,100) selects the same 100..200 region as
(100,100) to (200,200), subject to the supplied bounds. The default **Keep rectangle
order** mode continues to run the existing `CropPlan` activity.

`DragCropPlan` accepts image bounds and the two corners of a drag selection. It
normalizes only the selection with `RectangleCanon`, then uses the existing
`RectangleIntersect` and `RectangleEmpty` activities. Dragging left, right, up or
down therefore selects the same region when the two corners are the same.

The existing `CropPlan` activity is unchanged: it continues to treat reversed
requests as empty. `DragCropPlan` also leaves the supplied image bounds unchanged;
reversed or degenerate bounds remain empty. Choose the new activity when the
second rectangle represents two drag corners that need to be ordered.

This is a bounds calculation. It does not open, crop, resize or save an image,
and `has_pixels` does not prove that image data exists or that its dimensions
can be allocated. No width, height, area or coordinate difference is calculated.

## Run the native example

Run from the repository root with Git, a POSIX shell, Python 3 and Go
1.27.2 on amd64 or arm64. The compiler remains pinned to
`83928fa4538245abe33b08549da47f8cbdc50ed2` in `upstream.json`.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/drag-crop
./.gooo package execute --json --inputs examples/crop-planner/drag_inputs.json \
  drag-crop.workspace.json > out/drag-crop/execution.json
./.gooo package replay --json --receipt out/drag-crop/execution.json \
  --inputs examples/crop-planner/drag_inputs.json \
  drag-crop.workspace.json > out/drag-crop/replay.json
python3 tools/check-drag-crop-example.py \
  out/drag-crop/execution.json out/drag-crop/replay.json
```

The [saved differential summary](../evidence/rectangle-canon-20261011.json) records
371 Canon and 501 DragCropPlan comparisons within the 35,042-case native run;
every batch was executed and replayed. The input-only twelve-row example is
separate from those counts. Its native receipt says `OBSERVED`; the example checker
compares its twelve outputs with the literal answers below and checks saved
replay. It does not turn them into a claim of exhaustive equivalence.

## Inputs and exact coordinates

[drag_inputs.json](../examples/crop-planner/drag_inputs.json) contains inputs only.
`example/crop-planner:DragCropPlan.input0` is the original image bounds;
`example/crop-planner:DragCropPlan.input1` is the unnormalized drag selection.
Both use this record shape:

```json
{"min_x": 200, "min_y": 200, "max_x": 100, "max_y": 100}
```

For the selection, these names identify the two corners before normalization;
the first corner need not be smaller. `RectangleCanon` independently exchanges
the X pair and Y pair when reversed. Equal coordinates remain equal, so a zero
width or height remains empty. After normalization, the minimum is included and
the maximum is excluded. Boundary contact alone contains no pixels.

All four fields are exact signed 64-bit integers from −9223372036854775808 to
9223372036854775807. This corresponds to the full `int` domain of Go on amd64 and
arm64; 32-bit Go is outside this port's scope. Keep JSON values as integers, with
no float or JavaScript `Number` conversion. For example, `9007199254740993` must
remain exact. Bounds and selection reach the Gooo activity unchanged; adapters
must not call `image.Rect` or normalize either input in advance.

## Twelve documented results

Coordinates use `(min_x,min_y)–(max_x,max_y)`. Rows follow the input file exactly.
The first seven rows use bounds `(0,0)–(640,480)`.

| Row | Selection or situation | Result | `has_pixels` |
| --- | --- | --- | --- |
| 1 | `(100,100)–(200,200)`, ordered | `(100,100)–(200,200)` | true |
| 2 | `(200,100)–(100,200)`, X reversed | `(100,100)–(200,200)` | true |
| 3 | `(100,200)–(200,100)`, Y reversed | `(100,100)–(200,200)` | true |
| 4 | `(200,200)–(100,100)`, both reversed | `(100,100)–(200,200)` | true |
| 5 | `(200,550)–(-20,40)`, reverse and clip | `(0,40)–(200,480)` | true |
| 6 | `(7,20)–(7,9)`, zero width | `(0,0)–(0,0)` | false |
| 7 | `(800,100)–(640,0)`, right boundary contact | `(0,0)–(0,0)` | false |
| 8 | Reversed bounds `(640,0)–(0,480)`; selection `(30,30)–(10,10)` | `(0,0)–(0,0)` | false |
| 9 | Bounds `(-20,-30)–(5,10)`; selection `(12,4)–(-8,-40)` | `(-8,-30)–(5,4)` | true |
| 10 | Full int64 bounds; reversed selection at the maximum | `(9223372036854775806,9223372036854775806)–(9223372036854775807,9223372036854775807)` | true |
| 11 | Full int64 bounds; reversed selection at the minimum | `(-9223372036854775808,-9223372036854775808)–(-9223372036854775807,-9223372036854775807)` | true |
| 12 | Reversed selection and clipping beyond ±2⁵³ | `(9007199254740993,-9007199254740992)–(9007199254740994,-9007199254740991)` | true |

Every empty intersection returns all four coordinates as zero. The checker reads
the actual output at `result.runtime.traces[].deliveries[].actual`, preserves
exact int64 values and checks field IDs, original inputs, entry/export IDs, all
four source digests, pinned compiler and Go identities, generated source and the
saved replay's binding to the exact execution bytes. It rejects duplicate decoded
JSON keys, including in unknown nested fields.

The checker is for the unchanged twelve documented inputs. After editing inputs,
generate new execution and replay records and establish independent expectations;
do not reuse the fixed table as validation for a different task. Its regression
tests use synthetic receipts and do not count as native language execution:

```sh
python3 tools/check-drag-crop-example_test.py
```

## Source and verification scope

[canon.gooo](../ports/image/canon.gooo) implements `image.Rectangle.Canon` as
`go/image:RectangleCanon`, using the existing flat Rectangle record.
[drag_crop.gooo](../examples/crop-planner/drag_crop.gooo) combines that port with
the existing geometry activities and returns the existing `CropResult` record.
The [dedicated manifest](../drag-crop.workspace.json) includes these two files,
`geometry.gooo` and the unchanged `crop_planner.gooo`.

The source method is Go 1.27.2's
[`Rectangle.Canon`](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/geom.go#L231).
The immutable revision and source hash are recorded in
[upstream.json](../upstream.json); the Go authors' copyright and BSD license are
preserved in the port and [GO-LICENSE](../third_party/GO-LICENSE).

The independent Go reference constructs `image.Rectangle` and `image.Point`
directly from the original fields, then calls `selection.Canon()` and
`bounds.Intersect(...)`. It must not use `image.Rect`, which would normalize an
input before the tested method. Differential cases cover reversals, degeneracy,
boundary contact, negative coordinates, int64 extremes and precision boundaries.
The original `CropPlan` cases remain separate to detect accidental behavior
changes. These are finite observations, not an exhaustive test of all coordinate
combinations or evidence of image-file processing or established user demand.

## Browser build

The two modes use separate WebAssembly modules. `crop/crop.wasm` runs the original
`CropPlan`; `drag-crop/crop.wasm` runs `DragCropPlan`. Each is built from the exact
Go source in its checked native execution receipt. JavaScript keeps all eight
input coordinates as decimal strings and does not implement corner ordering or
intersection. Only the selected module is loaded; changing modes clears the old
result. No image is uploaded or processed.

From a checkout with the pinned compiler and Go toolchain:

```sh
go run ./cmd/build-demo --out out/drag-crop-browser
node tools/drag-crop-browser-smoke.cjs out/drag-crop-browser/site
python3 -m http.server 8766 --bind 127.0.0.1 --directory out/drag-crop-browser/site
```

Open `http://127.0.0.1:8766/crop-planner.html`. Use a fresh build output directory.
Each mode has 88 independent Go-reference fixtures and 210 rejected string/type/
arity calls. Seven further paired inputs check that both actual Wasm programs
coexist and keep their distinct reversed-selection behavior, including int64
endpoints and values beyond JavaScript's exact Number range. These finite browser
observations are separate from the native verifier and the 21-function demo.
