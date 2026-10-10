# A byte-domain guard with source-owned condition cases

바이트 범위 검사의 최종 반환값이 같더라도 내부 조건의 의미는 다를 수 있습니다.
새 개발 컴파일러의 조건 선언이 이 차이를 구분하는지 확인한 재현 자료입니다.
기존 포트 5,510건과 브라우저 실행의 호환성 관측은 [별도 기록](compatibility.md)에 있습니다.

This finite research probe tests Gooo engine commit `92dc06b38f72518443f8a90e5c98a645f0c756f1`. It does not change the published compiler pin, production consumers, or the count of 15 ported functions.

`ByteOrReject(input)` preserves an exact signed 64-bit integer in `0..255` and returns `-1` otherwise. This is a small input-validation adapter, not a Go `byte` conversion: it never clips, wraps, or narrows its input. The experiment deliberately gives the selector two output-equivalent implementations:

```text
inside/accept:  if 0 <= input && input <= 255 { return input } else { return -1 }
outside/reject: if input < 0 || input > 255   { return -1 }    else { return input }
```

Both implement the same output contract; their `if` conditions have opposite truth values. This constructed ambiguity is useful when the author also wants the branch predicate to mean “reject an input outside the byte domain.” It is not an ordinary runtime bug or evidence of model understanding.

The [baseline recipe](baseline.gooo) and [constrained recipe](constrained.gooo) differ only by 13 `condition_case` declarations. Both declare the same 13 output examples and the same two binary choices: `local_reference` at `0` switches the Boolean local read, and `branch_layout` at `0` switches the branch bodies. Four combinations are possible, with an attempt budget of four. The initial body intentionally fails most output examples so selection must occur.

| Observation | Baseline | With source conditions |
|---|---:|---:|
| Selected mask | 1 | 2 |
| Predicate / layout | `inside` / forward | `outside` / reverse |
| Distinct native output cases matched | 268/268 | 268/268 |
| Saved replay output cases matched | 268/268 | 268/268 |
| Declared source conditions | none | 13/13 matched |
| Independently observed `if` truth equals `outside` | 0/268 | 268/268 |
| Model calls | 0 | 0 |

The constrained search explicitly rejected mask 1 although its 13 output examples passed: all 13 condition observations were opposite to the declared expectations. It selected mask 2. The baseline has no source-condition metric; its `0/268` result is the separate observer's comparison against the desired outside predicate, not a compiler-reported failed requirement.

## What was executed and checked

The corpus contains all 256 byte values and 12 invalid inputs: `-9223372036854775808`, `-9007199254740993`, `-257`, `-256`, `-2`, `-1`, `256`, `257`, `511`, `512`, `9007199254740993`, and `9223372036854775807`. A separate [Go oracle](support/oracle.go.txt) computes outputs and both predicates using `int64`. JSON is parsed with exact integers, without a floating-point conversion. The caller construction uses four inputs (`-2`, `0`, `255`, `256`).

Each arm performs one fresh construction/evaluation on the first 128 inputs, then replays the saved construction on three batches of 128, 128, and 12 inputs. Thus there are 268 distinct evaluated inputs per arm; the first 128 are evaluated again during replay. All 13 source examples and four construction inputs overlap the evaluation corpus. This is not a held-out accuracy claim.

The validator checks actual delivery inputs, outputs, the independently known Main activity ID, source/generated-Go hashes, exact compiler revision, Go executable hash, and zero model-call fields. Saved replay receives the original construction receipt file. The emitted replay construction record has equal parsed content; some JSON key order changes during engine serialization, so each raw file is retained separately. Replay evaluation reuses a verified native artifact and toolchain within the same invocation, while its two evaluation executions run as new processes. It is not described here as a separate rebuild for every evaluation.

For an additional independent condition check, a [small Go AST extractor](support/instrument.go.txt) requires exactly one final `if` in the generated helper. It copies the preceding local statements and returns that original `if` expression from a new Boolean observer. The original generated Go file is copied byte-for-byte and compiled with the observer. Both the original output and observed condition are compared against the Go oracle for all 268 inputs. This observer is deliberately specific to this straight-line, one-`if` fixture; it is not a general control-flow validator. The engine's own intermediate-condition receipt still covers only the 13 declared conditions.

No model is loaded. Model flags are omitted, and `GOOO_LAYA_URL` and `GOOO_LAYA_API_KEY` are removed from the child environment. Selection is a deterministic finite search, with no model training or provider calls.

## Reproduce

Requirements: Python 3, Go **1.27.2**, and clean Gooo binaries built from these exact revisions:

- Candidate: `92dc06b38f72518443f8a90e5c98a645f0c756f1`, decision runtime `v0.2.28-experimental`.
- Published comparison: `366ee6ef8f9d2b9b57331dfd26b6f908c0289851`, decision runtime `v0.2.26-experimental`.

Both report `0.6.24-dev`, so the version string alone does not identify this feature. The supplied producer identities and hashes distinguish them. The [compatibility note](compatibility.md#reproduce-in-a-separate-directory) gives exact checkout and build commands for the candidate. The repository's `tools/bootstrap.sh` builds the published comparison. The candidate can also be built with `go build -trimpath -o /path/to/candidate-gooo ./cmd/gooo` from a clean checkout of the exact candidate commit; keep the production pin unchanged.

From this directory, run:

```sh
python3 reproduce.py \
  --candidate /path/to/candidate-gooo \
  --published /path/to/published-gooo \
  --go-bin /path/to/go1.27.2/bin/go \
  --out /tmp/gooo-byte-guard-reproduction
```

The output directory must not exist. The script copies the binaries into its output `tools/` directory and links the installed Go executable; it does not download or rebuild the engine. A separate output `go.mod` keeps generated observer files out of the parent repository's package checks. Its Go support files use `.go.txt` suffixes in the repository and are copied to the output directory before compilation. Both binary revisions and clean-source build identities are checked before execution. `support/validate.py` writes `verification-report.json` during reproduction and checks the copied tool binaries there; it is not an offline verifier for the tools-excluded archive. `FILES.sha256` records the evidence bytes, excluding tools.

The engine commands, as actually recorded relative to the output directory, include:

```sh
tools/candidate-gooo body-construct \
  --source constrained.gooo --entry Main \
  --construction-cases construction-cases.json --cases evaluation-0.json \
  --attempts 4 --go-bin tools/go --out constrained-fresh

tools/candidate-gooo body-construct \
  --source constrained-fresh/original.gooo \
  --construction constrained-fresh/construction.json \
  --cases evaluation-1.json --go-bin tools/go --out constrained-replay-1
```

The published binary is tested once against the constrained recipe. The expected result is exit **1** with `unknown assembly field condition_case`, not successful execution. The full diagnostic is preserved. Its error envelope has `generated_now: true` because new-construction mode was requested; that flag does not mean successful generation. The failure provides no generated program or successful runtime evidence. The reproducer fails if it unexpectedly accepts the new syntax.

## Evidence and scope

[summary.json](summary.json) is derived from the independently validated public rerun. [raw-evidence.tar.gz](raw-evidence.tar.gz) contains that run's unedited receipt files, stdout/stderr, generated sources, oracle/observer outputs, relative commands, and internal `FILES.sha256`. [SHA256SUMS](SHA256SUMS) covers the public fixture and evidence files. The archive excludes compiler/tool executables. Its checksums verify the recorded bytes; to run the validator, use the full reproducer above so the identified executables are available. Extract it into a new directory and verify its manifest with `sha256sum -c FILES.sha256` from inside `evidence/` (or `shasum -a 256 -c FILES.sha256` on macOS).

This adds a practical byte-domain example beyond upstream's absolute-value fixture: compound Boolean locals, local-reference selection, both byte boundaries, all byte values, and extreme exact integer rejection are exercised. It does not establish correctness for every signed 64-bit integer, add record/multi-input condition contracts, certify arbitrary consumer code, or measure performance. Existing color/Unicode consumer records remain outside the feature's Integer-to-Integer typed-path scope.

The exact upstream contract and tests are available at the candidate commit:

- [Syntax, observed-condition semantics, replay CLI, and finite limits](https://github.com/kimjooyoon/meta-ontology-go/blob/92dc06b38f72518443f8a90e5c98a645f0c756f1/docs/source-assembly.md#L192).
- [Condition declarations: choices, mixing restrictions, and 128-case limit](https://github.com/kimjooyoon/meta-ontology-go/blob/92dc06b38f72518443f8a90e5c98a645f0c756f1/internal/assemblyspec/conditions.go#L5).
- [Local-reference and branch-layout choices](https://github.com/kimjooyoon/meta-ontology-go/blob/92dc06b38f72518443f8a90e5c98a645f0c756f1/internal/bodycodegen/body_path_recipe_choices.go#L35).
- [Caller files: 1–128 cases, 32 KiB, and exact int64 input](https://github.com/kimjooyoon/meta-ontology-go/blob/92dc06b38f72518443f8a90e5c98a645f0c756f1/internal/bodyexecution/composition_cases.go#L20).
- [Upstream output-equivalent predicate rejection test](https://github.com/kimjooyoon/meta-ontology-go/blob/92dc06b38f72518443f8a90e5c98a645f0c756f1/internal/bodycodegen/body_path_conditions_test.go#L79).

In this revision, source conditions target a `branch_layout` choice or an `operand_order` expression that is the whole condition of exactly one `if`. The source assembly is limited to Integer → Integer typed paths; conditions cannot be mixed with `search`, `source_fill`, or record `value_case` assembly. A skipped branch is `NOT_REACHED`, not false. These limitations are retained by this experiment.
