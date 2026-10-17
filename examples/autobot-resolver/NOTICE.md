# AutoBot action-line resolver adaptation

Copyright 2025-2026 mrveiss. The adapted source and the independent Go reference
are provided under Apache-2.0; see [LICENSE-APACHE-2.0](LICENSE-APACHE-2.0).
The repository's BSD-style license does not replace these upstream conditions.

Original repository: https://github.com/mrveiss/AutoBot-AI

Pinned revision: `a9c47e296907f1e5f681c38c05146cc8d3c2e7a7`

Original file: [`autobot_shared/ai_lines.py`](https://github.com/mrveiss/AutoBot-AI/blob/a9c47e296907f1e5f681c38c05146cc8d3c2e7a7/autobot_shared/ai_lines.py)

The complete original [source](upstream/ai_lines.py.txt), [LICENSE](upstream/LICENSE)
and [NOTICE](upstream/NOTICE) are retained byte-for-byte. The original NOTICE's
references describe the upstream distribution; this example does not copy that
application or its third-party dependencies. Exact hashes are in
[source-pin.json](source-pin.json).

Changes: only `resolve_line`, `_unset` and `LineResolution.reason` are represented
as nine scalar facts and an eight-field Gooo record. Explicit presence flags
represent Python `None`; present empty baseline Text remains distinct from None.
The external-input converter enforces the finite wire/type/domain contract and
does not calculate a decision. The Go oracle is an independently written
verification reference, not the production policy or a fallback.

No tool catalogue, principal/scope resolution, live store, baseline retrieval,
approval lifecycle, historical-artifact resolver, authorization, dispatch or
side effect is included. This is not an AutoBot integration or endorsement.
