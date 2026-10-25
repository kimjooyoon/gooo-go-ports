// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../../LICENSE.
package main

import (
	"encoding/json"
	"errors"
	origin "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
	"io"
	"os"
	"runtime/debug"
)

const usage = `Compare saved MCP tool catalogs using rules written in Gooo and compiled to Go.

Usage:
  compare < saved-observations.json > report.json
  compare --definition-snapshot < saved-definition-pages.json
  compare --definitions --before-policy REV --after-policy REV < saved-observations.json
  compare --definitions --diagnostics < saved-observations.json
  compare --availability-template < saved-observations.json > availability.json
  compare --definitions --diagnostics --availability availability.json < saved-observations.json
  compare --example > synthetic-observations.json
  compare --version
  compare --help

With no arguments, read one JSON comparison from stdin (at most 1 MiB).
--example prints the complete synthetic input; compare it in a separate call.
--version prints JSON build metadata, not an authenticated source identity.
--help (-h) prints this text. These options never read stdin or compare inputs.
Use one display option at a time; unknown arguments are refused before reading stdin.
--definition-snapshot compares one original tool name using saved pages alone.
Its separate input has target_name and before/after pages of request_json and
response_json; no host mapping or policy flags are accepted. Reports retain
exact supplied input and row/page references. Incomplete page observations
remain explicit, and origin and authorization are NOT_ASSESSED.
--definitions compares definition and annotation-claim bindings as well as origin.
Policy revisions are optional supplied text; omitted revisions remain unobserved.
--diagnostics immediately after --definitions adds field observation diagnostics.
Keep the original saved input with the report; diagnostics do not repair missing facts.
--availability-template emits a complete source-bound roster with blank states.
Choose SUPPLIED or UNOBSERVED for each field using the retained evidence, then
pass that file with --definitions --diagnostics --availability. This separate
versioned report uses explicit availability; raw text, including UNKNOWN, stays
unchanged. Use identical policy arguments for the template and comparison.
The saved input and availability file together must be at most 1 MiB.

Successful comparison: exit 0, full JSON report on stdout.
Invalid arguments: exit 2. Input, comparison or I/O failure: exit 1.
Failures produce a JSON diagnostic on stderr. Preserve the first diagnostic.

The example marks its observations SYNTHETIC_FIXTURE. A collection may be
incomplete or unknown even when an alias can be compared. Remote atomicity
remains UNKNOWN; the report does not authorize execution or prove live removal.
This command does not collect a live catalog, run tools, or call a model.

Input format and bounds:
https://github.com/kimjooyoon/gooo-go-ports/tree/main/examples/recorded-catalog-comparison
Gooo language:
https://github.com/kimjooyoon/meta-ontology-go
`

type commandVersion struct {
	Schema             string            `json:"schema"`
	BuildInfoAvailable bool              `json:"build_info_available"`
	GoVersion          string            `json:"go_version"`
	PackagePath        string            `json:"package_path"`
	MainModule         *debug.Module     `json:"main_module"`
	VCS                map[string]string `json:"vcs"`
}

func versionInfo(info *debug.BuildInfo, ok bool) commandVersion {
	v := commandVersion{Schema: "gooo/recorded-catalog-command-build/v1", VCS: map[string]string{}}
	if !ok || info == nil {
		return v
	}
	v.BuildInfoAvailable = true
	v.GoVersion, v.PackagePath = info.GoVersion, info.Path
	v.MainModule = &info.Main
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision", "vcs.time", "vcs.modified":
			v.VCS[setting.Key] = setting.Value
		}
	}
	return v
}

func diagnostic(out io.Writer, err error) int {
	var problem *origin.Problem
	if !errors.As(err, &problem) {
		problem = &origin.Problem{State: "UNKNOWN", Stage: "command", Code: "IO_FAILURE"}
	}
	_ = json.NewEncoder(out).Encode(problem)
	return 1
}

func emit(raw []byte, out, diagnostics io.Writer) int {
	n, err := out.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	return 0
}

func runCommand(args []string, input io.Reader, out, diagnostics io.Writer) int {
	if len(args) == 0 {
		return run(input, out, diagnostics)
	}
	if args[0] == "--availability-template" {
		values, ok := policyAndAvailabilityArguments(args[1:], false)
		if !ok {
			return invalidArguments(diagnostics)
		}
		return runAvailabilityTemplate(input, out, diagnostics, values["--before-policy"], values["--after-policy"])
	}
	if args[0] == "--definitions" {
		remaining := args[1:]
		withDiagnostics := len(remaining) > 0 && remaining[0] == "--diagnostics"
		if withDiagnostics {
			remaining = remaining[1:]
		}
		values, ok := policyAndAvailabilityArguments(remaining, withDiagnostics)
		if !ok {
			return invalidArguments(diagnostics)
		}
		if path, explicit := values["--availability"]; explicit {
			return runExplicitAvailability(input, out, diagnostics, path, values["--before-policy"], values["--after-policy"])
		}
		if withDiagnostics {
			return runDefinitionDiagnostics(input, out, diagnostics, values["--before-policy"], values["--after-policy"])
		}
		return runDefinitions(input, out, diagnostics, values["--before-policy"], values["--after-policy"])
	}
	if len(args) == 1 {
		switch args[0] {
		case "--definition-snapshot":
			return runDefinitionSnapshot(input, out, diagnostics)
		case "--help", "-h":
			return emit([]byte(usage), out, diagnostics)
		case "--example":
			return emit(origin.ExampleInputJSON(), out, diagnostics)
		case "--version":
			info, ok := debug.ReadBuildInfo()
			raw, err := json.Marshal(versionInfo(info, ok))
			if err != nil {
				return diagnostic(diagnostics, err)
			}
			return emit(append(raw, '\n'), out, diagnostics)
		}
	}
	return invalidArguments(diagnostics)
}

func invalidArguments(diagnostics io.Writer) int {
	diagnostic(diagnostics, &origin.Problem{State: "INVALID_ARGUMENT", Stage: "parse_arguments", Code: "UNSUPPORTED_ARGUMENTS"})
	return 2
}

func runDefinitionDiagnostics(input io.Reader, out, diagnostics io.Writer, beforePolicy, afterPolicy string) int {
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	report, err := origin.CompareDefinitionDiagnosticsJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}

func runDefinitionSnapshot(input io.Reader, out, diagnostics io.Writer) int {
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	report, err := origin.CompareDefinitionSnapshotJSON(raw)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	if len(encoded) > origin.MaxReportBytes {
		return diagnostic(diagnostics, &origin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}

func runDefinitions(input io.Reader, out, diagnostics io.Writer, beforePolicy, afterPolicy string) int {
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	report, err := origin.CompareDefinitionJSON(raw, beforePolicy, afterPolicy)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return diagnostic(diagnostics, err)
	}
	if len(encoded) > origin.MaxReportBytes {
		return diagnostic(diagnostics, &origin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}

func run(input io.Reader, out, diagnostics io.Writer) int {
	fail := func(err error) int {
		return diagnostic(diagnostics, err)
	}
	raw, err := io.ReadAll(io.LimitReader(input, origin.MaxInputBytes+1))
	if err != nil {
		return fail(err)
	}
	report, err := origin.CompareJSON(raw)
	if err != nil {
		return fail(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return fail(err)
	}
	if len(encoded) > origin.MaxReportBytes {
		return fail(&origin.Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"})
	}
	return emit(append(encoded, '\n'), out, diagnostics)
}
func main() { os.Exit(runCommand(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
