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
		SourceSHA: map[string]string{}, Ports: []portPin{{Target: targetSource,
			Symbols: []string{"IsLetter", "IsDigit"}, UnicodeVersion: pinnedUnicode}}}
	for _, name := range requiredSources {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		raw := []byte("identity test fixture " + name)
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
		pin.SourceSHA[name] = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return pin, root
}

func TestIdentityRejectsWrongPinsAndChangedSource(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*upstream, *string, *string, string)
	}{
		{"pin toolchain", func(p *upstream, _, _ *string, _ string) { p.GoVersion = "go1.27.1" }},
		{"running toolchain", func(_ *upstream, v, _ *string, _ string) { *v = "go1.27.1" }},
		{"source revision", func(p *upstream, _, _ *string, _ string) { p.GoRevision = strings.Repeat("0", 40) }},
		{"running Unicode", func(_ *upstream, _, v *string, _ string) { *v = "16.0.0" }},
		{"pinned Unicode", func(p *upstream, _, _ *string, _ string) { p.Ports[0].UnicodeVersion = "16.0.0" }},
		{"missing category", func(p *upstream, _, _ *string, _ string) { p.Ports[0].Symbols = []string{"IsLetter"} }},
		{"missing digit hash", func(p *upstream, _, _ *string, _ string) { delete(p.SourceSHA, "src/unicode/digit.go") }},
		{"wrong table hash", func(p *upstream, _, _ *string, _ string) {
			p.SourceSHA["src/unicode/tables.go"] = strings.Repeat("0", 64)
		}},
		{"changed table source", func(_ *upstream, _, _ *string, root string) {
			if err := os.WriteFile(filepath.Join(root, "src/unicode/tables.go"), []byte("changed"), 0644); err != nil {
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
				t.Fatal("invalid identity accepted")
			}
		})
	}
}
