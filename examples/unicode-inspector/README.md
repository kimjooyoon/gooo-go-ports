# Unicode inspector

코드 포인트 하나 또는 UTF-16 서로게이트 쌍을 정수로 받아 유효성과 인코딩 길이를
진단하는 Gooo 앱입니다. [소스](main.gooo)는 기존 `go/unicode/utf8`와
`go/unicode/utf16` 포트를 가져옵니다. 새 포트 함수나 Go 계산 wrapper를 추가하지 않습니다.

## 실행과 입력 편집

Go 1.27.2, Git, POSIX shell을 사용하며 저장소 루트에서 실행합니다.
Gooo 컴파일러는 기존 `upstream.json`의 0.6.32-dev 리비전으로 고정합니다.
모델이나 외부 추론 서비스는 필요하지 않습니다.

```sh
./tools/bootstrap.sh
mkdir -p out/unicode-demo
for kind in scalar pair; do
  ./.gooo package execute --json --inputs "examples/unicode-inspector/${kind}-inputs.json" \
    "unicode-${kind}.workspace.json" > "out/unicode-demo/${kind}.json"
  ./.gooo package replay --json --receipt "out/unicode-demo/${kind}.json" \
    --inputs "examples/unicode-inspector/${kind}-inputs.json" \
    "unicode-${kind}.workspace.json" > "out/unicode-demo/${kind}-replay.json"
done
```

[scalar-inputs.json](scalar-inputs.json)의 정수를 편집하거나
[pair-inputs.json](pair-inputs.json)의 `input0`(high)·`input1`(low)을 편집해 실행합니다.
JSON에는 십진수 정수를 씁니다. 입력을 바꾼 뒤에는 execute로 새 기록을 만들고 replay합니다.
입력만 제공한 실행 결과는 `OBSERVED`이며 기대값과 비교한 `PASS`와 구분합니다.
실제 결과는 receipt의 `result.runtime.traces[].deliveries[].actual`에 있습니다.

## 결과를 읽는 법

`InspectScalar`의 결과:

| 필드 | 의미 |
| --- | --- |
| `in_rune_domain` | 입력이 Go rune의 int32 범위인지 여부 |
| `valid_scalar` | 유효한 Unicode scalar인지 여부. 서로게이트는 제외 |
| `is_surrogate` | D800..DFFF 구간인지 여부 |
| `utf8_bytes`, `utf16_units` | UTF-8 바이트 수와 UTF-16 코드 단위 수. 잘못된 scalar는 -1 |
| `needs_pair` | 유효한 BMP 밖 scalar로, UTF-16 코드 단위 두 개가 필요한지 여부 |
| `high`, `low` | Go `EncodeRune`에 대응하는 원시 반환값. `needs_pair=true`일 때만 유효한 서로게이트 쌍 |

정상 BMP 문자에서도 `EncodeRune`은 `65533,65533`을 반환합니다. 이는 그 문자의
UTF-16 인코딩이 아닙니다. 쌍만 보고 scalar를 잘못된 값으로 판정하면 안 됩니다.

| Scalar 입력 | `valid_scalar` | UTF-8 / UTF-16 길이 | `needs_pair` | `high`, `low` |
| --- | --- | --- | --- | --- |
| 65 (A) | true | 1 / 1 | false | 65533, 65533 |
| 65533 (U+FFFD) | true | 3 / 1 | false | 65533, 65533 |
| 65535 (U+FFFF) | true | 3 / 1 | false | 65533, 65533 |
| 128512 (U+1F600) | true | 4 / 2 | true | 55357, 56832 |
| 55296 (U+D800) | false | -1 / -1 | false | 65533, 65533 |

`InspectPair`는 high·low 순서의 범위를 검사해 `valid_pair`를 반환합니다.
`decoded_scalar`는 `DecodeRune` 결과이고, `decoded_utf8_bytes`와
`decoded_utf16_units`는 **디코딩 결과**의 길이입니다.

```json
{"in_rune_domain":true,"valid_pair":false,"decoded_scalar":65533,"decoded_utf8_bytes":3,"decoded_utf16_units":1}
```

위 결과는 입력 `(65533,65533)`에 대한 것입니다. 입력쌍은 잘못되었지만 대체 결과
U+FFFD 자체는 유효한 문자이며 길이는 3/1입니다. 디코딩 결과의 문자 유효성으로
입력쌍의 유효성을 대신 검사하면 안 됩니다. 입력 `(55357,56832)`는 유효하며
`decoded_scalar=128512`, 길이 4/2를 반환합니다.

## 범위와 검증

두 진입점 모두 각 입력이 -2147483648..2147483647 범위인지 먼저 검사합니다.
그 밖의 int64 입력은 `in_rune_domain=false`, 나머지 불리언은 false, 정수 필드는
모두 -1로 반환합니다. int32/uint16으로 잘라서 받아들이지 않습니다. 이 거부 계약은
원본 Go 함수와의 동등성 주장에 포함하지 않습니다.

전체 검증은 저장소 루트에서 `go run ./cmd/verify --out out/unicode-check`로 실행합니다.
출력 경로는 새 경로여야 합니다. 기존 3,147개 사례는 변경하지 않고, 이 소비 앱에
Go 원본 비교 334개(Scalar 164, Pair 170)와 영역 밖 거부 12개를 추가합니다.
모든 새 사례는 실제 native 실행과 saved replay를 거치며 입력값·순서·결과·실행 근거를 검사합니다.
레코드 기대값이 실행기의 32KiB JSON 한도를 넘지 않도록 최대 64개씩 6개 배치로 나눕니다.
Go oracle은 변환 전에 int32 영역을 검증하고, pair 유효성은 Go `DecodeRune`이
대체 문자를 반환했는지로 판단해 Gooo의 범위식을 복제하지 않습니다.

이 앱은 정수 진단 예제입니다. 문자열 디코딩, 전체 UTF-8 스트림 유효성 검사,
문자열 정규화, 사용자에게 보이는 문자 수 계산은 수행하지 않습니다.
유한 사례 관측이며 전체 Unicode 입력의 전수 검증이나 패키지 전체 호환성을 주장하지 않습니다.
기존 Go 원본 핀과 BSD-3-Clause [라이선스](../../LICENSE)·[고지](../../NOTICE)를 유지합니다.
