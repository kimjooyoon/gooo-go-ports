# Source attribution

The scalar example adapts the expression and bounded semantics of Google's CEL
Go codelab `exercise3` (Logical AND/OR).

Copyright 2020 Google LLC. Licensed under the Apache License, Version 2.0.
The upstream file retains the usual AS IS warranty disclaimer. The complete
original license is included as `oracle/upstream/LICENSE`
(and identically at `../../ports/cel/LICENSE-APACHE-2.0`); the separately used
CEL runtime release license is retained as `LICENSE-APACHE-2.0`. The Gooo adaptation is changed from
the original: it receives explicit raw scalar presence facts and returns a full
decision record instead of a CEL value/error. It does not implement CEL itself.

Source: https://github.com/google/cel-go/blob/6d32ef54e4f124929730b15d5eb9b9205da58992/codelab/solution/codelab.go

The separate `main.gooo` activity, typed transport, generator and binding checks
are original research code contributed to the Gooo ecosystem, licensed under
Apache-2.0. They add trusted-server fact gates and an original deterministic
explanation priority. These gates are not part of Google's codelab.

Actual oracle dependencies and source material are separately attributed in
`oracle/`; this directory does not imply endorsement by Google or CEL authors.
