# RGB → CMYK → RGB inspector

A native Gooo example that uses the ported Go `image/color` functions to convert
an RGB color to CMYK and back. It reports the recovered channels and their integer
differences, so a successful conversion is not mistaken for a lossless round trip.

[Gooo 소스](main.gooo)는 `go/image/color`의 `RGBToCMYK`와 `CMYKToRGB`를 가져옵니다.
입력은 red, green, blue 순서의 정수 세 개이며 각각 원본 Go `uint8`의 전체 범위인
0..255를 사용합니다. 계산과 차이 판정은 Gooo에서 수행합니다.

## 실행과 입력 편집

Go 1.27.2, Git, POSIX shell을 사용하며 저장소 루트에서 실행합니다.
컴파일러는 `upstream.json`의 Gooo 0.6.24-dev 리비전으로 고정합니다.
모델이나 외부 추론 서비스는 필요하지 않습니다.

```sh
./tools/bootstrap.sh
mkdir -p out/color-demo
./.gooo package execute --json --inputs examples/color-inspector/inputs.json \
  color-inspector.workspace.json > out/color-demo/execution.json
./.gooo package replay --json --receipt out/color-demo/execution.json \
  --inputs examples/color-inspector/inputs.json \
  color-inspector.workspace.json > out/color-demo/replay.json
```

[inputs.json](inputs.json)의 `input0`, `input1`, `input2`가 각각 red, green, blue입니다.
JSON에는 십진수 정수를 씁니다. 값을 바꾸면 execute로 새 기록을 만들고 replay합니다.
실제 결과는 실행 기록의 `result.runtime.traces[].deliveries[].actual`에 있습니다.
입력만 주는 실행은 `OBSERVED`이며, Go 기대값과 비교하는 검증의 `PASS`와 구분합니다.

## 결과를 읽는 법

| 필드 | 의미 |
| --- | --- |
| `in_byte_domain` | 세 입력 모두 0..255 범위인지 여부 |
| `cyan`, `magenta`, `yellow`, `black` | `RGBToCMYK`의 0..255 결과. 백분율이 아님 |
| `red`, `green`, `blue` | CMYK에서 다시 변환한 RGB 채널 |
| `red_delta`, `green_delta`, `blue_delta` | 복원 채널 − 원래 입력 채널 |
| `exact` | 세 복원 채널이 모두 원래 입력과 같은지 여부 |

예제 입력 `(142,83,202)`는 CMYK `(75,150,0,53)`으로 변환되고,
다시 RGB `(143,83,202)`로 복원됩니다. 따라서 채널 차이는 `(1,0,0)`, `exact=false`입니다.
검정색 `(0,0,0)`은 CMYK `(0,0,0,255)`를 거쳐 정확히 원래 RGB로 돌아옵니다.
정수 변환의 차이는 채널 감소만 뜻하지 않으며, 여기서는 red가 1 증가합니다.

`exact`는 정수 채널의 일치 여부입니다. 사람이 느끼는 색 차이나 프린터 출력 품질을
측정하지 않습니다. 원본 Go의 CMYK는 특정 색상 프로파일과 연결되지 않습니다.
이 예제는 ICC 변환, 이미지 파일 읽기, 인쇄 보정을 수행하지 않습니다.

각 입력의 범위를 계산 전에 검사합니다. 범위 밖 int64 입력은 `in_byte_domain=false`,
`exact=false`, 모든 정수 필드 -1을 반환합니다. byte로 잘라서 받아들이지 않습니다.
두 포트 함수도 범위 밖 입력에서 반환 레코드의 모든 성분을 -1로 표시합니다.
이 별도 거부 계약은 Go 함수와의 동등성 사례 수에 포함하지 않습니다.

## 원본 계산을 보존한 방법

Go의 다중 반환값은 Gooo의 `RGB{red,green,blue}` 또는
`CMYK{cyan,magenta,yellow,black}` 레코드로 매핑합니다.
`RGBToCMYK`는 최댓값 채널이 0인 검정색을 나눗셈 전에 처리합니다.
나머지 입력에서 `(최댓값 − 채널) × 255`는 65,025 이하입니다.

`CMYKToRGB`의 가장 큰 중간 곱은 `65535² = 4294836225`로 int64 안에 있습니다.
Go의 마지막 `>>8`은 비음수 정수에서 `/256`으로 표현합니다.
앞선 정수 나눗셈 순서와 버림을 유지하며 별도 반올림을 추가하지 않습니다.

원본은 고정 Go 리비전의
[ycbcr.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/color/ycbcr.go#L315)이며,
[Go API 문서](https://pkg.go.dev/image/color@go1.27.2#RGBToCMYK)에서 함수와 CMYK의 범위를 볼 수 있습니다.
원본 파일 해시는 [upstream.json](../../upstream.json), BSD-3-Clause 라이선스와
저작권은 [LICENSE](../../LICENSE)·[NOTICE](../../NOTICE)에 보존합니다.

## 검증

```sh
go run ./cmd/verify --out out/color-check
```

출력 경로는 새 경로여야 합니다. 고정 Go `image/color` 함수를 직접 호출한 기대값으로
포트와 소비 앱을 검사하고, 저장한 프로그램을 재실행합니다. 범위 검증은 Go byte
변환 전에 수행하며, 범위 밖 거부 사례는 원본 비교와 별도로 집계합니다.
입력, 기대값, 실제 결과, 원본·생성 코드·컴파일러의 연결도 함께 검사합니다.

검정·흰색·원색·채널 최댓값 동률·나눗셈 경계·회색과 고정 시드 표본을 사용합니다.
유한 사례 검증이며 모든 RGB 2²⁴ 조합이나 CMYK 2³² 조합의 전수 검사가 아닙니다.
색상 포트 원본 비교 1,300건과 별도 거부 56건, 이 앱의 원본 비교 637건과
별도 거부 24건을 [검증 요약](../../evidence/color-ports-20261010.json)에 분리해 기록합니다.
브라우저 데모는 기존 HTTP·Unicode·비트 함수 13개를 실행하며, 이 색상 예제는 위 native
명령으로 실행합니다.
