# Rectangle crop planner

A native Gooo example that intersects a requested crop with supplied bounds and
reports whether the intersection contains pixels. It returns exact coordinates;
it does not load, crop, or resize an image file.

[Try the browser crop planner](https://kimjooyoon.github.io/gooo-go-ports/crop-planner.html)
without installing a compiler. It runs this same importing program as a separate
precompiled WebAssembly module. Enter the two original rectangles using decimal
integers; coordinates remain exact through the full signed 64-bit range.
The page includes edge-touching, reversed, empty, negative and precision-boundary
examples. Editing an input clears the previous result; press **Plan crop** to run
the new values.

The browser adapter parses eight canonical decimal strings into the two records
and returns the four coordinates as strings with a Boolean `has_pixels`.
Intersection and empty-region decisions remain in Gooo. The bridge does not use
Go's `image` package; independent build-time comparison fixtures do.
The page links the exact generated Go, original native execution/replay and
fixture files under `crop/`. These build records describe the compiled artifact,
while pressing **Plan crop** creates a new local browser observation.

[Gooo 소스](crop_planner.gooo)는 `go/image`의 `RectangleIntersect`와
`RectangleEmpty`를 가져옵니다. 교집합과 빈 영역 판정은 Gooo에서 수행하며,
좌표를 더하거나 빼지 않습니다.

## 실행과 입력 편집

Git, POSIX shell과 Go 1.27.2를 사용하며 저장소 루트에서 실행합니다.
컴파일러는 `366ee6ef8f9d2b9b57331dfd26b6f908c0289851`로 고정합니다.
모델이나 외부 추론 서비스는 필요하지 않습니다.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
./tools/bootstrap.sh
mkdir -p out/crop-planner
./.gooo package execute --json --inputs examples/crop-planner/inputs.json \
  crop-planner.workspace.json > out/crop-planner/execution.json
./.gooo package replay --json --receipt out/crop-planner/execution.json \
  --inputs examples/crop-planner/inputs.json \
  crop-planner.workspace.json > out/crop-planner/replay.json
```

[inputs.json](inputs.json)의 `example/crop-planner:CropPlan.input0`는 원본 영역,
`input1`은 요청한 crop 영역입니다. 각각 다음 네 필드를 가진 레코드입니다.

```json
{"min_x": 0, "min_y": 0, "max_x": 640, "max_y": 480}
```

두 인수 모두 필드 값을 원본 그대로 전달합니다. 좌표는 부호 있는 int64 정수이며
−9223372036854775808부터 9223372036854775807까지 허용합니다.
`min`은 포함하고 `max`는 제외합니다. 따라서 위 영역에서 x=640이나 y=480은
포함되지 않습니다. 음수 좌표도 그대로 처리합니다.

값을 32비트로 잘라내거나 JavaScript `Number`를 거쳐 반올림하지 마세요.
예제의 `9007199254740993`도 JSON 정수로 직접 전달하면 정확히 보존됩니다.
필드 누락, `null`, 문자열, 소수, int64 범위 밖 값은 입력 단계에서 거부됩니다.
선언되지 않은 추가 필드와 중복 JSON 키도 거부되며 별도 sentinel 결과를 만들지 않습니다.

값을 바꾸면 execute로 새 기록을 만들고 replay합니다. 실제 결과는 실행 기록의
`result.runtime.traces[].deliveries[].actual`에 있습니다. 입력만 주는 실행은
`OBSERVED`이며 Go 기대값과 비교하는 검증의 `PASS`와 구분합니다.

## 결과를 읽는 법

| 필드 | 의미 |
| --- | --- |
| `min_x`, `min_y` | 교집합의 포함되는 최소 좌표 |
| `max_x`, `max_y` | 교집합의 포함되지 않는 최대 좌표 |
| `has_pixels` | 교집합이 비어 있지 않으면 true |

`has_pixels`는 주어진 사각형의 기하학적 판정입니다. 실제 이미지 파일의 존재,
픽셀 데이터, 저장 공간, 이미지 처리 라이브러리의 허용 크기를 검사하지 않습니다.

첫 입력은 640×480 영역에 `(-20,40)-(200,550)` crop을 요청합니다.
결과는 `(0,40)-(200,480)`, `has_pixels=true`입니다. 오른쪽 경계나 아래쪽 경계에
접촉하기만 하는 요청은 픽셀을 공유하지 않습니다. 빈 교집합은 원본 위치와 무관하게
네 좌표 모두 0이고 `has_pixels=false`입니다.

아래 결과는 입력 파일의 순서와 같습니다. 좌표 표기는 `(min_x,min_y)-(max_x,max_y)`입니다.

| 입력 순서 | 상황 | 결과 좌표 | `has_pixels` |
| --- | --- | --- | --- |
| 1 | 일부가 원본 영역 밖인 요청 | `(0,40)-(200,480)` | true |
| 2 | 원본 영역 전체를 덮는 요청 | `(0,0)-(640,480)` | true |
| 3 | 원본 영역 내부의 요청 | `(100,100)-(200,200)` | true |
| 4 | 오른쪽 경계에만 접촉 | `(0,0)-(0,0)` | false |
| 5 | 아래쪽 경계에만 접촉 | `(0,0)-(0,0)` | false |
| 6 | 최소·최대가 뒤집힌 요청 | `(0,0)-(0,0)` | false |
| 7 | 최소·최대가 뒤집힌 원본 영역 | `(0,0)-(0,0)` | false |
| 8 | 원점 밖에 있는 너비 0 영역 | `(0,0)-(0,0)` | false |
| 9 | 음수 좌표를 포함한 교집합 | `(-8,-30)-(5,4)` | true |
| 10 | int64 최댓값 부근 | `(9223372036854775806,9223372036854775806)-(9223372036854775807,9223372036854775807)` | true |
| 11 | int64 최솟값 부근 | `(-9223372036854775808,-9223372036854775808)-(-9223372036854775807,-9223372036854775807)` | true |
| 12 | ±2⁵³ 바깥의 정확한 좌표 | `(9007199254740993,-9007199254740992)-(9007199254740994,-9007199254740991)` | true |

## 원본 Rectangle API와의 대응

[geometry.gooo](../../ports/image/geometry.gooo)는 Go의 중첩 `Min/Max Point`를
`Rectangle{min_x,min_y,max_x,max_y}` 레코드 하나로 표현합니다.
메서드의 receiver는 첫 번째 Rectangle 인수이고, 이항 메서드의 상대 사각형은
두 번째 인수입니다. Gooo의 네 정수 필드는 amd64·arm64 Go `int`의 전체 범위와
대응합니다. Go의 32비트 `int` 동작을 대상으로 한 포트는 아닙니다.

| Gooo 활동 | Go 메서드 | 보존하는 동작 |
| --- | --- | --- |
| `RectangleEmpty` | `Rectangle.Empty` | 너비·높이 0 또는 뒤집힌 bounds는 빈 사각형 |
| `RectangleOverlaps` | `Rectangle.Overlaps` | 빈 사각형 제외, 경계 접촉만으로는 겹치지 않음 |
| `RectangleIntersect` | `Rectangle.Intersect` | 빈 교집합은 영 사각형 반환 |
| `RectangleUnion` | `Rectangle.Union` | 둘 다 비면 두 번째 원본 사각형을 그대로 반환 |

`image.Rect`나 `Rectangle.Canon`을 호출해 입력의 최소·최대를 뒤집어 정렬하지
않습니다. 특히 두 빈 사각형에 대한 `Union`은 좌표까지 비교하면 인수 순서에 따라
다를 수 있습니다. 이 crop planner는 `Intersect`와 `Empty` 두 활동만 조합합니다.

[루트 workspace](../../crop-planner.workspace.json)가 포트와 소비 앱을 함께
등록합니다. 앱의 `Rectangle` 입력 타입은 가져온 `go/image` 패키지에서 해석되므로
같은 레코드를 앱에 다시 선언하지 않습니다. 실행 manifest를 예제 디렉터리로 옮겨
`../../ports/...`를 참조하면 workspace 밖 소스 경로로 거부됩니다.

원본은 Go 1.27.2의 고정 리비전
[geom.go](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/image/geom.go)입니다.
원본 해시는 [upstream.json](../../upstream.json)에 기록하며, Copyright 2010 The Go Authors와
BSD 라이선스를 [포트 소스](../../ports/image/geometry.gooo) 및
[GO-LICENSE](../../third_party/GO-LICENSE)에 보존합니다.

## 검증 범위

```sh
go run ./cmd/verify --out out/crop-planner-check
python3 tools/check-crop-example.py out/crop-planner/execution.json out/crop-planner/replay.json
python3 tools/check-rectangle-inputs.py out/rectangle-input-rejections
```

검증 출력 경로는 새 경로여야 합니다. 독립 Go 기준값은 `image.Rectangle`과
`image.Point`의 필드에 원본 정수를 직접 넣은 뒤 해당 Go 메서드를 호출해 얻습니다.
입력을 정규화하는 `image.Rect`를 사용하지 않습니다. 포트와 소비 앱의 원본 입력,
기대값, 실제 결과, 레코드 필드 ID, 소스와 컴파일러 연결을 검사하고 저장 재실행합니다.

정규·비정규·빈 사각형, 포함·겹침·분리·경계 접촉, int64 극값과 ±2⁵³ 부근을 포함한
유한 사례 검증입니다. 모든 좌표 조합의 전수 검사가 아닙니다. 위 12개 입력만 실행한
`OBSERVED` 기록 자체는 독립 Go 비교 검증으로 집계하지 않습니다.

사각형 포트는 `Empty` 253건과 이항 메서드 각각 486건, 소비 앱은 486건을 비교합니다.
이 사각형 검사 2,197건은 도입 시점의 기존 24,770건과 합쳐 26,967건을 구성했습니다.
현재 저장소 전체의 검증 총계는 [루트 검증 범위](../../README.md#검증-범위)를 참고하세요.
위 명령의 실제 결과와 배치 수는 `out/crop-planner-check/summary.json`에 기록됩니다.
잘못된 레코드 18건의
입력 단계 거부와 위 표 12개 결과의 확인은 별도 검사입니다. 표 결과 확인 명령은
수정하지 않은 예제 입력 파일을 대상으로 하며, 입력을 바꾼 기록에는 사용하지 않습니다.
