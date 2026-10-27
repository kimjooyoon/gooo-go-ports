"""Prepare a fresh Docker context from a checksum-pinned native MCP bundle.

No subprocess, network, archive extraction helper, Go installation or container
execution occurs here. The caller supplies independently reviewed expected pins. The expected archive
SHA256 is the trust anchor: these internal consistency checks do not independently
parse the executable build info or authenticate its producer. A caller-selected
untrusted checksum cannot establish authenticity.
The format checks support exact flat Linux/amd64 v2, v3 and saved-definition v4 layouts.
"""

import gzip
import hashlib
import io
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import tarfile

COMPRESSED_LIMIT = 16 * 1024 * 1024
EXPANDED_LIMIT = 32 * 1024 * 1024
DOCKERFILE_LIMIT = 64 * 1024
BINARY = "gooo-service-grant-mcp"
SCHEMA = "gooo/service-grant-mcp-native-bundle/v2"
ALIAS_SCHEMA = "gooo/service-grant-mcp-native-bundle/v3"
SPEC = importlib.util.spec_from_file_location('snapshot_context_profile', Path(__file__).with_name('mcp_snapshot_profile.py'))
if hashlib.sha256(Path(SPEC.origin).read_bytes()).hexdigest() != 'a421d1ea56ca54eb005322eb180d06348b59f6dacf3deb1c7f7748bda766255a':
    raise ValueError('snapshot context support source differs')
SNAPSHOT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SNAPSHOT)
LAYOUTS = {SCHEMA: (63, 58), ALIAS_SCHEMA: (65, 60), SNAPSHOT.SCHEMA: (68, 62)}
ALIAS_DEPENDENCY = {
    "Path": "github.com/kimjooyoon/gooo-go-ports/examples/alias-origin-change",
    "Version": "v0.0.0-20261024154510-16d1e7864dc8",
    "Sum": "h1:YCIMYxnE7+rqIumnIdG0e5JCkYOnaoLIHMCUJbdTKQA=",
}
ALIAS_GENERATION = "b0c87b7865697c4b6433ef54e23defd7267ed8ea4da562cc8e1af24e283e7b06"
MANY_PROFILE_SHA256 = "5f5eb503299a7ccfc1007c366a95eb1033cffd6fc7c1cbc98328dc0b644423ba"
ALIAS_NOTICES = {"LICENSE": "099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48",
                 "NOTICE.md": "23cd2a8311b533aa6d14788b98824ff3c1d24675ac404dda1282086d350d2189"}
PROFILES = {
    "default": {"argv": [], "tools": ["assess_service_grant"]},
    "many_required": {"argv": ["--many-required"], "tools": ["assess_service_grant", "assess_service_grant_change"]},
    "alias_origin_change": {"argv": ["--alias-origin-change"], "tools": ["assess_service_grant", "gooo_compare_alias_origin"]},
    "many_and_alias": {"argv": ["--many-required", "--alias-origin-change"], "tools": ["assess_service_grant", "assess_service_grant_change", "gooo_compare_alias_origin"]},
}
GO_VERSION = "go1.27.3"
MODULE = "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-mcp"
HEX64 = re.compile(r"[0-9a-f]{64}\Z")
COMMIT = re.compile(r"[0-9a-f]{40}\Z")
FLAT = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{0,99}\Z")


class Refusal(ValueError):
    """A format, identity, freshness or resource-limit refusal."""


def require(condition, message):
    if not condition:
        raise Refusal(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_regular(path, limit):
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(str(path), flags), "rb") as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode), "input is not a regular file")
        require(info.st_size <= limit, "input byte limit exceeded")
        raw = stream.read(limit + 1)
        require(len(raw) <= limit, "input byte limit exceeded")
        require(len(raw) == info.st_size, "input changed while reading")
        return raw


def strict_json(raw, label):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, label + ": duplicate JSON key")
            result[key] = value
        return result

    def constant(_):
        raise Refusal(label + ": non-finite JSON number")

    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=pairs,
                          parse_constant=constant)
    except (UnicodeError, json.JSONDecodeError) as error:
        raise Refusal(label + ": invalid UTF-8 JSON") from error


def flat_name(name):
    require(isinstance(name, str) and FLAT.fullmatch(name) is not None,
            "unsafe or non-flat member name")
    return name


def unpack_checked(raw):
    """Return file bytes and modes; reject hidden tar records before decoding."""
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(raw), mode="rb") as stream:
            expanded = stream.read(EXPANDED_LIMIT + 1)
            require(len(expanded) <= EXPANDED_LIMIT,
                    "expanded archive byte limit exceeded")
            require(stream.read(1) == b"", "expanded archive byte limit exceeded")
    except (OSError, EOFError) as error:
        raise Refusal("invalid gzip stream") from error
    require(len(expanded) % 512 == 0, "tar record alignment mismatch")
    files, modes = {}, {}
    offset = 0
    while offset + 512 <= len(expanded):
        block = expanded[offset:offset + 512]
        if block == bytes(512):
            require(offset + 1024 <= len(expanded), "missing tar end records")
            require(not any(expanded[offset:]), "nonzero data after tar end")
            break
        require(len(files) < 68, "archive member count exceeds known layouts")
        try:
            info = tarfile.TarInfo.frombuf(block, "utf-8", "strict")
        except (tarfile.HeaderError, UnicodeError, ValueError) as error:
            raise Refusal("invalid tar header") from error
        require(info.type in (tarfile.REGTYPE, tarfile.AREGTYPE),
                "nonregular, link, sparse or extended tar member")
        name = flat_name(info.name)
        require(name not in files, "duplicate archive member")
        require(not info.linkname, "regular member has link target")
        require(info.mode in (0o644, 0o755), "unsupported member mode")
        require(0 <= info.size <= EXPANDED_LIMIT, "member byte limit exceeded")
        start = offset + 512
        end = start + info.size
        padded_end = start + ((info.size + 511) // 512) * 512
        require(padded_end <= len(expanded), "truncated tar member")
        require(not any(expanded[end:padded_end]), "nonzero tar member padding")
        files[name], modes[name] = expanded[start:end], info.mode
        offset = padded_end
    else:
        raise Refusal("missing tar end records")
    require(len(files) in (63, 65, 68), "archive must match an exact known regular-file layout")
    return files, modes, len(expanded)


def validate_build_info(build, source_commit):
    require(isinstance(build, dict) and build.get("GoVersion") == GO_VERSION and
            build.get("Path") == MODULE, "unexpected Go version or main package")
    entries = build.get("Settings")
    require(isinstance(entries, list), "invalid build settings")
    settings = {}
    for entry in entries:
        require(isinstance(entry, dict) and set(entry) == {"Key", "Value"} and
                isinstance(entry["Key"], str) and isinstance(entry["Value"], str),
                "invalid build setting entry")
        require(entry["Key"] not in settings, "duplicate build setting")
        settings[entry["Key"]] = entry["Value"]
    require(settings.get("CGO_ENABLED") == "0" and
            settings.get("GOOS") == "linux" and
            settings.get("GOARCH") == "amd64" and
            settings.get("vcs.revision") == source_commit and
            settings.get("vcs.modified") == "false",
            "build-info platform, CGO or clean source identity mismatch")


def validate_manifest(files, modes, source_commit):
    require("BUNDLE.json" in files, "missing BUNDLE.json")
    bundle = strict_json(files["BUNDLE.json"], "BUNDLE")
    require(isinstance(bundle, dict) and bundle.get("schema") in LAYOUTS and
            bundle.get("platform") == "linux/amd64" and
            bundle.get("source_commit") == source_commit,
            "bundle schema, platform or source mismatch")
    member_count, notice_count = LAYOUTS[bundle["schema"]]
    require(len(files) == member_count, "archive population differs from its bundle schema")
    members = bundle.get("members")
    require(isinstance(members, dict) and len(members) == member_count - 1 and
            set(members) == set(files) - {"BUNDLE.json"},
            "manifest must bind exactly the other layout members")
    require(modes["BUNDLE.json"] == 0o644, "BUNDLE mode mismatch")
    for name, meta in members.items():
        flat_name(name)
        require(isinstance(meta, dict) and set(meta) == {"bytes", "sha256", "mode"},
                "invalid member metadata")
        require(type(meta["bytes"]) is int and meta["bytes"] == len(files[name]) and
                isinstance(meta["sha256"], str) and
                HEX64.fullmatch(meta["sha256"]) is not None and
                meta["sha256"] == digest(files[name]),
                "manifest member byte count or hash mismatch: " + name)
        expected_mode = 0o755 if name == BINARY else 0o644
        require(type(meta["mode"]) is int and meta["mode"] == expected_mode and
                modes[name] == expected_mode, "manifest member mode mismatch")
    base = {"BUILD-INFO.json", "NOTICE-INVENTORY.json", "START-HERE.txt", BINARY}
    if bundle['schema'] == SNAPSHOT.SCHEMA:
        base.add(SNAPSHOT.SAMPLE)
    require(base <= set(members), "missing required runtime member")
    inventory = strict_json(files["NOTICE-INVENTORY.json"], "notice inventory")
    require(type(bundle.get("notice_texts")) is int and
            bundle["notice_texts"] == notice_count and
            isinstance(inventory, list) and len(inventory) == notice_count,
            "notice text count differs from bundle schema")
    notices = set()
    for notice in inventory:
        require(isinstance(notice, dict), "invalid notice inventory entry")
        name = flat_name(notice.get("file"))
        require(name not in notices and name in members and name not in base,
                "duplicate or missing notice file")
        notices.add(name)
        require((name.startswith("NOTICE-module-") or name.startswith("NOTICE-Go-")
                 or name in {"LICENSE-transport-BSD", "LICENSE-generation-tool-Gooo-MIT"})
                and files[name] and notice.get("sha256") == digest(files[name]),
                "notice name, text or digest mismatch")
        try:
            files[name].decode("utf-8")
        except UnicodeError as error:
            raise Refusal("notice is not UTF-8 text") from error
    require(notices == set(members) - base, "unlisted notice or extra runtime file")
    build = strict_json(files["BUILD-INFO.json"], "BUILD-INFO")
    require(build == bundle.get("build_info"), "BUILD-INFO differs from manifest")
    require(bundle.get("binary_sha256") == digest(files[BINARY]),
            "binary hash differs from manifest")
    validate_build_info(build, source_commit)
    if bundle["schema"] in (ALIAS_SCHEMA, SNAPSHOT.SCHEMA):
        validate_alias_profile(bundle, build, inventory, files)
    if bundle['schema'] == SNAPSHOT.SCHEMA:
        validate_snapshot_profile(bundle, build, inventory, files)
    return bundle


def validate_alias_profile(bundle, build, inventory, files):
    snapshot = bundle['schema'] == SNAPSHOT.SCHEMA
    expected_profiles = SNAPSHOT.profiles(PROFILES) if snapshot else PROFILES
    require(bundle.get("profiles") == expected_profiles, "exact default/many/alias/combined profiles required")
    optional = bundle.get("optional_profiles")
    expected_optional = {'many_required','alias_origin_change'} | ({'definition_snapshot'} if snapshot else set())
    require(isinstance(optional, dict) and set(optional) == expected_optional,
            "exact optional profile roster required")
    require(digest(json.dumps(optional["many_required"], sort_keys=True,
                              separators=(",", ":")).encode()) == MANY_PROFILE_SHA256,
            "existing many profile bindings differ")
    alias = optional["alias_origin_change"]
    require(isinstance(alias, dict) and set(alias) == {"argv", "additional_tool", "module",
            "compiler_source_sha", "generation_sha256", "bindings"}, "alias profile fields differ")
    require(alias["argv"] == ["--alias-origin-change"] and
            alias["additional_tool"] == "gooo_compare_alias_origin" and
            alias["module"] == ALIAS_DEPENDENCY and
            alias["compiler_source_sha"] == "205896a413e5dea92366309c35ae85e542d055f6" and
            alias["generation_sha256"] == "sha256:" + ALIAS_GENERATION,
            "alias profile immutable identity differs")
    binding_bytes = (json.dumps(alias["bindings"], sort_keys=True, indent=2) + "\n").encode()
    require(digest(binding_bytes) == ALIAS_GENERATION, "alias generation binding bytes differ")
    dependencies = build.get("Deps")
    require(isinstance(dependencies, list) and len(dependencies) == (13 if snapshot else 12) and
            [dep for dep in dependencies if isinstance(dep, dict) and
             dep.get("Path") == ALIAS_DEPENDENCY["Path"]] == [ALIAS_DEPENDENCY],
            "alias executable dependency pin differs")
    rows = [row for row in inventory if isinstance(row.get("module"), dict) and
            row["module"].get("Path") == ALIAS_DEPENDENCY["Path"]]
    require(len(rows) == 2 and {row.get("source_relative_path") for row in rows} == set(ALIAS_NOTICES),
            "alias license/notice inventory differs")
    for row in rows:
        name = row["source_relative_path"]
        require(row.get("module") == ALIAS_DEPENDENCY and row.get("sha256") == ALIAS_NOTICES[name] and
                digest(files[row["file"]]) == ALIAS_NOTICES[name], "alias notice source pin differs")


def validate_snapshot_profile(bundle, build, inventory, files):
    optional = bundle['optional_profiles']['definition_snapshot']
    require(isinstance(optional,dict) and set(optional) == {'argv','additional_tool','module',
        'compiler_source_sha','generation_sha256','bindings','output_schema','schema_origin',
        'transport_source_profile_sha256','example_member'}, 'snapshot profile fields differ')
    require(optional['argv']==[SNAPSHOT.FLAG] and optional['additional_tool']==SNAPSHOT.TOOL and
            optional['module']==SNAPSHOT.DEPENDENCY and optional['compiler_source_sha']==SNAPSHOT.PIN and
            optional['generation_sha256']=='sha256:'+SNAPSHOT.GENERATION and
            optional['transport_source_profile_sha256']==SNAPSHOT.PROFILE_SHA256 and
            optional['example_member']==SNAPSHOT.SAMPLE,'snapshot immutable profile differs')
    canonical=lambda value:(json.dumps(value,sort_keys=True,indent=2)+'\n').encode()
    require(digest(canonical(optional['bindings']))==SNAPSHOT.GENERATION,
            'snapshot generation binding bytes differ')
    frozen=bundle.get('transport_source_profile')
    require(isinstance(frozen,dict) and digest(canonical(frozen))==SNAPSHOT.PROFILE_SHA256 and
            frozen.get('reference_source')==SNAPSHOT.SOURCE,
            'complete immutable snapshot transport source profile differs')
    expected_sources={name:row['sha256'] for name,row in frozen['files'].items()}
    require(bundle.get('source_files')==expected_sources,'snapshot transport file bindings differ')
    require(digest(files[SNAPSHOT.SAMPLE])==expected_sources[SNAPSHOT.PREFIX+'/fixtures/snapshot.input.json'],
            'saved-definition example differs from source')
    shape,origin=optional['output_schema'],optional['schema_origin']
    require(digest(canonical(shape))==expected_sources[SNAPSHOT.PREFIX+'/schemas/snapshot-output.schema.json'] and
            digest(canonical(origin))==expected_sources[SNAPSHOT.PREFIX+'/schemas/snapshot-output.origin.json'] and
            origin.get('program_execution') is False and origin.get('inspection_kind')=='DECLARATION_INTERFACE_ONLY' and
            origin.get('kernel_generation_compiler_source_sha')==SNAPSHOT.PIN,
            'Gooo declaration-derived output shape/source profile differs')
    deps=build.get('Deps')
    require(isinstance(deps,list) and len(deps)==13 and
            [d for d in deps if isinstance(d,dict) and d.get('Path')==SNAPSHOT.MODULE]==[SNAPSHOT.DEPENDENCY] and
            all(isinstance(d,dict) and 'Replace' not in d for d in deps),
            'snapshot executable dependency pin/replacement differs')
    rows=[row for row in inventory if isinstance(row.get('module'),dict) and row['module'].get('Path')==SNAPSHOT.MODULE]
    require(len(rows)==2 and {row.get('source_relative_path') for row in rows}==set(SNAPSHOT.NOTICES),
            'snapshot license/notice inventory differs')
    for row in rows:
        expected=SNAPSHOT.NOTICES[row['source_relative_path']]
        require(row['module']==SNAPSHOT.DEPENDENCY and row['sha256']==expected and
                digest(files[row['file']])==expected,'snapshot notice source pin differs')


def write_new(path, raw, mode):
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(str(path), flags, mode), "wb") as stream:
        stream.write(raw)
        os.fchmod(stream.fileno(), mode)


def prepare(bundle: Path, sha256: str, source_commit: str,
            out: Path, dockerfile: Path) -> dict:
    """Validate everything in memory, then create one fresh non-overwriting context."""
    require(isinstance(sha256, str) and HEX64.fullmatch(sha256) is not None,
            "expected archive SHA256 must be 64 lowercase hexadecimal digits")
    require(isinstance(source_commit, str) and COMMIT.fullmatch(source_commit) is not None,
            "expected source commit must be 40 lowercase hexadecimal digits")
    raw = read_regular(Path(bundle), COMPRESSED_LIMIT)
    require(digest(raw) == sha256, "archive SHA256 mismatch")
    files, modes, expanded_bytes = unpack_checked(raw)
    manifest = validate_manifest(files, modes, source_commit)
    docker_raw = read_regular(Path(dockerfile), DOCKERFILE_LIMIT)
    require(docker_raw and b"\0" not in docker_raw, "invalid Dockerfile bytes")
    try:
        docker_raw.decode("utf-8")
    except UnicodeError as error:
        raise Refusal("Dockerfile is not UTF-8") from error
    context = {
        "schema": (SNAPSHOT.CONTEXT_SCHEMA if manifest['schema']==SNAPSHOT.SCHEMA else
                   "gooo/service-grant-mcp-container-context/v2" if manifest["schema"] == ALIAS_SCHEMA
                   else "gooo/preview14-container-context/v1"),
        "archive_sha256": sha256,
        "archive_bytes": len(raw),
        "expanded_archive_bytes": expanded_bytes,
        "source_commit": source_commit,
        "platform": manifest["platform"],
        "bundle_sha256": digest(files["BUNDLE.json"]),
        "binary_sha256": digest(files[BINARY]),
        "dockerfile_sha256": digest(docker_raw),
        "runtime_members": len(files),
        "notice_texts": manifest["notice_texts"],
        "runtime_directory": "runtime",
        "docker_execution_performed": False,
        "producer_authenticated": False,
        "trust_anchor": "caller-supplied independently reviewed archive SHA256",
        "binary_build_info_independently_parsed": False,
    }
    # All archive, manifest, build-info and Dockerfile reads/validation precede mkdir.
    out = Path(out)
    out.mkdir(mode=0o755, parents=False, exist_ok=False)
    out.chmod(0o755)
    runtime = out / "runtime"
    runtime.mkdir(mode=0o755)
    runtime.chmod(0o755)
    for name in sorted(files):
        write_new(runtime / name, files[name], modes[name])
    write_new(out / "Dockerfile", docker_raw, 0o644)
    write_new(out / "context.json",
              (json.dumps(context, sort_keys=True, indent=2) + "\n").encode(), 0o644)
    return context


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", type=Path, required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    dockerfile = Path(__file__).resolve().parents[1] / "containers/service-grant-mcp/Dockerfile"
    print(json.dumps(prepare(args.bundle, args.sha256, args.source_commit, args.out, dockerfile)))
