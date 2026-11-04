# Published preview17 OCI Registry entry and retained history

The official MCP Registry publishes `io.github.kimjooyoon/gooo-service-grant`
version `0.1.0-preview.17` with a digest-pinned Linux amd64 Docker/stdio package.
Read the [exact Registry entry](https://registry.modelcontextprotocol.io/v0.1/servers/io.github.kimjooyoon%2Fgooo-service-grant/versions/0.1.0-preview.17)
and use the [public image installation guide](service-grant-container.md#use-the-public-immutable-image)
to connect a host. Registry publication establishes discovery metadata; client
installation and a completed task require their own observations.

## Published preview17 saved-definition entry

The published preview17 descriptor selects `--definition-snapshot`. This adds
`gooo_compare_saved_tool_definitions` alongside the default
`assess_service_grant` tool. Starting the native binary without arguments exposes
only `assess_service_grant`; the ordered-request and alias tools require their
separate optional flags. The Registry descriptor does not select all four tools.
Use the [preview17 exact-image instructions](https://github.com/kimjooyoon/gooo-go-ports/releases/tag/v0.1.0-preview.17#container-for-mcp-clients-linux-amd64)
for the selected snapshot mode.

The original [Registry publication run 40650121951, attempt 1](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/40650121951/attempts/1)
published this version and retained exact anonymous readback. A separate
[public runtime readback run 40651313266, attempt 1](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/40651313266/attempts/1)
checked the same immutable image's runtime bytes and finite protocol sessions;
it did not publish another version. Both workflows used source
`f56b8e60141cbb2390f63433920392129af754b2`. A later exact-version Registry GET
on 2026-10-30 returned HTTP 200 with a server record matching the checked-in
descriptor. Preview17 is already registered; installation does not require
publishing it again. These observations do not establish every client's
installation or external adoption.

The publisher accepts `--release-version v0.1.0-preview.17`; its default remains
preview16. The corresponding manual Actions input selects the same version.
The byte-pinned [publication profile](../tools/profiles/mcp-registry-preview17.json)
keeps three identities separate: the original native release, the successful
[OCI assembly and anonymous readback](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/40632109714),
and the source revision containing Registry metadata. Preview17 uses that normal
publication run; preview16 retains the proof-recovery prerequisite below.

PR checks validate both descriptors through the pinned official publisher.
The shared installation job consumes each actual descriptor. Preview17 runs the
selected snapshot task on both supported protocols, compares complete reports
with the saved original references, checks malformed-input refusal and a repeat
after errors, and verifies copied runtime bytes and cleanup. Its task option is
passed after the image; Docker resource options remain before it. The optional
alias and ordered-request modes have separate container verification records.

The original reference packet supplies saved observations and byte-identical
CLIs. Its original Go identity reader is selected by the native profile, with
root Go kept separate. The installation check does not rerun those reference
programs or rewrite their inputs, outputs or source bindings.

After final-source checks, publication requires the owned manual main context,
attempt one, version absence, one official publish call and exact anonymous
version readback. A later preview needs a reviewed profile, descriptor and
selection update; arbitrary versions or asset URLs are refused.

The remaining sections retain the earlier candidate checks and publication
guards, with their original source and execution identities.

The preview16 descriptor declares one public, digest-pinned Linux amd64 OCI
package with stdio transport. It adds Docker arguments for network isolation,
a read-only root filesystem and the explicit platform. It does not add an API
key, host mount, shell command, mutable tag, or a remote MCP endpoint.

The candidate validation described below did not publish a Registry version.
Preview13–15 metadata and the original preview16 native release assets remain
unchanged. The original native input is `fceb89f8b53eabd6687d16258d7cee89853c2cfe`; Registry metadata and
proof recovery have their own source identities.

## Validation and execution are separate

The pinned official publisher1.8.1 calls the production Registry `/v0/validate`
endpoint for schema and semantic validation. That call does not authenticate
package ownership, install the MCP server or publish a Registry version.
The read-only PR validation job still cannot request an OIDC token or publish.

The shared `registry-install-smoke.yml` job pulls the exact descriptor reference with
an empty Docker client configuration. It binds that reference to the local
image ID before any session starts. The headless client reads runtime arguments
from the byte-pinned descriptor, creates containers and exercises the existing
complete default and optional multi-request proof comparisons. It also compares
all runtime file bytes/modes to the fixed original native archive and checks
container/process cleanup. Additional capability, memory and process limits are
client verification controls, beyond the three descriptor arguments.

It runs after Registry validation on pull requests. An explicit read-only
`verify` dispatch of the existing OCI workflow can also exercise it on a
candidate branch, without publishing an image or requesting signing permission.
A candidate-branch result is not a replacement for final-source full native CI.

The candidate at `be8c0c6577ca0f3ff03de2fabfaaf7138010fea8` passed
[read-only hosted verification
39685013716](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39685013716):
an anonymous pull of the descriptor digest, all 63 runtime file comparisons,
complete default and multi-request proof comparisons, malformed-input refusal,
and container cleanup. New source revisions require their own checks.

These finite checks do not prove installation by every agent host, a model-led
task or outside adoption. The assessment evaluates supplied facts and never
applies grants or authenticates live authority.

## Publication prerequisites

For preview16, `publish-service-grant-registry.py` checks the whole descriptor hash, its exact
package shape, the original public native assets and version absence before any
publication. A reviewed successful original-proof recovery run is required
before the publisher is bootstrapped or authenticated. It checks the exact run,
source, workflow/main identity, first attempt and both jobs including cleanup.

`OCI_RECOVERY` pins the reviewed [proof-only recovery run
39687579594](https://github.com/kimjooyoon/gooo-go-ports/actions/runs/39687579594),
at source `ace68b20d9be24f77dbb3ec9c8630aa9908996f2`. It preserved the original
build statement and signature from assembly run 39672180710, transported that
proof to the original image digest, and separately signed the retained SBOM.
The original failed assembly workflow remains failed; the image was not rebuilt
or pushed again. Recovery success does not constitute a current vulnerability
scan or final-source native CI acceptance. The descriptor cannot itself grant
permission to publish. The candidate source change required full native CI before
entering main; recovery acceptance did not replace that check.

After those prerequisites, publication retains one official CLI publish call,
no automatic retry, and anonymous exact-version readback. Readback must include
the exact requested package instead of accepting a metadata-only record. Do not
infer publication from the checked-in descriptor or from validation success.
