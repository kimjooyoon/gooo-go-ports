# gogcli tool selection in Gooo

This port implements `mcpToolAllowed` and the per-tool selection kernel of
`mcpFilterTools` from [openclaw/gogcli](https://github.com/openclaw/gogcli/tree/ec34fedc23dd9d3e90f0401144cec42e06a50783),
revision `ec34fedc23dd9d3e90f0401144cec42e06a50783`. The MIT license is in
[LICENSE](LICENSE); immutable source bytes, hashes and notices are retained in the
[catalog example](../../examples/tool-catalog-change/source-pin.json).

`ToolAllowed(GogSelectorInput)` matches selectors against raw name, service and
risk fields. It preserves Go's exact case-sensitive comparisons, the `*`/`all`
forms and service `.*` suffix form. `ToolSelected(GogSelectionFacts)` also applies
write/read-only, separate Gmail send/delete capabilities and the intersection of
configured and runtime selector lists. An unknown capability is not selected.

The represented domain is valid UTF-8 Text up to 1,024 bytes per field and at most
256 selectors per list. There is no trimming or string repair. Empty selector
lists mean no filter in this kernel; upstream configuration normalization is a
separate operation and can reject an explicitly empty configured list. The
host supplies the selected account policy and runtime facts. Account resolution,
configuration validation, baked command restrictions and tool invocation remain
outside these two functions.

The [original Gooo importing example](../../examples/tool-catalog-change/README.md)
compares the same selected policy over two source catalogs and preserves each
entry's before/after metadata and selection result.
