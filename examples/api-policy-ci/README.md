# Experimental Gooo readonly policy review Action

Use your own representative API facts to inspect a specific policy change in CI:
the sealed original API profile versus the sealed readonly candidate. For example,
a request that originally permits `write` becomes `NEWLY_DENIED` under the
readonly profile. A permitted `read` remains `UNCHANGED`.

This experimental Action is not listed on GitHub Marketplace. Its limited
question is useful before adopting **these two supplied profiles**. It cannot
load or compare your application's arbitrary policies. A workflow observation is not evidence of an outside installation.

The policy, all fifteen decision-field differences, classification and changed
count execute through existing generated [Gooo](https://github.com/kimjooyoon/meta-ontology-go)
code. The host handles files, version checks, saved report verification and the
optional CI exit status. There is no handwritten Go policy fallback, live API
dispatcher, GitHub token, PR comment, human approval step or GitHub API write.

## Try the supplied migration cases

Save the nine-field JSON sample below as `ci/policy-case.json` in your repository,
then add this workflow. The Action reference pins the supplied experimental
implementation to an immutable commit. For a checkout of this repository,
`./examples/api-policy-ci` also works.
Use a GitHub-hosted Ubuntu runner with Go **1.27.2** selected. The pilot uses Bash,
GNU `timeout`, standard tools and two checksum-pinned public Go modules; initial
dependency retrieval reads the Go proxy/checksum service. It does not install a
Gooo compiler, CEL interpreter or a Go toolchain.

```yaml
name: Review readonly migration cases
on: [push, pull_request]
permissions:
  contents: read
jobs:
  review:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
        with:
          go-version: '1.27.2'
          cache: false
      - id: policy
        uses: kimjooyoon/gooo-go-ports/examples/api-policy-ci@10363958cce2dd97418a3074e3c87743b96547dc
        with:
          input-file: ci/policy-case.json
          fail-on-newly-denied: 'false'
          summary: 'true'
```

Each file contains exactly these nine fields, using your representative values:

```json
{
  "group_present": true,
  "group_value": "admin",
  "principal_present": true,
  "principal_value": "synthetic:user",
  "authenticated": true,
  "caller_tenant": "tenant-a",
  "resource_tenant": "tenant-a",
  "requested_action": "write",
  "allowed_action": "write"
}
```

With these synthetic values the existing published API specifies `NEWLY_DENIED`
and changed count `2`. Change both action fields to `read` for the unchanged case.
These are prior API expectations, not observations from running this pilot.

Inputs travel through environment values and a workspace-relative file argument;
they are never interpolated into shell source. The reader refuses symlink input
components, noncanonical paths such as `link/../facts.json`, and nonregular files,
bounds facts to 4 MiB and saved reports to 16 MiB,
and uses the public strict typed decoders. Missing, duplicate, unknown, malformed
or wrongly typed facts fail rather than becoming an allow decision. Each decoded
Text remains bounded to 65,536 UTF-8 bytes by the original API.

## Optional job summary

Set `summary: 'true'` to show the saved comparison's classification, changed-field
count, original and candidate reason labels, and the names of changed fields in
the GitHub Actions job summary. This is off by default. It includes no raw
principal, group, tenant or other fact values and adds no action outputs; the
five existing outputs below remain unchanged. Summary formatting requires the
runner's Python 3 standard library and appends at most 4 KiB.

For the supplied write example, the summary shows `NEWLY_DENIED`, count `2`,
`ALLOW` → `ACTION_NOT_READ_ONLY`, and changed fields `allowed`, `reason`. The
supplied read example shows `UNCHANGED`, count `0`, and `ALLOW` → `ALLOW`.
The formatter displays allowlisted fields from the exact saved report after
checking its SHA-256. It does not calculate the policy, recompute the complete
typed comparison, or grant permission to execute an API call.

When `fail-on-newly-denied: 'true'` saves a valid `NEWLY_DENIED` report and then
fails the review, an enabled summary step can still run unless the job is
cancelled. The Action keeps the original failure; a successful summary does not
turn it into success. A malformed input that produces no saved report has no
summary to display. Opted-in summary rendering or append errors also fail.
Do not add `continue-on-error` to a consumer workflow just to obtain the summary.
Outputs from a failed composite Action may be unavailable to later caller
steps, so this option does not promise new failure-path output transport.

## Results and replay

Outputs are `classification`, `changed-count`, `report-path`, `receipt-path` and
`report-sha256`. A fresh private directory holds `comparison.json` with both
complete fifteen-field decisions, all difference flags and source bindings, plus
a small receipt linking the input bytes, report bytes and actual compiled module
identities. The comparison is rechecked through the complete published verifier
before being saved. A valid policy denial is a successful observation by default.

The normal runner-owned `GITHUB_OUTPUT` file must already exist under
`RUNNER_TEMP`, outside the workspace and this action's private work directory,
with one hardlink. It is opened before input processing and its descriptor/name
identity is checked through the append. Input or result aliases are refused;
caller JSON must never be used as a substitute output file.

Set `fail-on-newly-denied: 'true'` to turn the **Gooo-computed** `NEWLY_DENIED`
label into a failing CI status after saving outputs. That is an explicit workflow
assertion, not a new policy evaluator. Malformed inputs and verification failures
always fail with closed diagnostic labels.

To recheck a report copied into the workspace, use `mode: replay` and set
`input-file` to that saved file. Replay rejects changed decisions or bindings;
it does not repair or migrate an older-generation report. Original files remain
untouched. Runtime results and caches remain under this job's owned temporary
directory; this pilot does not upload them or erase caller files. Full reports
contain raw group/principal/tenant facts: review them before explicitly sharing
or adding your own artifact-upload step. Job retention is not permanent storage.

The inputs are supplied observations. Their `authenticated` Boolean does not
authenticate anyone. An internally consistent unsigned report neither attests an
event nor grants permission to execute now. Current trusted facts, authentication
and real effects belong to the integrating application. This is not a security
guarantee, generic policy rollout service or production validation.

## Reproduction and publication boundary

The revision module is pinned to public commit
`e723f9af6dc33ecfff002a951e4b09a310439338`; the original module is pinned to
`8cddcc4e65afb42431f8f42a86f29859ae2f4d69`. Their previously verified public
pseudo-versions and checksum lines are retained in `go.mod`/`go.sum`. Consumer
replacement modules and automatic toolchain switching are refused.

The separate `.github/workflows/api-policy-action.yml` records the checked source,
runs all twenty-one Linux CLI controls, sixteen formatter controls and two
summary-append transport controls. It preserves the complete synthetic write
report comparison and exercises enabled summaries
for the supplied write and read cases. A deliberately failed synthetic write
review checks both the emitted summary and the preserved failing step outcome;
`continue-on-error` appears only in that failure-path test. The two transport
controls exercise the actual append wrapper with a synthetic runner file,
including a late append refusal that must leave no completion file. These
controls do not replace the actual hosted composite runs. The actual hosted run
supplies the result; local source inspection or crossbuild alone does not
establish Linux composite behavior. Existing full repository verification remains
required. The standalone Action does not upload callers' reports; the validation
workflow retains only its own known synthetic records.

This in-repository path creates no separate Action repository and includes no
Marketplace listing or agreement. Any later listing requires a separate review of
[current Marketplace requirements](https://docs.github.com/en/actions/how-tos/create-and-publish-actions/publish-in-github-marketplace).
Use an independently checked immutable repository commit when referencing a
published action revision; no future commit is named here.
