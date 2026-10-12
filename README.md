# Gooo Go Ports

[Gooo](https://github.com/kimjooyoon/meta-ontology-go) is an experimental language for
describing what programs should do and inspecting how they are built and run.
This independent ecosystem project implements selected Go standard-library, x/image and Google UUID functions in Gooo,
with independent Go reference outputs and saved replay: 36 standard-library ports,
two ports from golang.org/x/image and two Google UUID adaptations. The current scope is
40 HTTP, Unicode, byte-search, UUID format and parsing, 8-bit integer, color, duration, rectangle and fixed-point APIs, seventeen native importing consumer programs (nineteen activities),
and an importing browser dispatcher.
The RectangleIn/FitsCrop suites have completed finite native execution and saved
replay with an independent raw-receipt audit; see the [verification evidence](evidence/rectangle-in-fits-crop-20261012.json).

## Start here

| What would you like to do? | Start with | What you can change |
| --- | --- | --- |
| Try Gooo without installing anything | [Interactive tutorial](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html) | Change a score and label, see the returned record, and read the two Gooo source files |
| Edit source and save generated Go in the browser | [Gooo code editor](https://kimjooyoon.github.io/gooo-go-ports/codegen/) | Edit one source file and generate Go for one activity; save the current Go code or original response JSON. Generated Go is not executed |
| Edit and run Gooo code locally | [Run and edit locally](#run-and-edit-locally) | Install the pinned compiler, then change the tutorial's condition and rerun it |
| Use a port in another project | [Import walkthrough](docs/use-in-your-project.md) | Build a separate application that imports `utf8.ValidRune` from a fixed source snapshot |

You can also [try 21 library functions in the browser](https://kimjooyoon.github.io/gooo-go-ports/).
The library tools and tutorial run fixed, precompiled programs. The experimental
[code editor](https://kimjooyoon.github.io/gooo-go-ports/codegen/) uses the actual
pinned Gooo compiler library as WebAssembly to generate Go from edited source.
It loads about 22 MB, accepts one file up to 64 KiB, and does not support package
imports or model-backed generation. Use the local toolchain to execute modified
Gooo programs. Browser compilation reports retain their engine
`UNBOUND_LOCAL_SOURCE` provenance; the separately hashed bridge does not turn
them into native execution receipts.

[Compare Gooo with generated Go](https://kimjooyoon.github.io/gooo-go-ports/source-view.html)
without setup: four saved CLI observations show a conditional body, a changed return,
one selected activity, and a rejected body. This read-only page preserves the exact
source and output from the linked pinned Linux verification; it does not compile
edited source or execute the generated Go.

Experimental tooling: [Compiler Explorer adapter and Linux verification](tools/compiler-explorer/README.md)
for inspecting generated Go. Upstream hosted support is under review.

한국어 안내: 설치 없이 [튜토리얼 실행](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html),
[브라우저에서 소스 수정·Go 생성](https://kimjooyoon.github.io/gooo-go-ports/codegen/),
[로컬에서 코드 수정](#run-and-edit-locally), [다른 프로젝트에 가져오기](#다른-gooo-프로그램에서-가져오기) 중에서 시작하세요.

## Run and edit locally

Start by running and editing the small [Gooo tutorial](examples/learn-gooo/README.md).
With Git, a POSIX shell and a Go installation that supports toolchain selection,
run these commands from this repository's root on macOS or Linux:

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
./.gooo package execute --inputs examples/learn-gooo/inputs.json \
  learn-gooo.workspace.json
```

The four examples include score `75` returning `sensor: high`. In
[examples/learn-gooo/main.gooo](examples/learn-gooo/main.gooo), change the one
`if score >= 60` condition to `if score >= 80`, then rerun the last command:
score `75` now returns `sensor: low`; its other three fields stay unchanged.
Follow the [edit-and-replay steps](examples/learn-gooo/README.md#make-one-visible-change)
to save a new execution record and replay it. Setup downloads the pinned compiler
and may also download Go 1.27.2, depending on the installed toolchain.
[Full native port verification](#full-native-port-verification) is a separate check.

Upgrading an existing checkout: bootstrap preserves an older `.compiler` and
refuses to replace it. Use a fresh checkout in a new directory, or move both
`.compiler` and `.gooo` into a separate backup directory before running bootstrap.
Keep the old compiler with its saved execution records; this upgrade does not
claim that every receipt from an older compiler can be replayed by the new one.

The original 3,147-case corpus is unchanged. A Unicode inspector adds 334 Go-reference
cases and 12 explicit domain-rejection cases; six unary byte-domain functions cover all 256 inputs.
Color ports and their consumer add separate reference and rejection cases, detailed below.
Other domains use finite boundary cases and samples. This is an independent experimental port,
not a complete Go package implementation or an official Google/Go project.

The native [fixed-point pixel bounds example](examples/fixed-pixel-bounds/README.md)
converts caller-supplied Q26.6 subpixel bounds outward to integer pixels and clips
them with the existing rectangle port. It keeps raw empty selections empty,
reports invalid/overflowing requests explicitly, and preserves upstream int32
wrap behavior in the two low-level Floor/Ceil ports. It performs no font or image
rendering and does not change the browser tools.

[Try the browser demo](https://kimjooyoon.github.io/gooo-go-ports/) to run 21 HTTP, Unicode, byte, color, and duration functions
without installing a compiler. It executes a precompiled Gooo program as WebAssembly;
it is not an editor for compiling arbitrary source. The page includes the Gooo source,
unmodified generated Go, execution receipts, and an optional reference-check button.
Use **Copy link to this input** to share an exact function input. Opening a link runs
it once in the recipient's browser when the compiled program matches; a changed program
restores the inputs and waits for **Run function**. Links carry inputs, not trusted results.
Enter three RGB or four CMYK channels (integers from 0 to 255) to see named output channels
and an RGB preview. The native [RGB → CMYK → RGB inspector](examples/color-inspector/README.md)
reports converted channels, recovered RGB values, and any integer round-trip differences.
Choose **bits.RotateLeft8** to enter a byte and a signed rotation count and see the
eight bits before and after. The native [byte rotation inspector](examples/byte-rotation/README.md)
also restores the byte and checks its one-bit count.
Choose **time.Duration.Round** or **time.Duration.Truncate** to adjust an exact
signed nanosecond duration to a chosen multiple. The native
[duration quantizer](examples/duration-quantizer/README.md) returns both results
and their adjustments. Zero or negative multiples preserve the original duration.
Choose **unicode.IsSpace** to check whether one code point is whitespace according to
Go and Unicode 17.0.0. Enter its numeric value or paste one code point without trimming.
The native [whitespace inspector](examples/whitespace-inspector/README.md) also reports
scalar validity, ASCII whitespace membership, and UTF-8/UTF-16 lengths.
This classification does not determine whether a character is visible or safe.
See the [reproducible Go–JavaScript whitespace comparison](docs/whitespace-comparison/README.md)
for the `U+0085` and `U+FEFF` differences in blank-input checks.

The [Unicode text inspector](https://kimjooyoon.github.io/gooo-go-ports/text-inspector.html)
combines six existing compiled Gooo functions for a whole pasted input. It shows
each code point's original UTF-16 position, scalar validity, Go whitespace, letter
and decimal-digit membership, and UTF-8 / UTF-16 encoding lengths. CR and LF remain separate;
combining and joined sequences are shown as their constituent code points.
The browser decodes the received UTF-16 text and displays the Gooo results.
Download a completed inspection as JSON to retain the exact input, returned
values and program version for a bug report or test case.
This is a browser application using existing ports, not a new Gooo string API.
See [input handling, limits, and verification](docs/text-inspector.md).

The native [identifier character classifier](examples/identifier-rune/README.md) adds
complete Unicode `IsLetter` and `IsDigit` ports. It distinguishes letters, decimal
digits and underscore for one identifier character. A reproducible generator
preserves every pinned range and stride; these two ports are available in native
Gooo workspaces and in the 21-function browser demo. Try the text inspector’s
**Letters + decimal digits** example to compare `가`, `١`, `𝟎`, `²`, `¼`, `Ⅷ`
and a combining accent. These per-character categories do not validate a whole identifier.
To use these ports outside this repository, follow the complete
[Unicode import walkthrough](docs/use-unicode-in-your-project.md). It builds a
separate application from a fixed snapshot, copies both required source files,
and saves an execution record for replay.

The native [byte-search example](examples/string-byte-search/README.md) imports
`strings.IndexByte` to find one byte in valid UTF-8 Text of at most 1,024 bytes.
Its Gooo loop checks the length before each byte access and preserves Go byte
offsets, including matches inside a multibyte character. The returned record
shows the offset, whether a match was found, the original needle domain and byte
length. An Integer outside 0..255 returns -1 as a separate extension. This is
not whole-string Unicode searching, arbitrary Go byte-string support, or the
whole `strings` package; the existing 21-function browser tools are unchanged.

The native [UUID format inspector](examples/uuid-format-inspector/README.md) imports
Google's `github.com/google/uuid` v1.6.0 `Validate` port. It accepts the original
hyphenated, URN, braced and plain-hex forms over valid UTF-8 Text of at most
1,024 bytes, preserving exact error text and the original length-error classification.
The inspector adds the original UTF-8 byte length. All checks and error formatting
run in Gooo, including a multibyte character cut by the first-nine-byte URN prefix.
This is a format check, without UUID generation or parsing into a binary UUID.
See [API mapping, source and limits](docs/google-uuid.md). The existing browser
library tools remain unchanged; this port and inspector run locally.

The native [UUID canonical parsing example](examples/uuid-parse-canonical/README.md)
uses a separate `ParseCanonical` Text API to preserve Google's `Parse` return
value through `UUID.String()`, plus the original error and length-error class.
An error can still return a nonzero UUID value, so check `parsed` before using
`canonical_uuid`. The parser's permissive 38-byte wrappers differ from the
`Validate` format check. See the [API mapping and scope](docs/google-uuid-parse-canonical.md).

The native [simple case-fold comparison](examples/simple-fold/README.md) imports
`unicode.SimpleFold` to compare two rune values and show each value's next member
in its Unicode simple-folding cycle. For example, `K`, `k`, and the Kelvin sign
`K` belong to one three-member cycle. This preserves the pinned Go rules for
special cycles and invalid Unicode values; it is a per-rune API, not whole-string
folding, normalization, or locale-sensitive text comparison. The browser tools
continue to provide their existing 21 functions.

The native [case-mapping inspector](examples/case-inspector/README.md) shows one
rune's uppercase, lowercase and titlecase results. The four ports `unicode.To`,
`ToUpper`, `ToLower` and `ToTitle` share the complete pinned case table in Gooo.
For `ǆ`, uppercase is `Ǆ` while titlecase is `ǅ`. The returned record keeps the
original integer and distinguishes the int32 rune domain from valid Unicode
scalars. These are simple rune mappings, without locale rules or multi-character
expansions such as `ß` to `SS`.

The native [alpha matte example](examples/alpha-matte/README.md) imports
`color.NRGBAToRGBA` to show premultiplied 16-bit channels, then composites one
foreground pixel over an opaque RGB background. For example, red with alpha
128 over white becomes `(255, 127, 127)`. The example preserves Go's encoded-channel
integer arithmetic; it does not decode images or perform linear-light blending.
All original foreground channels are accepted in the byte domain, including
hidden RGB with zero alpha. A channel outside that domain is rejected before
arithmetic. Both the port and this importing example are native-only.

The native [YCbCr round-trip inspector](examples/ycbcr-inspector/README.md) imports
Go's `RGBToYCbCr` and `YCbCrToRGB` ports. It preserves the full byte range and
the pinned Go fixed-point rules, and shows the recovered RGB channels and their
differences. Pure red `(255, 0, 0)` returns as `(254, 0, 0)` in this conversion.
Use the saved records to investigate integer color conversion; the example
does not read image files or perform JPEG compression or chroma subsampling.

The native [crop planner](examples/crop-planner/README.md) imports the rectangle
package and clips a requested region to image bounds. Rectangle records preserve
all four signed 64-bit coordinates, including reversed and empty rectangles. The
Go `Min.X`, `Min.Y`, `Max.X`, `Max.Y` fields map to `min_x`, `min_y`, `max_x`, `max_y`;
the port does not normalize them. Try the same importing program in the
[browser crop planner](https://kimjooyoon.github.io/gooo-go-ports/crop-planner.html):
enter bounds and a requested region to get exact intersection coordinates and
whether the result contains pixels. It preserves reversed and empty inputs,
exclusive maximum edges, and the full signed 64-bit coordinate range. It does
not load or modify image files. This separate browser program composes the
existing `RectangleIntersect` and `RectangleEmpty` ports; it adds no new port
and leaves the 21-function demo's input and shared-link formats unchanged.

The native [FitsCrop example](examples/fits-crop/README.md) returns whether a
nonempty requested rectangle already fits inside the supplied bounds. It imports
`RectangleEmpty` and the new `RectangleIn`; it does not clip or normalize the
request. Go considers every empty or reversed receiver inside any rectangle,
so `RectangleIn` can return true while `FitsCrop` returns false. Equal exclusive
maximum edges are allowed, and all four coordinates retain their original signed
64-bit values. This example runs locally; the browser programs are unchanged.

The experimental [drag-selection crop planner](docs/drag-crop.md) interprets the requested
rectangle as two unordered drag corners. Choose **Two-corner selection** in the
browser crop planner: (200,200) to (100,100) selects the same region as the reverse
direction. A separate compiled `DragCropPlan` program imports `RectangleCanon`,
then clips to the original bounds. The default **Keep rectangle order** mode
continues to run `CropPlan` with its original reversed-input behavior. Neither
mode normalizes image bounds or processes image files; the 21-function dispatcher
is unchanged.

To learn the language from a small program, try the
[score-and-label tutorial](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html).
Change two inputs, inspect the four returned fields, and read both Gooo source
files on the page. Download the same project and follow its
[native instructions](examples/learn-gooo/README.md) to change the code locally.
The browser runs this fixed program as WebAssembly and preserves exact 64-bit
scores. The example accompanies a pending Learn X in Y Minutes contribution;
it is separate from the Go port counts and does not imply that site's endorsement.

The separate [offline decision-policy experiment](examples/decision-policy-pilot/README.md)
applies explicit tenant, action, integer-limit and expiry rules to synthetic Choice
proposals. It compares Gooo with an independently written Go policy and manual
expectations, then counts calls to a mock operation. It is a small example for
discussing policy-review and incident-reproduction work, with no live model API,
real export or production-security claim. Its cases are separate from the port
compatibility counts. The guide includes a focused request for workflow feedback.

For a fixed snapshot, choose an [experimental release](https://github.com/kimjooyoon/gooo-go-ports/releases).
Each release page lists its included tools and separate source, execution/replay
evidence, and standalone browser archives, with licenses, provenance, and checksums.
Downloads do not require a GitHub login. Follow that version's download and verification
instructions; the live demo and main branch can be newer than the chosen snapshot.

If loading or running fails, [report a problem](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
with the page URL or command and what happened. A working demo or shared input link
is not required. See [reporting guidance](CONTRIBUTING.md#report-a-problem) for useful details.

Go로 작성된 오픈소스의 동작을 **Gooo 함수로 이식하고 원본과 비교**하는 공개 실험 저장소입니다.
현재 Go 표준 라이브러리, x/image와 Google UUID의 HTTP·Unicode·바이트 검색·UUID 형식 검사·파싱·8비트 정수·색상 변환·시간 간격·사각형·고정소수점 API 40개를 표현합니다.
Go 소스를 분석해 타입·경계값·실패 동작을 명시하고, 계산은 `ports/**/*.gooo`에 구현합니다.
Go 검증 도구는 원본 함수 호출, 입력 전달, 결과 비교와 실행 기록 저장을 맡습니다.

[Gooo 컴파일러](https://github.com/kimjooyoon/meta-ontology-go)를 사용하는 독립 생태계 프로젝트입니다.
현재 언어로 동작하는 라이브러리와 다음 이식을 막는 최소 재현 사례를 함께 쌓아
언어 개발에 간접적으로 기여합니다. Google·Go 프로젝트의 공식 이식판은 아닙니다.

## 이식한 함수

| Gooo 패키지 경로 | 함수 | 원본과의 대응 |
| --- | --- | --- |
| `go/net/http` | `StatusText` | Go의 상태 코드 62개와 설명. 미등록 코드는 빈 문자열 |
| `go/strings` | `IndexByte` | UTF-8 Text 최대 1,024바이트에서 0..255 byte의 첫 위치 또는 -1. 원본 바이트 offset과 길이 보존; 영역 밖 Integer는 별도 -1 확장. Native 전용 |
| `go/github.com/google/uuid` | `Validate` | 원본 네 표기와 정확한 오류·길이 오류 분류를 `{valid,error,invalid_length}`로 보존. 유효한 UTF-8 Text 최대 1,024바이트; native 전용 |
| `go/unicode` | `IsLetter`, `IsDigit` | Unicode 17.0.0 전체 Letter(L)·decimal digit(Nd) 범위와 stride 보존. 원본 표에서 결정적으로 Gooo 분기 생성 |
| `go/unicode` | `IsSpace` | Unicode 17.0.0 White_Space 속성의 25개 코드 포인트 판정. 전체 signed int32 rune 입력 유지 |
| `go/unicode` | `SimpleFold` | Unicode 17.0.0 단순 대소문자 접기의 다음 순환 멤버. ASCII·특수 순환·case range 우선순위 및 전체 int32 입력 보존. Native 전용 |
| `go/unicode` | `To`, `ToUpper`, `ToLower`, `ToTitle` | Unicode 17.0.0 한 rune의 단순 대문자·소문자·제목형 변환. 전체 CaseRanges를 공유하고 별도 title delta와 range 시작점 기준 교대 parity 보존. Native 전용 |
| `go/unicode/utf8` | `ValidRune`, `RuneLen` | Unicode 유효 코드 포인트 판정과 UTF-8 바이트 수 |
| `go/unicode/utf8` | `RuneStart` | 0..255의 바이트가 continuation byte가 아닌지 판정. 전체 문자열 유효성 검사는 아님 |
| `go/unicode/utf16` | `IsSurrogate`, `RuneLen` | 서로게이트 판정과 UTF-16 코드 단위 수 |
| `go/unicode/utf16` | `DecodeRune`, `EncodeRune` | 서로게이트 쌍 변환. EncodeRune의 두 반환값은 `{high, low}` 레코드 |
| `go/math/bits` | `Len8`, `LeadingZeros8`, `TrailingZeros8`, `OnesCount8`, `Reverse8` | 0..255 정수의 비트 길이, 앞·뒤 0 개수, 1 개수와 비트 순서 반전. 비교·산술로 구현 |
| `go/math/bits` | `RotateLeft8` | 바이트와 signed int64 회전 수를 받아 왼쪽 회전. 음수는 오른쪽 회전이며 반환값은 0..255 |
| `go/image/color` | `RGBToCMYK`, `CMYKToRGB` | 각 채널 0..255의 RGB 3성분·CMYK 4성분 변환. 다중 반환은 `CMYK`·`RGB` 레코드 |
| `go/image/color` | `RGBToYCbCr`, `YCbCrToRGB` | 각 채널 0..255의 RGB·Y′CbCr 3성분 변환. Go의 고정 소수점·반올림·포화 보존. 다중 반환은 `YCbCr`·`RGB` 레코드. Native 전용 |
| `go/image/color` | `NRGBAToRGBA` | Go `NRGBA.RGBA` 수신 객체의 네 uint8 채널을 레코드로 전달. 알파가 곱해진 16비트 RGB와 확장 알파를 `RGBA16` 레코드로 반환. Native 전용 |
| `go/time` | `DurationRound`, `DurationTruncate` | Go Duration 메서드의 수신 객체를 첫 인수로 전달. 두 signed int64 나노초 입력의 반올림·절삭과 범위 초과 시 포화 |
| `go/image` | `RectangleEmpty`, `RectangleOverlaps`, `RectangleIntersect`, `RectangleUnion`, `RectangleCanon` | 수신 사각형을 첫 레코드 인수로 전달. Go의 네 좌표를 signed int64 필드로 보존하며 기존 네 활동은 빈·뒤집힌 입력을 정규화하지 않음. `RectangleCanon`만 뒤집힌 축을 비교·교환. Native에서 다섯 함수를 실행하며 [브라우저 crop](examples/crop-planner/README.md)은 `Intersect` + `Empty`를 조합 |
| `go/image` | `RectangleIn` | 첫 수신 사각형이 두 번째 사각형에 포함되는지 원래 signed int64 좌표로 판정. 빈·뒤집힌 수신 사각형은 항상 true이며 같은 exclusive 최댓값을 허용. 정규화·좌표 산술 없음; native 전용 |
| `go/golang.org/x/image/math/fixed` | `Int26_6Floor`, `Int26_6Ceil` | signed int32 Q26.6 원시 좌표의 정수 경계. 원본 overflow를 보존하며 반환값은 `{in_fixed_domain, value}` 레코드. Native 전용 |

원본의 `rune` 입력은 int32 범위를 Gooo Integer로 전달합니다. `byte` 호환 범위는
0..255입니다. 묵시적으로 정수 폭을 줄이지 않습니다. `RuneStart`의 범위 밖 입력은
Gooo 확장 동작으로 false를 반환하지만 원본과의 동등성 집계에는 넣지 않습니다.
`IsSpace`, `IsLetter`, `IsDigit`도 int32 밖의 정수는 별도 확장 계약으로 false를 반환합니다. int32 안의 음수,
서로게이트, Unicode 최댓값보다 큰 수는 원본 API 입력이며 결과는 false입니다.
`SimpleFold`는 int32 안의 음수·서로게이트·Unicode 범위 밖 입력을 Go처럼 그대로 반환합니다.
int32 밖 Integer도 그대로 반환하는 별도 확장 계약이며, Go rune 변환이나 원본 동등성 집계에 넣지 않습니다.
두 rune 비교 예제의 `in_rune_domain`은 int32 범위를 뜻합니다. 유효 Unicode 스칼라 판정이 아니며,
같은 잘못된 Unicode 값 두 개도 같은 고정점으로 비교됩니다. `İ`·`ı`는 각각 고정점이고,
`K → k → K → K` 등의 특수 순환을 단순 소문자 변환으로 대체하지 않습니다.
`To`의 첫 인수는 0(대문자)·1(소문자)·2(제목형)이며, 다른 값이면 rune 범위 검사보다 먼저
U+FFFD를 반환합니다. 유효한 case와 세 단항 변환은 int32 안의 잘못된 Unicode 값을
그대로 보존하고, int32 밖 입력도 별도 identity 확장으로 유지합니다. 전체 문자열이나
지역별 변환이 아니며, [검사 예제](examples/case-inspector/README.md)는 원입력과 세 결과를 함께 반환합니다.
NEL(U+0085)은 공백이고 BOM(U+FEFF)·ZERO WIDTH SPACE(U+200B)는 공백이 아닙니다.
JavaScript의 `\s`·`trim`과는 다른 판정이며, 보이지 않는 문자 전체를 탐지하지 않습니다.
`go/math/bits`의 여섯 함수는 바이트 영역 밖에서 별도 확장 동작인 -1을 반환합니다.
Go의 byte 변환처럼 입력을 잘라서 계산하지 않습니다. 0의 앞·뒤 0 개수는 원본과 같은 8입니다.
색상 변환도 모든 채널을 먼저 검사하며 범위 밖에서는 레코드의 모든 성분을 -1로 반환합니다.
`NRGBAToRGBA`는 RGB가 알파보다 크거나 알파가 0이어도 원래 NRGBA 입력으로 받습니다.
[알파 합성 예제](examples/alpha-matte/README.md)는 이 포트를 사용해 불투명 배경 위의 한 픽셀을
합성하고 중간 16비트 성분과 최종 8비트 RGB를 함께 보여줍니다. Go의 정수 버림을 보존하며,
이미지 파일 처리나 선형 광량 공간의 합성을 구현한 것은 아닙니다.
원본의 정수 나눗셈 순서를 유지하므로 왕복 변환이 원래 RGB와 항상 같은 것은 아닙니다.
YCbCr 변환은 Go의 JFIF 계수와 채널 포화를 보존합니다. 역변환은 `Y′ * 65793`을
그대로 사용하며 음수·상한 밖 분자를 먼저 처리해 signed shift를 안전한 나눗셈으로 옮깁니다.
두 파일 `ports/image/color.gooo`와 `ports/image/ycbcr.gooo`를 함께 가져와야 합니다.
`StatusText`·`RotateLeft8`·`To`의 Go int 입력 비교는 int가 64비트인 amd64·arm64에서 실행합니다.
회전 수는 int64 전체를 보존하며, 나머지를 먼저 계산해 최솟값의 부호 반전을 피합니다.
원본 RotateLeft8의 입력에 무관한 실행시간 보장은 분기·산술로 구현한 포트에 계승하지 않습니다.
시간 간격의 두 입력은 signed int64 나노초 전체 범위입니다. 단위가 0 이하이면 원래 값을
반환하며, 정확한 중간값은 0에서 멀어지는 쪽으로 반올림합니다. 반올림 결과가 범위를
넘으면 int64 최솟값·최댓값으로 포화되므로 결과가 단위의 배수가 아닐 수 있습니다.
사각형은 Go의 `Min.X`, `Min.Y`, `Max.X`, `Max.Y`를 `min_x`, `min_y`, `max_x`, `max_y`로
표현합니다. 각 좌표는 amd64·arm64의 Go int 전체 범위를 유지합니다. 변이나 꼭짓점만
접촉하면 겹치지 않으며, 빈 교집합은 네 좌표가 모두 0입니다. 둘 다 빈 사각형이면
합집합은 두 번째 입력의 좌표를 그대로 돌려줍니다. 좌표를 정렬하거나 범위를 줄이지 않습니다.
HTTP 서버, UTF 인코딩, Go time·image 패키지 전체의 이식은 아닙니다.

## 실행하기

[브라우저 데모](https://kimjooyoon.github.io/gooo-go-ports/)는 설치 없이 21개 함수를
실행합니다. 실제 Gooo importing 예제를 고정 컴파일러로 생성하고, 생성된 Go를
한 글자도 수정하지 않은 채 WebAssembly로 빌드합니다. 입력·출력 경계의 Go bridge는
정수 범위 검사와 직렬화만 맡습니다. 임의 Gooo 소스를 브라우저에서 컴파일하지는 않습니다.
HTTP 입력은 int64, rune 입력은 int32, byte·색상 채널 입력은 0..255로 제한하며 범위 밖 입력을 거부합니다.
회전 수는 signed int64 전체 범위이며 음수는 오른쪽 회전입니다.
시간 간격과 단위도 signed int64 나노초이며 0 이하 단위를 포함합니다.
숫자 입력란의 값은 JavaScript Number를 거치지 않고 십진수 문자열로 전달합니다.
공백 붙여넣기 보조 입력은 정확히 한 코드 포인트를 숫자 입력란에 채우며, 공백 판정은 Gooo가 수행합니다.
RGB는 3개, CMYK는 4개 채널을 각각 전달하며 채널을 하나의 정수에 묶거나 잘라내지 않습니다.
결과에는 채널 이름과 정수 값이 표시됩니다. RGB 미리보기는 입력 또는 반환된 RGB를 보여주며
색상 프로파일이나 프린터 출력 보정은 수행하지 않습니다.

**Copy link to this input**으로 함수와 입력을 공유할 수 있습니다. 같은 프로그램이면
받는 브라우저에서 한 번 실행하고, 프로그램이 달라졌으면 입력을 복원한 뒤 Run을 기다립니다.
링크에는 입력 문자열, Wasm SHA-256, 원본 리비전만 담으며 결과를 저장하지 않습니다.
README만 바뀌어도 Wasm이 같으면 실행할 수 있습니다. 잘못된 링크는 실행을 막고
새 예제로 돌아갈 수 있게 표시합니다. 클립보드가 차단되면 복사할 링크를 화면에 보여줍니다.
일반 `#workbench` 링크와 기존 13개 함수의 `#gooo-share?v=1&...` 링크를 유지합니다.
색상 링크는 모든 채널을 담는 `v=2` 형식입니다. 이전 프로그램에서 만든 링크도 입력을 복원하지만,
Wasm 해시가 달라졌으면 자동 실행하지 않습니다.

데모를 소스부터 재현하려면 Git으로 받은 저장소에서 Go 1.27.2와 Node.js 22 이상을
사용합니다. 브라우저 빌드는 Git 리비전을 기록하므로 `.git`이 없는 소스 압축본은
이 절차에 사용할 수 없습니다. 배포된 browser-demo 압축본은 안의 README에 따라
`site/`를 HTTP로 제공하면 되며 컴파일러 설치는 필요하지 않습니다.

```sh
./tools/bootstrap.sh
go run ./cmd/build-demo --out out/browser
node tools/browser-smoke.cjs out/browser/site
node tools/drag-crop-browser-smoke.cjs out/browser/site
node tools/tutorial-browser-smoke.cjs out/browser/site
node tools/browser-codegen-smoke.cjs out/browser/site/codegen out/browser/codegen-evidence
node --test tools/share-link.test.cjs tools/share-app.test.cjs
python3 -m http.server 8766 --bind 127.0.0.1 --directory out/browser/site
```

마지막 명령은 Python 3가 필요하며, 실행 후 `http://127.0.0.1:8766`을 엽니다.
소스 편집기는 `/codegen/`에 있습니다. 생성 검사에서는 16개 예제와 저장되지 않은
새 편집 소스를 실제 컴파일러·WebAssembly와 비교하며, 생성한 Go는 실행하지 않습니다.
편집기만 빌드하려면 새 출력 경로와 `--codegen-only`를 사용하면 됩니다.
다른 정적 HTTP 서버를 사용해도 됩니다. `--out` 디렉터리는 새 경로여야 합니다.
여러 배포본을 로컬에서 열 때는 버전별 디렉터리와 URL을 유지하세요. 같은 URL의
파일을 교체했다면 캐시를 무시하고 새로고침해 HTML과 스크립트를 다시 받습니다.
실행 파일·메타데이터·비교 입력은 브라우저 캐시를 재사용하지 않고 요청합니다.
브라우저 비교는 기존 함수 입력 3,135개와 정밀도 경계 2개를 보존하고 색상 원본 비교 1,300개를
추가한 4,437건과 회전 원본 비교 2,472건을 합친 6,909건을 보존합니다.
시간 간격 두 함수에 각각 1,789건을 더한 **10,487건**도 유지하고, `IsSpace` **506건**을
추가한 **10,993건**을 유지합니다. `IsLetter` 4,128건과 `IsDigit` 915건을 더한
현재 브라우저 원본 비교는 **16,036건**입니다. 잘못된 입력·채널 개수 등의 거부 **420건**은 별도 집계합니다.
Node smoke 검사는 공유 입력 왕복 16,036건과 추가 호출 형태·타입 오류 63건도 검사하므로
총 거부 검사는 **483건**입니다. 아래 native 소비 예제는 브라우저 집계에 포함하지 않습니다.
HTTP·rune·색상 검사는 유한 입력 관측이며 패키지 전체 호환성이나 성능을 보장하지 않습니다.

별도 `crop-planner.html`은 기본 모드에서 기존 crop 소비 프로그램을 `crop/crop.wasm`으로 실행합니다.
Two-corner selection 모드는 별도 `drag-crop/crop.wasm`의 `DragCropPlan`을 실행하여 선택한 두 모서리만 정렬합니다.
좌표는 브라우저와 Wasm 사이에서 십진 문자열로 전달하므로 JavaScript의 안전한 정수
범위를 넘어도 원본 값을 보존합니다. `crop/` 아래에 이 프로그램의 생성 Go, 실행·재실행
기록, 독립 Go 비교 입력과 빌드 메타데이터가 있으며 `drag-crop/`에도 같은 형식의 별도 증거가 있습니다.
두 모드는 각각 Go 기준값 88건, 잘못된 문자열 96건, JS 타입 104건, 인수 개수 10건을 검사합니다.
추가로 두 Wasm 프로그램을 함께 실행해 7개 입력의 14회 호출을 비교합니다. crop smoke 결과는 기존
16,036건 비교 및 483건 거부 집계와 분리합니다.

별도 [언어 튜토리얼](https://kimjooyoon.github.io/gooo-go-ports/tutorial.html)은
점수와 이름표를 받아 네 필드를 돌려주는 두 파일짜리 Gooo 프로그램을 실행합니다.
화면에서 입력을 바꾸고 원본 소스를 읽거나 내려받을 수 있습니다. 소스 변경은
[native 실행 안내](examples/learn-gooo/README.md)를 따릅니다. 정수는 int64 전체 범위를
보존하며 Text 입력과 반환 필드의 1,024 UTF-8 바이트 제한을 각각 검사합니다.
이 예제의 실행·재실행 기록과 브라우저 비교는 기존 Go 포트 호환성 집계에 포함하지 않습니다.

브라우저 생성용 사례 **261개**도 기대 결과와 함께 native 실행·저장된 프로그램 재실행을 검증합니다.
입력 파일당 128개·32 KiB 제한을 지키도록 실제 JSON 바이트 크기로 나누며 사례를 생략하지 않습니다.
배포물의 `evidence-index.json`에 모든 배치 입력·실행·재실행 파일과 해시가 있습니다.
`execution.json`·`replay.json`은 첫 배치의 원본 바이트 별칭이며 전체 기록은 인덱스를 따릅니다.

### Full native port verification

This checks the port corpus and saved replay; it is separate from the small tutorial above.

Go 1.27.2, Git, POSIX shell이 필요합니다. `go.mod`를 인식하는 Go 설치는 필요한
도구 체인을 자동 선택할 수 있습니다. 저장소 루트에서 실행합니다.

```sh
./tools/bootstrap.sh
go run ./cmd/verify --out out/first
```

bootstrap은 고정한 컴파일러를 `.compiler/`에 받아 `.gooo`를 빌드합니다.
검증은 Go 도구 체인, 원본 소스 해시, 컴파일러 버전을 확인한 뒤 모든 입력을 실행하고,
저장된 프로그램을 다시 실행합니다. 결과는 `out/first/summary.json`, 각 함수·배치의
`execution.json`, `replay.json`, 입력 파일과 Gooo 소스에 남습니다.
`--out`에는 기존에 없는 디렉터리를 지정합니다.

### Bootstrap recovery

The current bootstrap stages downloads and builds before installing `.compiler`
and replacing `.gooo`. A failed fetch, checkout, build, or identity check leaves
the prior executable intact; correct the reported cause and rerun
`./tools/bootstrap.sh`. An installed compiler checkout must match the pinned
revision and have no modified, untracked, or ignored files. Unrecognized paths
are preserved and rejected with a diagnostic. An empty checkout left by the old
bootstrap's failed first fetch is recovered after its original Git metadata is
preserved under `.compiler-interrupted.*/compiler`.

Only one bootstrap can run at a time. Handled HUP, INT, and TERM signals clean up
its temporary files and lock; a shell may wait for its child command to stop
before running that cleanup. SIGKILL or a host crash cannot run cleanup. If
`.bootstrap.lock` remains, check its `pid` and confirm that the bootstrap and its
child commands have stopped. Inspect the directory, then **move it aside** to a
new backup path before retrying. The script does not automatically delete an
existing lock or unknown files. Do not move a lock while an install is running.

If the selected Go version differs, use `export GOTOOLCHAIN=go1.27.2` in your
shell before retrying. Bootstrap disables an enclosing Go workspace for its own
commands. Its temporary-directory utility, `mktemp`, is supplied by macOS and
Linux; the recovery regression suite additionally uses Python 3 in CI.

### Run importing examples

실제 Gooo 앱은 상태 코드를 받아 `{code, text, known}`을 만듭니다.

```sh
mkdir -p out/demo
./.gooo package execute --json --inputs examples/inputs.json \
  gooo.workspace.json > out/demo/execution.json
./.gooo package replay --json --receipt out/demo/execution.json \
  --inputs examples/inputs.json gooo.workspace.json > out/demo/replay.json
```

예제 입력 404의 실제 결과는 다음과 같습니다.

```json
{"code":404,"text":"Not Found","known":true}
```

입력만 주는 실행은 `OBSERVED`이며, 기대값을 비교한 검증 실행의 `PASS`와 구분합니다.
예제의 `examples/inputs.json`을 편집하면 다른 상태 코드도 실행할 수 있습니다.
새 모델이나 외부 추론 서비스가 필요하지 않습니다.

두 번째 앱은 바이트의 비트 길이, 1 개수, 반전 값을 레코드로 돌려줍니다.

```sh
mkdir -p out/demo
./.gooo package execute --json --inputs examples/byte-summary/inputs.json \
  byte-summary.workspace.json > out/demo/byte-summary.json
./.gooo package replay --json --receipt out/demo/byte-summary.json \
  --inputs examples/byte-summary/inputs.json byte-summary.workspace.json > out/demo/byte-replay.json
```

입력 170의 결과는 `{"length":8,"ones":4,"reversed":85}`입니다.
이 경로 역시 기대값 없는 입력 실행이며 위 원본 비교 검사와 별도로 사용할 수 있습니다.

세 번째 앱인 [Unicode inspector](examples/unicode-inspector/README.md)는 기존 UTF-8·UTF-16
포트를 함께 사용해 코드 포인트와 서로게이트 쌍을 진단합니다. 유효한 U+FFFD 문자와
잘못된 입력쌍의 대체 결과를 구분하고, 정상 BMP 문자에서 `EncodeRune`이 반환하는
`65533/65533`을 잘못된 문자로 판정하지 않습니다. 실행 명령과 결과 필드는 예제 문서에 있습니다.

네 번째 앱인 [색상 왕복 검사기](examples/color-inspector/README.md)는 RGB를 CMYK로
변환하고 다시 RGB로 복원해 채널별 차이와 일치 여부를 보여줍니다. 두 색상 포트를
Gooo에서 조합하며 원본의 전체 uint8 입력 범위를 유지합니다. 브라우저에서는 두 변환 함수를
각각 실행할 수 있고, 이 왕복 소비 앱은 native에서 실행합니다. 특정 색상 프로파일이나
프린터 보정은 수행하지 않습니다.

다섯 번째 앱인 [바이트 회전 검사기](examples/byte-rotation/README.md)는 바이트와 회전 수를
받아 회전 결과, 원상 복원 값과 전후 1비트 개수를 돌려줍니다. 최소 int64의 회전 수도
그대로 입력받으며, byte로 잘라내거나 원래 회전 수를 부호 반전하지 않습니다.
브라우저에서는 단일 회전 함수를 실행하고 입력·출력의 8비트를 비교할 수 있습니다.

여섯 번째 앱인 [시간 간격 조정기](examples/duration-quantizer/README.md)는 나노초 단위의
시간 간격과 단위를 받아 반올림·절삭 결과, 반올림 변경량과 절삭 나머지를 반환합니다.
두 Go Duration 메서드의 전체 int64 입력 범위를 보존하고, 시계나 외부 서비스를
호출하지 않습니다. 브라우저에서는 각 함수를 별도로 실행하고 정확한 입력을 공유합니다.

일곱 번째 앱인 [공백 검사기](examples/whitespace-inspector/README.md)는 하나의 정수를 받아
Go rune 영역 여부, Unicode 스칼라 유효성, Go·ASCII 공백 여부, UTF-8 바이트 수와
UTF-16 코드 단위 수를 반환합니다. `IsSpace`와 기존 UTF 포트를 Gooo에서 조합합니다.
브라우저에는 단일 `IsSpace` 함수와 한 코드 포인트 붙여넣기 보조 입력을 제공합니다.
문자열을 잘라내거나 정규화하지 않으며 여러 코드 포인트를 하나로 간주하지 않습니다.

여덟 번째 앱인 [식별자 문자 분류기](examples/identifier-rune/README.md)는 문자·십진 숫자·
밑줄을 구분합니다. 아홉 번째 앱인 [잘라내기 영역 계산기](examples/crop-planner/README.md)는
이미지 경계와 요청 사각형을 받아 교집합 좌표와 픽셀 존재 여부를 반환합니다.
`RectangleIntersect`와 `RectangleEmpty`를 가져오며, 원점이 0이 아닌 이미지 경계도
그대로 처리합니다. 계산할 좌표를 반환하는 예제이며 이미지 파일을 읽거나 수정하지 않습니다.

열 번째 앱인 [단순 대소문자 접기 비교](examples/simple-fold/README.md)는 두 rune의
다음 순환 멤버와 같은 접기 순환 여부를 반환합니다. 열한 번째 앱인
[YCbCr 왕복 검사기](examples/ycbcr-inspector/README.md)는 RGB를 Y′CbCr로 변환한 뒤
복원 RGB·채널 차이·정확 일치 여부를 반환합니다. 두 앱 모두 native 실행과 저장 재실행을 제공합니다.

## 다른 Gooo 프로그램에서 가져오기

[새 프로젝트에서 포트 사용하기](docs/use-in-your-project.md)는 고정 소스를 받고
포트·라이선스를 복사한 뒤, 아래 함수의 workspace·입력 파일을 만들어 실제 실행과
저장 재실행까지 진행하는 완전한 예제입니다. 새 앱은 이 저장소와 별도 디렉터리에 둡니다.

[문자·십진 숫자 분류를 새 프로젝트에서 사용하기](docs/use-unicode-in-your-project.md)는
`unicode.IsLetter`·`IsDigit`를 조합하는 예제입니다. 필요한 두 소스 파일과 타입 선언,
원본 정수 입력의 범위 구분, 실행·저장 재실행을 고정 버전으로 따라 할 수 있습니다.

```gooo
package checker
namespace checker
import utf8 "go/unicode/utf8"
entity Integer id "gooo://std/integer"
entity Boolean id "gooo://std/boolean"
activity Valid(Integer) -> Boolean id "gooo://example/checker/valid" computes `return utf8.ValidRune(input)`
```

원하는 포트 소스를 고정 Git 리비전에서 가져와 작업공간 안에 복사하고, 라이선스·NOTICE·
upstream 기록과 원본 주석을 함께 보존합니다. 사용하는 작업공간 명세에 패키지 경로와
manifest 기준 상대 파일 경로를 등록합니다. import 자체가 패키지를 내려받지는 않습니다.
기본 실행 구성은 [gooo.workspace.json](gooo.workspace.json),
실제 importing app은 [response.gooo](examples/response.gooo)에 있습니다.
모든 검증 대상의 소스 목록은 [verification-catalog.json](verification-catalog.json)에 있고,
YCbCr 소비 앱은 [별도 workspace](ycbcr-inspector.workspace.json)로 실행합니다.
비트 연산을 가져오는 예제는 [ByteSummary](examples/byte-summary/main.gooo)와
회전 수를 함께 받는 [Byte rotation inspector](examples/byte-rotation/README.md)입니다.
색상 변환 조합은 [Color inspector](examples/color-inspector/main.gooo)와
[color-inspector.workspace.json](color-inspector.workspace.json)에 있습니다.
현재 배포 단위는 Gooo 소스와 workspace 명세이며 Go의 패키지 관리자와는 별개입니다.

## 검증 범위

기존 **3,147건**과 Unicode 소비 앱의 Go 원본 비교 **334건**·영역 밖 거부 **12건**을
그대로 유지합니다. 색상 포트는 원본 비교 **1,300건**·거부 **56건**, 색상 소비 앱은
원본 비교 **637건**·거부 **24건**을 추가합니다. 이 **5,510건**은 바뀌지 않습니다.
바이트 회전 포트는 원본 비교 **2,472건**·거부 **36건**, 회전 소비 앱은 원본 비교 **672건**·
거부 **36건**을 추가한 **8,726건**도 유지합니다. 시간 간격 두 포트의 원본 비교
**3,578건**, 소비 앱 **1,789건**을 더한 **14,093건**도 유지합니다. 공백 포트와 소비 앱은
각각 원본 비교 **506건**·int32 밖 확장 **8건**을 추가한 **15,121건**도 유지합니다.
문자·숫자 분류 두 포트는 원본 비교 **5,043건**·int32 밖 확장 **24건**, 식별자 문자 소비 앱은
원본 비교 **4,570건**·확장 **12건**을 추가한 **24,770건**도 유지합니다.
사각형 네 포트의 원본 비교 **1,711건**과 잘라내기 소비 앱 **486건**을 추가해,
기존 native 검사 **26,967건**도 유지합니다. SimpleFold는 원본 비교 **3,505건**과
int32 밖 identity 확장 **11건**, 두 rune 소비 앱은 원본 비교 **497건**과 영역 밖
거부 **34건**을 추가한 기존 **31,014건**도 유지합니다. YCbCr 두 포트는 원본 비교
**2,160건**과 영역 밖 거부 **48건**, 왕복 소비 앱은 원본 비교 **924건**과 거부 **24건**을
추가한 기존 **34,170건**을 유지합니다. Canon **371건**과 드래그 선택 소비 활동
**501건**을 더한 기존 **35,042건**도 유지합니다. 여기까지가 표준 라이브러리 29개 포트이며,
x/image 두 포트와 픽셀 경계 소비 예제의 별도 2,190건을 더한 기존 **37,232건**도 유지합니다.
Unicode case mapping 네 포트의 원본 비교 **16,208건**·별도 확장 **198건**, 소비 앱의
원본 비교 **3,602건**·확장 **11건**을 더한 기존 **57,251건**도 유지합니다.
잘못된 case 선택자의 원본 비교 132건·확장 132건은 각각 위 포트 집계에 이미 포함됩니다.
NRGBA 포트의 원본 비교 **2,696건**·영역 밖 거부 **32건**, 알파 합성 소비자의 원본 비교
**2,158건**·거부 **104건**을 더한 기존 **62,241건**은 그대로 유지합니다. 합성 기대값은 실제
Go `draw.Draw`의 NRGBA→RGBA 최적 경로와 일반 인터페이스 경로를 함께 확인합니다.
바이트 검색 포트와 소비 예제는 각각 문자열 6개와 전체 byte 256개의 원본 비교
**1,536건**과 needle 영역 밖 확장 **16건**을 추가합니다. native 검증 코퍼스는
기존 **65,345건·54개 suite**를 이룹니다. 기존 833개 배치 경계는 유지하며, 이 두 suite는
실제 직렬화 JSON 30,000바이트와 128행 상한을 함께 적용해 전체 **882배치**로 실행합니다.
범위는 유효한 UTF-8 Text 최대 1,024바이트이며 임의 Go 바이트 문자열 전체가 아닙니다.
Google UUID `Validate`와 가져오기 예제는 각각 **21,745건**의 독립 원본 비교를 더합니다.
원본 9개 사례, 네 표기의 각 ASCII 위치 변경, 128개 URN 대소문자 조합,
0..1,024바이트 길이, Unicode·제어 문자와 접두 바이트 절단, 고정 표본을 포함합니다.
정확한 오류 문구와 길이 오류 분류까지 비교하는 이 단계의 합계는 **108,835건·56개 suite**입니다.
기존 54개 suite의 입력·소스 묶음과 882배치 경계를 유지하고 UUID 534배치를 추가합니다.
JSON의 잘못된 UTF-8·단독 surrogate가 원래 바이트 그대로 거부되거나 보존된다는 뜻은 아닙니다.
원래 유효한 UTF-8 문자열로 오라클을 호출하며, 일반 Go byte string·전체 UUID 패키지는 범위 밖입니다.

별도 `ParseCanonical` Text 조합 API는 원본 `Parse`의 반환 UUID를 `String()`으로 표현하고,
오류 시 부분 값과 오류 상태를 함께 보존합니다. 기존 UUID 입력 **21,745건**에
추가 anchor 136개의 중복을 제거해 **122건**을 더한 **21,867건**을 비교합니다.
38바이트 외곽 문자, 실패한 hex 쌍의 부분 반환, hyphen 오류 우선순위, UTF-8 접두 인용,
길이와 byte 패턴을 포함하며 이 단계의 corpus 합계는 **130,702건·57개 suite**입니다.
Go의 binary UUID 타입이나 임의 byte 입력 `ParseBytes`를 노출하는 API는 아닙니다.

새 `RectangleIn`과 [FitsCrop](examples/fits-crop/README.md)는 각각 기존 사각형 이항 입력
**486건**과 추가 입력 **84건**을 합친 **570건**을 참조 코퍼스로 사용합니다.
`RectangleIn(requested, bounds)`는 빈 요청에도 true를 반환하지만, 소비 앱은
`!requested.Empty()`를 먼저 요구하므로 빈·뒤집힌 요청을 false로 판정합니다.
네 경계의 초과, 포함·접촉·분리, 동일한 exclusive 최댓값, 빈 컨테이너와 int64·정밀도 경계를
다룹니다. **131,842건·59개 suite**의 전체 native 비교가 일치했고 **1,777개 배치**의
저장재실행과 독립 raw receipt 감사를 확인했습니다. [실행 증거](evidence/rectangle-in-fits-crop-20261012.json)에
새 두 suite의 각 570건·18배치와 별도 거부 관찰을 기록합니다. 이미지나 GIF를 처리하는 기능은 아닙니다.
기존 57개 suite의 case-document 바이트·SHA와 1,741개 배치 경계는 유지합니다.
공유 `geometry.gooo`에 선언을 추가하여 소스 해시는 바뀝니다. 기존 활동의 ID·body를
보존하고 새 소스 식별자로 실행·저장재실행을 검사하며, 이전 receipt identity를 재사용하지 않습니다.

사각형의 네 좌표는
각각 전체 signed int64 범위를 받으며 영역 밖 확장 계약이 없습니다.
시간 간격은 두 입력의 전체 int64 범위를 받으며 영역 밖 확장 계약이 없습니다.
기존 영역 밖 거부 사례는 Go 동등성 집계가 아닙니다.

| 대상 | 입력 수 | 구성 |
| --- | ---: | --- |
| Strings IndexByte | 1,536 + 16 | 빈·반복·ASCII·NUL·다중 바이트·1,024바이트 문자열 × byte 전체, 별도 needle 영역 밖 확장 |
| Byte search Search consumer | 1,536 + 16 | 원본 byte offset과 found·in_byte_domain·byte_length를 가져오기 예제로 비교 |
| HTTP StatusText | 625 | -10..610 전체 정수와 int64 양 끝 등 네 입력 |
| UTF-8 ValidRune / RuneLen | 각 162 | 경계값 34개와 고정 시드 표본 128개 |
| UTF-8 RuneStart | 256 | byte의 전체 영역 |
| UTF-16 IsSurrogate / RuneLen / EncodeRune | 각 162 | 같은 rune 경계값·표본 |
| UTF-16 DecodeRune | 164 | 경계값 조합 100개와 유효 쌍 64개 |
| HTTP importing app | 6 | 기존 상태 코드와 미등록 코드 |
| math/bits 다섯 함수 | 각 256 | byte의 전체 영역, 함수마다 한 번씩 |
| ByteSummary importing app | 6 | 0, 1, 127, 128, 170, 255의 길이·1 개수·비트 반전 레코드 |
| Unicode InspectScalar consumer | 164 + 4 | 기존 rune 162개, A·U+1F600, 별도 int32 영역 밖 거부 4개 |
| Unicode InspectPair consumer | 170 + 8 | 기존 pair 164개, 대체 문자·U+1F600·잘림 경계 6개, 양쪽 인수의 영역 밖 거부 8개 |
| Color RGBToCMYK | 637 + 24 | RGB 경계·회색·최댓값 동률·정수 나눗셈·고정 표본, 각 입력 위치의 별도 거부 |
| Color CMYKToRGB | 663 + 32 | CMYK 경계·정수 나눗셈·고정 표본, 각 입력 위치의 별도 거부 |
| Color InspectRGB consumer | 637 + 24 | RGB→CMYK→RGB 결과·채널 차이·일치 여부, 범위 밖 거부 |
| bits.RotateLeft8 | 2,472 + 36 | 256바이트 × 8회전 2,048쌍, 원래 회전 수의 음수·int64 경계·표본, 별도 바이트 영역 밖 거부 |
| Byte rotation Inspect consumer | 672 + 36 | 모든 byte의 1회전, 회전 수 경계·표본, 원상 복원·1비트 개수와 거부 계약 |
| DurationRound / DurationTruncate | 각 1,789 | 작은 정수 격자, 두 int64 인수의 경계, 정확한 배수·중간값·포화 전후, 고정 표본 |
| Duration quantizer Inspect consumer | 1,789 | 같은 원래 입력으로 반올림·절삭·변경량·나머지 레코드 비교 |
| unicode.IsSpace | 506 + 8 | Latin-1 전체, 참인 25개 값과 이웃, 혼동하기 쉬운 문자·int32 경계·고정 표본, 별도 int32 밖 확장 |
| Whitespace Inspect consumer | 506 + 8 | 같은 입력으로 rune 영역·스칼라·Go 및 ASCII 공백·UTF 길이 레코드 비교 |
| unicode.IsLetter | 4,128 + 12 | Latin-1 전체·647범위의 양끝·이웃·stride 멤버와 구멍·원래 int32 경계·표본, 별도 확장 |
| unicode.IsDigit | 915 + 12 | Latin-1 전체·72범위의 양끝·이웃·Nd와 다른 숫자 구분·경계·표본, 별도 확장 |
| Identifier rune Classify consumer | 4,570 + 12 | 두 포트 입력 합집합으로 식별자 시작·계속 문자·decimal digit과 rune 영역 레코드 비교 |
| RectangleEmpty | 253 | 작은 좌표 격자, 네 필드 각각의 int64·정밀도 경계, 극값 사각형, 고정 표본 |
| RectangleOverlaps / RectangleIntersect / RectangleUnion | 각 486 | 작은 좌표 조합, 포함·접촉·분리·다른 빈 표현의 양방향 입력, 여덟 좌표 각각의 경계와 고정 표본 |
| CropPlan consumer | 486 | 같은 원래 사각형 두 개로 교집합의 네 좌표·픽셀 존재 여부 비교 |
| RectangleCanon | 371 | 기존 단항 좌표 격자·경계·표본과 모든 관측된 canonical 결과를 다시 입력하여 멱등성 비교 |
| DragCropPlan | 501 | 기존 두 레코드 격자·경계·표본과 네 드래그 방향, clipping·접촉·퇴화·int64 경계 |
| RectangleIn | 570 | 기존 사각형 이항 입력 486건과 추가 84건; 원래 receiver의 빈·뒤집힌 상태, 네 경계와 동일한 exclusive 최대 좌표를 비교 |
| FitsCrop consumer | 570 | 같은 bounds/requested 쌍에서 비어 있지 않은 요청의 완전 포함만 허용; clipping·정규화 없이 독립 Go 조건식으로 비교 |
| unicode.SimpleFold | 3,505 + 11 | 모든 변경 코드 포인트, Latin-1, 원본 case range 경계, invalid Unicode·int32 경계·표본, 별도 int64 identity 확장 |
| Simple-fold Compare consumer | 497 + 34 | 모든 3/4-member 순환의 순서 있는 쌍, 이름 있는 고정점·2-member 순환, 다른 순환·invalid 값·표본, 별도 영역 밖 거부 |
| Color RGBToYCbCr | 924 + 24 | RGB 경계 격자·모든 회색·반올림 경계·고정 표본, 별도 영역 밖 거부 |
| Color YCbCrToRGB | 1,236 + 24 | Y′CbCr 경계 격자·회색·채널 포화 전환·고정 표본, 별도 영역 밖 거부 |
| YCbCr InspectRGB consumer | 924 + 24 | RGB→Y′CbCr→RGB 결과·채널 차이·일치 여부, 별도 영역 밖 거부 |
| unicode.To | 8,170 + 165 | 세 case의 모든 변경 rune와 범위 경계·교대 parity·int32 표본, invalid case와 별도 int64 rune 확장 |
| unicode.ToUpper / ToLower | 각 2,691 + 11 | 각 변환의 모든 변경 rune, Latin-1·case range 경계·잘못된 Unicode·정수 경계·표본, 별도 int64 identity 확장 |
| unicode.ToTitle | 2,656 + 11 | 별도 title delta를 포함한 전체 변경점과 같은 경계 범위, 별도 int64 identity 확장 |
| Case inspector Describe consumer | 3,602 + 11 | 원입력·세 변환·int32 범위와 scalar 유효성을 독립 Go 함수로 비교, 별도 int64 identity 확장 |
| Color NRGBAToRGBA | 2,696 + 32 | 알파 경계·전체 알파 램프·채널별 변화·정수 버림·숨은 RGB·고정 표본, 각 수신 필드의 별도 영역 거부 |
| Alpha matte MattePixel consumer | 2,158 + 104 | 원본 NRGBA와 불투명 배경 RGB, Go draw.Over의 두 경로·중간 16비트·최종 8비트 결과, 단일·복합 필드 영역 거부 |

별도 `go run ./cmd/check-unicode --out out/unicode-exhaustive`는 실제 importing 프로그램의
수정하지 않은 생성 Go를 빌드해 **0..U+10FFFF의 1,114,112개 입력**에서 두 분류 함수를
**2,228,224번** Go 원본과 비교하고 소비 레코드도 검사합니다. 이 수치는 위 native 영수증
집계에 합산하지 않습니다. signed int32 나머지 구간과 int32 밖 확장은 유한 경계 검사이며,
signed int32 전체 전수 검사 또는 전체 Unicode 패키지 호환성을 주장하지 않습니다.
재생성 방법과 API 의미는 [식별자 문자 예제](examples/identifier-rune/README.md)에 있습니다.

`cmd/verify`는 SimpleFold의 검증된 실행·재실행에서 생성 Go를 한 바이트도 바꾸지 않고
별도 검사기로 빌드합니다. **0..U+10FFFF의 1,114,112개 입력**을 원본 SimpleFold와
비교하며 결과를 `simple-fold-exhaustive/`와 요약의 `simple_fold_exhaustive`에 기록합니다.
이 검사는 위 유한 영수증 집계와 별개이고, 전체 int32 또는 모든 두-rune 쌍을
전수 검사했다는 뜻이 아닙니다. 정수 영역 밖 샘플도 따로 집계합니다.

case mapping 네 포트도 검증된 생성 Go를 수정 없이 실행합니다. `To`의 세 유효 case와
`ToUpper`·`ToLower`·`ToTitle` 각각에 대해 **0..U+10FFFF**를 비교한 총 **6,684,672번**의
관측은 `case_mapping_exhaustive`에 따로 기록합니다. 다른 int32 값·잘못된 case·int64 확장은
별도의 유한 비교이며, 소비 앱 전체 입력이나 모든 int32·case 선택자를 전수 검사한 것은 아닙니다.

YCbCr 두 변환도 검증된 실행·재실행의 생성 Go를 그대로 빌드해 각 **16,777,216개**
byte 3성분 입력을 Go 원본과 비교합니다. 총 **33,554,432번**의 비교는 위 영수증
사례 수와 따로 집계하며, 요약의 `ycbcr_exhaustive`와 각 `ycbcr-exhaustive-*` 폴더에
생성 소스 해시·도구체인·결과를 기록합니다. 소비 앱의 모든 RGB 왕복을 실행하거나
이미지 압축·색상 프로파일을 검증한 것으로 해석하지 않습니다.

Gooo 실행 입력 상한에 맞춰 현재 catalog를 총 **1,777개 배치**로 나눕니다. 기존 833개 배치와
그 경계를 유지하며 IndexByte 21개·Search 소비 앱 28개 배치를 추가합니다.
IndexByte·Search와 UUID의 세 suite는 128행 이하에서 실제 JSON 30,000바이트 상한에 맞춰 최대 접두 배치를 선택합니다.
UUID Validate 257개·Inspect 277개 배치를 추가하며 이전 882배치의 경계는 바뀌지 않습니다.
ParseCanonical을 추가했던 단계에서는 325개 배치를 더하고 기존 1,416개 배치의 입력·소스·경계를 유지했습니다.
RectangleIn과 FitsCrop은 각각 18개 배치를 더하며 기존 1,741개 배치의 case JSON 바이트와
행 경계를 보존합니다. 공유 사각형 소스의 새 해시는 새 실행·저장재실행에 연결합니다.
기존 YCbCr까지의 465개 배치를 유지하고,
Canon 6개·드래그 선택 소비 활동 16개, x/image Floor/Ceil 각각 14개와 픽셀 경계 소비 활동
16개 배치까지의 기존 531개를 보존합니다. case mapping 네 포트 131개와 새 소비 앱 57개
배치까지의 기존 719개를 보존합니다. NRGBA 포트 43개와 알파 합성 소비 앱 71개 배치를 추가합니다.
case mapping 포트는 배치당 128건 이하, 소비 앱은 64건 이하입니다.
Unicode 앱과 색상 포트는 64건 이하, 색상 소비 앱은 32건 이하입니다.
사각형의 단항 연산은 64건 이하, 이항 연산과 소비 앱은 32건 이하입니다.
새 레코드 기대값도 들여쓴 JSON과 마지막 개행을 포함해 32KiB 한도를 검사합니다.
원본 비교 기대값은 Gooo 구현과 별개인 Go 표준 함수 및 고정한 x/image 원본 메서드로 계산합니다.
x/image 입력 영역 밖 false/0 레코드와 소비자 거부는 별도 계약으로 검사합니다.
Unicode·색상·회전·공백 앱과 색상·회전·공백 포트의 영역 밖 기대값은 명시한 false/-1 계약으로 따로 검사합니다.
검증 도구는 결과뿐 아니라 입력 순서, 원본 기대값, 함수·입력 타입 식별자,
생성 코드 해시, 컴파일러·Go 버전, workspace·입력 파일 해시, 저장 재실행 연결을 검사합니다.
함수 식별자는 실행 기록 밖의 [명시적 계약](cmd/verify/contracts.go)을 기준으로
선택한 패키지 선언, 생성 보고서, 각 실행 결과에 각각 대조합니다. 여러 입력은 전달된
타입 식별자도 검사하며, 타입 식별자가 없는 단일 입력 기록은 패키지 선언의 입력 타입과 연결합니다.
원본 파일의 내용과 패키지 소속은 실행 전에 읽은 소스에 대조하고, 생성 Go의 실제 내용도
기록된 해시와 비교합니다. [브라우저 빌드 검사](cmd/build-demo/evidence.go)도 독립적인
Dispatch 계약과 입력 타입·소스 연결을 확인합니다.
사각형 입력의 네 필드는 원래 정수 값과 필드 식별자·순서를 함께 비교합니다.
`python3 tools/check-rectangle-inputs.py out/rectangle-input-rejections`는 양쪽 인수에서
누락·문자열·소수·int64 초과·null·추가 필드·중복 JSON 키의 거부 18건을 재현합니다.
이는 컴파일러 입력 검증이며 위 Go 원본 비교 수나 영역 밖 확장 수에 합산하지 않습니다.
새 `tools/check-fits-crop.py`는 문서의 14개 input-only 기록을 원래 int64 입력·Boolean 값·
소스/프로그램/producer와 저장재실행에 연결합니다. 이 기록의 판정은 `OBSERVED`이며
기대값을 공급하지 않은 입력에 correctness score를 만들지 않습니다.
`tools/check-fits-crop-inputs.py`는 별도 90개 malformed 입력을 실제 CLI에 전달하도록 구성되어
있습니다. 반환된 진단·stage·counter만 보존하며, runtime 정보가 없다고 모델 호출이나
native 실행 횟수를 0으로 간주하지 않습니다. 이 거부 관찰도 위 원본 비교 수에 합산하지 않습니다.
검사 도중 소스가 바뀌어도 처음 읽은 동일한 소스 스냅샷을 모든 배치에 사용합니다.
고정 컴파일러는 한 생성 프로그램의 레코드 타입을 16개로 제한하므로, 검증기는
전체 소스 목록에서 공통 포트와 해당 배치의 소비 앱만 작업공간에 넣습니다.
실행에 포함된 소스의 식별자·해시도 이 선택된 목록에 정확히 대조합니다.
사례나 이식 함수의 입력 범위를 줄여 한도를 피하지 않습니다.
보고서·선언·실행 결과의 식별자를 함께 바꾸거나 입력 타입·소스 소속을 바꾼 기록을
거부하는 회귀 테스트도 포함합니다.

`cmd/verify`, `cmd/build-demo`, `cmd/check-unicode`의 영수증 검사는 각 JSON 객체에서
이스케이프를 해석한 키 이름이 두 번 나오면 거부합니다. 값이 같거나 배열 안의 객체에
중복이 있어도 거부하며, 서로 다른 객체가 같은 키를 쓰는 것은 허용합니다.
키의 대소문자 통합이나 Unicode 정규화는 하지 않고, 알 수 없는 유일한 필드에 대한
기존 정책과 정수 정밀도, 원본 영수증 바이트·해시 연결을 유지합니다.
정책 예제의 저장된 실패 영수증과 `check-crop-example.py`, `check-rectangle-inputs.py`의
영수증 읽기에도 중복 키 거부를 적용합니다. 위의 컴파일러 입력 거부 18건과는 별도 검사입니다.
CI의 회전·시간 반올림·공백 관측 예제도 같은 Python 영수증 디코더를 사용합니다.

`RuneStart`와 `math/bits`의 기존 단일 입력 다섯 함수는 Go byte 영역을 전수 비교합니다.
`RotateLeft8`의 전수 비교는 byte × 회전 수 0..7의 2,048쌍입니다. 원래 signed int64
회전 수 전체를 실행한 것은 아니며, 나머지는 음수·양 끝·큰 정수 경계와 고정 표본입니다.
YCbCr 두 함수는 위 별도 검사에서 각 byte 3성분 영역을 전수 비교합니다.
그 밖의 함수는 모든 입력을 검사한 것이 아닙니다. 여러 함수가 같은 정수 입력을 재사용하므로
이 사례 수는 전역적으로 서로 다른 정수 값의 개수가 아닙니다.
별도 모델 학습, 성능 우위, 패키지 전체의 호환성은 검증 범위에 포함하지 않습니다.
[첫 8개 함수의 관측](evidence/initial.json), [13개 함수의 관측](evidence/math-bits-20261010.json),
[Unicode 소비 앱 추가 관측](evidence/unicode-inspector-20261010.json),
[색상 포트·소비 앱 추가 관측](evidence/color-ports-20261010.json),
[바이트 회전 포트·소비 앱 추가 관측](evidence/byte-rotation-20261010.json),
[시간 간격 포트·소비 앱 추가 관측](evidence/duration-20261010.json),
[공백 포트·소비 앱 추가 관측](evidence/whitespace-20261010.json),
[사각형 포트·소비 앱 추가 관측](evidence/image-rectangles-20261011.json),
[YCbCr 포트·소비 앱과 전수 비교 관측](evidence/ycbcr-20261011.json),
[릴리스별 실행·재실행 기록](https://github.com/kimjooyoon/gooo-go-ports/releases)을
참고하세요. 릴리스 기록은 해당 버전에 고정되어 있으며, 이후 변경의 기록은 각 GitHub
Actions 실행에서 확인할 수 있습니다.

## 언어 개발로 돌려주는 자료

```sh
./tools/probe.sh --out out/probes
```

반복문·바이트 인덱싱·비트 연산의 작은 Gooo 예제는 선언 검사를 통과하지만
본문 생성 결과는 서로 다릅니다. 현재 고정 컴파일러에서 조건만 사용하는
`loop.gooo`는 지원되는 카운터 반복 형식이 아니므로 거부되고,
`bitwise.gooo`는 `&` 연산자 때문에 거부됩니다. 반면 `byte-index.gooo`는
생성에 성공하지만 비어 있지 않은 Text를 전제로 하는 `input[0]` 접근을
그대로 보존합니다. 따라서 생성 성공만으로 인덱스 안전성이나 실행 정확성이
입증되는 것은 아닙니다.

[현재 generation boundary 표](probes/README.md#current-generation-boundaries)는
이 세 fixture의 선언 검사·본문 생성 결과와 진단을 함께 기록합니다. 스크립트는
모든 fixture가 실패한다고 가정하지 않고 이 기대 결과를 검사하며, 컴파일러 지원이
바뀌면 조용히 지나가지 않고 검사가 실패하도록 합니다.

- [언어 분석](docs/language-assessment.md): 실제 지원 범위, API 변환과 제약
- [다음 이식 로드맵](docs/roadmap.md): 문자열 순회, hex, google/uuid의 선행 조건
- [기여 방법](CONTRIBUTING.md): 원본 추적과 동작 비교를 포함한 이식 절차
- [경계 재현](probes/README.md): 컴파일러 개선에 사용할 최소 예제
- [바이트 입력 조건 실험](docs/research/20261010-source-conditions/README.md): 개발 컴파일러에서 같은 최종 출력과 다른 내부 조건을 구분한 재현 자료. 이 실험 자체는 기본 컴파일러 핀이나 포트 수를 늘리지 않습니다.

## 원본과 라이선스

[upstream.json](upstream.json)에 Go **1.27.2**의 커밋
`022c8636110ebac86a9b88724cda168ed713c21f`, 파일 SHA-256,
Gooo **0.6.32-dev** 컴파일러 커밋
`83928fa4538245abe33b08549da47f8cbdc50ed2`을 고정했습니다.
테스트의 Go 도구 체인에서도 원본 파일 해시가 일치해야 합니다.

BSD 3-Clause 라이선스입니다. Go 원본 저작권과
[원본 라이선스](third_party/GO-LICENSE), [NOTICE](NOTICE)를 보존합니다.
기존 [Gooo standard library](https://github.com/kimjooyoon/gooo-standard-library)의
기초 숫자·논리·문자열 함수와 함께 사용할 수 있도록 별도 패키지 경로를 사용합니다.

## x/image 고정소수점 경계 → 픽셀 사각형

[작은 native 예제](examples/fixed-pixel-bounds/README.md)는 raw Q26.6 좌표를
Floor/Ceil로 바꾸고 기존 RectangleIntersect로 잘라냅니다. 기존 35,042건은 그대로
유지하며 포트 참조 1,664건·영역 밖 거부 16건, 소비자 참조 471건·raw 영역 거부
33건·Ceil overflow 거부 6건을 더합니다. 이 단계까지의 finite 코퍼스 37,232건·45개 suite,
31개 포트는 보존됩니다. 픽셀 경계는 12번째 소비 예제이며, 이후 case mapping 네 포트와
13번째 소비 예제까지의 총계는 35개 포트·15개 소비 활동입니다. 이후 NRGBA 변환과
알파 합성 예제까지의 36개 포트·14개 소비 프로그램·16개 소비 활동은 유지하며,
바이트 검색까지의 37개 포트·15개 소비 프로그램·17개 소비 활동을 유지합니다.
Google UUID Validate와 importing inspector까지의 38개 포트·16개 소비 프로그램·18개 소비 활동을 유지합니다.
별도 ParseCanonical Text API까지의 39개 포트·16개 소비 프로그램·18개 소비 활동을 유지합니다.
RectangleIn과 FitsCrop을 추가한 현재 총계는 40개 포트·17개 소비 프로그램·19개 소비 활동입니다.
원본 int32 전체의 동작을 구현하되 2^32 전수 검사를 주장하지 않습니다.
브라우저 21함수와 두 crop 모드는 그대로입니다.
