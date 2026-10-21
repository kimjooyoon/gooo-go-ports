package main

import (
	"encoding/json"
	"testing"
)

func TestCompilerIdentityMustMatchEveryPinnedField(t *testing.T) {
	pin := compilerPin{"0123456789012345678901234567890123456789", "0.6.24-dev", "go1.27.3"}
	good := map[string]any{"compiler_source_sha": pin.Revision, "version": pin.Version,
		"go_version": pin.Go, "native_go_required": pin.Go, "source_status": "CLEAN_VCS", "vcs_modified": "false",
		"module_version":   compilerModuleVersion,
		"module":           map[string]any{"path": compilerModulePath, "version": compilerModuleVersion},
		"decision_runtime": map[string]any{"module_path": "github.com/kimjooyoon/gooo-decision-runtime", "version": compilerRuntimeVersion, "replaced": false}}
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
	for _, name := range []string{"module", "decision_runtime"} {
		object := good[name].(map[string]any)
		for key, original := range object {
			for _, wrong := range []any{nil, "different", true} {
				object[key] = wrong
				if err := validateBuild(marshal(good), pin); err == nil {
					t.Fatalf("accepted changed %s.%s", name, key)
				}
			}
			delete(object, key)
			if err := validateBuild(marshal(good), pin); err == nil {
				t.Fatalf("accepted missing %s.%s", name, key)
			}
			object[key] = original
		}
	}
	for _, replacement := range []any{nil, map[string]any{}, "local"} {
		good["module"].(map[string]any)["replacement"] = replacement
		if err := validateBuild(marshal(good), pin); err == nil {
			t.Fatal("accepted replaced module")
		}
	}
	delete(good["module"].(map[string]any), "replacement")
	for _, raw := range [][]byte{nil, []byte("null"), []byte("{}"), []byte("not json")} {
		if err := validateBuild(raw, pin); err == nil {
			t.Fatalf("accepted invalid identity %q", raw)
		}
	}
	if err := validateBuild(marshal(good), compilerPin{}); err == nil {
		t.Fatal("accepted incomplete pin")
	}
}
