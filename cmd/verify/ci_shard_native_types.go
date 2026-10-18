package main

import (
	"fmt"
	"path/filepath"
)

// Experimental native work uses the same oracle suites and selected workspace
// bytes as the existing full verifier. It has separate partial output schemas.
type ciShardNativeInventory struct {
	Suites     []suite
	Lock       upstream
	Catalog    frozenVerificationCatalog
	Workspaces []suiteWorkspace
}

func ciShardSuiteDirectory(root string, shard, original int) string {
	return filepath.Join(root, ciShardNamespace(shard), fmt.Sprintf("suite-%04d", original))
}

func ciShardBatchDirectory(root string, shard, original, batch int) string {
	return filepath.Join(root, filepath.FromSlash(ciShardBatchNamespace(shard, original, batch)))
}
