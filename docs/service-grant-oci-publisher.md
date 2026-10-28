# Fixed preview16 OCI publisher

This page records the fixed delivery workflow and original publication of the
native Linux amd64 preview16 bundle. The public image is available through the
[digest-pinned installation guide](service-grant-container.md#use-the-public-immutable-image).
The workflow does not rebuild Go or Gooo; preparing it alone did not publish or
verify an image. Its original push and proof recovery have separate identities
from the later [MCP Registry publication](service-grant-registry-oci.md).

The fixed input is the public
[v0.1.0-preview.16 release](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.16):

| Identity | Fixed value |
| --- | --- |
| Archive | `gooo-service-grant-mcp-linux-amd64.tar.gz`, 7,217,754 bytes |
| Archive SHA256 | `0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524` |
| Native source | `fceb89f8b53eabd6687d16258d7cee89853c2cfe` |
| Native executable SHA256 | `a848a8747669e46e9afef6c0f1ed8b2014df4838cc370416f60ab9c6acccfc0a` |
| Platform | `linux/amd64`; no multi-platform claim |
| Go build identity | `go1.27.3`, CGO disabled |
| Image repository | `ghcr.io/kimjooyoon/gooo-service-grant-mcp` |

The helper accepts these pins only. The existing preparer checks the whole archive,
63 flat regular files, the BUNDLE manifest's other 62 files and all 58 notices.
`prepare` creates a fresh context. `verify-runtime` independently prepares another
expected context from that same fixed archive and checks a copied image runtime's
63 names, file bytes and modes. Neither command runs Docker or proves execution.
The image uses the existing `scratch` Dockerfile and absolute executable entrypoint.

## Reviewed release profiles

The current workflow reads the archive URL, byte limit, archive/executable hashes,
source commit and runtime/notice counts from one checksum-pinned repository
profile. Preview16 remains the default. Public native preview17 is also registered
for explicit selection; its source, executable and saved reference packet have
separate immutable identities. Inspect either profile without downloading or
executing anything:

```sh
python3 -I -B tools/service_grant_mcp_oci.py profile --version v0.1.0-preview.16
python3 -I -B tools/service_grant_mcp_oci.py profile --version v0.1.0-preview.17
```

`native_version` on manual dispatch selects only a reviewed version from the
loader's allowlist. It accepts no path, arbitrary URL or replacement checksum.
Preparation, publication and anonymous readback each load the same selected
profile. Publication also binds the profile SHA256 in its current-run manifest;
copied runtime checks still compare every actual file byte and mode. The profile's
PROV-O links distinguish archive-byte identity from native-source identity. They
do not authenticate a producer or establish execution, authorization or adoption.

Adding release data does not provide verification for a new tool capability.
The saved-definition v4 path additionally requires a checksum-pinned reference
packet containing the original saved alias/snapshot captures and byte-identical
reference CLIs. Its reader checks the native BUNDLE's original report/CLI hashes,
source, complete notices, archive roster, paths and expansion bounds before
materializing unchanged captures. It does not execute a comparison case.

The workflow acquires the original Go identity reader and fixed module sources,
checks their identities, and restores only fresh owned original Linux runner
locations. Existing argv/cwd records are not edited. The complete sixteen-session
image client then runs against the actual image. Both new image verification
and anonymous digest readback use these same references, with separate owned
file cleanup. Reference preparation alone is not image verification. No v4
public image or Registry update is implied here. The Registry install smoke stays
bound to its existing preview16 descriptor, and the historical recovery workflow
below keeps its original immutable inputs.

## Verification and publication

Pull requests run the read-only `verify` job on a GitHub-hosted Linux runner. They
have no package-write or OIDC permission. Manual dispatch defaults to `verify`.
It runs the existing preparation and cleanup tests plus the OCI helper tests,
builds one image with build networking disabled, and reuses the existing
`verify-service-grant-container.py` client against that exact image ID.

The client checks real default and optional sessions, full proof fields, the
malformed-input refusal, actual runtime files, read-only rootfs, network mode,
nonroot user and separate Docker CLI/container termination and removal. It uses
protocol `2025-11-25` for the default legacy path. When the reviewed release
requires saved references, it exercises the optional combinations with both
`2025-11-25` and `2026-07-28`; the v4 path includes sixteen sessions. These are
finite observations of that image and supplied inputs.

Trivy `0.76.0` is downloaded directly from its fixed public release and checked
against its fixed 52,253,735-byte SHA256
`e24b1c51293c1b15e4ba8d45bbc7891ba053f90278ec2698f3562c993e145fe7`.
A fresh scanner cache, clean environment and empty config/ignore inputs prevent
repository or inherited exclusions. The workflow retains CycloneDX, the JSON HIGH/CRITICAL
scan, scanner/database version metadata and diagnostic output. HIGH or CRITICAL
findings, scanner failure, missing dependency inventory or incomplete metadata
stop publication. `--ignore-unfixed` and arbitrary ignore policies are not used.
Scanner/database acquisition is build-time network activity; the MCP runtime
still runs with `--network=none` and needs no key, login or runtime download.

The fixed binary is built with Go1.27.3. The official
[Go release history](https://go.dev/doc/devel/release) reports Go1.27.3 with a
crypto/x509 security fix. Workflow preparation alone establishes no actual
scanner or reachability result. The retained original scan is a point-in-time
assessment and does not establish whether that fix affects this MCP path.
There is no silent source/toolchain replacement or networkless-runtime waiver.
A failed HIGH/CRITICAL gate requires a separately reviewed input-release decision.

Only manual `publish` on this repository's default `main`, on attempt1, can enter
the package-write/OIDC job. It downloads the current verification job's exact
artifact ID, verifies its run/attempt/checkout identity and every declared file
hash, and loads the saved image. It checks the loaded image ID rather than
rebuilding. It pushes once under
`NATIVE_VERSION-RUN_ID-a1`; no release alias or `latest` tag is overwritten.
A timeout or a later failure does not authorize another publisher dispatch.
Inspect the original push receipt first.

The workflow records the pushed registry manifest digest separately from the
local Docker image ID and saved-image archive hash. Both provenance and SBOM
attestations name that exact registry digest, use the reviewed immutable
`actions/attest` action and set `create-storage-record: false` for this personal
account. The retained build-material record links the fixed native archive,
binary, context and scanner artifacts to the image assembly workflow/checkout.
Default SLSA build provenance establishes this wrapper image's assembly workflow;
it does not retroactively authenticate the original native release producer or
turn an unsigned BUNDLE into a native-build attestation. The SBOM must retain
all embedded Go module identities declared by that bundle and its embedded Go version even though
`scratch` has no operating-system packages. Notices with tooling-only scope do
not imply that all those tools are present in the executable.

## Recovering the original partial publication

The original main-source run [39672180710](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39672180710)
pushed one image successfully, then failed registry attestation transport because
the pinned `@sigstore/oci`0.7.2 reader uses `os.homedir()/.docker/config.json`
and ignores `DOCKER_CONFIG`. The build signature was already created on GitHub.
Its original overall failure remains a failure; a recovery must not rerun the
publisher, rebuild the image, or sign a replacement build statement.

`registry/oci-recovery/` retains the exact public manifest, original signed bundle,
original SBOM and historical scan materials. `contract.json` binds their bytes
and the immutable action/dependency credential path. Its PROV-O Plan declaration
is documentation and regression input, not an execution authorization or a safety
proof. The retained scan is a snapshot, not a current vulnerability assessment.

The separate `service-grant-oci-proof-recovery.yml` defaults to read-only checks.
Only an explicit `recover` dispatch on this repository's default main, attempt1,
can transport the fixed original bundle and sign the retained SBOM. Before any
write, `gh attestation verify` checks the original bundle's signature against
its exact manifest digest, original source/workflow/main identity and hosted
runner requirement. The original signed bytes are transported unchanged.
The new SBOM attestation names the same image digest but truthfully uses the
new recovery workflow's source identity. Both proofs are then fetched from the
registry and cryptographically checked against their distinct signers; complete
verified build/SBOM contents must match the original retained statements.

Both writer jobs temporarily copy only their private GHCR login to the actual
reader path using `tools/registry_action_auth.py`. The helper stores the old
configuration privately, restores its exact bytes and mode in an `always()`
step, and refuses to overwrite concurrent changes. Neither the configuration
nor its state/backup directory is uploaded. Tests exercise the actual pinned
reader with a disposable fake home and synthetic credentials. Recovery results
are pending until the manual workflow and both registry verification steps
actually succeed; the checked-in materials alone are not evidence of recovery.

## Public visibility and readback gate

GitHub documents that a first GHCR publication is private by default. Repository
association and public visibility are separate. A successful authenticated push
or attestation therefore does not establish anonymous availability.

After a successful publish, a separate read-only job uses a fresh empty
`DOCKER_CONFIG`, removes token/auth environment inputs and pulls the exact digest
from the fixed image repository. It prepares the fixed native archive again,
copies `/opt/gooo/.` from the pulled image without starting it, and applies the
helper's exact runtime comparison before executing the existing finite
container wire verifier. It retains raw frames and separate cleanup records.
The fresh configuration proves no client credentials were supplied for that
pull; it is not a claim of a fresh physical host, empty daemon blob cache or
packet-level network tracing.

If that first anonymous pull fails because the package is private, retain the
failure and original digest. The owner may change the package's visibility in
GitHub's package settings. Then dispatch `verify-public` with that exact saved
`sha256:...` value. This mode cannot push or attest another image, and its result
is a public-readback/runtime result only; it does not replace a missing successful
publication, attestation or scan record. Reconcile all records before advertising
public installation. Do not rerun `publish` to fix visibility.

No consumer command containing a guessed digest is supplied here. Add an actual
consumer digest only after the full publication and public-readback gates pass.
The default server exposes `assess_service_grant`. An explicit `--many-required`
argument adds `assess_service_grant_change`. Both evaluate supplied facts; neither
authenticates current authority, applies permissions or invokes a remote service.
This delivery does not establish ToolHive installation, every MCP host's
compatibility, an AI-host task, independent adoption or a Glama image identity.

Official interfaces used by this workflow:

- [GHCR publication, visibility and anonymous pulls](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Pinned attestation action inputs and default provenance](https://github.com/actions/attest/blob/1e69f48acb82d1966a394da916b4c1698aa569d6/README.md)
- [Pinned artifact-download action inputs](https://github.com/actions/download-artifact/blob/9000827ccba6bdab643e8b6fd33ac0654aef8333/action.yml)
- [Trivy0.76 image CLI](https://github.com/aquasecurity/trivy/blob/v0.76.0/docs/guide/references/configuration/cli/trivy_image.md)
