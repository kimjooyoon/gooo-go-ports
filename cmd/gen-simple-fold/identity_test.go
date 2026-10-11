package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func identityFixture(t *testing.T) (upstream, string) {
	t.Helper()
	root := t.TempDir()
	pin := upstream{Schema: "gooo/go-ports-upstream/v1", GoVersion: pinnedGo,
		GoRepository: "https://github.com/golang/go", GoRevision: pinnedGoRevision,
		CompilerRevision: pinnedCompiler, SourceSHA: map[string]string{},
		Ports: []portPin{{Target: targetSource, Symbols: []string{"SimpleFold"}, UnicodeVersion: pinnedUnicode}}}
	for _, name := range requiredSources {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		raw := []byte("identity fixture: " + name)
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
		pin.SourceSHA[name] = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return pin, root
}

func TestIdentityRejectsWrongPinsAndChangedSources(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*upstream, *string, *string, string)
	}{
		{"Go pin", func(p *upstream, _, _ *string, _ string) { p.GoVersion = "go1.27.1" }},
		{"running Go", func(_ *upstream, v, _ *string, _ string) { *v = "go1.27.1" }},
		{"Go repository", func(p *upstream, _, _ *string, _ string) { p.GoRepository = "other" }},
		{"Go revision", func(p *upstream, _, _ *string, _ string) { p.GoRevision = strings.Repeat("0", 40) }},
		{"compiler pin", func(p *upstream, _, _ *string, _ string) { p.CompilerRevision = strings.Repeat("0", 40) }},
		{"running Unicode", func(_ *upstream, _, v *string, _ string) { *v = "16.0.0" }},
		{"pinned Unicode", func(p *upstream, _, _ *string, _ string) { p.Ports[0].UnicodeVersion = "16.0.0" }},
		{"symbol", func(p *upstream, _, _ *string, _ string) { p.Ports[0].Symbols = []string{"ToLower"} }},
		{"duplicate target", func(p *upstream, _, _ *string, _ string) { p.Ports = append(p.Ports, p.Ports[0]) }},
		{"missing target", func(p *upstream, _, _ *string, _ string) { p.Ports = nil }},
		{"missing letter hash", func(p *upstream, _, _ *string, _ string) { delete(p.SourceSHA, "src/unicode/letter.go") }},
		{"table hash", func(p *upstream, _, _ *string, _ string) {
			p.SourceSHA["src/unicode/tables.go"] = strings.Repeat("0", 64)
		}},
		{"changed source", func(_ *upstream, _, _ *string, root string) {
			if err := os.WriteFile(filepath.Join(root, "src/unicode/letter.go"), []byte("changed"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			pin, root := identityFixture(t)
			version, unicodeVersion := pinnedGo, pinnedUnicode
			if err := verifyIdentity(pin, version, unicodeVersion, root); err != nil {
				t.Fatal(err)
			}
			test.change(&pin, &version, &unicodeVersion, root)
			if err := verifyIdentity(pin, version, unicodeVersion, root); err == nil {
				t.Fatal("invalid source identity accepted")
			}
		})
	}
}
