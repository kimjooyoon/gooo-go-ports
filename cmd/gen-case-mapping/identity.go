package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
)

const pinnedGo = "go1.27.2"
const pinnedUnicode = "17.0.0"
const pinnedGoRevision = "022c8636110ebac86a9b88724cda168ed713c21f"
const pinnedCompiler = "83928fa4538245abe33b08549da47f8cbdc50ed2"

var requiredSources = []string{"src/unicode/letter.go", "src/unicode/tables.go", "LICENSE"}

type upstream struct {
	Schema           string            `json:"schema"`
	GoVersion        string            `json:"go_version"`
	GoRepository     string            `json:"go_repository"`
	GoRevision       string            `json:"go_revision"`
	CompilerRevision string            `json:"compiler_revision"`
	SourceSHA        map[string]string `json:"source_sha256"`
	Ports            []portPin         `json:"ports"`
}

type portPin struct {
	Target         string   `json:"target"`
	Symbols        []string `json:"symbols"`
	UnicodeVersion string   `json:"unicode_version"`
}

func verifyIdentity(pin upstream, version, unicodeVersion, goroot string) error {
	if pin.Schema != "gooo/go-ports-upstream/v1" || pin.GoRepository != "https://github.com/golang/go" || pin.GoRevision != pinnedGoRevision || pin.CompilerRevision != pinnedCompiler {
		return fmt.Errorf("generator requires the pinned Go revision, compiler and upstream schema")
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("case mapping requires a 64-bit Go int domain")
	}
	if pin.GoVersion != pinnedGo || version != pinnedGo || unicodeVersion != pinnedUnicode {
		return fmt.Errorf("generator requires %s and Unicode %s; got %s and %s", pinnedGo, pinnedUnicode, version, unicodeVersion)
	}
	matched := 0
	for _, port := range pin.Ports {
		if port.Target != targetSource {
			continue
		}
		matched++
		if port.UnicodeVersion != pinnedUnicode || !reflect.DeepEqual(port.Symbols, []string{"To", "ToUpper", "ToLower", "ToTitle"}) {
			return fmt.Errorf("incorrect case mapping or Unicode pin for %s", targetSource)
		}
	}
	if matched != 1 {
		return fmt.Errorf("upstream.json must pin case mapping exactly once for %s", targetSource)
	}
	for _, name := range requiredSources {
		want := pin.SourceSHA[name]
		if len(want) != 64 {
			return fmt.Errorf("missing pinned Go source hash: %s", name)
		}
		raw, err := os.ReadFile(filepath.Join(goroot, name))
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != want {
			return fmt.Errorf("Go upstream source hash differs: %s", name)
		}
	}
	return nil
}
