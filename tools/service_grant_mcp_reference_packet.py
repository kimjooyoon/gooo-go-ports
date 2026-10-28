"""Materialize saved v4 comparison references from a pinned local packet.

This reader launches no executable, compiler, network request or container.
The caller must supply separately reviewed whole-byte anchors. A caller-chosen
checksum establishes no authenticity. Original capture bytes and argv records
are retained; the existing image client still owns their semantic verification.
"""

import importlib.util
import io
import json
from pathlib import Path, PurePosixPath
import stat
import tarfile
import zipfile

SPEC = importlib.util.spec_from_file_location(
    "reference_native_context", Path(__file__).with_name("service_grant_mcp_container.py"))
C = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(C)

SCHEMA = "gooo/mcp-saved-native-reference-packet/v1"
PACKET_LIMIT = 16 << 20
EXPANDED_LIMIT = 32 << 20
WIRE_ZIP_LIMIT = 4 << 20
WIRE_EXPANDED_LIMIT = 128 << 20
WIRE_MEMBER_LIMIT = 16 << 20
REPOSITORY = "https://github.com/kimjooyoon/gooo-go-ports"


def unpack_packet(raw):
    files, modes = {}, {}
    expanded = 0
    with tarfile.open(fileobj=io.BytesIO(raw), mode="r:gz") as archive:
        for member in archive:
            C.require(len(files) < 128 and member.type in (tarfile.REGTYPE, tarfile.AREGTYPE) and
                      not member.pax_headers and
                      C.FLAT.fullmatch(member.name) and member.name not in files,
                      "packet requires unique flat regular members")
            C.require(type(member.size) is int and 0 <= member.size <= EXPANDED_LIMIT,
                      "packet member exceeds byte bound")
            expanded += member.size
            C.require(expanded <= EXPANDED_LIMIT, "expanded packet exceeds byte bound")
            expected_mode = 0o755 if member.name in ("alias-compare", "snapshot-compare") else 0o644
            C.require(member.mode == expected_mode, "packet member mode differs")
            raw_member = archive.extractfile(member).read(member.size + 1)
            C.require(len(raw_member) == member.size, "packet member byte count differs")
            files[member.name], modes[member.name] = raw_member, member.mode
    return files, modes


def wire_members(raw):
    """Preflight names and expansion before writing anything; return an open ZIP."""
    C.require(len(raw) <= WIRE_ZIP_LIMIT, "source wire ZIP exceeds byte bound")
    archive = zipfile.ZipFile(io.BytesIO(raw))
    try:
        infos = archive.infolist()
        C.require(0 < len(infos) <= 512 and sum(x.file_size for x in infos) <= WIRE_EXPANDED_LIMIT,
                  "source wire ZIP exceeds member/expanded bounds")
        names, folded = set(), set()
        selected = []
        for info in infos:
            path = PurePosixPath(info.filename)
            C.require(not info.is_dir() and not path.is_absolute() and
                      "\\" not in info.filename and "\0" not in info.filename and
                      0 < len(info.filename.encode("utf-8")) <= 512 and
                      str(path) == info.filename and all(p not in (".", "..") for p in path.parts),
                      "unsafe source wire member path")
            C.require(info.filename not in names and info.filename.casefold() not in folded,
                      "duplicate source wire member path")
            C.require(stat.S_IFMT(info.external_attr >> 16) in (0, stat.S_IFREG) and
                      not info.flag_bits & 1 and info.compress_type in
                      (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED),
                      "source wire member type/compression differs")
            C.require(0 <= info.file_size <= WIRE_MEMBER_LIMIT, "source wire member exceeds byte bound")
            names.add(info.filename)
            folded.add(info.filename.casefold())
            if path.parts[0] in ("alias-wire", "snapshot-wire"):
                selected.append(info)
        for name in names:
            C.require(not any(str(parent).casefold() in folded for parent in PurePosixPath(name).parents
                              if str(parent) != "."), "source wire file/directory conflict")
        C.require({"alias-wire/acceptance.json", "snapshot-wire/acceptance.json"} <= names,
                  "source wire acceptance reports missing")
        return archive, selected
    except BaseException:
        archive.close()
        raise


def validate_roles(files, modes, native_files, native_manifest, native_sha, native_bytes):
    C.require("REFERENCE.json" in files and len(files["REFERENCE.json"]) <= 1 << 20,
              "packet reference manifest missing or oversized")
    document = C.strict_json(files["REFERENCE.json"], "reference packet")
    C.require(document.get("schema") == SCHEMA and document.get("platform") == "linux/amd64" and
              document.get("native_source") == native_manifest["source_commit"],
              "reference packet source/schema differs")
    ci = document.get("original_CI")
    C.require(type(ci) is dict and set(ci) == {"repository", "run", "attempt", "job", "url"} and
              ci["repository"] == "kimjooyoon/gooo-go-ports" and
              all(type(ci[key]) is int and ci[key] > 0 for key in ("run", "attempt", "job")) and
              ci["attempt"] == 1 and ci["url"] == REPOSITORY + "/actions/runs/" + str(ci["run"]),
              "reference observation identifiers differ; no producer authenticity implied")
    C.require(document.get("associated_native_archive") == {
        "name": "gooo-service-grant-mcp-linux-amd64.tar.gz", "bytes": native_bytes,
        "sha256": native_sha, "binary_sha256": C.digest(native_files[C.BINARY])},
        "reference packet native archive/binary roles differ")
    inventory = C.strict_json(native_files["NOTICE-INVENTORY.json"], "native notice inventory")
    notice_names = {row["file"] for row in inventory}
    expected = notice_names | {"NOTICE-INVENTORY.json", "source-wire.zip", "alias-compare",
                               "snapshot-compare", "REFERENCE.json", "START-HERE.txt"}
    C.require(set(files) == expected and len(notice_names) == document.get("notice_texts") == 62,
              "reference packet member/notice roster differs")
    for name in notice_names | {"NOTICE-INVENTORY.json"}:
        C.require(files[name] == native_files[name], "reference packet native notice bytes differ: " + name)
    members = document.get("members")
    C.require(type(members) is dict and set(members) == expected - {"REFERENCE.json"},
              "reference packet member inventory differs")
    for name, record in members.items():
        C.require(record == {"bytes": len(files[name]), "sha256": C.digest(files[name]), "mode": modes[name]} and
                  type(record.get("bytes")) is int and type(record.get("mode")) is int,
                  "reference packet member identity differs: " + name)
    wire = document.get("source_wire", {})
    C.require(wire.get("file") == "source-wire.zip" and wire.get("bytes") == len(files["source-wire.zip"]) and
              wire.get("sha256") == C.digest(files["source-wire.zip"]) and
              wire.get("alias_report") == "alias-wire/acceptance.json" and
              wire.get("snapshot_report") == "snapshot-wire/acceptance.json" and
              wire.get("max_expanded_bytes") == WIRE_EXPANDED_LIMIT,
              "source wire ZIP role differs")
    C.require(type(wire.get("original_provider_artifact")) is int and wire["original_provider_artifact"] > 0,
              "source wire artifact identifier differs")
    C.require(document.get("@context") == {"prov": "http://www.w3.org/ns/prov#"} and
              document.get("prov:wasDerivedFrom") == [
                  {"@id": "urn:sha256:" + wire["sha256"]}, {"@id": "urn:sha256:" + native_sha},
                  {"@id": REPOSITORY + "/tree/" + native_manifest["source_commit"]}],
              "reference provenance archive/source roles differ")
    dependencies = {"alias": C.ALIAS_DEPENDENCY, "snapshot": C.SNAPSHOT.DEPENDENCY}
    references = document.get("references")
    C.require(type(references) is dict and set(references) == set(dependencies),
              "reference CLI roles differ")
    for role, dependency in dependencies.items():
        r = references[role]
        name = role + "-compare"
        C.require(r.get("file") == name and r.get("bytes") == len(files[name]) and
                  r.get("sha256") == C.digest(files[name]) and r.get("module") == dependency and
                  r.get("go_version") == C.GO_VERSION and r.get("dependencies") == [] and
                  r.get("recorded_report") == role + "-wire/acceptance.json" and
                  r.get("original_reference_bytes_equal") is True,
                  "reference CLI identity/module differs: " + role)
    return document


def prepare(packet, expected_sha, expected_bytes, native_bundle, native_sha, source_commit, out):
    """Read anchored inputs and write one fresh directory; never execute references."""
    C.require(type(expected_bytes) is int and 0 < expected_bytes <= PACKET_LIMIT,
              "expected packet byte count differs")
    C.require(type(expected_sha) is str and C.HEX64.fullmatch(expected_sha) and
              type(native_sha) is str and C.HEX64.fullmatch(native_sha), "expected raw SHA256 required")
    raw = C.read_regular(Path(packet), PACKET_LIMIT)
    C.require(len(raw) == expected_bytes and C.digest(raw) == expected_sha,
              "reference packet whole-byte identity differs")
    native_raw = C.read_regular(Path(native_bundle), C.COMPRESSED_LIMIT)
    C.require(C.digest(native_raw) == native_sha, "reference native archive SHA256 differs")
    native_files, native_modes, _ = C.unpack_checked(native_raw)
    native_manifest = C.validate_manifest(native_files, native_modes, source_commit)
    C.require(native_manifest["schema"] == C.SNAPSHOT.SCHEMA,
              "saved reference packet requires a v4 native bundle")
    files, modes = unpack_packet(raw)
    document = validate_roles(files, modes, native_files, native_manifest, native_sha, len(native_raw))
    archive, selected = wire_members(files["source-wire.zip"])
    try:
        for role in ("alias", "snapshot"):
            name = role + "-wire/acceptance.json"
            C.require(archive.getinfo(name).file_size <= 1 << 20, "source report exceeds byte bound")
            report_raw = archive.read(name)
            report = C.strict_json(report_raw, "saved " + role + " acceptance")
            if role == "alias":
                C.require(report.get("schema") == "gooo/alias-origin-mcp-wire-verification/v1" and
                          report.get("status") == "INTERNAL_ALIAS_MCP_TWO_PROTOCOL_COMPARISON_PASS" and
                          report.get("scope") == "full" and report.get("saved_evidence_validated") is True and
                          report.get("actual_protocol_sessions") == 8 and
                          report.get("all_direct_wait_EOF_PG_ESRCH") is True,
                          "saved alias report format/full scope differs")
            else:
                C.require(report.get("schema") == "gooo/snapshot-MCP-wire-verification/v1" and
                          report.get("accepted") is True and
                          report.get("status") == "INTERNAL_SNAPSHOT_MCP_TWO_PROTOCOLS_ALL_EIGHT_PROFILES_PASS",
                          "saved snapshot report format/full scope differs")
            C.require(report.get("first_failure") is None and
                      report["binary"]["sha256"] == C.digest(native_files[C.BINARY]) and
                      report[role + "_cli"]["sha256"] == C.digest(files[role + "-compare"]),
                      "saved report native/reference identity differs: " + role)
            C.require(native_manifest.get("finite_checks", {}).get(role + "_full_report_sha256") == C.digest(report_raw) and
                      native_manifest["finite_checks"].get(role + "_reference_cli_sha256") ==
                      C.digest(files[role + "-compare"]),
                      "native bundle does not bind this original report/reference: " + role)
            if role == "snapshot":
                C.require(report["go"] == document["original_observation_go_tool"] and
                          report["schemas"]["output"] == native_manifest["optional_profiles"]["definition_snapshot"]["output_schema"],
                          "saved snapshot Go/schema identity differs")
        out = Path(out)
        out.mkdir(mode=0o700, parents=False, exist_ok=False)
        for name in sorted(files):
            C.write_new(out / name, files[name], modes[name])
        # Preserve ZIP-relative producer directories. The caller chooses this
        # fresh root; no absolute pathname from a captured report is followed.
        # Strict saved validators still require the original recorded cwd/argv.
        wire = out
        captures = []
        for info in selected:
            target = wire.joinpath(*PurePosixPath(info.filename).parts)
            target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
            capture = archive.read(info)
            C.require(len(capture) == info.file_size, "source wire capture length differs")
            C.write_new(target, capture, 0o644)
            captures.append({"file": info.filename, "bytes": len(capture), "sha256": C.digest(capture)})
        result = {"schema": "gooo/prepared-saved-native-reference-packet/v1",
                  "status": "PINNED_REFERENCE_BYTES_MATERIALIZED_NO_EXECUTION",
                  "packet_sha256": expected_sha, "packet_bytes": len(raw),
                  "native_archive_sha256": native_sha, "native_source": source_commit,
                  "source_wire_sha256": document["source_wire"]["sha256"],
                  "original_observation_go_tool": document["original_observation_go_tool"],
                  "references": document["references"], "captures": captures,
                  "source_report_argv_or_outputs_rewritten": False,
                  "reference_or_native_executed": False, "producer_authenticated": False,
                  "image_verified": False, "new_native_Gooo_receipts": False}
        C.write_new(out / "prepared-references.json", (json.dumps(result, sort_keys=True, indent=2) + "\n").encode(), 0o644)
        return result
    finally:
        archive.close()
