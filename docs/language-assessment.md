# Go 라이브러리 이식을 위한 Gooo 관측

2026-10-10 KST 기준으로 Gooo **0.6.24-dev**, 컴파일러 소스
`366ee6ef8f9d2b9b57331dfd26b6f908c0289851`, Go **1.27.2**를 사용한다.
`gooo version --build --json`에서 `CLEAN_VCS`, 해당 소스 SHA,
`go_version=go1.27.2`, `native_go_required=go1.27.2`를 확인했다.
이는 이 저장소가 고정한 조합이며 모든 설치 환경의 기본 버전을 뜻하지 않는다.

## 이번에 옮기는 동작

Go 1.27.2의 공개 소스와 동작을 읽고, 다음 **13개 함수**를 Gooo 활동으로 표현한다.
패키지 전체, HTTP 서버, 문자열 인코더 전체 또는 Go API와의 바이너리 호환을 주장하지 않는다.

| 원본 | Gooo 원본 | 표현 방법과 호환 범위 |
| --- | --- | --- |
| `net/http.StatusText` | [http.gooo](../ports/net/http.gooo) | 상태 코드별 텍스트를 `if`로 선택. 등록되지 않은 코드는 빈 문자열 |
| `unicode/utf8.ValidRune`, `RuneLen` | [utf8.gooo](../ports/unicode/utf8.gooo) | 코드 포인트 경계와 서로게이트 구간 비교. Go `rune` 입력 범위를 Integer로 전달 |
| `unicode/utf8.RuneStart` | 같은 파일 | Go `byte` 범위인 0..255에서 연속 바이트 구간 128..191 제외. 범위 밖은 별도 확장 계약인 false |
| `unicode/utf16.IsSurrogate`, `RuneLen` | [utf16.gooo](../ports/unicode/utf16.gooo) | 범위 비교로 서로게이트와 UTF-16 길이를 판별 |
| `unicode/utf16.DecodeRune`, `EncodeRune` | 같은 파일 | 유효 범위를 먼저 검사한 뒤 곱셈·나눗셈·나머지로 두 10비트 성분 계산. EncodeRune의 두 결과는 `high`, `low` 레코드로 전달 |
| `math/bits.Len8`, `LeadingZeros8`, `TrailingZeros8`, `OnesCount8`, `Reverse8` | [bits.gooo](../ports/math/bits.gooo) | 0..255를 먼저 검사한 뒤 비교와 정수 산술로 계산. 테이블·비트 연산·반복문을 쓰지 않으며 영역 밖은 -1이라는 별도 확장 계약 |

Go의 `rune`은 int32이고 Gooo Integer는 int64다. 테스트에서 int64를 Go rune/byte로
무조건 변환하면 잘림으로 잘못된 동등성 주장이 생긴다. 원본 입력 범위의 비교와 Gooo
확장 입력의 계약을 나눈다. 반환 레코드는 다중 반환값을 표현하는 명시적 API 변경이다.
비트 연산의 산술 변환도 검사한 비음수 구간에서만 적용한다.

원본은 Go의 [status.go](https://github.com/golang/go/blob/go1.27.2/src/net/http/status.go),
[utf8.go](https://github.com/golang/go/blob/go1.27.2/src/unicode/utf8/utf8.go),
[utf16.go](https://github.com/golang/go/blob/go1.27.2/src/unicode/utf16/utf16.go)다.
`math/bits`는 같은 리비전의 `bits.go`, `bits_tables.go`, `make_tables.go`에 근거한다.
테이블 값을 수동 복제하지 않고 길이 임계값과 8단계 산술 계산으로 표현한다.
저작권 표기와 BSD 라이선스를 보존한다.

## 확인한 언어 기능

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
