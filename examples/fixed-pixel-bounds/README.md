# From subpixel bounds to a clipped pixel rectangle

Use this native example when an upstream layout or glyph API gives you signed
26.6 fixed-point bounds and you need an integer pixel rectangle. Each raw unit
is 1/64 pixel: raw `(32,32)–(96,96)` is `(0.5,0.5)–(1.5,1.5)` pixels, so the
outward integer bounds are `(0,0)–(2,2)` before clipping.

This follows the Floor/Ceil boundary conversion used by the pinned
[x/image OpenType glyph rasterizer](https://github.com/golang/image/blob/b06f1de3f4900ff828b8f114c37eb9ea10dfed90/font/opentype/opentype.go#L159-L178).
It accepts coordinates already supplied by the caller. It does not read fonts,
shape text, rasterize glyphs, open images or crop pixel data. The browser tools
and existing `CropPlan` / `DragCropPlan` activities are unchanged.

The [illustrated technical note](https://kimjooyoon.github.io/gooo-go-ports/fixed-point-bounds.html)
shows raw values, pixel values and the empty-before-rounding rule in a compact table.

## Run and verify

From the repository root, with Go 1.27.3, Git, a POSIX shell and Python 3 on
amd64 or arm64:

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/fixed-pixel-bounds
./.gooo package execute --json --inputs examples/fixed-pixel-bounds/inputs.json \
  fixed-pixel-bounds.workspace.json > out/fixed-pixel-bounds/execution.json
./.gooo package replay --json --receipt out/fixed-pixel-bounds/execution.json \
  --inputs examples/fixed-pixel-bounds/inputs.json \
  fixed-pixel-bounds.workspace.json > out/fixed-pixel-bounds/replay.json
python3 tools/check-fixed-pixel-bounds.py \
  out/fixed-pixel-bounds/execution.json out/fixed-pixel-bounds/replay.json
```

The input-only receipt says `OBSERVED`; the checker separately checks 16 literal
answers, original exact inputs, entry and record field identities, three source
bindings, compiler/toolchain identity, generated source and saved replay binding.
The full differential verifier additionally runs the finite corpus below:

```sh
go run ./cmd/verify --out out/fixed-full
```

Use a new output directory. This catalog-based run includes all previous suites;
the compiler entry is selected per suite to avoid unrelated consumer records.
The standalone `fixed-pixel-bounds.workspace.json` is the example's runnable
manifest. `verification-catalog.json` is a source inventory, not a runnable
compiler manifest. The aggregate `gooo.workspace.json` is unchanged.

## Exact input and rejection contract

`example/fixed-pixel-bounds:PixelBounds.input0` is the **original image bounds**.
Its four coordinates use the full signed int64 range, matching Go `int` on
amd64/arm64. They are never normalized or narrowed.

`example/fixed-pixel-bounds:PixelBounds.input1` is a **raw Q26.6 rectangle**.
The record field names are `min_x`, `min_y`, `max_x`, `max_y`; all four JSON
values must be integers. Keep their exact values instead of passing through
floating-point conversion. In particular, large image bounds above 2^53 must
remain exact.

The Gooo consumer applies this order:

1. Reject any raw coordinate outside signed int32 (`−2147483648..2147483647`).
2. Accept a raw reversed or zero-width/height selection as empty, before rounding.
3. For a nonempty selection, reject `max_x` or `max_y` above `2147483584`, where
   upstream Ceil wraps. This restriction belongs to the consumer, not the ports.
4. Call the actual imported Gooo Floor on each minimum and Ceil on each maximum,
   then call the existing `RectangleIntersect` against the unchanged image bounds.

The result contains `accepted`, four integer coordinates, and `has_pixels`.
Rejection returns `accepted:false`, zero coordinates and `has_pixels:false`.
Accepted empty results also have zero coordinates, with `accepted:true`.
`has_pixels` describes a nonempty rectangle; it does not establish that image
storage exists or can be allocated. No widths or areas are calculated.

A fractional line such as raw X `[32,32]` stays empty; rounding it first would
incorrectly expand it to pixel X `[0,1]`. Raw X `[2,1]` also stays empty.
An empty selection in the upper overflow region is accepted as empty at step 2.
Out-of-domain input is rejected even if another axis is already empty.

## Two ports and the deliberate API difference

`go/golang.org/x/image/math/fixed:Int26_6Floor` and `Int26_6Ceil` receive one
Integer containing the original signed raw value. They return
`{in_fixed_domain, value}` instead of Go's single `int`. Outside signed int32,
the record is `{false,0}`. The flag distinguishes rejection from legitimate
zero and negative values; no outside-domain input is converted to int32.

Within the **entire original int32 domain**, the implementation preserves Go's
int32 addition overflow and arithmetic right shift behavior. In particular,
`Ceil(2147483584)=33554431`, but all 63 raw values from `2147483585` through
`2147483647` produce `−33554432`. The port does not repair this wrap into a
mathematical ceiling. Negative examples include `Floor(−65)=−2`,
`Ceil(−65)=−1`, and `Ceil(−1)=0`. Round and other x/image APIs are not included.

The exact upstream revision, source SHA-256 and API mapping are in
[upstream.json](../../upstream.json). The independent Go oracle calls the
[byte-preserved original fixed.go](../../third_party/x-image/math/fixed/fixed.go),
under its [BSD license](../../third_party/x-image/LICENSE). A separate local
provenance helper binds its compiled-in source bytes to the pinned SHA-256.
Gooo execution does not call that oracle or a Go wrapper around these methods.

The [retained native summary](../../evidence/fixed-pixel-bounds-20261011.json) records
37,232 passing finite cases in 45 suites and 531 batches, each with saved replay.
The finite corpus adds 1,664 port reference cases and 16 outside-domain cases,
plus 471 consumer reference cases, 33 raw-domain rejections and 6 overflow
rejections. It covers all 64 fractional residues at eleven signed quotients,
negative/overflow boundaries, fixed samples, empty/reversed selections and
original extreme image bounds. It preserves all 35,042 previous cases.
This is **not** a check of all 2^32 raw inputs or all rectangle pairs.
