# Experimental portable Gooo workflows

This source package groups two existing instruction workflows: edit a small
Gooo program, or compare one MCP tool definition in saved before/after pages.
The [language and compiler](https://github.com/kimjooyoon/meta-ontology-go) are
developed separately. Native tools and example checkouts are prepared using the
included instructions; this package contains no executable or MCP server.

Version **0.1.0** freezes the six skill files from
[`e8d4c554a77b5ea90e58ef63927915b97725933c`](https://github.com/kimjooyoon/gooo-go-ports/commit/e8d4c554a77b5ea90e58ef63927915b97725933c).
It does not automatically follow `main`. Read the packaged
[first-program skill](../plugins/gooo-workflows/skills/gooo-first-program/SKILL.md),
[saved-snapshot skill](../plugins/gooo-workflows/skills/gooo-compare-tool-snapshots/SKILL.md)
and [source record](../plugins/gooo-workflows/SOURCE.json) before trying it.

## What has been checked

- A separate `skills@1.7.4` installation copied the snapshot skill's three files
  into a fresh project with matching source bytes. That was a standalone skill
  installation, not an installation of this portable plugin.
- Codex CLI **0.165.0-alpha.1.2** listed the local package name, version, source
  path and marketplace policies with temporary source configuration. It reported
  `installed: false` and `enabled: false`; it did not return skill metadata.
- Portable plugin installation, skill loading, activation, workflow completion
  and publication in the official OpenAI directory have not been verified.
  A metadata listing or matching file hashes do not establish those outcomes.

For an actual local installation, review the
[official plugin instructions](https://developers.openai.com/plugins/build/plugins#install-a-local-plugin-manually).
This repository does not register a global marketplace or change host settings
on your behalf. This route is separate from the Claude `gooo-porting` adapter.

## Inspect local metadata

From this checkout, use its absolute path as the local marketplace source:

```sh
codex -C /absolute/path/to/gooo-go-ports \
  -c 'marketplaces.gooo-go-ports.source_type="local"' \
  -c 'marketplaces.gooo-go-ports.source="/absolute/path/to/gooo-go-ports"' \
  plugin list --marketplace gooo-go-ports --available --json
```

Replace both path placeholders with the same checkout path. This is a metadata
query, not an install command. The retained internal trial denied network access
and writes to global agent settings; it made no model or native workflow call.
An empty list is not evidence that the plugin was parsed or rejected. Host
versions, configured sources and managed policy can affect discovery.

## Report the first useful result or failure

Use the existing [first-attempt feedback path](ai-agents.md#report-a-first-attempt-or-a-real-task).
Tell us which task you needed, your host and OS, whether you used a standalone
skill or portable plugin, and the stage reached: metadata, installation, skill
loading or actual work. Include the package version/source ref and the first
diagnostic or observed result when available. A short failure report is enough;
private MCP pages, credentials and full environment dumps are not required.

## Maintain the fixed snapshot

Keep the canonical `.agents/skills/` files and this versioned copy separate.
Do not edit the packaged skill bodies to follow current `main`. Check their
declared Git source and exact ten-file roster with:

```sh
python3 tools/package-gooo-workflows.py --check
python3 tools/package-gooo-workflows.py --output /absolute/new/export-directory
```

The output directory must not exist. The export reconstructs skill/license
bytes from the declared local Git commit and preserves the reviewed metadata
and artwork. It does not overwrite this package or the original review ZIP.
For an explicitly reviewed future revision, add `--source-ref FULL_COMMIT_SHA`
and `--version NEW_VERSION` with a different new output directory. Review its
source record and changes before replacing a checked-in version; generated
provenance is not execution evidence. The helper is optional and adds no CI
exception: this package's new/nested files require the repository's full checks.
