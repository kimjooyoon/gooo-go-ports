"""Prepare a fresh Docker context from a checksum-pinned preview.14 bundle.

No subprocess, network, archive extraction helper, Go installation or container
execution occurs here. The caller supplies independently reviewed expected pins. The expected archive
SHA256 is the trust anchor: these internal consistency checks do not independently
parse the executable build info or authenticate its producer. A caller-selected
untrusted checksum cannot establish authenticity.
The format checks support the pinned flat Linux/amd64 v2 distribution only.
"""

import gzip
import hashlib
import io
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
        require(len(files) < 63, "archive member count exceeds 63")
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
    require(len(files) == 63, "archive must contain exactly 63 regular files")
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
    require(isinstance(bundle, dict) and bundle.get("schema") == SCHEMA and
            bundle.get("platform") == "linux/amd64" and
            bundle.get("source_commit") == source_commit,
            "bundle schema, platform or source mismatch")
    members = bundle.get("members")
    require(isinstance(members, dict) and len(members) == 62 and
            set(members) == set(files) - {"BUNDLE.json"},
            "manifest must bind exactly the other 62 members")
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
    require(base <= set(members), "missing required runtime member")
    inventory = strict_json(files["NOTICE-INVENTORY.json"], "notice inventory")
    require(type(bundle.get("notice_texts")) is int and
            bundle["notice_texts"] == 58 and
            isinstance(inventory, list) and len(inventory) == 58,
            "expected 58 notice texts")
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
    return bundle


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
        "schema": "gooo/preview14-container-context/v1",
        "archive_sha256": sha256,
        "archive_bytes": len(raw),
        "expanded_archive_bytes": expanded_bytes,
        "source_commit": source_commit,
        "platform": manifest["platform"],
        "bundle_sha256": digest(files["BUNDLE.json"]),
        "binary_sha256": digest(files[BINARY]),
        "dockerfile_sha256": digest(docker_raw),
        "runtime_members": len(files),
        "notice_texts": 58,
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
