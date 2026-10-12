// Copyright (c) 2026 Gooo Go Ports contributors.
// This source-binding helper belongs to Gooo Go Ports, not google/uuid v1.6.0.
// It does not change UUID behavior. Original upstream source files stay intact.
// The repository BSD-3-Clause license applies to this added helper.

package uuid

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

//go:embed uuid.go
var goooPinnedUUIDSource []byte

//go:embed util.go
var goooPinnedUtilSource []byte

//go:embed LICENSE
var goooPinnedLicense []byte

// PinnedSourceSHA256 returns the original source digests embedded into this
// compiled oracle package. Its only keys are uuid.go, util.go and LICENSE.
// The caller must compare these with independently reviewed pinned digests.
func PinnedSourceSHA256() map[string]string {
	return map[string]string{
		"uuid.go": goooPinnedSourceSHA256(goooPinnedUUIDSource),
		"util.go": goooPinnedSourceSHA256(goooPinnedUtilSource),
		"LICENSE": goooPinnedSourceSHA256(goooPinnedLicense),
	}
}

func goooPinnedSourceSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
