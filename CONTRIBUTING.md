# Contributing / 기여하기

This repository studies original Go behavior and expresses it in Gooo. It records
inputs, results, and limits from a user's perspective, separately from work on the
language compiler.

이 저장소는 원본 Go 프로그램의 동작을 읽고 Gooo로 표현하는 작업장입니다.
언어 컴파일러의 구현과는 별도로, 실제 이용자 관점의 입력·결과·제약을 축적합니다.

## Report a problem

Use the [problem report form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml)
for a page that will not load, a failed first install, or an unexpected code-generation,
function, or text-inspection result.
Include the page URL or command, what you saw, and your browser/operating system if known.
For a function result, include its name, exact integer inputs, and the result you expected.
For the [Gooo source editor](https://kimjooyoon.github.io/gooo-go-ports/codegen/),
choose **Gooo source editor** in the form and include the activity name and minimal
public source if possible. A visible error is enough if you cannot share the source.
After a successful generation, **Download generated Go** saves the current Go code
and **Download response JSON** saves the unmodified **Raw compiler response**.
The optional response includes generated Go, selected Gooo code,
and the compiler report; it is not a receipt of executing the generated program.
Review any source code and paths before sharing them publicly.
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
Gooo 소스 편집기는 양식에서 **Gooo source editor**를 선택하고, 가능하면 activity 이름과
공개 가능한 최소 소스를 적어 주세요. 소스를 공유하기 어렵다면 보이는 오류만으로도 괜찮습니다.
생성이 성공하면 **Download generated Go**로 현재 Go 코드를, **Download response JSON**으로
원본 응답을 저장할 수 있습니다. **Raw compiler response**에는 생성된 Go, 선택한 Gooo 코드와 컴파일러 보고서가 들어 있으며,
생성된 프로그램의 실행 증명은 아닙니다. 코드와 경로를 확인한 뒤 공유할 내용을 선택해 주세요.
텍스트 검사 JSON은 원래 입력 전체를 포함하므로 내용을 확인한 뒤 공개해도 되는 자료만
선택해서 첨부해 주세요. 보고서 없이 페이지 주소와 오류만 제보해도 됩니다.

You can start with a documentation correction or a concrete problem report using
the form above. Point to the affected page or command and describe the correction
or observed problem. For documentation changes, cite existing source or evidence
and describe what you checked; a text edit is not a new execution observation.
Reports and contributions in English or Korean are welcome.

문서 수정이나 위 양식을 통한 구체적인 문제 제보부터 참여할 수 있습니다.
관련 페이지나 명령과 수정할 내용 또는 관찰한 문제를 적어 주세요.
문서 변경은 기존 소스나 근거를 연결하고 확인한 내용을 설명합니다.
문장 수정 자체를 새로운 실행 관찰로 기록하지 않습니다.

## Contribute a port

1. Record the upstream repository, files, commit, functions, and license. Preserve
   upstream copyright and license notices when copying or adapting source, and
   check each project's license separately.
2. Implement the calculation in Gooo under `ports/` with a unique stable ID.
   Document API differences such as integer width, string encoding, failure
   values, and multiple returns. A wrapper that returns values computed in Go
   does not count as a Gooo port.
3. Register the port and a real importing example's package paths and sources in
   `verification-catalog.json`, and add a separate runnable workspace for the
   example. The verifier freezes all source bytes before placing shared ports
   and the selected consumer in each execution workspace. The pinned compiler
   allows at most 16 record types per program, so do not accumulate every example
   in the default `gooo.workspace.json`. Also register ports needed by the default
   example in that workspace. The catalog is a source inventory; adding an entry
   does not register an execution suite.
4. Add inputs that use the original Go implementation as an independent oracle.
   Include normal, boundary, and invalid inputs, and fix the seed for random
   samples. Check the original input type's domain before converting values.
   Add the corpus and suite to the roster assembled in `cmd/verify/main.go`.
   Wire its activity/input contract in `cmd/verify/contracts.go` or the applicable
   `*_contracts.go`, its expected result contract, and its selected source closure
   in the workspace helpers. Keep existing suites' source rosters and batch
   boundaries unchanged; the catalog does not discover suites or Go oracles.
5. Update `upstream.json` and the API inventory. Add exact source hashes for new
   upstream files. Preserve observation and compiler/toolchain pins unless this
   contribution deliberately updates those pinned sources and identities.
6. Run the validation commands below and record input counts, actual matches,
   failures, and scope. Do not edit generated code or execution receipts to make
   a result appear successful. In the new execution's `summary.json`, confirm that
   `runs` contains the new package/activity, positive `cases` and `batches`,
   `passed == cases`, and `replay_equal: true`. A passing run of only the existing
   entries does not validate the new port.

## 포트에 기여하기

1. 원본 저장소와 파일, 커밋, 함수, 라이선스를 기록합니다. 원본을 복사하거나 변형하면
   해당 저작권과 라이선스를 보존합니다. 각 프로젝트의 라이선스를 개별 확인합니다.
2. `ports/`에 Gooo 계산을 구현하고 고유한 stable ID를 부여합니다.
   정수 폭, 문자열 인코딩, 실패값, 다중 반환과 같은 API 차이를 문서에 씁니다.
   Go에서 계산한 값을 반환하는 wrapper를 Gooo 이식으로 집계하지 않습니다.
3. `verification-catalog.json`에 포트와 실제 importing example의 패키지·소스를 등록하고,
   예제를 실행할 별도 workspace를 추가합니다. 검증기는 전체 소스를 먼저 고정한 뒤
   공통 포트와 해당 소비 앱만 각 실행 workspace에 넣습니다. 고정 컴파일러의
   프로그램당 레코드 타입 16개 한도를 고려해 모든 예제를 기본 `gooo.workspace.json`에
   누적하지 않습니다. 기본 예제에 필요한 포트는 기본 workspace에도 등록합니다.
   catalog는 소스 목록이며, 항목을 추가해도 실행 suite가 자동 등록되지는 않습니다.
4. Go 원본을 독립 oracle로 사용하는 입력군을 추가합니다. 정상값·경계값·잘못된 입력을
   포함하고 난수를 사용하면 시드를 고정합니다. 변환 전에 원본 입력 타입 범위를 확인합니다.
   corpus와 suite를 `cmd/verify/main.go`에서 조립하는 실행 목록에 추가합니다.
   `cmd/verify/contracts.go` 또는 해당 `*_contracts.go`의 activity·입력 contract,
   예상 결과 contract, workspace helper의 소스 선택도 연결합니다. 기존 suite의
   소스 목록과 batch 경계는 유지합니다. catalog는 suite나 Go oracle을 자동 탐색하지 않습니다.
5. `upstream.json`과 API 목록을 갱신합니다. 새로운 원본 파일은 정확한 source hash를
   추가합니다. 관측 소스와 compiler/toolchain pin을 함께 바꾸지 않았다면 유지합니다.
6. 아래 검사를 실행한 뒤 입력 수, 실제 일치 수, 실패 사례와 범위를 기록합니다.
   생성 코드와 실행 receipt를 직접 수정해 성공한 결과를 만들지 않습니다.
   새 실행의 `summary.json`에서 `runs`에 새 package·activity가 있고 `cases`와
   `batches`가 양수이며, `passed == cases`와 `replay_equal: true`인지 확인합니다.
   기존 항목만 통과한 실행은 새 포트를 검증한 결과가 아닙니다.

Verify actual original inputs and outputs, entry and source bindings, producer
and compiler/Go identities, and the execution's own saved replay. A `PASS` label
alone is insufficient. Finite function observations do not establish whole-package
compatibility.

실제 원본 입력·출력, entry와 source 연결, producer와 컴파일러·Go 식별자,
해당 실행의 저장 재실행을 검증합니다. `PASS` 표지만으로는 충분하지 않으며,
함수의 유한 관찰로 패키지 전체 호환성을 주장하지 않습니다.

## Required CI results

Accept a change only when all three current producer checks succeed:
`observer-checks`, `native-execution`, and `browser-demo`. The first runs the
common Go/Python and generator checks, the second runs the pinned native
validation for the classified scope, and the third runs the full browser checks.
No single producer check substitutes for the other two. The separate
`native-ports` aggregate job is intentionally removed; this changes the check-name
contract rather than moving that name onto a producer with weaker meaning.

Main-branch Pages deployment directly depends on those same three jobs and
requires all three results to be exactly `success`, a main-branch push, and no
workflow cancellation. Failure, cancellation, skipping, or an absent result does
not permit deployment. The individual validation commands and coverage remain
required. GitHub branch protection/rules are separate repository configuration:
verify the current required-check settings before adopting this check-name change;
the workflow does not rewrite them or establish that they are configured.

변경 수락에는 현재 `observer-checks`, `native-execution`, `browser-demo` 세 검사의
성공이 모두 필요합니다. 공통 Go·Python·생성기 검사, 분류된 범위의 고정 native 검증,
전체 브라우저 검사를 각각 유지하며 한 검사로 나머지를 대신하지 않습니다.
별도 `native-ports` 집계 작업을 제거하는 명시적인 검사 이름 계약 변경입니다.
배포도 세 작업을 직접 의존하며, 취소되지 않은 main push에서 세 결과가 모두 정확히
`success`일 때만 허용합니다. 실패·취소·건너뜀·결과 누락은 배포 성공으로 취급하지
않습니다. 실제 적용 전 GitHub의 현재 필수 검사 설정을 확인해야 하며, 워크플로 파일
변경 자체가 원격 보호 규칙을 변경하거나 설정 완료를 증명하지는 않습니다.

Documentation CI is limited to content-only changes in these six exact paths:

- `README.md`
- `CONTRIBUTING.md`
- `.agents/skills/gooo-first-program/SKILL.md`
- `.agents/skills/gooo-first-program/agents/openai.yaml`
- `docs/ai-agents.md`
- `context7.json`

Every changed file must already exist with mode `100644` in both compared trees.
The classifier checks the clean tested checkout and exact PR merge parents.
Additions, deletions, renames, mode changes and other paths require full native
validation. Basic Go, Python,
generator, compiler identity and declaration checks, and the full browser job
still run. The native-execution job omits differential/exhaustive validation,
unsupported-body probes, native examples, malformed-input checks and policy
execution. All other changes,
main-branch runs and tag runs retain the full checks. The documentation-scope
artifact preserves the change classification and compiler identity. The job
summary explicitly records omitted checks. Neither is a new native port execution
receipt. The commands below remain the full validation checklist for port and
executable changes.

위 여섯 경로의 기존 일반 파일 내용만 바꾸는 PR이 문서 CI 대상입니다.
깨끗한 실제 checkout과 정확한 PR merge 부모를 검사하며, 비교하는 두 tree에서
각 파일의 mode가 모두 `100644`여야 합니다. 추가·삭제·이름 변경·mode 변경과
그 밖의 경로는 전체 native 검증이 필요합니다. 기본 Go·Python·생성기·컴파일러 식별자·선언 검사와 전체
브라우저 작업은 그대로 실행합니다. native-execution 작업에서는 differential·전수
검증, 지원하지 않는 본문 구문 검사, native 예제, 잘못된 입력 검사와 정책 실행을
생략합니다.
그 밖의 변경, main 브랜치 실행과 태그 실행은 전체 검사를 유지합니다.
문서 범위 artifact는 변경 분류와 컴파일러 식별자를 보존하고, 작업 요약은 생략한
검사를 명시합니다. 둘 다 새로운 native 포트 실행 receipt가 아닙니다. 아래 명령은
포트와 실행 동작 변경에 대한 전체 검증 목록으로 유지합니다.

Run these commands from the repository root on macOS or Linux in the same shell,
with Python 3 and Go toolchain selection support or Go 1.27.2. The selected Go
toolchain and pinned compiler may be downloaded on the first run. Stop if a
command fails. Repeat the exports when continuing in a new shell.

macOS 또는 Linux의 저장소 루트에서 Python 3와 도구 체인 선택을 지원하는 Go
또는 Go 1.27.2를 사용해 같은 셸에서 순서대로 실행합니다. 첫 실행은 선택한 Go
도구 체인과 고정 컴파일러를 다운로드할 수 있습니다. 명령이 실패하면 중단하고,
새 셸에서 이어갈 때는 두 환경변수를 다시 설정합니다.

```sh
export GOTOOLCHAIN=go1.27.2
export GOWORK=off
test "$(go env GOVERSION)" = go1.27.2
./tools/bootstrap.sh
test -z "$(gofmt -l cmd)"
go vet ./...
go test -race ./...
python3 tools/bootstrap.test.py
python3 tools/receipt_json_test.py
python3 tools/check-drag-crop-example_test.py
python3 tools/check-fixed-pixel-bounds_test.py
python3 -B tools/check-alpha-matte_test.py -v
python3 -B tools/check-fits-crop_test.py -v
python3 -B tools/check-fits-crop-inputs_test.py -v
go run ./cmd/gen-unicode --check
go run ./cmd/gen-simple-fold --check
go run ./cmd/gen-case-mapping --check
python3 tools/gen-uuid-quote.py --check
python3 -I -B tools/generate-payload-corpus.py --check
go run ./cmd/verify --out out/contribution
./tools/probe.sh --out out/contribution-probes
```

The Python receipt tests above may skip checks that require actual example
receipts. The [CI workflow](.github/workflows/ci.yml) also executes the importing
examples and supplies their saved execution/replay files to those checks; a
skipped test does not verify a receipt.

위 Python receipt 검사는 실제 예제 receipt가 필요한 항목을 건너뛸 수 있습니다.
[CI workflow](.github/workflows/ci.yml)는 importing 예제도 실행하고 저장한 실행·재실행
파일을 해당 검사에 전달합니다. 건너뛴 검사를 receipt 검증으로 기록하지 않습니다.

CI runs the complete native differential verifier with three bounded workers;
the receipt must record that worker count. The historical 66 suites retain all
159,909 ordered cases: their legacy partition had 2,337 batches/2,471 timing
records; the explicit UUID compact profile had 2,109 batches/2,243 timing records.
The decimal parser adds 256 inputs and PayloadAdmissionFacts adds 101 unique
seven-fact tuples, with the original 17 policy inputs first. That historical
roster totals 68 suites and 160,266 cases; the partition counts below describe
that 68-suite prefix. The current full CI roster is 82 suites and 175,649 cases,
with 2,305 batches and 2,471 timing records under the CI case profile. See
[Full native port verification](README.md#full-native-port-verification) for the
case/workspace profiles and the difference from the default local command.
CI selects
`uuid-and-literal-report-compact-v1`: the same UUID3 plus literal Report only.
Report has 235 compact batches; Index/Count and the Cut entries retain their
prior partitions. The original66 combined-profile requirement is 2,063 batches;
adding the unchanged parser5/policy4 requires 2,072 full68 batches and 2,210
timing records. These are source-partition requirements, not native completion.
The default legacy profile still requires 2,346/2,484 for full68, and the explicit
UUID-only profile still requires 2,118/2,256. Keep both historical profiles and
their tests. Summary/header must bind the new exact four-entry identity,
`basis_profile`, source map and boundary scope. Require a COMPLETE footer and reject failed,
canceled, missing or incomplete records. All original cases, oracles, source
workspaces, independent regressions and other gates remain required. Every new
batch needs fresh native execution and its own saved replay; input generation or
a prior private observation is not public current-source acceptance. Verifier correctness tests share only
compiled Go packages in a private cache owned and removed by that test process.
Each fixture still executes fresh source and writes separate evidence; the
nil-cache path and warm-cache changed-source rejection remain tested.
Use the workflow's native oracle/replay checks and automated Node/Wasm checks
for reproducible regression testing. Manual Chrome checks are optional UX
observations and do not replace those checks.


For the decimal/payload addition, preserve decoded valid UTF-8 Text<=1024 bytes
and exact signed64 metadata before conversions. Keep syntax/range distinctions
and original input echoes; Go `+1`/`-0` parse success must not imply unsigned
payload spelling or HTTP eligibility. Expected records must come from actual
pinned Go parsing plus an independently implemented policy. The 101-row fixture
is input-only and is checked reproducibly by the generator command above.
Host binding/current access remain caller facts; this pure example does not
implement request framing, body I/O, authorization enforcement or atomic quotas.

`--timing-out out/contribution/timing.jsonl` adds optional suite and batch phase
wall-time observations under the new evidence directory. CI requires its complete
footer as well as the original differential summary. The records bind source,
compiler and Go identities and report the worker/CPU context; CPU quota and memory
limits remain unknown. Incomplete, failed and canceled runs are not successful
cost samples. Concurrent wall times are not CPU time or a speed forecast.
Each batch also records optional execute/replay OS wait accounting for CPU and
MaxRSS. Missing accounting is null with an explicit status, never an invented
zero. Preserve OS units and scope; do not add MaxRSS values across commands.
See the README for the distinction between waited-command accounting and a
complete process tree, memory limit or effect authorization.
CI also prints a compact phase-total and per-suite timing summary, so analysis does
not require downloading the complete native evidence archive.

CI는 제한된 워커 3개로 전체 native 비교를 실행하고 receipt의 워커 수도 확인합니다.
기존66개 suite의159,909개 입력 순서·기대값·workspace를 보존합니다. 해당 역사적 목록은
legacy에서2,337개 배치·2,471개 계측 기록, UUID compact profile에서2,109개 배치·2,243개
기록이었습니다. 십진수 파서256개와 원본17개를 앞에 보존한 정책101개 입력을 더한
역사적 목록은68개 suite·160,266개 case이며, 아래 분할 수치는 이68개 prefix 기준입니다.
현재 전체 CI 범위는82개 suite·175,649개 case이며, CI case profile의 배치는2,305개,
계측 기록은2,471개입니다.
기본 로컬 명령과 CI의 case/workspace profile 차이는
[Full native port verification](README.md#full-native-port-verification)을 참고합니다.
현재 CI의 `uuid-and-literal-report-compact-v1`은
기존 UUID3와 literal Report만 compact 형식을 사용합니다. Report는235배치이며 Index·Count·Cut과
십진수·payload 경계는 그대로입니다. 해당66개는2,063배치를, parser5·policy4까지 더한 전체68개는
2,072배치·2,210계측기록을 요구합니다. 기본 legacy 전체68개는2,346·2,484,
명시적 UUID-only 프로필은2,118·2,256을 계속 요구하며 기존 테스트를 유지합니다.
summary/header의 새4entry 식별자·basis_profile·전체소스·경계 범위를 함께 검사합니다.
이 분할 요구 수치는 native 완료가 아닙니다. 실패·취소·누락·
미완료 없이 COMPLETE footer가 있어야 하며 모든 기존 독립 회귀 검사를 유지합니다.
입력 생성이나 이전 private 관측은 현재 공개 소스의 native 실행·저장 replay 성공이 아닙니다.
검증기 테스트는
프로세스가 소유하고 종료 시 제거하는 전용 캐시에서 Go 컴파일 결과만 공유합니다.
입력 소스의 실행과 증거는 매번 새로 만들며, 캐시가 없는 경로와 캐시가 따뜻해진 뒤
변경된 소스를 거부하는 검사도 유지합니다. 회귀 검증은 native oracle·replay와
자동화된 Node/Wasm 검사를 기준으로 삼습니다. Chrome 수동 확인은 선택적인 사용성
관찰이며 이 검사를 대신하지 않습니다.

`--timing-out out/contribution/timing.jsonl` 옵션은 새 증거 디렉터리에 suite와 batch
단계별 경과 시간을 기록합니다. CI는 기존 비교 요약과 계측 완료 footer를 함께
검사합니다. 소스·컴파일러·Go 식별자와 워커·CPU 정보를 기록하며, CPU 할당량과 메모리
제한은 미확인으로 둡니다. 미완료·실패·취소된 실행은 성공 비용 자료로 쓰지 않습니다.
동시에 실행한 작업의 경과 시간은 CPU 사용 시간이나 성능 예측값이 아닙니다.
배치별 execute·replay에는 종료 후 OS 회계가 제공하는 CPU·MaxRSS도 선택적으로
기록합니다. 관측할 수 없는 값은 상태와 null로 남기며 0으로 만들지 않습니다.
OS별 단위와 측정 범위를 보존하고, 여러 명령의 MaxRSS를 합산하지 않습니다.
전체 프로세스 트리·메모리 제한·효과 승인과의 차이는 README를 참고합니다.
CI 로그에는 단계별 합계와 suite별 소요 시간도 작게 남기므로, 병목 분석을 위해 전체
native 증거 묶음을 내려받을 필요가 없습니다.

For an unsupported behavior, add a small failing example under `probes/`.
Describe the desired Go behavior, actual Gooo diagnostic, pinned version, and
the port blocked by that limit. When compiler support arrives, turn the failing
probe into a successful regression case and execute the port for comparison.
Declaration checking or source generation alone does not establish native
execution or correct behavior on the supplied inputs.

If contributing to the compiler, follow its own `AGENTS.md` and `agent/* -> dev`
process. Evidence prepared here can support a separate related issue or PR.

현재 지원되지 않는 동작은 작은 실패 예제를 `probes/`에 추가합니다.
원하는 Go 동작, 실제 Gooo 진단, 고정 버전, 이 제한으로 막히는 포트를 함께 설명합니다.
컴파일러의 새로운 지원이 들어오면 해당 실패 probe를 성공 회귀 사례로 바꾸고
이식 함수를 실행해 비교합니다. 선언 검사나 소스 생성 성공만으로 native 실행이나
제공한 입력에서의 올바른 동작이 확인됐다고 취급하지 않습니다.

컴파일러에 기여하는 경우 그 저장소의 `AGENTS.md`와 `agent/* -> dev` 절차를 따릅니다.
이 저장소의 준비된 자료를 근거로 관련 이슈나 PR을 별도로 작성할 수 있습니다.

### Bounded Clean and original alias facts

`go/path:Clean` and `app/path-alias-facts:Report` append two normal verifier
suites over 9,841 standalone strings and 120 input pairs. Preserve the accepted
Gooo implementation, original input-only fixtures and first 24 Report anchors.
Go `path.Clean`/`path.IsAbs` supply independent expectations; no Go candidate
fallback or historical native receipt is permitted. Keep all eleven required
Report field identities and both original Texts. Decoded valid UTF-8 Text is
bounded to 1,024 bytes; lexical equality is not authorization or safe-path proof.

The new suites select only `ports/path/path.gooo`, `ports/path/clean.gooo` and
the optional Report source. They must not add a package, record or source to
any earlier 68-suite workspace. Existing sidecar scopes stay separate. Recheck
source-derived raw/translated byte partitions before changing CI counters.
Use the existing verifier/cache/workers and automated checks; do not add another
lifecycle runner or duplicate the full corpus in a separate example job.
The local quickstart checks both entries through actual execute/saved replay;
required vet/race and the fresh original hosted full70 differential CI establish
the wider integration separately. No Chrome or manual UI testing is required.
