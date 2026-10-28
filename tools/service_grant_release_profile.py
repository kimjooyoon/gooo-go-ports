"""Read reviewed public native-release data; never acquire or run an asset.

Version selection is an allowlist, not a path, URL or caller-selected checksum.
Profiles bind asset, executable, source and layout roles to original bytes.
They do not authenticate a producer or replace archive/runtime verification.
"""

import hashlib
import json
import os
from pathlib import Path
import re
import stat
from types import MappingProxyType

DEFAULT_VERSION = "v0.1.0-preview.16"
REPOSITORY = "https://github.com/kimjooyoon/gooo-go-ports"
PROFILE_LIMIT = 65536
RELEASES = MappingProxyType({
    "v0.1.0-preview.16": (
        "mcp-public-preview16-linux-amd64.json",
        "b7717b80abd8df232ecc0d6f1a0886c4d9e6650c236d5c296f483dab3a9e26f4"),
})


class ProfileRefusal(ValueError):
    """Unknown version, changed source or inconsistent release roles."""


def require(condition, message):
    if not condition:
        raise ProfileRefusal(message)


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, "duplicate profile member: " + key)
        result[key] = value
    return result


def _validate(document, version):
    fields = {
        "schema", "version", "native_source", "go_version", "platform",
        "archive", "binary", "bundle", "provenance"}
    require(type(document) is dict and set(document) in (fields, fields | {"reference_packet"}),
            "release profile fields differ")
    require(document["schema"] == "gooo/public-native-release-profile/v1" and
            document["version"] == version and
            re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+-preview\.[0-9]+", version),
            "release profile version differs")
    source = document["native_source"]
    require(type(source) is str and re.fullmatch(r"[0-9a-f]{40}", source),
            "release source must be an exact Git commit")
    require(document["go_version"] == "go1.27.3" and document["platform"] == "linux/amd64",
            "unsupported release toolchain or platform")
    archive, binary, bundle = (document[key] for key in ("archive", "binary", "bundle"))
    require(type(archive) is dict and set(archive) == {"name", "bytes", "sha256"},
            "archive role fields differ")
    require(type(binary) is dict and set(binary) == {"name", "sha256"},
            "binary role fields differ")
    require(archive["name"] == "gooo-service-grant-mcp-linux-amd64.tar.gz" and
            binary["name"] == "gooo-service-grant-mcp", "release filenames differ")
    require(type(archive["bytes"]) is int and 0 < archive["bytes"] <= 16 << 20,
            "archive byte bound differs")
    for role in (archive, binary):
        require(type(role["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", role["sha256"]),
                "release role requires a raw SHA256")
    require(type(bundle) is dict and set(bundle) == {"schema", "runtime_members", "notice_texts"},
            "bundle role fields differ")
    require(bundle["schema"] in {"gooo/service-grant-mcp-native-bundle/v2",
                                "gooo/service-grant-mcp-native-bundle/v3",
                                "gooo/service-grant-mcp-native-bundle/v4"},
            "unknown native bundle schema")
    for key in ("runtime_members", "notice_texts"):
        require(type(bundle[key]) is int and 0 < bundle[key] <= 128,
                "bundle count must be a bounded integer")
    require(bundle["notice_texts"] < bundle["runtime_members"], "bundle notice/member roles differ")
    reference = document.get("reference_packet")
    if bundle["schema"] == "gooo/service-grant-mcp-native-bundle/v4":
        require(type(reference) is dict and set(reference) == {"name", "bytes", "sha256"},
                "v4 requires a reviewed reference packet")
        require(reference["name"] == "gooo-mcp-saved-reference-linux-amd64.tar.gz" and
                type(reference["bytes"]) is int and 0 < reference["bytes"] <= 16 << 20 and
                type(reference["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", reference["sha256"]),
                "reference packet role differs")
    else:
        require("reference_packet" not in document, "legacy release cannot acquire new reference material")
    release_url = REPOSITORY + "/releases/tag/" + version
    origins = [{"@id": "urn:sha256:" + archive["sha256"]},
               {"@id": REPOSITORY + "/tree/" + source}]
    if reference is not None:
        origins.append({"@id": "urn:sha256:" + reference["sha256"]})
    require(document["provenance"] == {
        "@context": {"prov": "http://www.w3.org/ns/prov#"}, "@id": release_url,
        "prov:wasDerivedFrom": origins},
        "release provenance does not bind the archive and source roles")
    return document


def load_release(version=DEFAULT_VERSION):
    require(type(version) is str and version in RELEASES, "unreviewed public release version")
    name, expected_sha = RELEASES[version]
    directory = Path(__file__).with_name("profiles")
    require(stat.S_ISDIR(directory.lstat().st_mode), "profile directory is not regular")
    path = directory / name
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= PROFILE_LIMIT,
            "release profile is not a bounded regular file")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    with os.fdopen(os.open(path, flags), "rb") as stream:
        opened = os.fstat(stream.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino),
                "release profile changed before open")
        raw = stream.read(PROFILE_LIMIT + 1)
    after = path.lstat()
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_mode) ==
            (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_mode),
            "release profile changed during read")
    require(len(raw) <= PROFILE_LIMIT and hashlib.sha256(raw).hexdigest() == expected_sha,
            "reviewed release profile SHA256 differs")
    document = _validate(json.loads(raw.decode("utf-8"), object_pairs_hook=pairs), version)
    document["profile_sha256"] = expected_sha
    document["profile_file"] = "tools/profiles/" + name
    document["release_url"] = REPOSITORY + "/releases/tag/" + version
    document["archive_url"] = REPOSITORY + "/releases/download/" + version + "/" + document["archive"]["name"]
    if "reference_packet" in document:
        document["reference_packet_url"] = REPOSITORY + "/releases/download/" + version + "/" + document["reference_packet"]["name"]
    return document


def environment(profile):
    """Return fixed CI variable names and newline-free validated values."""
    result = {
        "NATIVE_VERSION": profile["version"], "NATIVE_SOURCE": profile["native_source"],
        "ARCHIVE_SHA256": profile["archive"]["sha256"], "BINARY_SHA256": profile["binary"]["sha256"],
        "ARCHIVE_BYTES": str(profile["archive"]["bytes"]), "ARCHIVE_URL": profile["archive_url"],
        "RUNTIME_MEMBERS": str(profile["bundle"]["runtime_members"]),
        "NOTICE_TEXTS": str(profile["bundle"]["notice_texts"]),
        "RELEASE_PROFILE_SHA256": profile["profile_sha256"],
        "NATIVE_GO_SEMVER": profile["go_version"].removeprefix("go"),
        "HAS_NATIVE_REFERENCES": "true" if "reference_packet" in profile else "false",
        "REFERENCE_PACKET_URL": profile.get("reference_packet_url", ""),
        "REFERENCE_PACKET_BYTES": str(profile["reference_packet"]["bytes"]) if "reference_packet" in profile else "",
        "REFERENCE_PACKET_SHA256": profile.get("reference_packet", {}).get("sha256", ""),
    }
    require(all(type(v) is str and "\n" not in v and "\r" not in v for v in result.values()),
            "release environment value contains a newline")
    return result
