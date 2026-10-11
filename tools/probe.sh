#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
compiler="$root/.gooo"
out=

usage() {
    printf 'Usage: %s --out NEW_DIRECTORY [--compiler PATH]\n' "$0"
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --out)
            [ "$#" -ge 2 ] || { usage >&2; exit 2; }
            out=$2
            shift 2
            ;;
        --compiler)
            [ "$#" -ge 2 ] || { usage >&2; exit 2; }
            compiler=$2
            shift 2
            ;;
        --help|-h) usage; exit 0 ;;
        *) usage >&2; exit 2 ;;
    esac
done

[ -n "$out" ] || { usage >&2; exit 2; }
[ -x "$compiler" ] || { printf 'Compiler is not executable: %s\n' "$compiler" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { printf 'Probe validation requires Python 3\n' >&2; exit 2; }
[ ! -e "$out" ] || { printf 'Output directory must be new: %s\n' "$out" >&2; exit 2; }
mkdir -p -- "$out"
out=$(CDPATH= cd -- "$out" && pwd)
cp -- "$root/upstream.json" "$out/upstream.json"
"$compiler" version --build --json > "$out/compiler-build.json"
printf 'probe\tcheck_exit\tbody_codegen_exit\n' > "$out/command-statuses.tsv"

for name in loop byte-index bitwise; do
    cp -- "$root/probes/$name.gooo" "$out/$name.gooo"
    check_status=0
    "$compiler" check "$out/$name.gooo" > "$out/$name.check.stdout.txt" \
        2> "$out/$name.check.stderr.txt" || check_status=$?
    generation_status=0
    "$compiler" body-codegen --json --activity Probe "$out/$name.gooo" \
        > "$out/$name.body-codegen.stdout.json" \
        2> "$out/$name.body-codegen.stderr.txt" || generation_status=$?
    printf '%s\t%s\t%s\n' "$name" "$check_status" "$generation_status" \
        >> "$out/command-statuses.tsv"
done

python3 - "$root" "$out" <<'PY_VERIFY'
import csv
import hashlib
import json
from pathlib import Path
import sys


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON key: " + key)
        result[key] = value
    return result


def read_json(path):
    return json.loads(path.read_text(), object_pairs_hook=unique_object)


def digest(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()


def validate(root, out):
    pin = read_json(out / "upstream.json")
    current = read_json(root / "upstream.json")
    for key in ("compiler_revision", "compiler_version", "go_version"):
        require(pin[key] == current[key], "upstream compiler pin changed during probe")
    identity = read_json(out / "compiler-build.json")
    require(identity.get("schema") == "gooo/build-identity/v1" and
            identity.get("version") == pin["compiler_version"] and
            identity.get("compiler_source_sha") == identity.get("vcs_revision") == pin["compiler_revision"] and
            identity.get("source_status") == "CLEAN_VCS" and identity.get("vcs_modified") == "false" and
            identity.get("go_version") == identity.get("native_go_required") == pin["go_version"],
            "compiler identity differs from upstream.json")
    with (out / "command-statuses.tsv").open() as stream:
        statuses = list(csv.DictReader(stream, delimiter="\t"))
    require([row["probe"] for row in statuses] == ["loop", "byte-index", "bitwise"], "probe command roster changed")
    errors = {
        "loop": "for requires an existing counter initialized to 0, counter < literal bound, and counter = counter + 1",
        "bitwise": "unsupported binary operator &",
    }
    rows = []
    for status in statuses:
        name = status["probe"]
        check_exit, generation_exit = int(status["check_exit"]), int(status["body_codegen_exit"])
        accepted = name == "byte-index"
        require(check_exit == 0 and generation_exit == (0 if accepted else 1), name + ": unexpected command exit")
        copied = (out / (name + ".gooo")).read_bytes()
        require(copied == (root / "probes" / (name + ".gooo")).read_bytes(), name + ": source copy changed")
        source_digest = digest(copied)
        raw = read_json(out / (name + ".body-codegen.stdout.json"))
        report = raw.get("report") if accepted else raw
        require(isinstance(report, dict) and report.get("schema") == "gooo/body-codegen-report/v3", name + ": report shape")
        require(accepted or "report" not in raw, name + ": rejection must be a raw report")
        decision = "PASS" if accepted else "FAIL_CLOSED"
        require(report.get("decision") == decision and report.get("compiler_source_sha") == pin["compiler_revision"], name + ": decision or producer binding")
        require(report.get("activity_id") in (None, "", "gooo://go-ports/probe/" + name), name + ": changed activity identity")
        require(type(report.get("repository_writes")) is int and report["repository_writes"] == 0, name + ": repository write boundary")
        receipt = report.get("completeness_receipt", {})
        scope = receipt.get("scope", {})
        require(scope.get("compiler_source_sha") == pin["compiler_revision"] and
                scope.get("toolchain") == pin["go_version"] and
                scope.get("boundary", {}).get("non_executing") is True,
                name + ": receipt producer, toolchain or generation-only boundary")
        # Failed raw reports may omit the top-level source_digest. Their
        # completeness dimensions still expose the exact source binding.
        bindings = []
        if "source_digest" in report:
            bindings.append(report["source_digest"])
        for dimension in receipt.get("dimensions", []):
            for item in dimension.get("evidence", []):
                if isinstance(item, str) and item.startswith("source_digest:"):
                    bindings.append(item.removeprefix("source_digest:"))
        require(bindings and all(value == source_digest for value in bindings), name + ": missing or changed source binding")
        generated_digest = ""
        if accepted:
            source = raw.get("source")
            require(isinstance(source, str) and source and
                    report.get("error") in (None, "") and
                    report.get("activity") == "Probe" and
                    report.get("activity_id") == "gooo://go-ports/probe/byte-index" and
                    report.get("input_type") == "string" and report.get("output_type") == "int64" and
                    report.get("typecheck_passed") is True and report.get("deterministic_replay") is True,
                    name + ": accepted activity or generated source binding")
            generated_digest = digest(source.encode("utf-8"))
            require(report.get("generated_digest") == report.get("replay_digest") == generated_digest,
                    name + ": generated source or emission replay digest")
        else:
            require(report.get("error") == errors[name] and report.get("source") == "", name + ": changed rejection diagnostic or retained source")
        rows.append({"probe": name, "check_exit": check_exit, "body_codegen_exit": generation_exit,
                     "generation_decision": decision, "diagnostic": report.get("error", ""),
                     "source_sha256": source_digest, "generated_go_sha256": generated_digest})
    with (out / "summary.tsv").open("w") as stream:
        fields = ["probe", "check_exit", "body_codegen_exit", "generation_decision", "diagnostic", "source_sha256", "generated_go_sha256"]
        writer = csv.DictWriter(stream, fields, delimiter="\t", lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)
    summary = {"schema": "gooo/generation-boundary-probes/v1", "status": "GENERATION_BOUNDARIES_VALIDATED",
               "compiler_revision": pin["compiler_revision"], "compiler_version": pin["compiler_version"],
               "go_version": pin["go_version"], "rows": rows, "native_execution": False,
               "scope": "Declaration checks and source-bound generation acceptance/rejection only. No generated program was executed; PASS does not establish safe indexing, bounded input behavior or full-domain compatibility."}
    (out / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")


try:
    validate(Path(sys.argv[1]), Path(sys.argv[2]))
except (ValueError, KeyError, TypeError, AttributeError, OSError) as error:
    raise SystemExit("probe validation failed: " + str(error))
PY_VERIFY

printf 'Recorded generation boundaries: 1 accepted body and 2 rejected bodies; no native execution. Evidence: %s\n' "$out"
