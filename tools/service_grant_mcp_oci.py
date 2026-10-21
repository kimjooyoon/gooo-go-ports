"""Prepare an OCI build context from the fixed public preview16 Linux bundle.

This command only reads a local archive and writes one fresh build context.
It never fetches a URL, rebuilds a binary, invokes Docker or publishes an image.
The release pins below come from the reviewed whole public GitHub asset; the
existing container preparer owns all archive/member/notice validation.
"""

import argparse
import importlib.util
import json
from pathlib import Path
import stat

SPEC = importlib.util.spec_from_file_location(
    "service_grant_container_context", Path(__file__).with_name("service_grant_mcp_container.py"))
CONTEXT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CONTEXT)

VERSION = "v0.1.0-preview.16"
SOURCE_COMMIT = "fceb89f8b53eabd6687d16258d7cee89853c2cfe"
PLATFORM = "linux/amd64"
ARCHIVE_NAME = "gooo-service-grant-mcp-linux-amd64.tar.gz"
ARCHIVE_BYTES = 7217754
ARCHIVE_SHA256 = "0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524"
BINARY_SHA256 = "a848a8747669e46e9afef6c0f1ed8b2014df4838cc370416f60ab9c6acccfc0a"
RELEASE_URL = "https://github.com/kimjooyoon/gooo-go-ports/releases/tag/" + VERSION
ARCHIVE_URL = "https://github.com/kimjooyoon/gooo-go-ports/releases/download/" + VERSION + "/" + ARCHIVE_NAME
DOCKERFILE = Path(__file__).resolve().parents[1] / "containers/service-grant-mcp/Dockerfile"


def prepare_public(bundle, sha256, source_commit, version, out):
    """Require the reviewed public release before using the existing preparer.

    A local filename is not an identity. Exact bytes are checked against the
    fixed whole-archive SHA, not a caller's arbitrary replacement checksum.
    A failed or incomplete context is retained; the output must be fresh.
    """
    CONTEXT.require(version == VERSION, "only the fixed public preview16 version is supported")
    CONTEXT.require(source_commit == SOURCE_COMMIT, "public preview16 source commit differs")
    CONTEXT.require(sha256 == ARCHIVE_SHA256, "public preview16 expected archive SHA256 differs")
    bundle = Path(bundle)
    info = bundle.lstat()
    CONTEXT.require(stat.S_ISREG(info.st_mode), "public archive must be a regular non-symlink file")
    CONTEXT.require(info.st_size == ARCHIVE_BYTES, "public preview16 archive byte count differs")
    context = CONTEXT.prepare(bundle, ARCHIVE_SHA256, SOURCE_COMMIT, Path(out), DOCKERFILE)
    CONTEXT.require(context["archive_bytes"] == ARCHIVE_BYTES and
                    context["archive_sha256"] == ARCHIVE_SHA256 and
                    context["binary_sha256"] == BINARY_SHA256 and
                    context["source_commit"] == SOURCE_COMMIT and
                    context["platform"] == PLATFORM and
                    context["runtime_members"] == 63 and context["notice_texts"] == 58,
                    "prepared public release context identity differs")
    return {
        "schema": "gooo/public-preview16-oci-context/v1",
        "release": {"version": VERSION, "source_commit": SOURCE_COMMIT,
                    "platform": PLATFORM, "url": RELEASE_URL,
                    "archive": ARCHIVE_NAME, "archive_url": ARCHIVE_URL},
        "context": context,
        "binary_rebuilt": False,
        "network_requests": 0,
        "image_built_or_published": False,
        "runtime_or_client_verification_performed": False,
        "external_adoption_established": False,
    }



def verify_runtime(bundle, sha256, source_commit, version, runtime, out):
    """Compare copied runtime bytes/modes to the fixed archive, without running it."""
    runtime, out = Path(runtime), Path(out)
    directory_before = runtime.lstat()
    CONTEXT.require(stat.S_ISDIR(directory_before.st_mode),
                    "runtime must be a regular non-symlink directory")
    real_runtime, real_out = runtime.resolve(), out.resolve()
    CONTEXT.require(real_out != real_runtime and real_runtime not in real_out.parents,
                    "output must be outside the runtime directory")
    prepared = prepare_public(bundle, sha256, source_commit, version, out)
    expected = out / "runtime"
    names = {path.name for path in expected.iterdir()}
    actual = {path.name for path in runtime.iterdir()}
    CONTEXT.require(len(names) == 63 and actual == names, "runtime file roster differs")
    checked = []
    for name in sorted(names):
        wanted = CONTEXT.read_regular(expected / name, CONTEXT.EXPANDED_LIMIT)
        path = runtime / name
        before = path.lstat()
        CONTEXT.require(stat.S_ISREG(before.st_mode), "runtime member is not regular: " + name)
        mode = stat.S_IMODE((expected / name).lstat().st_mode)
        CONTEXT.require(stat.S_IMODE(before.st_mode) == mode, "runtime member mode differs: " + name)
        raw = CONTEXT.read_regular(path, len(wanted))
        CONTEXT.require(raw == wanted, "runtime member bytes differ: " + name)
        after = path.lstat()
        CONTEXT.require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_mode) ==
                        (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_mode),
                        "runtime member changed during comparison: " + name)
        checked.append({"name": name, "bytes": len(raw), "sha256": CONTEXT.digest(raw), "mode": mode})
    directory_after = runtime.lstat()
    CONTEXT.require((directory_before.st_dev, directory_before.st_ino, directory_before.st_mtime_ns, directory_before.st_mode) ==
                    (directory_after.st_dev, directory_after.st_ino, directory_after.st_mtime_ns, directory_after.st_mode) and
                    {path.name for path in runtime.iterdir()} == names,
                    "runtime directory changed during comparison")
    report = {
        "schema": "gooo/public-preview16-oci-runtime-identity/v1",
        "status": "EXACT_FIXED_PUBLIC_RUNTIME_BYTES_AND_MODES_MATCH",
        "release": prepared["release"], "context": prepared["context"],
        "runtime": str(real_runtime), "expected_context": str(out.resolve()),
        "checked_members": len(checked), "checked_files": checked,
        "byte_identity_only": True, "docker_execution_performed": False,
        "producer_authenticated": False, "binary_executed": False,
        "client_compatibility_or_adoption_established": False,
    }
    CONTEXT.write_new(out / "runtime-identity.json",
                      (json.dumps(report, sort_keys=True, indent=2) + "\n").encode(), 0o644)
    return report


def verify_embedded_go_inventory(embedded_go, components, packages):
    """Check exact toolchain inventory across two already parsed scanner rosters.

    Trivy 0.76 reports the fixed binary's go1.27.3 as stdlib v1.27.3.
    Only explicit representations of this same pinned version are accepted.
    This checks inventory presence, not vulnerabilities or image execution.
    """
    CONTEXT.require(embedded_go == CONTEXT.GO_VERSION,
                    "unreviewed embedded Go toolchain")
    bare = embedded_go.removeprefix("go")
    aliases = {embedded_go, bare, "v" + bare}
    matches = {}
    for roster, label in ((components, "SBOM"), (packages, "scan")):
        found = sorted((name, version) for name, version in roster
                       if name in ("stdlib", "go") and version in aliases)
        CONTEXT.require(bool(found), label + " missing embedded Go toolchain")
        matches[label] = [{"name": name, "version": version} for name, version in found]
    return {"status": "PINNED_EMBEDDED_GO_PRESENT_IN_BOTH_INVENTORIES",
            "embedded_go": embedded_go, "observed": matches,
            "vulnerability_or_image_execution_checked": False}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    for name in ("prepare", "verify-runtime"):
        command = commands.add_parser(name)
        command.add_argument("--bundle", type=Path, required=True)
        command.add_argument("--sha256", required=True)
        command.add_argument("--source-commit", required=True)
        command.add_argument("--version", required=True)
        command.add_argument("--out", type=Path, required=True)
        if name == "verify-runtime":
            command.add_argument("--runtime", type=Path, required=True)
    args = parser.parse_args(argv)
    if args.command == "verify-runtime":
        result = verify_runtime(args.bundle, args.sha256, args.source_commit, args.version, args.runtime, args.out)
    else:
        result = prepare_public(args.bundle, args.sha256, args.source_commit, args.version, args.out)
    print(json.dumps(result))


if __name__ == "__main__":
    main()
