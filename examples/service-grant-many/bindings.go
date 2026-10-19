package servicegrantmany

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
)

//go:embed generation.json
var generation []byte

type Bindings struct {
	CompilerSourceSHA string            `json:"compiler_source_sha"`
	GenerationSHA256  string            `json:"generation_sha256"`
	Sources           map[string]string `json:"sources"`
	Assets            map[string]string `json:"assets"`
}

func SourceBindings() Bindings {
	var doc struct {
		CompilerSourceSHA string            `json:"compiler_source_sha"`
		PublicSources     map[string]string `json:"public_sources"`
		Assets            map[string]string `json:"assets"`
	}
	if err := json.Unmarshal(generation, &doc); err != nil {
		panic(err)
	}
	sum := sha256.Sum256(generation)
	return Bindings{CompilerSourceSHA: doc.CompilerSourceSHA,
		GenerationSHA256: "sha256:" + hex.EncodeToString(sum[:]),
		Sources:          doc.PublicSources, Assets: doc.Assets}
}
