package tutorialchange

import (
	"embed"
	"fmt"
	"reflect"
)

// Current observations are separate from the immutable compiler839 profile.
//
//go:embed testdata/go1273/*.json
var currentProfileFiles embed.FS

type evidenceProfile struct {
	name, goVersion, compiler string
	current                   bool
	resultProfile             []byte
}

func (p evidenceProfile) record(name string) ([]byte, error) {
	if p.current {
		return currentProfileFiles.ReadFile("testdata/go1273/" + name)
	}
	return profileFiles.ReadFile("testdata/" + name)
}

// Only an exact separately supplied build matching an embedded build selects
// a profile. Every record must then match that profile and its own bindings.
func selectEvidenceProfile(raw []byte) (evidenceProfile, error) {
	var none evidenceProfile
	if len(raw) == 0 || len(raw) > MaxCompilerBuildBytes {
		return none, fmt.Errorf("compiler-build: size outside finite profile")
	}
	got, err := decode(raw)
	if err != nil {
		return none, fmt.Errorf("compiler-build: %w", err)
	}
	for _, p := range []evidenceProfile{
		{name: "learn-gooo/threshold-60-to-80/compiler839", goVersion: "go1.27.2", compiler: compilerRevision, resultProfile: resultProfileJSON},
		{name: "learn-gooo/threshold-60-to-80/go1273", goVersion: "go1.27.3", current: true},
	} {
		buildRaw, err := p.record("compiler-build.json")
		if err != nil {
			return none, fmt.Errorf("embedded compiler build: %w", err)
		}
		expected, err := decode(buildRaw)
		if err != nil {
			return none, fmt.Errorf("embedded compiler build: %w", err)
		}
		if !reflect.DeepEqual(got, expected) {
			continue
		}
		if p.current {
			p.compiler = stringAt(object(expected), "compiler_source_sha")
			p.resultProfile, err = currentProfileFiles.ReadFile("testdata/go1273/result-profile.json")
			if err != nil {
				return none, fmt.Errorf("embedded current result profile: %w", err)
			}
		}
		return p, nil
	}
	return none, fmt.Errorf("compiler-build: exact clean pinned compiler BuildInfo differs (unsupported profile)")
}
