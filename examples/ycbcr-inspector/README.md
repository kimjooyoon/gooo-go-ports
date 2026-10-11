# RGB → Y′CbCr → RGB inspector

This native Gooo example imports the ported Go `image/color.RGBToYCbCr` and
`YCbCrToRGB` functions. It returns the intermediate Y′CbCr channels, recovered
RGB values, and each channel's integer difference. Use it to reproduce the
pinned Go conversion when investigating color conversion or quantization.

The functions preserve the full three-channel `uint8` input domain. They follow
Go's JFIF conversion, including its integer rounding and saturation. Other
video matrices, limited-range video levels, chroma subsampling, JPEG compression,
ICC profiles and image file processing are outside this example's scope.

## Run and change the inputs

Use Go 1.27.2, Git and a POSIX shell on macOS or Linux, from the repository root:

```sh
./tools/bootstrap.sh
mkdir -p out/ycbcr-demo
./.gooo package execute --json --inputs examples/ycbcr-inspector/inputs.json \
  ycbcr-inspector.workspace.json > out/ycbcr-demo/execution.json
./.gooo package replay --json --receipt out/ycbcr-demo/execution.json \
  --inputs examples/ycbcr-inspector/inputs.json \
  ycbcr-inspector.workspace.json > out/ycbcr-demo/replay.json
python3 tools/check-ycbcr-example.py out/ycbcr-demo/execution.json out/ycbcr-demo/replay.json
```

The last command also requires Python 3 and checks the twelve unchanged sample
inputs. After editing [inputs.json](inputs.json), run execute and replay again
to observe your own inputs; the fixed sample checker is for the supplied file.
The keys ending in `input0`, `input1`, and `input2` are red, green, and blue.
Each is a decimal integer from 0 to 255. Results are in
`result.runtime.traces[].deliveries[].actual` in the saved JSON records.
Input-only execution reports `OBSERVED`; it does not compare expected results.

## Read the result

| Field | Meaning |
| --- | --- |
| `in_byte_domain` | Whether every input is in 0..255 |
| `y`, `cb`, `cr` | Intermediate Y′ luma and two chroma channels, each 0..255 |
| `red`, `green`, `blue` | RGB recovered from that Y′CbCr triple |
| `red_delta`, `green_delta`, `blue_delta` | Recovered value minus the original input |
| `exact` | Whether all three recovered integer values equal the input |

Named sample results, extracted from the JSON records:

| Original RGB | Y′CbCr | Recovered RGB | Difference |
| --- | --- | --- | --- |
| `(0, 0, 0)` | `(0, 128, 128)` | `(0, 0, 0)` | `(0, 0, 0)` |
| `(255, 255, 255)` | `(255, 128, 128)` | `(255, 255, 255)` | `(0, 0, 0)` |
| `(255, 0, 0)` | `(76, 85, 255)` | `(254, 0, 0)` | `(-1, 0, 0)` |
| `(0, 255, 0)` | `(150, 44, 21)` | `(0, 255, 1)` | `(0, 0, 1)` |
| `(0, 0, 255)` | `(29, 255, 107)` | `(0, 0, 254)` | `(0, 0, -1)` |
| `(142, 83, 202)` | `(114, 178, 148)` | `(142, 82, 203)` | `(0, -1, 1)` |

`exact` compares integer channels. It does not measure perceived color quality.
The conversion can increase one channel and decrease another. A zero difference
for a sample is not a guarantee of lossless image conversion.

Out-of-byte inputs preserve their original integer values and are rejected before
arithmetic. Both port functions return all `-1` fields. The inspector returns
`in_byte_domain=false`, `exact=false`, and `-1` in every integer field.
This is an explicit Gooo extension, counted separately from Go equivalence;
inputs are never silently converted or wrapped to a byte.

## Import the ports in another program

Copy both [color.gooo](../../ports/image/color.gooo) and
[ycbcr.gooo](../../ports/image/ycbcr.gooo) from the same fixed repository commit,
along with `LICENSE`, `NOTICE`, `third_party/GO-LICENSE` and `upstream.json`.
Register both source files under the `go/image/color` package in your workspace.
`color.gooo` supplies the shared Integer and RGB declarations.
The [example workspace](../../ycbcr-inspector.workspace.json) shows the complete
two-package arrangement; `import` does not download dependencies.

The Gooo signatures are:

```gooo
activity RGBToYCbCr(Integer, Integer, Integer) -> YCbCr
activity YCbCrToRGB(Integer, Integer, Integer) -> RGB
```

These lines describe the API; the runnable declarations and their IDs are in
the linked port source. Three Go return values become one named Gooo record.
All conversion and round-trip calculations in [main.gooo](main.gooo) run in Gooo.

## Preserve the fixed-point rules

The source is Go 1.27.2
[ycbcr.go at the pinned revision](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/color/ycbcr.go).
Its SHA-256 and BSD-3-Clause attribution are preserved in the source and
[upstream.json](../../upstream.json). This is an independent experimental port.

RGB→Y′CbCr retains Go's integer coefficients and offsets. Its numerators are
nonnegative for valid byte inputs, so division by 65,536 preserves `>>16`.
Cb and Cr saturate to 255 where the computed value reaches 256.

Y′CbCr→RGB uses `Y′ × 65793` and chroma offsets of 128. The factor includes
Go's `257 × Y′` rounding adjustment; replacing it with a constant 32,768 changes
results. A negative channel numerator saturates to 0, a numerator above
16,777,215 saturates to 255, and only the remaining nonnegative interval is
divided by 65,536. All intermediate values fit signed 64-bit integers.

## Reproduce the comparisons

```sh
go run ./cmd/verify --out out/ycbcr-check
```

Use a fresh output directory. The verifier calls the pinned Go standard library
for independent expected results and saves Gooo execution and replay records.
Boundary grids, every gray value, rounding and clipping transitions, and fixed
random samples exercise both functions and their importing consumer. Invalid
integer inputs are counted separately and never passed through Go byte casts.

The same command also checks each unchanged generated conversion function on
all **16,777,216** valid triples, for **33,554,432** direct function comparisons.
This exhaustive check is separate from the batched receipt counts and from the
consumer's finite round-trip cases. Its report binds the generated source digest
and pinned Go version. It does not establish whole-package compatibility,
cross-platform equivalence, performance parity, or correctness of an image codec.
The [saved observation](../../evidence/ycbcr-20261011.json) records the source
digests, finite case counts, replay results and both exhaustive reports.

Gooo 계산으로 RGB→Y′CbCr→RGB의 채널 변화와 정확한 정수 결과를 확인하는 예제입니다.
입력은 원본 Go의 전체 byte 범위를 보존하며, 범위 밖 거부와 원본 비교를 별도로 집계합니다.
현재 이 예제는 위 native 명령으로 실행합니다.
