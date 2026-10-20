# Duration quantizer

A Gooo program that rounds and truncates a signed time interval to a chosen
multiple, then reports how much each operation changed it. Both inputs and all
four outputs are exact signed 64-bit **nanoseconds**. This can help inspect
measurement buckets and interval precision; it does not read a clock or schedule work.

예를 들어 `1,500,000,000`나노초를 `1,000,000,000`나노초 단위로 맞추면
반올림은 2초, 절삭은 1초입니다. 음수의 정확한 중간값은 0에서 멀어지는 쪽으로
반올림하므로 −1.5초는 −2초가 됩니다. 절삭은 0을 향하므로 −1초가 됩니다.

## Run and replay

From the repository root, with Go 1.27.3, Git and a POSIX shell:

```sh
export GOTOOLCHAIN=go1.27.3
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/duration-demo
./.gooo package execute --json --inputs examples/duration-quantizer/inputs.json \
  duration-quantizer.workspace.json > out/duration-demo/execution.json
./.gooo package replay --json --receipt out/duration-demo/execution.json \
  --inputs examples/duration-quantizer/inputs.json \
  duration-quantizer.workspace.json > out/duration-demo/replay.json
```

In [inputs.json](inputs.json), `app/duration-quantizer:Inspect.input0` is the
duration `d`; `input1` is the multiple `m`. The input-only receipt is `OBSERVED`:
it records execution without asserting expected outputs. Results are in
`result.runtime.traces[].deliveries[].actual`.

| Field | Meaning |
| --- | --- |
| `rounded` | Nearest multiple of `m`, with halfway values away from zero |
| `truncated` | Multiple of `m` obtained by moving toward zero |
| `round_adjustment` | `rounded - d`, including when rounding saturates |
| `truncation_remainder` | `d - truncated`, retaining the original sign |

For `d=1500000000`, `m=1000000000`:

```json
{"rounded":2000000000,"truncated":1000000000,"round_adjustment":500000000,"truncation_remainder":500000000}
```

## Go API mapping and boundaries

The original methods `time.Duration(d).Round(time.Duration(m))` and
`.Truncate(time.Duration(m))` become Gooo activities
`go/time:DurationRound(Integer, Integer)` and `DurationTruncate(Integer, Integer)`.
The receiver becomes the first argument. The consumer imports these activities;
its calculation stays in Gooo. Go's `time` package is used only for independent
reference results in the verifier.

Both arguments accept the complete range
**−9223372036854775808..9223372036854775807**. Zero and negative multiples return
`d` unchanged, as Go does; they are valid inputs, not rejection cases. If the
rounded result cannot fit, it saturates at MinInt64 or MaxInt64. A saturated
result need not be a multiple of `m`. For example, `(9000000000000000000,
5000000000000000000)` rounds to `9223372036854775807` and truncates to
`5000000000000000000`.

For positive `m`, rounding first finds the remainder and compares its magnitude
with `m - remainder`. It checks the distance to the int64 boundary before adding
or subtracting. It never negates MinInt64, doubles a large remainder, or relies
on overflowing arithmetic. Truncation removes the signed remainder. The
consumer's adjustment and remainder also fit int64 for every allowed pair.

The source is pinned Go
[time.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/time/time.go#L1110).
Its hash and license are retained in [upstream.json](../../upstream.json),
[NOTICE](../../NOTICE), and [GO-LICENSE](../../third_party/GO-LICENSE).
This is a duration operation, not `time.Time.Round`, date arithmetic,
`ParseDuration`, a timer, or the whole Go time package.

## Check against Go and try in the browser

```sh
go run ./cmd/verify --out out/duration-check
```

Use a new output directory. The verifier compares each port and the importing
consumer with independent Go results, runs saved replay, and checks the original
inputs, activity and type identities, sources and compiler identity. Cases cover
nonpositive multiples, positive and negative halfway values, exact multiples,
saturation boundaries and distinct integers above 2^53. These are finite
observations, not an exhaustive execution of all int64 pairs.

There are 1,789 distinct input pairs per activity: 3,578 reference cases for the
two ports and 1,789 for this consumer. The 14 input-only examples are recorded
separately and are not added to these comparison counts. See the
[execution summary](../../evidence/duration-20261010.json) for the saved observations.

The [browser demo](https://kimjooyoon.github.io/gooo-go-ports/) provides
**time.Duration.Round** and **time.Duration.Truncate** separately. Enter decimal
integer nanoseconds for both inputs. Inputs, outputs and shared links preserve
the decimal strings without conversion through JavaScript Number. The four-field
inspection record above is the native importing consumer's output.

When editing JSON yourself, use an editor or tool that preserves integer values.
For example, `9007199254740992` and `9007199254740993` are distinct inputs whose
results can differ; parsing and saving through JavaScript Number can merge them.
To copy this port into another project, follow the
[standalone project guide](../../docs/use-in-your-project.md) with a fixed source
revision that contains `ports/time/duration.gooo`. That guide's older snapshot
demonstrates a Unicode port and does not contain these new duration activities.
