# 기여하기

이 저장소는 원본 Go 프로그램의 동작을 읽고 Gooo로 표현하는 작업장입니다.
언어 컴파일러의 구현과는 별도로, 실제 이용자 관점의 입력·결과·제약을 축적합니다.

## Report a problem

Use the [problem report form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
for a page that will not load, a failed first install, or an unexpected function or text-inspection result.
Include the page URL or command, what you saw, and your browser/operating system if known.
For a function result, include its name, exact integer inputs, and the result you expected.
If the demo works, **Copy link to this input** can help reproduce the input; it does not
include the result. For a completed text inspection, **Download inspection JSON** provides
an optional file with the exact original text, results, and program identity. Review it
before sharing: the issue and any supplied text or files will be public. Opening the form
does not attach your input or submit anything; choose what to include and review it on GitHub.
A shared link, inspection report, version, or build metadata is optional. For a loading
failure, the page URL and visible error are enough.

처음 설치하거나 데모를 여는 단계에서 막혀도 같은 양식으로 제보할 수 있습니다.
한국어·영어 모두 괜찮습니다. 페이지 주소 또는 실행 명령, 실제 오류나 결과를 적어 주세요.
버전을 알면 릴리스 이름이나 리비전을 덧붙이고, 공유 링크는 데모가 동작할 때만 넣으면 됩니다.
텍스트 검사 JSON은 원래 입력 전체를 포함하므로 내용을 확인한 뒤 공개해도 되는 자료만
선택해서 첨부해 주세요. 보고서 없이 페이지 주소와 오류만 제보해도 됩니다.

## 포트에 기여하기

1. 원본 저장소와 파일, 커밋, 함수, 라이선스를 기록합니다. 원본을 복사하거나 변형하면
   해당 저작권과 라이선스를 보존합니다. 각 프로젝트의 라이선스를 개별 확인합니다.
2. `ports/`에 Gooo 계산을 구현하고 고유한 stable ID를 부여합니다.
   정수 폭, 문자열 인코딩, 실패값, 다중 반환과 같은 API 차이를 문서에 씁니다.
   Go에서 계산한 값을 반환하는 wrapper를 Gooo 이식으로 집계하지 않습니다.
3. `gooo.workspace.json`에 패키지를 등록하고 실제 importing example을 추가합니다.
4. Go 원본을 독립 oracle로 사용하는 입력군을 추가합니다. 정상값·경계값·잘못된 입력을
   포함하고 난수를 사용하면 시드를 고정합니다. 변환 전에 원본 입력 타입 범위를 확인합니다.
5. `upstream.json`과 API 목록을 갱신합니다. 새로운 원본 파일은 정확한 source hash를
   추가합니다. 관측 소스와 compiler/toolchain pin을 함께 바꾸지 않았다면 유지합니다.
6. 아래 검사를 실행한 뒤 입력 수, 실제 일치 수, 실패 사례와 범위를 기록합니다.
   생성 코드와 실행 receipt를 직접 수정해 성공한 결과를 만들지 않습니다.

```sh
./tools/bootstrap.sh
test -z "$(gofmt -l cmd)"
go vet ./...
go test -race ./...
python3 tools/bootstrap.test.py
python3 tools/receipt_json_test.py
go run ./cmd/verify --out out/contribution
./tools/probe.sh --out out/contribution-probes
```

현재 지원되지 않는 동작은 작은 실패 예제를 `probes/`에 추가합니다.
원하는 Go 동작, 실제 Gooo 진단, 고정 버전, 이 제한으로 막히는 포트를 함께 설명합니다.
컴파일러의 새로운 지원이 들어오면 해당 실패 probe를 성공 회귀 사례로 바꾸고
이식 함수를 실행해 비교합니다. 단순 파서 성공을 구현 지원으로 취급하지 않습니다.

컴파일러에 기여하는 경우 그 저장소의 `AGENTS.md`와 `agent/* -> dev` 절차를 따릅니다.
이 저장소의 준비된 자료를 근거로 관련 이슈나 PR을 별도로 작성할 수 있습니다.
