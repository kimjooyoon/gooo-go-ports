"""Package an already checked API-policy command; never build or execute code."""
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import stat
import tarfile

PARENT = "github.com/kimjooyoon/gooo-go-ports/examples/api-policy-revision"
ORIGINAL = "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
PIN = "83928fa4538245abe33b08549da47f8cbdc50ed2"
MAX_BINARY, MAX_ARCHIVE = 16 << 20, 32 << 20
BOOLS = {"group_present", "principal_present", "authenticated"}
TEXTS = {"group_value", "principal_value", "caller_tenant", "resource_tenant",
         "requested_action", "allowed_action"}


def need(condition, message):
    if not condition:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(path, cap):
    need(stat.S_ISREG(path.lstat().st_mode), "regular file required: " + str(path))
    need(path.stat().st_size <= cap, "file exceeds bundle bound")
    with path.open("rb") as stream:
        raw = stream.read(cap + 1)
    need(len(raw) <= cap, "file grew over bundle bound")
    return raw


def matched(path, reference, cap):
    raw = read(path, cap)
    need(type(reference.get("bytes")) is int and reference["bytes"] == len(raw)
         and reference.get("sha256") == sha(raw), "saved file identity mismatch")
    return raw


def encoded(value):
    return (json.dumps(value, sort_keys=True, indent=2, ensure_ascii=True,
                       allow_nan=False) + "\n").encode()


def unique(items):
    result = {}
    for key, value in items:
        need(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def facts(value, action):
    need(type(value) is dict and set(value) == BOOLS | TEXTS, "exact nine facts required")
    for name in BOOLS:
        need(type(value[name]) is bool, "Boolean fact required")
    for name in TEXTS:
        need(type(value[name]) is str, "Text fact required")
        need(len(value[name].encode("utf-8")) <= 65536, "Text fact exceeds bound")
    need(value["requested_action"] == value["allowed_action"] == action,
         "synthetic action differs from checked case")
    raw = encoded(value)
    need(len(raw) <= 4 << 20, "facts exceed CLI input bound")
    return raw


def checked_batch(direct, accepted):
    """Advertise batch use only when the producer actually checked that mode."""
    keys = {"direct_install_batch_rows", "direct_install_batch_tampered_report_refused"}
    if "batch" not in direct and not keys.intersection(accepted):
        return None
    batch = direct.get("batch")
    need(isinstance(batch, dict) and accepted.get("direct_install_batch_rows") == 3
         and accepted.get("direct_install_batch_tampered_report_refused") is True,
         "complete batch acceptance required")
    need(batch.get("rows") == 3 and batch.get("order") == ["write", "read", "write"]
         and batch.get("complete_records_exact") is True and batch.get("replay_bytes_exact") is True
         and re.fullmatch(r"[0-9a-f]{64}", batch.get("comparison_sha256", "")),
         "ordered complete batch replay not checked")
    tamper = batch.get("late_tampered_report", {})
    need(type(tamper.get("exit_code")) is int and tamper["exit_code"] == 1
         and type(tamper.get("stdout_bytes")) is int and tamper["stdout_bytes"] == 0,
         "late batch tamper was not refused without output")
    diagnostic = tamper.get("diagnostic", {})
    items = diagnostic.get("diagnostics", [])
    need(diagnostic.get("schema") == "gooo/api-policy-revision-diagnostic/v1"
         and diagnostic.get("command") == "compare" and diagnostic.get("mode") == "replay"
         and diagnostic.get("status") == "REFUSED" and len(items) == 1,
         "batch refusal envelope differs")
    item = items[0]
    need(item.get("severity") == "error" and item.get("code") == "REPORT_MISMATCH"
         and item.get("stage") == "verify_comparison" and type(item.get("item_index")) is int
         and item["item_index"] == 2 and isinstance(item.get("message"), str) and item["message"],
         "late batch refusal location differs")
    return batch


def verify_archive(path, expected):
    """Read all gzip bytes (including CRC), then verify the exact flat tar roster."""
    raw = read(path, MAX_ARCHIVE)
    with gzip.GzipFile(fileobj=io.BytesIO(raw)) as stream:
        unpacked = stream.read(MAX_ARCHIVE + 1)
    need(len(unpacked) <= MAX_ARCHIVE, "expanded archive exceeds bound")
    with tarfile.open(fileobj=io.BytesIO(unpacked), mode="r:") as archive:
        members = archive.getmembers()
        need([item.name for item in members] == sorted(expected), "archive roster differs")
        for item in members:
            content, mode = expected[item.name]
            need(item.isfile() and item.mode == mode and item.uid == item.gid == item.mtime == 0
                 and not item.uname and not item.gname and not item.pax_headers
                 and item.size == len(content), "archive member metadata differs")
            with archive.extractfile(item) as stream:
                need(stream.read(len(content) + 1) == content, "archive member bytes differ")
        need(not any(unpacked[archive.offset:]), "unexpected data after tar members")
    return {"path": str(path), "bytes": len(raw), "sha256": sha(raw)}


def make_bundle(binary: Path, out: Path, report: dict, go: Path, root: Path) -> dict:
    need(report.get("status") == "PUBLIC_API_POLICY_CONSUMER_PASS"
         and report.get("temporary_sources_binaries_build_cache_removed") is True,
         "successful consumer checks and cleanup required")
    need(out.is_dir() and not out.is_symlink(), "owned output directory required")
    direct, accepted = report["direct_install"], report["acceptance"]
    need(accepted.get("direct_install") is True and accepted.get("direct_install_cases") == 2
         and accepted.get("direct_install_tampered_report_refused") is True
         and accepted.get("runtime_empty_path") is True and direct.get("runtime_empty_path") is True,
         "direct-install finite acceptance required")
    command = matched(binary, direct["binary"], MAX_BINARY)
    need(bool(binary.stat().st_mode & 0o111), "tested command must be executable")
    reference = direct["build_info_source"]
    need(Path(reference["path"]).parent == out, "BuildInfo must be a producer output")
    raw_build = matched(Path(reference["path"]), reference, 1 << 20)
    build = json.loads(raw_build.decode("utf-8"), object_pairs_hook=unique)
    settings = unique((item["Key"], item["Value"]) for item in build["Settings"])
    platform = settings["GOOS"] + "/" + settings["GOARCH"]
    need(platform in {"linux/amd64", "darwin/arm64"} and settings.get("CGO_ENABLED") == "0"
         and report["go_toolchain"]["version"] == "go version go1.27.2 " + platform
         and build["GoVersion"] == "go1.27.2" and build["Path"] == PARENT + "/cmd/compare",
         "native toolchain/platform differs")
    need(go == Path(report["go_toolchain"]["executable"]["path"]), "Go license root differs")
    main, deps = build["Main"], build["Deps"]
    need(main["Path"] == PARENT and len(deps) == 1 and deps[0]["Path"] == ORIGINAL,
         "module graph differs")
    for module in [main, *deps]:
        need(module.get("Replace") is None and isinstance(module.get("Version"), str)
             and re.fullmatch(r"h1:[A-Za-z0-9+/]{43}=", module.get("Sum", "")),
             "module version/checksum/replacement differs")
    normalized = {"go_version": build["GoVersion"], "package": build["Path"],
                  "main": {k: main[k] for k in ("Path", "Version", "Sum")},
                  "dependencies": [{k: deps[0][k] for k in ("Path", "Version", "Sum")}],
                  "no_replacements": True}
    need(normalized == direct["build_info"] and direct["target"] == build["Path"] + "@" + main["Version"],
         "saved BuildInfo summary differs")
    revision = report["requested_revision"]
    need(re.fullmatch(r"[0-9a-f]{40}", revision)
         and report["resolved"]["parent"]["Origin"]["Hash"] == revision
         and report["resolved"]["parent"]["Version"] == main["Version"], "source revision differs")
    identities = report["consumer_fixture_source_contract"]["identities"]
    need(identities["compiler_source_sha"] == PIN, "compiler source differs")
    for side in ("original", "revision"):
        digest = identities[side + "_generation_sha256"]
        source_key = "current_" + ("original" if side == "original" else "parent") + "_generation"
        need(re.fullmatch(r"sha256:[0-9a-f]{64}", digest)
             and digest == report["source"][source_key], "policy generation differs")
    cases = direct["cases"]
    need(len(cases) == 2 and [case["action"] for case in cases] == ["read", "write"], "two checked cases required")
    for case in cases:
        need(all(case.get(k) is True for k in ("baseline_complete", "comparison_complete", "replay_bytes_exact"))
             and re.fullmatch(r"[0-9a-f]{64}", case["report_sha256"]), "case replay not checked")
    tamper = direct["tampered_report"]
    need(tamper["field"] == "differences.reason" and type(tamper["exit_code"]) is int
         and tamper["exit_code"] == 1 and type(tamper["stdout_bytes"]) is int
         and tamper["stdout_bytes"] == 0, "tampered report was not refused")
    diagnostic = tamper["diagnostic"]
    need(diagnostic["status"] == "REFUSED" and len(diagnostic["diagnostics"]) == 1
         and diagnostic["diagnostics"][0]["code"] == "REPORT_MISMATCH"
         and diagnostic["diagnostics"][0]["stage"] == "verify_comparison", "tamper refusal differs")
    batch = checked_batch(direct, accepted)
    text = ("Gooo API policy comparison — " + platform + "\n"
            "Gooo language: https://github.com/kimjooyoon/meta-ontology-go\n"
            "Source guide: https://github.com/kimjooyoon/gooo-go-ports/blob/" + revision + "/examples/api-policy-revision/README.md\n"
            "No Gooo compiler, Go or Python runtime installation is needed.\n"
            "From this extracted directory, run:\n"
            "set -eu\n"
            "./gooo-policy-compare --diagnostics=json --mode baseline --input write-facts.json > request.json\n"
            "./gooo-policy-compare --diagnostics=json --mode compare --input request.json > comparison.json\n"
            "./gooo-policy-compare --diagnostics=json --mode replay --input comparison.json > replayed.json\n"
            "Repeat with read-facts.json, or supply your own exact nine-field facts object.\n"
            "These are two fixed policy profiles, not a runtime policy loader.\n"
            "DENY/NEWLY_DENIED are successful results (exit 0); refusals are nonzero.\n"
            "Preserve exit, stdout and stderr; failed writes can leave partial output.\n"
            "Inputs and observations are unsigned supplied facts; replay checks consistency.\n"
            "This command authenticates no live identity and performs no API effect.\n")
    if batch is not None:
        text += ("\nCompare a batch with this same command; no rebuild or language runtime is needed.\n"
                 "Use a fresh private directory for the supplied read/write pair:\n"
                 "set -eu\n"
                 "umask 077\n"
                 "batch_run=\"$(mktemp -d)\"\n"
                 "printf 'Saved batch files: %s\\n' \"$batch_run\"\n"
                 "{ printf '[\\n'; cat read-facts.json; printf ',\\n'; cat write-facts.json; printf ']\\n'; } > \"$batch_run/facts.json\"\n"
                 "./gooo-policy-compare --diagnostics=json --batch --mode baseline --input \"$batch_run/facts.json\" > \"$batch_run/requests.json\" 2> \"$batch_run/baseline.stderr.json\"\n"
                 "./gooo-policy-compare --diagnostics=json --batch --mode compare --input \"$batch_run/requests.json\" > \"$batch_run/comparisons.json\" 2> \"$batch_run/compare.stderr.json\"\n"
                 "./gooo-policy-compare --diagnostics=json --batch --mode replay --input \"$batch_run/comparisons.json\" > \"$batch_run/replayed.json\" 2> \"$batch_run/replay.stderr.json\"\n"
                 "The ordered summaries are UNCHANGED (read) and NEWLY_DENIED (write); both commands succeed.\n"
                 "comparisons.json and replayed.json are byte-identical. Edit copies of the supplied facts to try your own synthetic cases.\n"
                 "Use 1..128 complete facts per array. Whole inputs: 4 MiB baseline/compare, 16 MiB replay.\n"
                 "Outputs including newline: 4 MiB baseline, 16 MiB compare/replay. Byte limits still apply below 128 items.\n"
                 "Input or saved-report validation failure emits no result and stops this shell sequence.\n"
                 "Inspect the corresponding stderr file and exit status; item_index is zero-based when the failure belongs to an item.\n"
                 "Shell output redirection is not an atomic file transaction.\n")
    files = {"gooo-policy-compare": (command, 0o755), "START-HERE.txt": (text.encode(), 0o644),
             "LICENSE": (read(root / "LICENSE", 1 << 20), 0o644),
             "NOTICE-original.md": (read(root / "examples/api-policy-revision/profiles/original/NOTICE.md", 1 << 20), 0o644),
             "LICENSE-upstream": (read(root / "examples/api-policy-revision/profiles/original/oracle/upstream/LICENSE", 1 << 20), 0o644),
             "LICENSE-Go": (read(go.parent.parent / "LICENSE", 1 << 20), 0o644)}
    compiler_license = read(root / "tools/licenses/gooo-83928fa-MIT.txt", 1 << 20)
    need(sha(compiler_license) == "5177457bc30b96eae6f786d0fbe3d1fda382edef61ecef25eb6f446a8b35b4f3",
         "compiler license differs from pinned source")
    files["LICENSE-Gooo"] = (compiler_license, 0o644)
    for action in ("read", "write"):
        files[action + "-facts.json"] = (facts(report["bundle_facts"][action], action), 0o644)
    manifest = {"schema": "gooo/api-policy-cli-bundle/v1", "platform": platform,
                "source_commit": revision, "build_info": normalized,
                "policy_identities": identities, "finite_checks": {"cases": cases, "tampered_report_refused": True},
                "members": {name: {"bytes": len(raw), "sha256": sha(raw), "mode": mode}
                            for name, (raw, mode) in sorted(files.items())}}
    if batch is not None:
        manifest["finite_checks"]["batch"] = batch
    files["BUNDLE.json"] = (encoded(manifest), 0o644)
    archive = out / ("gooo-policy-compare-" + platform.replace("/", "-") + ".tar.gz")
    checksums = out / "SHA256SUMS"
    created = []
    try:
        with archive.open("xb") as target:
            created.append(archive)
            with gzip.GzipFile(filename="", fileobj=target, mode="wb", mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as tar:
                    for name, (raw, mode) in sorted(files.items()):
                        member = tarfile.TarInfo(name)
                        member.size, member.mode = len(raw), mode
                        tar.addfile(member, io.BytesIO(raw))
        checked = verify_archive(archive, files)
        checksum_raw = (checked["sha256"] + "  " + archive.name + "\n").encode()
        with checksums.open("xb") as target:
            created.append(checksums)
            target.write(checksum_raw)
        need(read(checksums, 4096) == checksum_raw, "checksum file readback differs")
        return {"status": "VALIDATED_NATIVE_CLI_BUNDLE", "archive": checked, "checksums": {"path": str(checksums),
                "bytes": checksums.stat().st_size, "sha256": sha(checksums.read_bytes())},
                "platform": platform, "members": sorted(files), "archive_readback_verified": True}
    except BaseException as error:
        for path in reversed(created):
            try:
                path.unlink(missing_ok=True)
            except OSError as cleanup_error:
                if hasattr(error, "add_note"):
                    error.add_note("Bundle cleanup failed for " + str(path) + ": " + str(cleanup_error))
        raise
