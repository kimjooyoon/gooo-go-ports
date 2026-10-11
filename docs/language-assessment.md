# Go 라이브러리 이식을 위한 Gooo 관측

2026-10-11 KST 기준으로 Gooo **0.6.24-dev**, 컴파일러 소스
`366ee6ef8f9d2b9b57331dfd26b6f908c0289851`, Go **1.27.2**를 사용한다.
`gooo version --build --json`에서 `CLEAN_VCS`, 해당 소스 SHA,
`go_version=go1.27.2`, `native_go_required=go1.27.2`를 확인했다.
이는 이 저장소가 고정한 조합이며 모든 설치 환경의 기본 버전을 뜻하지 않는다.

별도 [바이트 입력 조건 실험](research/20261010-source-conditions/README.md)은 개발 리비전
`92dc06b38f72518443f8a90e5c98a645f0c756f1`의 `condition_case`를 사용한다.
두 리비전은 같은 `0.6.24-dev` 문자열을 출력하므로 재현할 때 소스 SHA도 확인해야 한다.
이 실험에서 기본 컴파일러 핀을 바꾸거나 새로운 Go 포트를 추가하지는 않았다.

## 이번에 옮기는 동작

Go 1.27.2의 공개 소스와 동작을 읽고, 다음 **29개 함수**를 Gooo 활동으로 표현한다.
패키지 전체, HTTP 서버, 문자열 인코더 전체 또는 Go API와의 바이너리 호환을 주장하지 않는다.

| 원본 | Gooo 원본 | 표현 방법과 호환 범위 |
| --- | --- | --- |
| `net/http.StatusText` | [http.gooo](../ports/net/http.gooo) | 상태 코드별 텍스트를 `if`로 선택. 등록되지 않은 코드는 빈 문자열 |
| `unicode.IsLetter`, `IsDigit` | [classification.gooo](../ports/unicode/classification.gooo) | Unicode 17.0.0의 Letter 647범위·Digit 72범위를 균형 분기로 생성. stride·원래 signed int32 입력을 보존하고 int32 밖은 false 확장 계약 |
| `unicode.IsSpace` | [space.gooo](../ports/unicode/space.gooo) | Unicode 17.0.0 White_Space의 25개 값을 범위·동등 비교로 표현. 전체 signed int32 입력을 보존하고 int32 밖은 false 확장 계약 |
| `unicode.SimpleFold` | [simple_fold.gooo](../ports/unicode/simple_fold.gooo) | 고정 ASCII 표·특수 순환·case range delta를 분기로 생성. 다음 순환 멤버 및 invalid int32 identity 보존; int32 밖은 별도 identity 확장 |
| `unicode/utf8.ValidRune`, `RuneLen` | [utf8.gooo](../ports/unicode/utf8.gooo) | 코드 포인트 경계와 서로게이트 구간 비교. Go `rune` 입력 범위를 Integer로 전달 |
| `unicode/utf8.RuneStart` | 같은 파일 | Go `byte` 범위인 0..255에서 연속 바이트 구간 128..191 제외. 범위 밖은 별도 확장 계약인 false |
| `unicode/utf16.IsSurrogate`, `RuneLen` | [utf16.gooo](../ports/unicode/utf16.gooo) | 범위 비교로 서로게이트와 UTF-16 길이를 판별 |
| `unicode/utf16.DecodeRune`, `EncodeRune` | 같은 파일 | 유효 범위를 먼저 검사한 뒤 곱셈·나눗셈·나머지로 두 10비트 성분 계산. EncodeRune의 두 결과는 `high`, `low` 레코드로 전달 |
| `math/bits.Len8`, `LeadingZeros8`, `TrailingZeros8`, `OnesCount8`, `Reverse8` | [bits.gooo](../ports/math/bits.gooo) | 0..255를 먼저 검사한 뒤 비교와 정수 산술로 계산. 테이블·비트 연산·반복문을 쓰지 않으며 영역 밖은 -1이라는 별도 확장 계약 |
| `math/bits.RotateLeft8` | 같은 파일 | byte 0..255와 원래 signed int64 회전 수. 회전 수의 나머지를 먼저 구해 overflow 없이 산술로 순환 이동. 원본의 입력에 무관한 실행시간 보장은 계승하지 않음 |
| `image/color.RGBToCMYK`, `CMYKToRGB` | [color.gooo](../ports/image/color.gooo) | 원래 uint8 3/4성분의 전체 범위를 검사한 뒤 정수 변환. 결과는 이름 있는 `CMYK`·`RGB` 레코드이며 영역 밖은 모든 성분 -1 |
| `image/color.RGBToYCbCr`, `YCbCrToRGB` | [ycbcr.gooo](../ports/image/ycbcr.gooo) | 원래 uint8 3성분과 고정 정수 계수·반올림·포화 보존. 결과는 `YCbCr`·`RGB` 레코드이며 영역 밖은 모든 성분 -1 |
| `time.Duration.Round`, `Truncate` | [duration.gooo](../ports/time/duration.gooo) | 수신 객체를 첫 번째 인수로 옮긴 `DurationRound(d, m)`, `DurationTruncate(d, m)`. 두 인수 모두 전체 signed int64 나노초 범위이며 중간값 처리·포화·0 이하 단위의 동작을 유지 |
| `image.Rectangle.Empty`, `Overlaps`, `Intersect`, `Union` | [geometry.gooo](../ports/image/geometry.gooo) | 수신 사각형을 첫 레코드 인수로 전달. 네 signed int64 좌표로 Go의 중첩 Point 필드를 명시적으로 펼침. 원본 좌표·입력 순서·빈 사각형 표현을 보존 |

`Rectangle.Canon`은 [canon.gooo](../ports/image/canon.gooo)에서 기존 Rectangle
레코드를 재사용해 두 축을 독립적으로 비교·교환한다. 입력은 전체 signed int64이며
새 산술·축소·영역 밖 확장이 없다. [드래그 crop 예제](drag-crop.md)는 선택 영역만
정규화하며 기존 CropPlan과 브라우저 기능은 바꾸지 않는다.

Go의 `rune`은 int32이고 Gooo Integer는 int64다. 테스트에서 int64를 Go rune/byte로
무조건 변환하면 잘림으로 잘못된 동등성 주장이 생긴다. 원본 입력 범위의 비교와 Gooo
확장 입력의 계약을 나눈다. 반환 레코드는 다중 반환값을 표현하는 명시적 API 변경이다.
비트 연산의 산술 변환도 검사한 비음수 구간에서만 적용한다.

`RotateLeft8`은 회전 수 `k % 8`의 음수 결과에만 8을 더한다. 원래 `k`를 부호 반전하거나
8을 더하지 않아 int64 양 끝을 보존한다. 정규화된 회전이 0이면 원래 바이트를 반환하고,
나머지는 2..128의 배수 선택과 나눗셈·나머지로 계산한다. 최대 중간 곱은 32,640이다.
[소비 예제](../examples/byte-rotation/README.md)는 반대 방향으로 복원하고 1비트 개수를 비교한다.
범위 밖 바이트를 -1로 반환하는 확장 계약은 Go 원본 비교와 분리한다.

색상 변환은 검정색의 나눗셈 0을 먼저 처리한다. CMYK→RGB의 최대 중간 곱은
`65535² = 4294836225`이므로 Gooo int64에서 overflow가 없다. 마지막 비음수 `>>8`은
`/256`으로 표현하고 앞선 나눗셈과 버림 순서는 그대로 둔다. RGB→CMYK→RGB의 정수
결과는 원래 RGB와 다를 수 있어 [소비 예제](../examples/color-inspector/README.md)가
채널별 차이와 정확 일치 여부를 보여준다. 이미지 파일·ICC 프로파일 API는 이식 범위가 아니다.

Y′CbCr 변환은 같은 원본의 정수 계수와 포화를 보존한다. 순변환의 비음수 분자에만
`/65536`을 적용하며, 역변환은 각 분자의 음수·상한 밖을 먼저 포화시키고 나머지에
나눗셈을 적용한다. 역변환의 `Y′ * 65793`을 고정 반올림 상수로 바꾸지 않는다.
[YCbCr 소비 예제](../examples/ycbcr-inspector/README.md)는 두 포트를 조합해 복원 RGB와
채널 차이를 반환한다. 수정 없는 생성 함수의 각 256³ 입력 비교는 유한 영수증 사례와
따로 기록하며, 이미지 코덱이나 다른 색상 변환 행렬의 검증을 뜻하지 않는다.

원본은 Go의 [status.go](https://github.com/golang/go/blob/go1.27.2/src/net/http/status.go),
[utf8.go](https://github.com/golang/go/blob/go1.27.2/src/unicode/utf8/utf8.go),
[utf16.go](https://github.com/golang/go/blob/go1.27.2/src/unicode/utf16/utf16.go)다.
`math/bits`는 같은 리비전의 `bits.go`, `bits_tables.go`, `make_tables.go`에 근거한다.
테이블 값을 수동 복제하지 않고 길이 임계값과 8단계 산술 계산으로 표현한다.
색상 변환은 고정 리비전의
[image/color/ycbcr.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/color/ycbcr.go)에 근거한다.
저작권 표기와 BSD 라이선스를 보존한다.

시간 간격은 [고정 time.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/time/time.go#L1110)의
두 Duration 메서드를 옮긴다. `m <= 0`이면 원래 `d`를 돌려주며, 양수 `m`에서는
나머지와 반대쪽 배수까지의 거리를 비교한다. 정확한 중간값은 0에서 멀어지는 쪽으로
반올림한다. 최솟값을 부호 반전하거나 나머지를 두 배로 만들지 않고, 최종 덧셈·뺄셈
전에 int64 경계를 검사해 원본의 포화 동작을 보존한다. 절삭은 부호 있는 나머지를
제거해 0으로 향한다. [소비 예제](../examples/duration-quantizer/README.md)는 두 포트를
함께 호출하고 변경량을 보여준다. 시계·타이머·날짜·기간 문자열 처리는 범위에 포함하지 않는다.

`unicode.IsSpace`는 고정 Go 리비전의 [graphic.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/graphic.go),
[letter.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/letter.go),
[tables.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/tables.go)에 근거한다.
Latin-1 특례와 Unicode 17.0.0 White_Space 테이블을 25개 값의 범위 비교로 펼쳤다.
Go rune은 Unicode 스칼라로 제한된 타입이 아니므로 음수·서로게이트·U+10FFFF 초과 입력도
int32 안이면 그대로 비교한다. NEL(U+0085)은 참, BOM(U+FEFF)·ZERO WIDTH SPACE(U+200B)는
거짓이며 JavaScript `\s`나 `trim`으로 대체할 수 없다. [소비 예제](../examples/whitespace-inspector/README.md)는
기존 UTF 포트를 가져와 스칼라 유효성·길이를 함께 반환한다. 전체 문자열 처리나 문자 가시성 판정은 포함하지 않는다.

`IsLetter`·`IsDigit`는 같은 `graphic.go`, `letter.go`, `tables.go`와
[digit.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/unicode/digit.go)를 기준으로 한다.
[생성기](../cmd/gen-unicode)는 원본 해시·Go/Unicode 버전을 검사하고 R16·R32의
전체 범위를 Gooo 비교·나머지·분기로 바꾼다. Letter의 119개 범위는 stride가 1보다
커서 단순한 연속 구간으로 바꿀 수 없다. 균형 분기에서도 각 범위의 나머지 조건을 보존한다.
실행 중에는 Go의 unicode 표나 함수를 호출하지 않는다.

고정 컴파일러의 Gooo 소스 상한은 128 KiB다. 예비 실험에서 깊은 들여쓰기를 포함한
분류표 합본은 이 한도로 거절됐고, 조건을 유지하며 들여쓰기를 줄인 합본은 실제
native 실행·저장 재실행을 통과했다. 정식 생성물은 간결한 분기를 사용한다.
[식별자 문자 소비 예제](../examples/identifier-rune/README.md)는 두 함수를 import해
밑줄을 포함한 시작/계속 문자 규칙을 적용한다. 이는 전체 문자열·예약어·보안 검사가 아니다.
`cmd/check-unicode`의 0..U+10FFFF 전수 비교는 일반 native 영수증의 유한 사례와 따로
기록한다. 음수와 U+10FFFF를 넘는 int32 입력은 양쪽의 false 경계 조건 및 유한 사례로
확인하며, signed int32 전체를 전수 검사했다고 표현하지 않는다.

## 확인한 언어 기능

`SimpleFold`는 같은 고정 `letter.go`·`tables.go`를 사용한다. 생성기는 공개되지 않은
`asciiFold`·`caseOrbit` 원본 리터럴을 읽고, `CaseRanges`의 범위와 delta를 Gooo 분기로
옮긴다. ASCII 처리 다음 특수 순환을 먼저 찾고, 해당 항목이 없을 때만 lower/upper
delta를 적용한다. `UpperLower` 교대 구간의 parity는 코드 포인트 자체가 아닌
원본 범위 시작점과의 차이로 구한다. 실행 시 Go Unicode 함수를 호출하지 않는다.
입력 범위는 전체 signed int32이며 음수·서로게이트·U+10FFFF 초과 입력도 보존한다.
int32 밖 Integer는 값을 그대로 반환하는 확장으로 분리한다. 자세한 API와 두 rune
비교 범위는 [예제](../examples/simple-fold/README.md)에 있다.

사각형 네 메서드는 [고정 geom.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/geom.go)의
비교와 복사만 사용한다. 중첩 `Min`, `Max`의 Point 표현은 네 Integer 필드로 바꾸되
입력 정규화·잘림을 하지 않는다. `Empty(Rectangle)`과 나머지의 `(Rectangle, Rectangle)`
레코드 입력, 레코드 반환, import된 활동으로의 전달을 실제 실행·저장 재실행에서 검사한다.
관측 범위는 amd64·arm64의 64비트 Go int다. 좌표 차이·면적을 계산하지 않아 전체 int64
좌표를 받아도 산술 overflow가 없다. 변·꼭짓점 접촉은 겹침이 아니며 빈 교집합은
영 사각형이다. 합집합에서 첫 입력이 비면 두 번째 원본을 반환하므로, 서로 다른 빈
사각형 두 개의 합집합은 입력 순서에 따라 반환 좌표가 다르다. 검증에서 `image.Rect`,
`Canon`, 빈 사각형을 동등하게 취급하는 `Rectangle.Eq`를 쓰지 않고 네 좌표를 비교한다.
[잘라내기 예제](../examples/crop-planner/README.md)는 요청·이미지 경계의 교집합과
픽셀 존재 여부를 반환한다. 실제 이미지 읽기·픽셀 처리·면적 계산은 포함하지 않는다.

첫 8개 활동과 추가한 `math/bits` 5개 활동은 고정한 컴파일러의 `body-codegen --json`에서 `PASS`,
`route=preserve`를 반환했다. 정수·불리언·문자열 반환, 여러 입력, 조기 반환,
지역 변수, 정수 나눗셈/나머지, 레코드 생성이 이번 소스의 생성 경로를 통과했다.
이 관측은 코드 생성의 근거다. 원본 Go와 실제 출력이 같은지와 저장 재실행이 같은지는
별도의 실행 보고서에서 입력 수·불일치 수와 함께 확인한다.

컴파일러의 [본문 설명](https://github.com/kimjooyoon/meta-ontology-go/blob/366ee6ef8f9d2b9b57331dfd26b6f908c0289851/docs/language/body-codegen.md)은
Text의 `len`, 바이트 단위 슬라이싱, 정수 `/`, `%`, 스칼라·레코드 입력을 설명한다.
[workspace 설명](https://github.com/kimjooyoon/meta-ontology-go/blob/366ee6ef8f9d2b9b57331dfd26b6f908c0289851/docs/language/workspace-manifest.md)은
Gooo 패키지 import와 실행 기록의 소스 연결 및 재실행 경로를 제공한다.
Text 길이는 문자 수가 아니며 UTF-8 정규화도 수행하지 않는다.

## 다음 이식을 막는 경계

아래는 고정 리비전의 일반 `computes` 본문 검증기에서 확인한 범위다.
문법 파싱, 본문 생성, 네이티브 실행의 성공은 서로 다른 관측이다.

| 경계 | 소스 근거 | 이식에 미치는 영향 |
| --- | --- | --- |
| `for`, `range`, `switch`가 허용 문장 목록에 없음 | [body_statement_validation.go](https://github.com/kimjooyoon/meta-ontology-go/blob/366ee6ef8f9d2b9b57331dfd26b6f908c0289851/internal/bodycodegen/body_statement_validation.go) | 유한 상수 선택은 if로 옮길 수 있지만 일반 문자열 순회·컬렉션 변환은 후속 언어 작업 |
| 바이트 인덱싱 `text[i]`, `&`, `\|`, `^`, `<<`, `>>`가 허용 표현식에 없음 | [bodycodegen.go](https://github.com/kimjooyoon/meta-ontology-go/blob/366ee6ef8f9d2b9b57331dfd26b6f908c0289851/internal/bodycodegen/bodycodegen.go) | UTF-8 디코더, UUID·해시 등에서 정확한 바이트·정수 연산 지원이 필요 |
| 일반 스칼라 투영은 Integer/int64, Boolean/bool, Text/string | 같은 파일의 `goTypeForEntity` | byte/rune/uint의 폭과 overflow, 배열·슬라이스, error를 독립된 API 계약으로 설계해야 함 |
| 활동의 반환값은 하나 | `body_statement_validation.go` | Go 다중 반환은 명시적 레코드로 모델링 |
| 기본 실행 입력은 1..128개 사례, Text 1,024바이트 등의 제한 | [native-body-composition.md](https://github.com/kimjooyoon/meta-ontology-go/blob/366ee6ef8f9d2b9b57331dfd26b6f908c0289851/docs/native-body-composition.md#current-boundaries) | 대규모 비교는 제한에 맞게 나누고 장문·임의 바이트 지원을 별도로 검증 |

이 저장소의 [반복문](../probes/loop.gooo), [바이트 인덱싱](../probes/byte-index.gooo),
[비트 연산](../probes/bitwise.gooo) 예제는 이 경계를 작게 재현한다.
[probe 실행 도구](../tools/probe.sh)와 [설명](../probes/README.md)에서 실제
`body-codegen` 진단을 보존하는 방법을 확인한다.

기존 [gooo-standard-library](https://github.com/kimjooyoon/gooo-standard-library)의
Min/Max/Clamp, 논리 함수, HasPrefix/HasSuffix/StripSuffix를 새 이식 실적으로 중복 집계하지 않는다.
해당 라이브러리와 이 저장소의 목적은 각각 재사용 기본 함수와 원본 Go에 대한 동작 비교다.
후속 기능과 재현 자료의 우선순위는 [로드맵](roadmap.md)에 기록한다.
