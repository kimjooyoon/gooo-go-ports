# Pinned tutorial evidence profile

`before.json`, `after.json`, and `replay.json` are unchanged, synthetic records
retained from a maintainer-owned internal trial on 2026-10-17. The four scores
are -5, 42, 75, and 105 and every input label is `sensor`. They contain public
tutorial/generated source, digests and process observations, with no account
identity, credential or absolute host path. `compiler-build.json` is the
separately captured build identity; it is not runtime process accounting.

The source snapshot was `5a5f3010f6be5923630722850156a9ec3400ab39`;
the tutorial source/input/manifest bytes also equal the public snapshot
`b19b5517f8e9716affb0f75fd07045b8217cdaf1`. Only `if score >= 60` was changed
to `if score >= 80` in the edited source. The compiler was
`83928fa4538245abe33b08549da47f8cbdc50ed2`, using Go 1.27.2.

These are finite observations, not new execution, external adoption, a signed
attestation or proof for other inputs. Their schemas and static tutorial values
define this checker's supported profile. Original metric values are not expected
performance values: process timing/CPU/RSS, platform/tool bytes and exact parent
digests are checked separately. Unsupported record shapes fail closed.

The pin's five source-defined Go selection tags are accepted independently for
each record: `explicit`, `path`, `compiler_goroot`, `toolchain_cache`, and
`path_fallback`. Only `explicit` and `path` have retained native observations
here; offline shape support for the other tags is not execution coverage. The
actual Go version and binary hash must still match across all three records.
The fixed build and four-input driver stdout hashes remain bound to the
profile; only Go version-command stdout varies with the supported platform.

`result-profile-83928fa.json` and the typed Result codec are retained from
`cmd/check-path-facts`, SHA256
`9eafadab81a6c7fe33a4f59777e3776cc27f31c7962e472f3828e77867142d44`.
That codec reconstructs pin839's typed Result serialization for the replay
`prior_result_sha256`; the outer `replayed_from_sha256` binds complete original
receipt bytes. Neither is interchangeable with the other or a sorted whole-map
hash. Own Composition uses its producer-serialized raw subtree with Go JSON
HTML escaping, a separate coordinate from prior typed Result serialization.

File-byte SHA256 is distinct from a source-image `source_digest`, which hashes
Go JSON encoding of the source string. `semantic_digest` is another coordinate;
this checker does not rederive arbitrary semantic or completeness proofs.
