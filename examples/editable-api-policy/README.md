# Edit a Gooo policy and compare its decisions

Change `after.gooo`, evaluate the same supplied facts with both versions, and
keep each result attached to its own source and official saved replay. This is
an experimental offline example, not live authorization or a trace importer.
The existing sealed API-policy consumer and Action remain separate.

Start in the repository root with the pinned **Gooo compiler83928fa** and
**Go1.27.2** already prepared; see [setup](../../README.md#run-and-edit-locally).
The script uses Python3's standard library and an existing repository process
helper. It does not install tools, download modules, or build an engine.
Use canonical absolute tool/output paths and a new output directory whose
parent exists. On macOS/Linux, for example:

```sh
(
  set -eu
  fixed_go="$(GOTOOLCHAIN=go1.27.2 go env GOROOT)/bin/go"
  python3 -I -B examples/editable-api-policy/run.py \
    --root "$PWD" --compiler "$PWD/.gooo" --go "$fixed_go" \
    --out "$HOME/gooo-policy-demo-1"
)
```

With the supplied policies, expect `UNCHANGED`, `NEWLY_DENIED`, `UNCHANGED`.
Only the ordinary user's `write` changes `allowed` and `reason` to
`WRITE_REQUIRES_DEPLOY_BOT`; a read and the deploy bot's write remain allowed.
Both complete fifteen-field decisions and changed field names are saved in
`summary.json`. Raw command stdout, stderr, arguments and process status remain
in separate subdirectories. These files include the original supplied facts;
review them before sharing.

Now edit the condition or reason in `after.gooo`, keeping the same nine scalar
inputs, fifteen-field record and call to `original.APIAccess`. For example,
replace `synthetic:deploy-bot` with `synthetic:user` and change the reason to
`WRITE_REQUIRES_USER`, then run the same command with **a different fresh
`--out`**. The user write now stays allowed and the deploy bot write is denied.
Keep reason labels consistent with your rule: the runner reports the literal
reason and does not infer its meaning. The actual comparison is accepted without
forcing the original demo outcome. `--check-demo` additionally checks the
separate original three-row literals and is intended for the unchanged demo
and CI; omit it after editing the rule.

`--inputs /absolute/path/facts.json` accepts one to three rows using the exact
`demo-inputs.json` schema: three Boolean facts and six Unicode-scalar UTF8 Text
facts per row, each Text at most1024bytes and the whole document at most30000bytes.
Missing/unknown/duplicate fields, wrong types, isolated surrogates and trailing
JSON are errors. Facts are not trimmed or repaired. Valid individual strings
can still exceed the compiler's joint translated-input budget and be refused.
Do not fill in redacted facts or missing rate-limit state; this example does
not support stateful traces.

Each version gets one input-only `execute` and its own official `replay`.
Each command performs two fresh native runs. Input-only receipts remain
`OBSERVED` with `finite_total:0`; comparison and optional literal checks happen
in this consumer. The ordered observed records are also matched to both native
runs' reported stdout hashes. Replaying the before receipt against the after
workspace must be refused at the envelope/manifest stage; that expected exit1
is recorded separately. This control checks different manifests and does not
by itself test the changed-source guard under the same manifest. Source and tool
hashes are taken from the current run, so editing the rule is allowed between
runs. Editing sources during a run is refused. Inner canonical verification
belongs to the official replay command. No saved private proof is rebound.

The runner limits each command to90seconds, the whole run to300seconds, owned
work to1GiB and retained evidence to64MiB, with2GiB free required at start and
before commands. It uses offline Go settings and private caches, preserves the
actual HOME, and removes only its own temporary work after observed processes
close. Existing output directories are refused. Cold cache preparation is part
of the observed run; no performance guarantee or OS network sandbox is claimed.

The [Gooo API composition](../cel-api-policy/main.gooo) and
[CEL scalar adaptation](../../ports/cel/scalar_access.gooo) are reused in place.
See [NOTICE](NOTICE.md), the [local Apache-2.0 license](../../ports/cel/LICENSE-APACHE-2.0)
and [original source attribution](../cel-api-policy/NOTICE.md).

If setup or an edit fails, [report the first failing command and visible error](https://github.com/kimjooyoon/gooo-go-ports/issues/new?template=problem.yml).
No successful run or receipt is required to report a problem. Describe one real
task this comparison would help with, and remove private facts from any logs.
