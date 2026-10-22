# Source and attribution

The alias-origin comparison activity in `ports/tool-origin/alias_origin.gooo`, its independent Go reference and the public wrapper are original Gooo Go Ports work under the repository BSD-3-Clause license. The generated Go kernel is derived from the pinned Gooo compiler's actual native output. It is not a handwritten Go comparison implementation.

The use case was informed by the public discussion in [google/adk-go #1126](https://github.com/google/adk-go/issues/1126). The ongoing [upstream PR #1821](https://github.com/google/adk-go/pull/1821) is separate work. This module copies no ADK implementation, does not dispatch through ADK and claims no security defect or full SDK compatibility.

The lossless JSON transport helper is adapted from the exact repository source recorded in `transport-source-provenance.json`, under BSD-3-Clause. Its original chain of attribution is retained in that pinned module's notices. The portable verifier uses the repository's existing `service-grant-many/verify.py` process runner.

Schema contract tests use `github.com/google/jsonschema-go v0.4.3` under its MIT-style license. This is a test dependency; the native command has no third-party runtime packages. No validator implementation is vendored. `schema-validation.json` records the test dependency and module checksums used to generate the module files.
