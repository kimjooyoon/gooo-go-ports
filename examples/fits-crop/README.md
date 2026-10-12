# Reject crop requests that need clipping

Use `FitsCrop` when a caller must supply a nonempty rectangle entirely inside
the original image bounds. It returns a Boolean; your application decides
whether to accept the request. It does not clip coordinates, open images,
allocate storage or process pixels.

The existing [CropPlan](../crop-planner/README.md) deliberately returns an
intersection. This separate example leaves that behavior intact.

## Run the example

From this repository's root on macOS or Linux, with Git, a POSIX shell,
Python 3 and a Go installation supporting toolchain selection:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/fits-crop
./.gooo package execute --json --inputs examples/fits-crop/inputs.json \
  fits-crop.workspace.json > out/fits-crop/execution.json
./.gooo package replay --json --receipt out/fits-crop/execution.json \
  --inputs examples/fits-crop/inputs.json \
  fits-crop.workspace.json > out/fits-crop/replay.json
python3 tools/check-fits-crop.py \
  out/fits-crop/execution.json out/fits-crop/replay.json
```

Bootstrap selects the repository's pinned Gooo 0.6.32-dev compiler and Go 1.27.2.
This native example supports amd64/arm64. Keep the compiler with its receipts.
For an existing older checkout, follow the root README's upgrade instructions.
The first installation may download the compiler and Go toolchain.

`fits-crop.workspace.json` is the runnable manifest. The verification catalog is
a source inventory. The input-only receipts report `OBSERVED`, with no supplied
expectations or correctness score. The checker separately compares the documented
Boolean answers and validates original inputs, source/entry/compiler identities,
generated Go and the saved replay binding.

## Original coordinates and the two rules

Each input row supplies two records with required integer fields `min_x`,
`min_y`, `max_x`, `max_y`. `FitsCrop.input0` is the original bounds;
`FitsCrop.input1` is the requested rectangle. Preserve exact signed int64 values,
including those beyond 2^53. Missing, noninteger or outside-int64 fields are
input errors, rather than a legitimate `false` result.

The library's `RectangleIn(receiver, container)` matches the pinned
[Go Rectangle.In](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/geom.go#L218-L227):
every empty or reversed receiver is inside every container. A nonempty receiver
fits when all four boundary comparisons hold. Equal maximum edges fit even
though maximum coordinates are exclusive.

The importing Gooo example first rejects an empty request, then calls
`RectangleIn(requested, bounds)`. It accepts a contained or equal-bounds request
and rejects requests crossing any boundary. It does not normalize reversed
coordinates, subtract endpoints, or calculate potentially overflowing sizes.
The original Go oracle uses `image.Rectangle` literals; `image.Rect` would
silently normalize the inputs.

The port preserves the upstream copyright and [BSD license](../../third_party/GO-LICENSE).
Pinned source hashes and API mappings are in [upstream.json](../../upstream.json).
The finite differential corpus and replay check cover declared boundary cases
and deterministic samples; they do not establish all rectangle-pair behavior,
available image storage, or whole-package compatibility.

The [native verification evidence](../../evidence/rectangle-in-fits-crop-20261012.json)
records 570 original Go comparisons and 18 saved-replay batches for each activity,
within the completed 59-suite run and its independent raw-receipt audit. The 14
documented input-only rows also executed and replayed with `OBSERVED` receipts;
malformed-input rejection checks remain separate from Go reference cases.

## Documented inputs

The fourteen rows in `inputs.json` have these independent Boolean expectations.
The full corpus separately checks additional raw coordinate tuples against Go.

| Input | `requested.In(bounds)` | `FitsCrop(bounds, requested)` |
| --- | --- | --- |
| contained | true | true |
| empty_receiver_empty_container | true | false |
| cross_left | false | false |
| cross_top | false | false |
| cross_right | false | false |
| cross_bottom | false | false |
| touching_disjoint_right | false | false |
| zero_width_outside | true | false |
| zero_height_outside | true | false |
| reversed_x_extremes | true | false |
| reversed_y_extremes | true | false |
| nonempty_receiver_reversed_extreme_container | false | false |
| equal_full_int64_bounds | true | true |
| above_2pow53_one_unit_left_cross | false | false |
