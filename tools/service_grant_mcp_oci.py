"""Prepare an OCI build context from a reviewed public Linux native release.

This command only reads a local archive and writes one fresh build context.
It never fetches a URL, rebuilds a binary, invokes Docker or publishes an image.
The version-selected pins come from reviewed whole public GitHub assets; the
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

SPEC = importlib.util.spec_from_file_location(
    "service_grant_public_release", Path(__file__).with_name("service_grant_release_profile.py"))
RELEASE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RELEASE)

SPEC = importlib.util.spec_from_file_location(
    "service_grant_saved_references", Path(__file__).with_name("service_grant_mcp_reference_packet.py"))
REFERENCES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REFERENCES)


def release_profile(version):
    try:
        profile = RELEASE.load_release(version)
    except RELEASE.ProfileRefusal as error:
        raise CONTEXT.Refusal(str(error)) from error
    layout = profile["bundle"]
    CONTEXT.require(CONTEXT.LAYOUTS.get(layout["schema"]) ==
                    (layout["runtime_members"], layout["notice_texts"]),
                    "release layout differs from the known archive contract")
    CONTEXT.require(profile["go_version"] == CONTEXT.GO_VERSION,
                    "release toolchain differs from the archive contract")
    return profile


# Keep the existing default-release Python API; checks below use selected data.
DEFAULT = release_profile(RELEASE.DEFAULT_VERSION)
VERSION = DEFAULT["version"]
SOURCE_COMMIT = DEFAULT["native_source"]
PLATFORM = DEFAULT["platform"]
ARCHIVE_NAME = DEFAULT["archive"]["name"]
ARCHIVE_BYTES = DEFAULT["archive"]["bytes"]
ARCHIVE_SHA256 = DEFAULT["archive"]["sha256"]
BINARY_SHA256 = DEFAULT["binary"]["sha256"]
RELEASE_URL = DEFAULT["release_url"]
ARCHIVE_URL = DEFAULT["archive_url"]
DOCKERFILE = Path(__file__).resolve().parents[1] / "containers/service-grant-mcp/Dockerfile"


def reviewed_inputs(bundle, sha256, source_commit, version):
    profile = release_profile(version)
    archive = profile["archive"]
    CONTEXT.require(source_commit == profile["native_source"], "public release source commit differs")
    CONTEXT.require(sha256 == archive["sha256"], "public release expected archive SHA256 differs")
    bundle = Path(bundle)
    info = bundle.lstat()
    CONTEXT.require(stat.S_ISREG(info.st_mode), "public archive must be a regular non-symlink file")
    CONTEXT.require(info.st_size == archive["bytes"], "public release archive byte count differs")
    return profile, bundle


def prepare_public(bundle, sha256, source_commit, version, out):
    """Require the reviewed public release before using the existing preparer.

    A local filename is not an identity. Exact bytes are checked against the
    fixed whole-archive SHA, not a caller's arbitrary replacement checksum.
    A failed or incomplete context is retained; the output must be fresh.
    """
    profile, bundle = reviewed_inputs(bundle, sha256, source_commit, version)
    archive, layout = profile["archive"], profile["bundle"]
    context = CONTEXT.prepare(bundle, archive["sha256"], profile["native_source"], Path(out), DOCKERFILE)
    CONTEXT.require(context["archive_bytes"] == archive["bytes"] and
                    context["archive_sha256"] == archive["sha256"] and
                    context["binary_sha256"] == profile["binary"]["sha256"] and
                    context["source_commit"] == profile["native_source"] and
                    context["platform"] == profile["platform"] and
                    context["runtime_members"] == layout["runtime_members"] and
                    context["notice_texts"] == layout["notice_texts"],
                    "prepared public release context identity differs")
    return {
        "schema": ("gooo/public-preview16-oci-context/v1" if version == "v0.1.0-preview.16"
                   else "gooo/public-native-oci-context/v1"),
        "release": {"version": profile["version"], "source_commit": profile["native_source"],
                    "platform": profile["platform"], "url": profile["release_url"],
                    "archive": archive["name"], "archive_url": profile["archive_url"],
                    "profile_sha256": profile["profile_sha256"], "profile_file": profile["profile_file"],
                    "bundle_schema": layout["schema"], "runtime_members": layout["runtime_members"],
                    "notice_texts": layout["notice_texts"]},
        "context": context,
        "binary_rebuilt": False,
        "network_requests": 0,
        "image_built_or_published": False,
        "runtime_or_client_verification_performed": False,
        "external_adoption_established": False,
    }


def prepare_references(bundle, sha256, source_commit, version, packet, out):
    profile, bundle = reviewed_inputs(bundle, sha256, source_commit, version)
    reference = profile.get("reference_packet")
    CONTEXT.require(reference is not None and profile["bundle"]["schema"] == CONTEXT.SNAPSHOT.SCHEMA,
                    "this reviewed release has no v4 native reference packet")
    return REFERENCES.prepare(Path(packet), reference["sha256"], reference["bytes"], bundle,
                              profile["archive"]["sha256"], profile["native_source"], Path(out))



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
    CONTEXT.require(len(names) == prepared["release"]["runtime_members"] and actual == names,
                    "runtime file roster differs")
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
        "schema": ("gooo/public-preview16-oci-runtime-identity/v1" if version == "v0.1.0-preview.16"
                   else "gooo/public-native-oci-runtime-identity/v1"),
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
    metadata = commands.add_parser("profile")
    metadata.add_argument("--version", default=VERSION)
    metadata.add_argument("--format", choices=("json", "github-env"), default="json")
    for name in ("prepare", "verify-runtime", "prepare-references"):
        command = commands.add_parser(name)
        command.add_argument("--bundle", type=Path, required=True)
        command.add_argument("--sha256", required=True)
        command.add_argument("--source-commit", required=True)
        command.add_argument("--version", required=True)
        command.add_argument("--out", type=Path, required=True)
        if name == "verify-runtime":
            command.add_argument("--runtime", type=Path, required=True)
        elif name == "prepare-references":
            command.add_argument("--packet", type=Path, required=True)
    args = parser.parse_args(argv)
    if args.command == "profile":
        profile = release_profile(args.version or VERSION)
        if args.format == "github-env":
            print("\n".join(key + "=" + value for key, value in RELEASE.environment(profile).items()))
        else:
            print(json.dumps(profile, sort_keys=True))
        return
    if args.command == "prepare-references":
        result = prepare_references(args.bundle, args.sha256, args.source_commit, args.version, args.packet, args.out)
    elif args.command == "verify-runtime":
        result = verify_runtime(args.bundle, args.sha256, args.source_commit, args.version, args.runtime, args.out)
    else:
        result = prepare_public(args.bundle, args.sha256, args.source_commit, args.version, args.out)
    print(json.dumps(result))


if __name__ == "__main__":
    main()
