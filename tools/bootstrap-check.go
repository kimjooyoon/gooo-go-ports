// Validate the new executable's identity before bootstrap publishes it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const compilerModulePath = "github.com/kimjooyoon/meta-ontology-go"
const compilerModuleVersion = "v0.6.37-dev.0.20261020180505-205896a413e5"
const compilerRuntimeVersion = "v0.2.51-experimental"

type compilerPin struct {
	Revision string `json:"compiler_revision"`
	Version  string `json:"compiler_version"`
	Go       string `json:"go_version"`
}

type compilerBuild struct {
	Revision      string `json:"compiler_source_sha"`
	Version       string `json:"version"`
	Go            string `json:"go_version"`
	Native        string `json:"native_go_required"`
	Status        string `json:"source_status"`
	Modified      string `json:"vcs_modified"`
	ModuleVersion string `json:"module_version"`
	Module        *struct {
		Path        string          `json:"path"`
		Version     string          `json:"version"`
		Replacement json.RawMessage `json:"replacement"`
	} `json:"module"`
	Runtime *struct {
		Path     string `json:"module_path"`
		Version  string `json:"version"`
		Replaced *bool  `json:"replaced"`
	} `json:"decision_runtime"`
}

func validateBuild(raw []byte, pin compilerPin) error {
	var build compilerBuild
	if err := json.Unmarshal(raw, &build); err != nil {
		return fmt.Errorf("invalid compiler build identity: %w", err)
	}
	if pin.Revision == "" || pin.Version == "" || pin.Go == "" ||
		build.Revision != pin.Revision || build.Version != pin.Version ||
		build.Go != pin.Go || build.Native != pin.Go ||
		build.Status != "CLEAN_VCS" || build.Modified != "false" ||
		build.ModuleVersion != compilerModuleVersion || build.Module == nil ||
		build.Module.Path != compilerModulePath || build.Module.Version != compilerModuleVersion ||
		len(build.Module.Replacement) != 0 || build.Runtime == nil ||
		build.Runtime.Path != "github.com/kimjooyoon/gooo-decision-runtime" ||
		build.Runtime.Version != compilerRuntimeVersion || build.Runtime.Replaced == nil || *build.Runtime.Replaced {
		return fmt.Errorf("compiler build identity differs from upstream.json/current module profile; existing executable preserved")
	}
	return nil
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: bootstrap-check identity.json")
	}
	raw, err := os.ReadFile("upstream.json")
	if err != nil {
		return err
	}
	var pin compilerPin
	if err := json.Unmarshal(raw, &pin); err != nil {
		return err
	}
	raw, err = os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	return validateBuild(raw, pin)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		os.Exit(1)
	}
}
