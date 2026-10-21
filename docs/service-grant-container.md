# Run the Gooo service-grant MCP tool in Docker

This packages the native Linux amd64 command in a `scratch` image. The runtime
needs no Go, Gooo compiler, Python, API key, application network or writable root
filesystem. It keeps all 58 license/notice texts and the native build identity.
Docker and Python 3.9 or later are needed to prepare and build the image. The image is built
locally; no hosted container image or automatic MCP client install is provided.

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
