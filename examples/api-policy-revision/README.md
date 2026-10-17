# Compare two sealed API policies in Gooo

Keep a saved API decision tied to its original policy, then evaluate exactly the
same nine facts with a candidate policy. This example compares two explicit
profiles:

- `cel-api-original-v1`: the existing CEL-derived API policy, frozen unchanged.
- `cel-api-readonly-principal-v1`: Gooo composition that preserves every base
  denial, then denies an otherwise allowed action other than `read` with
  `ACTION_NOT_READ_ONLY`. An otherwise allowed `read` also requires an explicitly
  present, nonempty raw principal; otherwise it returns `EXPLICIT_PRINCIPAL_REQUIRED`.

For the supplied synthetic `write` facts, the original allows and the candidate
denies. Exactly `allowed` and `reason` differ: `NEWLY_DENIED`, changed count `2`.
The synthetic `read` facts remain allowed with `UNCHANGED`, changed count `0`.
Both complete fifteen-field decisions remain in the report.

This is historical consistency and a hypothetical policy comparison. Supplied
authentication, tenant and action facts are not verified against a live system.
There is no API dispatcher, current authorization grant or per-call human
approval step. Consistently fabricated unsigned facts and observations can be
internally consistent; a report does not attest that an event occurred.

## Native quickstart

Start in the repository root with **Go 1.27.2 already installed and selected**.
The module requires the exact published original module and uses standard-library-only
native code. Consuming the generated profiles requires no Gooo compiler or CEL
evaluator. Toolchain installation and its initial cache setup are separate.

Create a new private directory, build once, and run each mode in a fresh process:

```sh
export GOTOOLCHAIN=local GOWORK=off GOENV=off
GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  GOPRIVATE= GONOPROXY= GONOSUMDB= \
  go -C examples/api-policy-revision mod download \
  github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy@v0.0.0-20261014100709-8cddcc4e65af
export GOPROXY=off GOSUMDB=off
umask 077
revision_run="$(mktemp -d)"
go -C examples/api-policy-revision build -trimpath \
  -o "$revision_run/compare" ./cmd/compare

for action in read write; do
  "$revision_run/compare" -mode baseline \
    -input "examples/api-policy-revision/testdata/$action-facts.json" \
    > "$revision_run/$action.request.json"
  "$revision_run/compare" -mode compare \
    -input "$revision_run/$action.request.json" \
    > "$revision_run/$action.comparison.json"
  env PATH="" "$revision_run/compare" -mode replay \
    -input "$revision_run/$action.comparison.json" \
    > "$revision_run/$action.replayed.json"
done
```

The command also accepts one JSON object on stdin when `-input` is omitted.
`baseline` creates a **new** original proof for supplied facts; it does not
repair a changed saved proof. `compare` first verifies that saved original proof,
then runs the candidate and both Gooo classifiers. `replay` recomputes and checks
the entire saved report, including both facts, all decisions, differences,
summary and complete bindings.

Keep the printed/private directory path for later replay. Files can contain raw
group, principal and tenant facts; inspect them before sharing. Validation errors
produce a nonzero exit and no JSON result. Writing stdout or a redirected file
is not an atomic file transaction.

### Read failures from an AI agent or automation

Pass `--diagnostics=json` before the other options to receive one versioned JSON
refusal on stderr. Successful programs keep their existing stdout and exit zero;
normal policy decisions such as `DENY` and `NEWLY_DENIED` are successful results.
Failures still exit one. The default `--diagnostics=text` preserves the existing
human-readable error message.

```sh
"$revision_run/compare" --diagnostics=json -mode replay \
  -input "$revision_run/write.comparison.json" \
  > "$revision_run/replayed.json" 2> "$revision_run/refusal.json"
```

The refusal schema is `gooo/api-policy-revision-diagnostic/v1`, with `command`,
`mode`, `status: "REFUSED"`, and one `diagnostics` entry containing `severity`,
`code`, `stage`, and `message`. Branch on the stable code and stage, not the
message. Codes identify invalid arguments/input, input/output I/O, input limits,
profile/generation/saved-decision mismatches, and a changed saved report.
Unclassified evaluation failures retain their known operation boundary.

Options follow Go's flag parsing rules: the last parsed diagnostics option wins.
An unknown option or a positional argument stops parsing, so a diagnostics option
after that point is not selected; an invalid format falls back to text. Nothing
after `--` is interpreted as an option.

Preserve the process exit, stdout, and stderr together. A failed output write
can leave partial stdout; failed diagnostic delivery can leave partial stderr.
Neither is a valid complete record. Diagnostics do not authorize a retry or an
API effect. This CLI remains an offline computation with no dispatcher.

## Import the published policy from another Go project

With Go 1.27.2 installed and selected, this separate project imports the public
module at immutable commit `e723f9af6dc33ecfff002a951e4b09a310439338`.
Initial dependency downloads use the public Go proxy and checksum service.
There is no consumer-owned `replace`, workspace or Gooo compiler requirement.

```sh
(
  set -eu
  export GOTOOLCHAIN=local GOWORK=off GOENV=off
  export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
  export GOPRIVATE= GONOPROXY= GONOSUMDB=
  consumer_dir="$(mktemp -d)"
  cd "$consumer_dir"
  go mod init example.local/gooo-api-review
  go get github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision@e723f9af6dc33ecfff002a951e4b09a310439338
  cat > main.go <<'GO'
package main

import (
    "fmt"
    revision "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
    original "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

func main() {
    facts, err := original.DecodeAPI([]byte(`{"group_present":true,"group_value":"admin","principal_present":true,"principal_value":"synthetic:user","authenticated":true,"caller_tenant":"tenant-a","resource_tenant":"tenant-a","requested_action":"write","allowed_action":"write"}`))
    if err != nil { panic(err) }
    saved, err := revision.Baseline(facts)
    if err != nil { panic(err) }
    report, err := revision.Compare(saved)
    if err != nil { panic(err) }
    fmt.Println(report.Summary.Classification, report.Summary.ChangedCount)
}
GO
  go run .
)
```

Expected output: `NEWLY_DENIED 2`. Both `allowed` and `reason` change; all nine
facts and both complete decisions remain in the report. This synthetic
comparison does not authenticate the facts or authorize an API action.

The parent now requires the original module at the actual Go-resolved version
`v0.0.0-20261014100709-8cddcc4e65af`. Repository builds retain the local frozen
replacement; dependency replacements are not inherited by an external project.
Go module ZIPs exclude nested modules, so the public parent ZIP omits
`profiles/original`; Go resolves that original module separately. Build and
generator reproduction still use the full source checkout described above.
`go install .../cmd/compare@version` is not a supported installation path.

Changing this dependency declaration creates a new parent generation even
though the Gooo policies and original 28 assets are unchanged. Original API
proofs retain their original generation. A saved comparison from the earlier
parent generation must be replayed with its original source version; it is not
migrated or silently rebound to this module version.

## Exact input and evidence contracts

The facts object has exactly nine fields: `group_present`, `group_value`,
`principal_present`, `principal_value`, `authenticated`, `caller_tenant`,
`resource_tenant`, `requested_action`, and `allowed_action`. Presence and
authentication fields are Boolean; the other fields are Text. Every decoded
Text is valid UTF-8 and at most **65,536 bytes**, including inactive placeholders.
The adapter preserves those placeholders; it does not trim, normalize or fold
their case. Invalid UTF-8 and isolated UTF-16 surrogate escapes are refused.

Each CLI input is one object. Raw facts and comparison requests are bounded to
**4 MiB**; saved comparison reports and the `replay` input are bounded to
**16 MiB**. The larger report limit accommodates both proofs and JSON-escaped
copies of all valid raw Text values without truncating or normalizing them.
Each nested original proof retains its original **4 MiB** decoder limit.
Missing, unknown, duplicate (including escape-equivalent), `null`, wrongly typed
or trailing fields are refused throughout the nested transport. Difference
records require all fifteen Boolean fields. Summaries require a known
classification and an integer count from `0` through `15`.

`baseline` returns `before_profile`, `candidate_profile` and `before`. An API
proof contains `facts`, its fifteen-field `decision`, and complete `bindings`.
Comparison reports use schema `gooo/api-policy-revision-comparison/v1`, status
`COMPARED`, and scope `same_saved_facts_two_sealed_profiles`. They additionally
retain `after`, fifteen `differences`, `summary`, and `classifier_bindings`.
Neither an allow nor a successful replay grants permission to execute now.

The original profile has **28 unchanged sealed assets** and the original raw
generation manifest digest
`sha256:79b76bda704cb63b9390802766a66ac72ba02c04ae135600cb05de6797c97d46`.
The candidate and classifiers have their own **38-asset generation**, including
the exact native projections and adapter. Both complete rosters are checked.
The original `VerifyAPIProof` remains strict: it still rejects a candidate or
unavailable older generation. Exactly the two named profiles are available;
there is no arbitrary runtime loader, inferred legacy profile or schema migration.

## Where the semantics live

[`readonly.gooo`](readonly.gooo) imports the original API policy and owns the
candidate decision and denial priority. [`compare.gooo`](compare.gooo) owns all
fifteen field comparisons, classification and changed count. Its two scalar-record
outputs stay within the pinned compiler's record limits. Go validates exact
transport and bound generations, carries the facts, and checks complete evidence;
it has no policy or difference-calculation fallback.

The readonly example demonstrates `UNCHANGED` and `NEWLY_DENIED`. The separate
**23 synthetic classifier pairs** exercise all fields and the additional
`NEWLY_ALLOWED`, `REASON_CHANGED` and `OBSERVATIONS_CHANGED` branches. Those pairs
are classifier inputs, not additional readonly policy paths, external requests,
ported-function cases or evidence of independent adoption.

## Reproduce the finite validation

This developer check also needs Python 3, the installed race-test C toolchain,
and an existing compiler from source
`83928fa4538245abe33b08549da47f8cbdc50ed2`, version `0.6.32-dev`, runtime
`v0.2.41-experimental`, built with Go `1.27.2`. Other compiler/runtime identities
are not substitutes. From the repository root, pass the actual installed Go
executable and choose an output directory that does not already exist:

```sh
python3 examples/api-policy-revision/verify.py \
  --compiler "$PWD/.gooo" \
  --go "$(go env GOROOT)/bin/go" \
  --out "$PWD/out/api-policy-revision-validation-unique-name"
```

The verifier runs generation checking, `go vet`, and the entire race-enabled
module test suite once, including maximum-length escaped-fact roundtrip and
oversized-input refusal checks. It then builds a temporary standalone CLI and runs
baseline, comparison and replay as three fresh processes for each supplied fact
case. Those consumer processes use an absolute binary, empty executable search
path and fresh temporary working directories. It checks every raw fact, all
fifteen before/after observations, all flags, counts, classifications and both
binding rosters against independent literal expectations. A changed report must
be refused with exit `1` and empty stdout. It also confirms source files and the
original generation remained unchanged.

`verification.json` and exact process outputs are private evidence. The expected
successful status is `FINITE_TWO_PROFILE_NATIVE_AND_SAVED_REPLAY_PASS`; it is
written only after those checks finish. Timing fields describe that machine and
its existing setup/cache. They are not an independent first-user measurement or
a promise of a ten-minute installation. Empty `PATH` is an observed process
configuration, not system-call instrumentation proving zero network, compiler
or external-effect calls.

## Frozen source and attribution

[`profiles.json`](profiles.json) pins the original module copied from
[`gooo-go-ports` commit `8cddcc4`](https://github.com/kimjooyoon/gooo-go-ports/tree/8cddcc4e65afb42431f8f42a86f29859ae2f4d69/examples/cel-api-policy).
Its original module path, internal imports, 28 asset bytes and raw generation
manifest are preserved under [`profiles/original`](profiles/original).
The parent requires that exact published module, with both Go checksums sealed
in its `go.sum`; it has no local `replace`. The archived original `go.mod` and
license/notice packaging files remain separately bound rather than retroactively
added to an old proof's asset roster.

Google's bounded CEL codelab adaptation and the original Gooo API policy retain
their [source notice](profiles/original/NOTICE.md), [upstream license](profiles/original/oracle/upstream/LICENSE)
and [CEL runtime release license](profiles/original/LICENSE-APACHE-2.0).
The new readonly policy, classifiers and comparison transport are original Gooo
ecosystem research, with AI-assisted development. This example implies no
Google/CEL/AgentMesh endorsement and implements no live mesh session, token
verification, tool-definition continuity or real API effect.


---

## 정책 변경의 생성 미리보기와 실제 반영

현재 후보 `cel-api-readonly-principal-v1`은 아래 principal 규칙을 포함합니다.
이전 `cel-api-readonly-v1`로 저장한 요청이나 비교 결과는 해당 버전의 모듈에서
다시 확인해야 합니다. 새 후보의 이름으로 바꾸어 연결할 수 없습니다.

`proposals/api-policy-revision.patch`는 이 변경을 고정 원본에 전달하는 입력입니다.
미리보기는 원본에서 복사한 임시 디렉터리에만 적용하며 현재 checkout을 덮어쓰지 않습니다.
원본 생성기를 실행하고 소스·생성 결과의 Git 패치와 실제 종료 상태를 남깁니다.
제안을 담은 커밋과 생성 입력인 원본·패치, 컴파일러의 빌드 출처를 각각 기록합니다.

현재 대상은 `examples/api-policy-revision`의 `readonly.gooo`, `model.go`,
`profiles.json` 세 파일입니다. Gooo가 요청의 허용 조건을 소유합니다.
Go 변경은 후보 프로필의 식별 상수 한 곳이며, 기존 원본 프로필을 보존합니다.
Git의 full-index 패치가 세 파일의 원문 변경을 전달합니다.
현재 파일의 raw 본문 한 개를 대상으로 하고, 본문 앞뒤의 선언을 보존합니다.

생성 성공은 전체 정책 검증 성공과 별개입니다. 생성 결과는 일반 후속 PR로
반영하고, 독립적인 기대값·기존 네 필수 작업·실제 main CI를 확인해야 합니다.
생성 상태는 미실행·미확정·실패·성공으로 구분합니다. 기존 generation과 관측한
generation을 따로 보존하고, 성공과 모든 파일의 연결을 확인한 뒤 새 생성물로 표시합니다.
미리보기의 쓰기 범위는 임시 복사본과 결과 파일입니다.

이번 제안은 명시적이고 비어 있지 않은 principal이 없는 read 요청을 거절합니다.
기존 거절의 우선순위, 입력 아홉 개, 결과 필드 열다섯 개, 활동 ID를 유지합니다.
입력이 제공한 principal 원문을 평가하며, 공백 문자열도 그대로 유지합니다.
독립적인 일곱 사례에서 원본·후보의 전체 결과 14개와 차이·요약을 비교합니다.
이전 프로필의 요청과 저장된 결과를 새 후보에 연결하는 경우도 별도로 거절해야 합니다.

매개변수 위치는 유지하고 기존 `let`으로 본문에 역할 이름을 연결합니다.

```gooo
let principal_present = input2
let principal = input3
if allowed && (!principal_present || principal == "") { allowed = false
 reason = "EXPLICIT_PRINCIPAL_REQUIRED" }
```

현재 추가 비용도 함께 셉니다. 조건 두 줄과 역할 연결 두 줄 외에 프로필 연결, 미리보기
워크플로, 테스트와 문서가 필요합니다. Go 코드 비중이 낮다는 사실만으로
확장 비용이 줄었다고 판단하지 않습니다. 미리보기 생성의 원본 실행과 파일 해시를
확인했으며, 전체 기능·배포 수용은 각 소스와 실제 main의 필수 CI를 따로 확인합니다.
일곱 사례의 성공도 모든 입력에 대한 완전성을 뜻하지 않습니다.

준비가 중단되면 그 단계와 종료 상태를 보존하고, 생성 시도 여부를 따로 기록합니다.
본문 추출은 실제 원본과 후보에 적용하며 빈 줄·추가 선언·import·활동 ID 경계를 검사합니다.


### 이 변경에서 맡기는 일

Gooo 본문은 허용 조건과 거절 우선순위를 표현합니다. 기존 입력 위치를 유지하고
`let` 두 개로 principal의 역할 이름을 붙였습니다. Go는 생성된 함수를 호출하고,
저장한 입력·출처 연결·전체 결과를 다룹니다. 정책 조건을 Go에 한 번 더 구현하지 않습니다.

현재 배포 연결부에는 프로필과 generation의 고정값을 확인하는 여러 소비자가 있습니다.
같은 checkout의 프로필 상수와 파일 수는 한 곳에서 관리하고, 긴 브라우저 빌더는
준비·실행·원문 보존 확인·게시 단계로 나눴습니다. 독립적인 테스트 기대값과 브라우저의
바이트 확인은 각 검사의 역할로 유지합니다. 예전 모듈에 고정된 CI Action도 기원을 유지합니다.

공백 principal은 비어 있지 않은 원문으로 취급합니다. 이 예제는 입력에 담긴 값을
비교하므로 실제 사용자의 신원이나 현재 접근 권한을 확인해 주지는 않습니다.


### 생성물의 기원

이번 생성물은 [원본 미리보기 실행](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39059588536)에서
만들어졌습니다. 이 실행의 성공을 새 소스의 기능 검증 결과로 사용하지 않습니다.
새 소스 CI는 생성 확인, 독립적인 전체 결과와 저장 결과 재검사를 다시 수행합니다.

| 구분 | 고정값 |
| --- | --- |
| 제안을 담은 커밋 | `8818f85dfa620c30a298b976f60c6af0c2a05070` |
| 생성 입력의 원본 | `b19b5517f8e9716affb0f75fd07045b8217cdaf1` |
| 입력 Git 패치 SHA-256 | `3afdc031b5fa32bb63d9768a8a67d2a76c86c85ad894c7583f2ba2f30ee2ae0f` |
| 컴파일러 소스 | `83928fa4538245abe33b08549da47f8cbdc50ed2` |
| Go | `1.27.2` |
| 생성 결과 Git 패치 SHA-256 | `3993fc7cc28336f97c223c62b35049febc4617fc6acd1a17dff399767752a266` |
| 이 정책 제안 반영 당시 generation.json SHA-256 | `af9452cce284a6b944adf50c62f6b9f5c1a0647d6f1794d3549dd5dae7ff43ee` |

`generation.json`의 37개 파일 연결과 원문 해시를 확인하고 그대로 가져왔습니다.
활동 ID와 입력·결과의 형태는 유지하며, 구체적인 정책 본문이 다른 새 후보로 구별합니다.


분리한 브라우저 빌더 파일도 기존 소스 관측 목록에 등록합니다. 이 목록은 기존
282개 원문과 새 파일 1개를 합한 283개이며, 현재 API 모듈의 지원 파일 83개와는 다른
범위입니다. 파일이 빠지거나 같은 개수의 다른 파일·해시로 바뀌는 경우를 거절합니다.
기존 포팅의 사례와 분할은 이 목록 변경에서 그대로 보존합니다.

The current packaging seals `go.sum` as the 38th asset and requires the original
public module at its unchanged immutable version above. Its generation is
`sha256:24f37267a7f4ad723b7228eeecd55fdc151c8534fa8a95bca19bb73d9b3973d0`.
The earlier 37-asset generation recorded above remains historical; its saved
reports are not migrated to this generation. The policy source and generated
algorithms are unchanged. Dependency preparation uses the Go proxy and checksum
service once; the generated command and saved-report replay run offline.
