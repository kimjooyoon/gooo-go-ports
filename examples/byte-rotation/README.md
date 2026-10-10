# Byte rotation inspector

A Gooo example that rotates a byte by a user-supplied signed count, rotates it
back, and compares the number of one bits before and after. It imports
`bits.RotateLeft8` and `bits.OnesCount8`; both calculations stay in Gooo.

바이트를 비트 묶음으로 해석할 때 회전 방향과 경계에서 넘어오는 비트를 확인하는
작은 진단 예제입니다. 입력 `129`는 이진수 `10000001`이며, 왼쪽으로 한 번 회전하면
`00000011`, 즉 `3`이 됩니다. 오른쪽으로 한 번 회전하려면 회전 수에 `-1`을 넣습니다.
회전은 밀려난 비트를 반대쪽으로 되돌리므로 1비트 개수는 바뀌지 않습니다.

## 실행하기

저장소 루트에서 Go 1.27.2, Git, POSIX shell을 사용합니다. 컴파일러 소스는
[upstream.json](../../upstream.json)의 리비전으로 고정하며 모델 호출이 필요하지 않습니다.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/rotation-demo
./.gooo package execute --json --inputs examples/byte-rotation/inputs.json \
  byte-rotation.workspace.json > out/rotation-demo/execution.json
./.gooo package replay --json --receipt out/rotation-demo/execution.json \
  --inputs examples/byte-rotation/inputs.json \
  byte-rotation.workspace.json > out/rotation-demo/replay.json
```

[inputs.json](inputs.json)의 `app/byte-rotation:Inspect.input0`은 바이트,
`app/byte-rotation:Inspect.input1`은 회전 수입니다. 값을 바꾸고 위 실행을 반복할 수 있습니다.
입력만 제공한 기록의 판정은 `OBSERVED`입니다. 실제 반환 레코드는
`result.runtime.traces[].deliveries[].actual`에 있습니다.

| 필드 | 의미 |
| --- | --- |
| `in_byte_domain` | 원래 바이트 입력이 0..255 범위인지 여부 |
| `normalized_rotation` | 원래 회전 수와 같은 결과를 내는 0..7의 왼쪽 회전 수 |
| `rotated` | 회전 후 바이트 |
| `restored` | 반대 방향으로 되돌린 바이트 |
| `ones_before`, `ones_after` | 원래 값과 회전 후 값의 1비트 개수 |

`(129, 1)`의 결과는 다음과 같습니다. 원래 두 입력은 실행·재실행 기록에도 남습니다.

```json
{"in_byte_domain":true,"normalized_rotation":1,"rotated":3,"restored":129,"ones_before":2,"ones_after":2}
```

## 원본 입력 범위와 차이

Go의 `RotateLeft8(uint8, int)`를 Gooo의 `RotateLeft8(Integer, Integer)`로 표현합니다.
바이트는 **0..255**, 회전 수는 64비트 Go 대상의 전체 signed int 범위인
**−9223372036854775808..9223372036854775807**입니다. 음수는 오른쪽 회전입니다.
바이트를 8비트로 잘라내거나 회전 수를 작은 정수로 변환한 뒤 전달하지 않습니다.

바이트가 범위를 벗어나면 포트는 `-1`을 반환합니다. 이 소비 앱은
`in_byte_domain=false`, 다섯 정수 필드 모두 `-1`을 반환합니다. 이 확장 거부 계약은
Go 원본 함수와의 동등성 집계와 분리합니다. 회전 수에는 추가 범위 제한이 없습니다.

회전 수를 JSON 숫자로 기록할 때 원래 십진수 정수를 보존하세요. 특히
`9007199254740993`처럼 JavaScript의 정확한 정수 범위를 넘는 값을 `Number`로
읽고 다시 저장하면 입력이 달라질 수 있습니다. 브라우저 데모는 회전 수를 문자열로
받아 Go int64로 전달하며, 표시나 공유 전에 JavaScript Number로 바꾸지 않습니다.

원본은 고정 Go 리비전의
[bits.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/math/bits/bits.go#L183)입니다.
파일 해시와 라이선스는 [upstream.json](../../upstream.json),
[NOTICE](../../NOTICE), [Go 라이선스](../../third_party/GO-LICENSE)에 연결합니다.

원본 Go는 입력에 무관한 실행시간을 명시합니다. 이 포트는 분기와 산술을 사용하므로
그 실행시간 보장을 계승하지 않습니다. 암호 계산의 일정 시간 구현으로 제시하지 않습니다.

## 비트 연산 없이 표현한 방법

회전 수 `k`에서 먼저 `s = k % 8`을 계산하고 음수일 때만 8을 더합니다.
최솟값을 부호 반전하거나 원래 값에 8을 더하지 않으므로 int64 양 끝에서도 overflow가 없습니다.
`s = 0`이면 원래 바이트를 반환합니다. 나머지 경우에는 2..128 중 `p = 2^s`를 선택하고
`(x * p) % 256 + x / (256 / p)`를 계산합니다. 가장 큰 중간 곱은 32,640입니다.
두 항은 각각 남은 비트와 반대쪽으로 넘어온 비트를 표현합니다.

소비 앱은 `8 - s`만큼 다시 회전해 원래 바이트를 복원합니다. 원래 `k`를 부호 반전하지
않으므로 `k = −9223372036854775808`에도 같은 경로를 사용할 수 있습니다.

## 검증과 브라우저 체험

```sh
go run ./cmd/verify --out out/rotation-check
```

출력 디렉터리는 새 경로여야 합니다. 독립 Go 원본 함수의 기대값으로 포트와 소비 앱을
검사하고 저장한 프로그램을 재실행합니다. 입력 값·순서와 활동·타입·소스·생성 코드·
컴파일러 식별자도 검사합니다. 256바이트 × 8회전의 2,048쌍은 전수 비교하며,
원래 signed int64 회전 수의 전체 영역은 경계값과 고정 표본을 관측합니다.
정규화된 여덟 회전의 전수 비교가 모든 원래 int64 입력을 실행했다는 뜻은 아닙니다.
범위 밖 바이트는 Go uint8로 변환하기 전에 거부하고 별도 집계합니다.
포트 원본 비교 2,472건·거부 36건, 소비 앱 원본 비교 672건·거부 36건을
[검증 요약](../../evidence/byte-rotation-20261010.json)에 기록합니다.
이 앱의 입력 전용 예제 12건은 이 원본 비교 사례 수에 추가하지 않습니다.

[브라우저 데모](https://kimjooyoon.github.io/gooo-go-ports/)에서 **bits.RotateLeft8**을 선택하면
바이트와 회전 수를 입력하고 회전 전후의 8비트를 볼 수 있습니다. 입력 공유 링크는
큰 회전 수도 십진수 문자열 그대로 보존합니다. 위 복원·1비트 개수 레코드는 native 소비 앱의
출력이며 브라우저에서 실행하는 단일 함수와 구분합니다.
다른 프로젝트에 포트를 가져오는 절차는 [독립 프로젝트 안내](../../docs/use-in-your-project.md)를
참고하세요. 그 안내는 자체 고정 버전을 쓰므로, 이 새 회전 함수를 복사할 때는
`RotateLeft8`이 포함된 소스 리비전을 고정해야 합니다.
