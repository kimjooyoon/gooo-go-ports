# Fixed preview16 OCI publisher

This workflow is a proposed delivery path for the existing native Linux amd64
preview16 bundle. It does not rebuild Go or Gooo. No image has been published or
verified by preparing this workflow. The existing local-image instructions remain
in [service-grant-container.md](service-grant-container.md).

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

## Verification and publication

Pull requests run the read-only `verify` job on a GitHub-hosted Linux runner. They
have no package-write or OIDC permission. Manual dispatch defaults to `verify`.
It runs the existing preparation and cleanup tests plus the OCI helper tests,
builds one image with build networking disabled, and reuses the existing
`verify-service-grant-container.py` client against that exact image ID.

The client checks real default and optional sessions, full proof fields, the
malformed-input refusal, actual runtime files, read-only rootfs, network mode,
nonroot user and separate Docker CLI/container termination and removal. It uses
protocol `2025-11-25`. Earlier native tests cover other documented protocol
versions, but are not new results from this workflow.

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
crypto/x509 security fix. This candidate has no actual scanner/reachability
result and does not claim that fix affects or cannot affect this MCP path.
There is no silent source/toolchain replacement or networkless-runtime waiver.
A failed HIGH/CRITICAL gate requires a separately reviewed input-release decision.

Only manual `publish` on this repository's default `main`, on attempt1, can enter
the package-write/OIDC job. It downloads the current verification job's exact
artifact ID, verifies its run/attempt/checkout identity and every declared file
hash, and loads the saved image. It checks the loaded image ID rather than
rebuilding. It pushes once under
`v0.1.0-preview.16-RUN_ID-a1`; no preview16 alias or `latest` tag is overwritten.
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
all eleven embedded Go module identities and the embedded Go version even though
`scratch` has no operating-system packages. Notices with tooling-only scope do
not imply that all those tools are present in the executable.

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
