package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

const pinnedGo = "go1.27.2"
const pinnedUnicode = "17.0.0"
const pinnedGoRevision = "022c8636110ebac86a9b88724cda168ed713c21f"

var requiredSources = []string{
	"src/unicode/graphic.go", "src/unicode/digit.go", "src/unicode/letter.go", "src/unicode/tables.go", "LICENSE",
}

type upstream struct {
	Schema       string            `json:"schema"`
	GoVersion    string            `json:"go_version"`
	GoRepository string            `json:"go_repository"`
	GoRevision   string            `json:"go_revision"`
	SourceSHA    map[string]string `json:"source_sha256"`
	Ports        []portPin         `json:"ports"`
}

type portPin struct {
	Target         string   `json:"target"`
	Symbols        []string `json:"symbols"`
	UnicodeVersion string   `json:"unicode_version"`
}

func verifyIdentity(pin upstream, version, unicodeVersion, goroot string) error {
	if pin.Schema != "gooo/go-ports-upstream/v1" || pin.GoRepository != "https://github.com/golang/go" ||
		pin.GoRevision != pinnedGoRevision {
		return fmt.Errorf("generator requires the pinned Go source repository, schema and revision")
	}
	if pin.GoVersion != pinnedGo || version != pin.GoVersion {
		return fmt.Errorf("generator toolchain %s, require pinned %s", version, pinnedGo)
	}
	if unicodeVersion != pinnedUnicode {
		return fmt.Errorf("generator Unicode version %s, require %s", unicodeVersion, pinnedUnicode)
	}
	symbols := map[string]bool{}
	for _, port := range pin.Ports {
		if port.Target != targetSource {
			continue
		}
		if port.UnicodeVersion != pinnedUnicode {
			return fmt.Errorf("missing or incorrect Unicode version for %s", targetSource)
		}
		for _, symbol := range port.Symbols {
			symbols[symbol] = true
		}
	}
	if !symbols["IsLetter"] || !symbols["IsDigit"] {
		return fmt.Errorf("upstream.json must pin IsLetter and IsDigit for %s", targetSource)
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
