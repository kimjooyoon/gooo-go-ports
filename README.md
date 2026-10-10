# Gooo Go Ports

Go standard-library functions implemented in [Gooo](https://github.com/kimjooyoon/meta-ontology-go),
with independent Go reference outputs and saved replay. The current scope is
15 HTTP, Unicode, 8-bit integer, and color-conversion functions, four native importing examples,
and an importing browser dispatcher.
Use Go 1.27.2, Git, and a POSIX shell: run `./tools/bootstrap.sh`, then
`go run ./cmd/verify --out out/first` from this repository. The output directory must be new.
The original 3,147-case corpus is unchanged. A Unicode inspector adds 334 Go-reference
cases and 12 explicit domain-rejection cases; six byte-domain functions cover all 256 inputs.
Color ports and their consumer add separate reference and rejection cases, detailed below.
Other domains use finite boundary cases and samples. This is an independent experimental port,
not a complete Go package implementation or an official Google/Go project.

[Try the browser demo](https://kimjooyoon.github.io/gooo-go-ports/) to run 13 HTTP, Unicode, and byte functions
without installing a compiler. It executes a precompiled Gooo program as WebAssembly;
it is not an editor for compiling arbitrary source. The page includes the Gooo source,
unmodified generated Go, execution receipts, and an optional reference-check button.
Use **Copy link to this input** to share an exact function input. Opening a link runs
it once in the recipient's browser when the compiled program matches; a changed program
restores the inputs and waits for **Run function**. Links carry inputs, not trusted results.
For color conversion, the native [RGB → CMYK → RGB inspector](examples/color-inspector/README.md)
reports converted channels, recovered RGB values, and any integer round-trip differences.

For a fixed snapshot, download [experimental release v0.1.0-preview.1](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.1).
It includes separate source, native execution/replay evidence, and standalone browser
archives, with licenses, provenance, and checksums. Downloads do not require a GitHub
login. The release page explains which archive to choose and how to verify it;
the live demo and main branch can be newer than this snapshot.

If loading or running fails, [report a problem](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
with the page URL or command and what happened. A working demo or shared input link
is not required. See [reporting guidance](CONTRIBUTING.md#report-a-problem) for useful details.

Go로 작성된 오픈소스의 동작을 **Gooo 함수로 이식하고 원본과 비교**하는 공개 실험 저장소입니다.
현재 Go 표준 라이브러리의 HTTP·Unicode·8비트 정수·색상 변환 함수 15개를 제공합니다.
Go 소스를 분석해 타입·경계값·실패 동작을 명시하고, 계산은 `ports/**/*.gooo`에 구현합니다.
Go 검증 도구는 원본 함수 호출, 입력 전달, 결과 비교와 실행 기록 저장을 맡습니다.

[Gooo 컴파일러](https://github.com/kimjooyoon/meta-ontology-go)를 사용하는 독립 생태계 프로젝트입니다.
현재 언어로 동작하는 라이브러리와 다음 이식을 막는 최소 재현 사례를 함께 쌓아
언어 개발에 간접적으로 기여합니다. Google·Go 프로젝트의 공식 이식판은 아닙니다.

## 이식한 함수

| Gooo 패키지 경로 | 함수 | 원본과의 대응 |
| --- | --- | --- |
| `go/net/http` | `StatusText` | Go의 상태 코드 62개와 설명. 미등록 코드는 빈 문자열 |
| `go/unicode/utf8` | `ValidRune`, `RuneLen` | Unicode 유효 코드 포인트 판정과 UTF-8 바이트 수 |
| `go/unicode/utf8` | `RuneStart` | 0..255의 바이트가 continuation byte가 아닌지 판정. 전체 문자열 유효성 검사는 아님 |
| `go/unicode/utf16` | `IsSurrogate`, `RuneLen` | 서로게이트 판정과 UTF-16 코드 단위 수 |
| `go/unicode/utf16` | `DecodeRune`, `EncodeRune` | 서로게이트 쌍 변환. EncodeRune의 두 반환값은 `{high, low}` 레코드 |
| `go/math/bits` | `Len8`, `LeadingZeros8`, `TrailingZeros8`, `OnesCount8`, `Reverse8` | 0..255 정수의 비트 길이, 앞·뒤 0 개수, 1 개수와 비트 순서 반전. 비교·산술로 구현 |
| `go/image/color` | `RGBToCMYK`, `CMYKToRGB` | 각 채널 0..255의 RGB 3성분·CMYK 4성분 변환. 다중 반환은 `CMYK`·`RGB` 레코드 |

원본의 `rune` 입력은 int32 범위를 Gooo Integer로 전달합니다. `byte` 호환 범위는
0..255입니다. 묵시적으로 정수 폭을 줄이지 않습니다. `RuneStart`의 범위 밖 입력은
Gooo 확장 동작으로 false를 반환하지만 원본과의 동등성 집계에는 넣지 않습니다.
`go/math/bits`의 다섯 함수는 이 영역 밖에서 별도 확장 동작인 -1을 반환합니다.
Go의 byte 변환처럼 입력을 잘라서 계산하지 않습니다. 0의 앞·뒤 0 개수는 원본과 같은 8입니다.
색상 변환도 모든 채널을 먼저 검사하며 범위 밖에서는 레코드의 모든 성분을 -1로 반환합니다.
원본의 정수 나눗셈 순서를 유지하므로 왕복 변환이 원래 RGB와 항상 같은 것은 아닙니다.
`StatusText` 비교는 int가 64비트인 amd64·arm64에서 실행합니다.
HTTP 서버나 UTF 인코딩 패키지 전체의 이식은 아닙니다.

## 실행하기

[브라우저 데모](https://kimjooyoon.github.io/gooo-go-ports/)는 설치 없이 13개 함수를
실행합니다. 실제 Gooo importing 예제를 고정 컴파일러로 생성하고, 생성된 Go를
한 글자도 수정하지 않은 채 WebAssembly로 빌드합니다. 입력·출력 경계의 Go bridge는
정수 범위 검사와 직렬화만 맡습니다. 임의 Gooo 소스를 브라우저에서 컴파일하지는 않습니다.
HTTP 입력은 int64, rune 입력은 int32, byte 입력은 0..255로 제한하며 범위 밖 입력을 거부합니다.
JavaScript Number를 거치지 않고 십진수 문자열로 전달합니다.

**Copy link to this input**으로 함수와 입력을 공유할 수 있습니다. 같은 프로그램이면
받는 브라우저에서 한 번 실행하고, 프로그램이 달라졌으면 입력을 복원한 뒤 Run을 기다립니다.
링크에는 입력 문자열, Wasm SHA-256, 원본 리비전만 담으며 결과를 저장하지 않습니다.
README만 바뀌어도 Wasm이 같으면 실행할 수 있습니다. 잘못된 링크는 실행을 막고
새 예제로 돌아갈 수 있게 표시합니다. 클립보드가 차단되면 복사할 링크를 화면에 보여줍니다.
일반 `#workbench` 링크는 유지하며 공유 링크는 별도 `#gooo-share?v=1&...` 형식입니다.

데모를 소스부터 재현하려면 Git으로 받은 저장소에서 Go 1.27.2와 Node.js 22 이상을
사용합니다. 브라우저 빌드는 Git 리비전을 기록하므로 `.git`이 없는 소스 압축본은
이 절차에 사용할 수 없습니다. 배포된 browser-demo 압축본은 안의 README에 따라
`site/`를 HTTP로 제공하면 되며 컴파일러 설치는 필요하지 않습니다.

```sh
./tools/bootstrap.sh
go run ./cmd/build-demo --out out/browser
node tools/browser-smoke.cjs out/browser/site
node --test tools/share-link.test.cjs tools/share-app.test.cjs
python3 -m http.server 8766 --bind 127.0.0.1 --directory out/browser/site
```

마지막 명령은 Python 3가 필요하며, 실행 후 `http://127.0.0.1:8766`을 엽니다.
다른 정적 HTTP 서버를 사용해도 됩니다. `--out` 디렉터리는 새 경로여야 합니다.
브라우저 비교는 기존 원본 corpus의 함수 입력 3,135개와 정밀도 경계 2개를 재사용하고,
잘못된 입력의 거부도 별도 검사합니다. 아래 native 소비 예제는 브라우저 집계에 포함하지 않습니다.
HTTP·rune 검사는 유한 입력 관측이며 패키지 전체 호환성이나 성능을 보장하지 않습니다.

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
Gooo에서 조합하며 원본의 전체 uint8 입력 범위를 유지합니다. 위 브라우저의 13함수와
별도로 native에서 실행하며, 특정 색상 프로파일이나 프린터 보정은 수행하지 않습니다.

## 다른 Gooo 프로그램에서 가져오기

```gooo
package checker
namespace checker
import utf8 "go/unicode/utf8"
entity Integer id "gooo://std/integer"
entity Boolean id "gooo://std/boolean"
activity Valid(Integer) -> Boolean computes `return utf8.ValidRune(input)`
```

원하는 포트 소스를 복사하거나 Git 리비전으로 고정하고, 사용하는 작업공간 명세에
패키지 경로와 파일을 등록합니다. 전체 구성은 [gooo.workspace.json](gooo.workspace.json),
실제 importing app은 [response.gooo](examples/response.gooo)에 있습니다.
비트 연산을 가져오는 예제는 [ByteSummary](examples/byte-summary/main.gooo)입니다.
색상 변환 조합은 [Color inspector](examples/color-inspector/main.gooo)와
[color-inspector.workspace.json](color-inspector.workspace.json)에 있습니다.
현재 배포 단위는 Gooo 소스와 workspace 명세이며 Go의 패키지 관리자와는 별개입니다.

## 검증 범위

기존 **3,147건**과 Unicode 소비 앱의 Go 원본 비교 **334건**·영역 밖 거부 **12건**을
그대로 유지합니다. 색상 포트는 원본 비교 **1,300건**·거부 **56건**, 색상 소비 앱은
원본 비교 **637건**·거부 **24건**을 추가합니다. 전체 native 검사는 **5,510건**이며
이식 함수는 15개입니다. 각 영역 밖 거부 사례는 Go 동등성 집계가 아닙니다.

| 대상 | 입력 수 | 구성 |
| --- | ---: | --- |
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

Gooo 실행 입력 상한에 맞춰 총 80개 배치를 실행합니다. 기존 31개 배치는 128건 이하,
Unicode 앱 6개와 색상 포트 22개 배치는 64건 이하, 색상 소비 앱 21개 배치는 32건 이하입니다.
새 레코드 기대값도 들여쓴 JSON과 마지막 개행을 포함해 32KiB 한도를 검사합니다.
원본 비교 기대값은 Gooo 구현과 별개인 Go 표준 함수로 계산합니다.
Unicode·색상 앱과 색상 포트의 영역 밖 기대값은 명시한 false/-1 계약으로 따로 검사합니다.
검증 도구는 결과뿐 아니라 입력 순서, 원본 기대값, 함수·입력 타입 식별자,
생성 코드 해시, 컴파일러·Go 버전, workspace·입력 파일 해시, 저장 재실행 연결을 검사합니다.
함수 식별자는 실행 기록 밖의 [명시적 계약](cmd/verify/contracts.go)을 기준으로
선택한 패키지 선언, 생성 보고서, 각 실행 결과에 각각 대조합니다. 여러 입력은 전달된
타입 식별자도 검사하며, 타입 식별자가 없는 단일 입력 기록은 패키지 선언의 입력 타입과 연결합니다.
원본 파일의 내용과 패키지 소속은 실행 전에 읽은 소스에 대조하고, 생성 Go의 실제 내용도
기록된 해시와 비교합니다. [브라우저 빌드 검사](cmd/build-demo/evidence.go)도 독립적인
Dispatch 계약과 입력 타입·소스 연결을 확인합니다.
검사 도중 소스가 바뀌어도 처음 읽은 동일한 소스 스냅샷을 모든 배치에 사용합니다.
보고서·선언·실행 결과의 식별자를 함께 바꾸거나 입력 타입·소스 소속을 바꾼 기록을
거부하는 회귀 테스트도 포함합니다.

`RuneStart`와 `math/bits` 다섯 함수는 Go byte 영역을 전수 비교합니다.
그 밖의 함수는 모든 입력을 검사한 것이 아닙니다. 여러 함수가 같은 정수 입력을 재사용하므로
이 사례 수는 전역적으로 서로 다른 정수 값의 개수가 아닙니다.
별도 모델 학습, 성능 우위, 패키지 전체의 호환성은 검증 범위에 포함하지 않습니다.
[첫 8개 함수의 관측](evidence/initial.json), [13개 함수의 관측](evidence/math-bits-20261010.json),
[Unicode 소비 앱 추가 관측](evidence/unicode-inspector-20261010.json),
[색상 포트·소비 앱 추가 관측](evidence/color-ports-20261010.json),
[v0.1.0-preview.1의 전체 실행·재실행 기록](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.1)을
참고하세요. 릴리스 기록은 해당 버전에 고정되어 있으며, 이후 변경의 기록은 각 GitHub
Actions 실행에서 확인할 수 있습니다.

## 언어 개발로 돌려주는 자료

```sh
./tools/probe.sh --out out/probes
```

반복문·바이트 인덱싱·비트 연산의 작은 Gooo 예제는 선언 검사는 통과하지만
현재 본문 생성에서는 거부됩니다. 스크립트는 정확한 진단과 컴파일러 정보를 저장합니다.
지원이 추가되면 기존 실패 가정을 조용히 유지하지 않고 검사가 실패하도록 했습니다.

- [언어 분석](docs/language-assessment.md): 실제 지원 범위, API 변환과 제약
- [다음 이식 로드맵](docs/roadmap.md): 문자열 순회, hex, google/uuid의 선행 조건
- [기여 방법](CONTRIBUTING.md): 원본 추적과 동작 비교를 포함한 이식 절차
- [경계 재현](probes/README.md): 컴파일러 개선에 사용할 최소 예제
- [바이트 입력 조건 실험](docs/research/20261010-source-conditions/README.md): 개발 컴파일러에서 같은 최종 출력과 다른 내부 조건을 구분한 재현 자료. 기본 컴파일러와 15개 포트 집계는 유지합니다.

## 원본과 라이선스

[upstream.json](upstream.json)에 Go **1.27.2**의 커밋
`022c8636110ebac86a9b88724cda168ed713c21f`, 파일 SHA-256,
Gooo **0.6.24-dev** 컴파일러 커밋
`366ee6ef8f9d2b9b57331dfd26b6f908c0289851`을 고정했습니다.
테스트의 Go 도구 체인에서도 원본 파일 해시가 일치해야 합니다.

BSD 3-Clause 라이선스입니다. Go 원본 저작권과
[원본 라이선스](third_party/GO-LICENSE), [NOTICE](NOTICE)를 보존합니다.
기존 [Gooo standard library](https://github.com/kimjooyoon/gooo-standard-library)의
기초 숫자·논리·문자열 함수와 함께 사용할 수 있도록 별도 패키지 경로를 사용합니다.
