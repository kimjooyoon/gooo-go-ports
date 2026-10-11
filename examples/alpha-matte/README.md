# Flatten one transparent pixel onto a background

This native Gooo example imports the ported Go `color.NRGBA.RGBA` method and
combines one foreground pixel with an opaque background color. It returns the
premultiplied 16-bit channels and the final 8-bit RGB channels, making the
conversion and integer rounding visible when preparing a color for an opaque
destination.

The foreground has four independent `uint8` channels: red, green, blue and
alpha. The background has three `uint8` RGB channels and implicit alpha 255.
Every channel can span 0..255. A foreground's RGB values can exceed alpha;
even `(255, 0, 0, 0)` is valid and must preserve its original input values.

This example calculates one pixel. It does not decode PNG files, traverse
images, perform gamma correction, or implement the general `image/draw` API.
Its arithmetic follows Go's encoded-channel values and truncation rules.

## Execute and replay

Run from the repository root with Git, a POSIX shell and Go toolchain selection
enabled:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/alpha-matte-demo
./.gooo package execute --json --inputs examples/alpha-matte/inputs.json \
  alpha-matte.workspace.json > out/alpha-matte-demo/execution.json
./.gooo package replay --json --receipt out/alpha-matte-demo/execution.json \
  --inputs examples/alpha-matte/inputs.json alpha-matte.workspace.json \
  > out/alpha-matte-demo/replay.json
```

The compiler pin is in [upstream.json](../../upstream.json). Edit
[inputs.json](inputs.json) to supply your own records. `input0` is the NRGBA
foreground; `input1` is the opaque RGB background. For example:

```json
{
  "app/alpha-matte:MattePixel.input0": {
    "red": 255, "green": 0, "blue": 0, "alpha": 128
  },
  "app/alpha-matte:MattePixel.input1": {
    "red": 0, "green": 0, "blue": 255
  }
}
```

Read `result.runtime.traces[].deliveries[].actual` in the saved execution.
This half-alpha red pixel on blue returns:

```json
{
  "in_byte_domain": true,
  "premultiplied_red": 32896,
  "premultiplied_green": 0,
  "premultiplied_blue": 0,
  "premultiplied_alpha": 32896,
  "red": 128,
  "green": 0,
  "blue": 127
}
```

The final result is opaque, so its alpha is always 255 when `in_byte_domain`
is true. Only the final RGB channels are returned. Intermediate channels
describe the foreground and span 0..65535, including its alpha.

The 24 supplied rows contain 18 valid original-domain examples and six
out-of-byte extension examples. They have no expected outputs, so execution
reports `OBSERVED`: a record of execution, not an independent Go comparison.
Saved replay reproduces that execution. To run the independent reference
checks for the unchanged supplied rows and validate their receipt bindings,
use the Python 3 checker:

```sh
python3 tools/check-alpha-matte.py out/alpha-matte-demo/execution.json \
  out/alpha-matte-demo/replay.json
```

It compares literal pinned-Go answers and deliberately requires the documented
inputs. Your edited input-only rows remain observations unless compared with
an independent reference. For the repository's separate fixed reference corpus,
choose a fresh output directory:

```sh
go run ./cmd/verify --out out/alpha-matte-reference
```

## Read the rounding and transparency

Selected supplied examples:

| Foreground RGBA | Background RGB | Premultiplied RGBA16 | Final RGB |
| --- | --- | --- | --- |
| `(255,0,0,0)` | `(10,20,30)` | `(0,0,0,0)` | `(10,20,30)` |
| `(13,77,201,255)` | `(201,77,13)` | `(3341,19789,51657,65535)` | `(13,77,201)` |
| `(255,0,0,128)` | `(0,0,255)` | `(32896,0,0,32896)` | `(128,0,127)` |
| `(255,0,0,127)` | `(0,0,255)` | `(32639,0,0,32639)` | `(127,0,128)` |
| `(255,255,255,1)` | `(0,0,0)` | `(257,257,257,257)` | `(1,1,1)` |
| `(1,0,0,128)` | `(0,0,0)` | `(129,0,0,32896)` | `(0,0,0)` |

For a foreground channel `c` and alpha `A`, the port computes
`c * 257 * A / 255`; the output alpha is `A * 257`. Expanding to 16 bits before
division is necessary. For `c=1, A=128`, the correct intermediate is 129;
dividing at 8-bit precision first would lose that value.

The consumer calls the port, then computes `inverse = (65535 - p.alpha) * 257`.
Each final channel is `(background * inverse / 65535 + p.channel) / 256`.
Both divisions truncate nonnegative integers, preserving the rounding for
Go's 8-bit RGBA destination. The largest product is 4,294,836,225, well inside
signed int64. No floating-point conversion is used.

The independent consumer oracle uses `draw.Draw` with a one-pixel
`*image.NRGBA` source, an opaque `*image.RGBA` destination and `draw.Over`.
It also compares the generic image-interface path. A finite agreement does
not establish compatibility for arbitrary image types, masks or draw operators.

## Domains and reuse

The port signature is `NRGBAToRGBA(NRGBA) -> RGBA16`: the original Go receiver
stays one record. Its four Go `uint32` return values become four required
Integer fields, ordered red, green, blue, alpha. The `RGBA16` name describes
their 16-bit value range. The consumer signature is
`MattePixel(NRGBA, RGB) -> MatteResult`.

Gooo Integer can represent values outside the original byte domain. The port
checks the four original values before arithmetic and returns all four fields
as `-1` for such inputs. The consumer checks all seven foreground/background
values first, returning `in_byte_domain=false` and `-1` in all seven numeric
fields. These are explicit extension results, counted separately from Go
equivalence; the original values are never wrapped, clamped or cast to uint8.

Copy [nrgba.gooo](../../ports/image/nrgba.gooo) and
[color.gooo](../../ports/image/color.gooo) from the same fixed repository
revision. Keep their copyright headers, `LICENSE`, `NOTICE`,
`third_party/GO-LICENSE` and `upstream.json`. Register both under
`go/image/color`; `color.gooo` supplies the existing RGB and Integer declarations.
The [workspace](../../alpha-matte.workspace.json) and [consumer](main.gooo)
show the complete import arrangement. Imports resolve registered local files.

The original method is Go 1.27.2
[color.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/color/color.go#L55)
(Copyright 2011 The Go Authors), SHA-256
`03511ed250d0b4dd20605d5dc3465164e559033a6ff7e5cd8ac89d2e19c3ab6c`.
The consumer's pixel arithmetic follows
[drawNRGBAOver](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/draw/draw.go#L507)
(Copyright 2009 The Go Authors), SHA-256
`24f42de1373014deefce87d4dd02856010ac8d385c4e61eb0c52a1a1273b0207`.
Both come from revision `022c8636110ebac86a9b88724cda168ed713c21f` and retain
the upstream BSD license. All port and consumer calculations execute in Gooo.
