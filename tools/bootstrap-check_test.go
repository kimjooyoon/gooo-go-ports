package main

import (
	"encoding/json"
	"testing"
)

func TestCompilerIdentityMustMatchEveryPinnedField(t *testing.T) {
	pin := compilerPin{"0123456789012345678901234567890123456789", "0.6.24-dev", "go1.27.3"}
	good := map[string]string{"compiler_source_sha": pin.Revision, "version": pin.Version,
		"go_version": pin.Go, "native_go_required": pin.Go, "source_status": "CLEAN_VCS", "vcs_modified": "false"}
	marshal := func(value any) []byte {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if err := validateBuild(marshal(good), pin); err != nil {
		t.Fatal(err)
	}
	for key, original := range good {
		for _, wrong := range []string{"", "different"} {
			good[key] = wrong
			if err := validateBuild(marshal(good), pin); err == nil {
				t.Fatalf("accepted changed %s", key)
			}
		}
		delete(good, key)
		if err := validateBuild(marshal(good), pin); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
		good[key] = original
	}
	for _, raw := range [][]byte{nil, []byte("null"), []byte("{}"), []byte("not json")} {
		if err := validateBuild(raw, pin); err == nil {
			t.Fatalf("accepted invalid identity %q", raw)
		}
	}
	if err := validateBuild(marshal(good), compilerPin{}); err == nil {
		t.Fatal("accepted incomplete pin")
	}
}
