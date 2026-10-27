# Run the Gooo service-grant MCP tool in Docker

The public Linux amd64 image packages the existing preview16 native command.
Running it needs Docker, but no Go, Gooo compiler, Python or API key.
The application runs as a nonroot user with a read-only root filesystem and no
application network. Docker needs network access for the initial image pull.

## Use the public immutable image

Pull this exact digest before connecting an MCP host:

```sh
docker image pull ghcr.io/kimjooyoon/gooo-service-grant-mcp@sha256:a83d8fa57657bb877453cce7ed11f1ff1000a84b1e80c65a4e1c0b20196d3ee4
```

Use your actual Docker executable path and this command/argument array in the
host's stdio MCP settings. Keep stdin interactive without allocating a TTY:

```json
{
  "command": "/usr/bin/docker",
  "args": ["run", "--rm", "--interactive", "--pull=never",
    "--platform=linux/amd64", "--network=none", "--read-only",
    "--cap-drop=ALL", "--security-opt=no-new-privileges",
    "--pids-limit=32", "--memory=128m",
    "ghcr.io/kimjooyoon/gooo-service-grant-mcp@sha256:a83d8fa57657bb877453cce7ed11f1ff1000a84b1e80c65a4e1c0b20196d3ee4"]
}
```

Append `--many-required` after the image reference to expose the optional
multi-request tool. Both modes calculate supplied policy facts; they do not
apply grants or invoke a service. Hosted Linux checks pulled this digest and
compared its 63 runtime files and complete protocol results with the original
native archive. This does not prove activation in every agent host or support
for native arm64 execution. Other architectures require Docker emulation.
See [native onboarding](../examples/service-grant-mcp/README.md) for the
represented service-pattern domain and a concrete first call.

Keep the image digest when reporting the first failed step or actual task
result in the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).

## Build the same native input locally

The local build below keeps all 58 license/notice texts and the native build
identity in a `scratch` image. Preparation needs Docker and Python 3.9 or later.

From a checkout containing the container tools, download the public preview16
bundle and prepare a fresh build context:

```sh
mkdir -p out
curl --fail --location --max-time 120 --max-filesize 16777216 \
  --output out/gooo-service-grant-mcp-linux-amd64.tar.gz \
  https://github.com/kimjooyoon/gooo-go-ports/releases/download/v0.1.0-preview.16/gooo-service-grant-mcp-linux-amd64.tar.gz
python3 -I -B tools/service_grant_mcp_container.py \
  --bundle out/gooo-service-grant-mcp-linux-amd64.tar.gz \
  --sha256 0ae9bc25eef2ad73535642a66b8b7602e9f64a148b3e60696e36048a3b4a0524 \
  --source-commit fceb89f8b53eabd6687d16258d7cee89853c2cfe \
  --out out/service-grant-container-context
docker build --network=none --pull=false --platform=linux/amd64 \
  --iidfile out/service-grant-container.image \
  out/service-grant-container-context
```

Preparation checks the **whole archive against the supplied expected hash**,
its Linux/source identity and every included file before creating the context.
The expected hash must come from a source you trust; a matching manifest alone
does not authenticate a publisher. Fresh contexts avoid stale or mixed files.
This is the existing preview16 executable, not a rebuild from the current tree.

An MCP host can start it with the following command and argument array, using
the actual absolute Docker path and the image ID saved above:

```json
{
  "command": "/usr/bin/docker",
  "args": ["run", "--rm", "--interactive", "--pull=never",
    "--platform=linux/amd64", "--network=none", "--read-only",
    "--cap-drop=ALL", "--security-opt=no-new-privileges",
    "--pids-limit=32", "--memory=128m", "sha256:YOUR_LOCAL_IMAGE_ID"]
}
```

Use stdin without a TTY. To additionally expose `assess_service_grant_change`,
append `--many-required` after the image ID. Default mode exposes only
`assess_service_grant`. Both return decisions on supplied facts; neither
authenticates authority, changes permissions or invokes another service.
See the [native onboarding](../examples/service-grant-mcp/README.md) for inputs
and the represented policy domain. Linux amd64 is the supported image platform;
other architectures require Docker emulation and have not been verified here.

To check your locally built image without an AI model:

```sh
python3 -I -B tools/verify-service-grant-container.py \
  --docker "$(command -v docker)" \
  --image "$(cat out/service-grant-container.image)" \
  --context out/service-grant-container-context \
  --out out/service-grant-container-check
```

The finite smoke checks actual image contents, startup, tool discovery, complete
proof objects and malformed input in two sessions (default and optional mode).
It records the Docker CLI and the container's separate termination and removes
only its own containers. Native verification continues to cover both documented
MCP protocol versions; this image smoke uses `2025-11-25`. It does not establish
every client's compatibility, Glama approval or external adoption.

Linux CI reuses the bundle it already built and verified. The container step
adds no Go or language-engine build and does not alter the full porting checks.
Docker's [exec entrypoint](https://docs.docker.com/reference/dockerfile/#entrypoint)
and [interactive stdin](https://docs.docker.com/reference/cli/docker/container/run/)
provide the stdio transport. Please report a real first-run failure or task result
in the [feedback form](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).

## Saved-definition source profile

The current source adds an explicit `--definition-snapshot` profile and the
`gooo_compare_saved_tool_definitions` tool. It accepts exact saved `tools/list`
pages; it does not collect a catalog or invoke a remote tool. Compare the
decision with both page-coverage fields before using a result. Missing pages
are not evidence that a tool disappeared, and a matching definition does not
prove origin, authorization or behavioral equivalence.

This profile uses native bundle **v4**: 68 flat runtime files, including 62
license/notice texts and `snapshot-example.json`. The fixed preview16 download
and registry descriptor above retain their original v2 behavior and do not
include the new flag. Use a separately verified current-source v4 candidate
when testing it; do not relabel an earlier archive or its observations.

The MCP workflow checks the fixed public comparison CLI against the source
executable and the freshly extracted candidate, including all eight flag
combinations under both protocol versions. The v4 Linux container check repeats
the saved comparison corpus in 16 real stdio sessions, with the network disabled
and a read-only filesystem. It reads the existing native reference evidence;
it does not rerun Go or a compiler inside the container. The legacy service,
ordered-request and alias tools remain available according to their own flags.
These finite checks are internal transport checks, not an external-adoption or
general client-conformance claim.

[The source profile](../tools/profiles/mcp-definition-snapshot-v4.json) binds all
26 MCP source files to an immutable commit with raw-file and Git-blob identities
and PROV-O derivation entities. For the next source revision,
`tools/freeze_mcp_source_profile.py` exports a complete roster in one operation
to a new file outside the MCP module. Reviewing/promoting that new profile is
still explicit; working-tree edits and newly tracked files cannot silently pass
the old profile. The declaration-derived output shape also records its separate
inspection compiler, stable field IDs and source/workspace hashes. It does not
change the generated kernel's compiler pin or imply another Gooo execution.
