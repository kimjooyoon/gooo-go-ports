# Installable OCI Registry candidate

The preview16 descriptor declares one public, digest-pinned Linux amd64 OCI
package with stdio transport. It adds Docker arguments for network isolation,
a read-only root filesystem and the explicit platform. It does not add an API
key, host mount, shell command, mutable tag, or a remote MCP endpoint.

This development branch is not a Registry publication. Preview13–15 metadata
and the original preview16 native release assets remain unchanged. The original
native input is `fceb89f8b53eabd6687d16258d7cee89853c2cfe`; Registry metadata and
proof recovery have their own source identities.

## Validation and execution are separate

The pinned official publisher1.8.1 calls the production Registry `/v0/validate`
endpoint for schema and semantic validation. That call does not authenticate
package ownership, install the MCP server or publish a Registry version.
The read-only PR validation job still cannot request an OIDC token or publish.

The separate `install-candidate` CI job pulls the exact descriptor reference with
an empty Docker client configuration. It binds that reference to the local
image ID before any session starts. The headless client reads runtime arguments
from the byte-pinned descriptor, creates containers and exercises the existing
complete default and optional multi-request proof comparisons. It also compares
all runtime file bytes/modes to the fixed original native archive and checks
container/process cleanup. Additional capability, memory and process limits are
client verification controls, beyond the three descriptor arguments.

These finite checks do not prove installation by every agent host, a model-led
task or outside adoption. The assessment evaluates supplied facts and never
applies grants or authenticates live authority.

## Publication prerequisites

`publish-service-grant-registry.py` checks the whole descriptor hash, its exact
package shape, the original public native assets and version absence before any
publication. A reviewed successful original-proof recovery run is required
before the publisher is bootstrapped or authenticated. It checks the exact run,
source, workflow/main identity, first attempt and both jobs including cleanup.

`OCI_RECOVERY` is deliberately unset on this local development branch until the
actual proof-only recovery succeeds and its source/result is reviewed. The live
publication path therefore refuses. This is a missing observed prerequisite,
not a request for a new human approval gate. The descriptor cannot itself grant
permission to publish. The source change must receive the required full native
CI checks before it can enter main.

After those prerequisites, publication retains one official CLI publish call,
no automatic retry, and anonymous exact-version readback. Readback must include
the exact requested package instead of accepting a metadata-only record. Do not
infer publication from the checked-in descriptor or from validation success.
