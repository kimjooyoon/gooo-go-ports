// host demonstrates caller-owned private persistence and historical replay.
// The synthetic facts are not an authentication provider or a real API call.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	nativepolicy "github.com/kimjooyoon/gooo-go-ports/examples/cel-api-policy"
)

type application struct {
	current       nativepolicy.APIFacts
	providerCalls int
	effectCalls   int
	state         int
}

// These instrumented host operations deliberately are not passed to replay.
func (a *application) currentFacts() nativepolicy.APIFacts { a.providerCalls++; return a.current }
func (a *application) dispatch()                           { a.effectCalls++; a.state++ }

type hostReport struct {
	Status        string                  `json:"status"`
	Historical    *nativepolicy.APIReplay `json:"historical,omitempty"`
	Bindings      nativepolicy.Bindings   `json:"bindings"`
	ProviderCalls int                     `json:"current_provider_calls"`
	EffectCalls   int                     `json:"effect_calls"`
	StateBefore   int                     `json:"state_before"`
	StateAfter    int                     `json:"state_after"`
}

func main() {
	mode := flag.String("mode", "", "save or verify")
	path := flag.String("proof", "", "new proof in caller-owned private directory, or existing private proof")
	decision := flag.String("decision", "allow", "save synthetic allow or deny observation")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(1)
	}
	if err := run(*mode, *path, *decision, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func syntheticFacts(allowed bool) nativepolicy.APIFacts {
	group := "reader"
	if allowed {
		group = "admin"
	}
	return nativepolicy.APIFacts{
		Facts:         nativepolicy.Facts{GroupPresent: true, GroupValue: group, PrincipalPresent: true, PrincipalValue: "synthetic:user"},
		Authenticated: true, CallerTenant: "example-tenant", ResourceTenant: "example-tenant", RequestedAction: "read", AllowedAction: "read",
	}
}

func run(mode, path, decision string, output io.Writer) error {
	if path == "" || (mode != "save" && mode != "verify") || (decision != "allow" && decision != "deny") {
		return fmt.Errorf("expected save/verify, a private proof path and allow/deny demo")
	}
	app := &application{current: syntheticFacts(false), state: 42}
	if mode == "verify" {
		raw, err := readPrivateProof(path)
		if err != nil {
			return err
		}
		report, err := verifyHistory(raw, app)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(report)
	}
	proof, err := nativepolicy.EvaluateAPI(syntheticFacts(decision == "allow"))
	if err != nil {
		return err
	}
	raw, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	if err := saveNewProof(path, append(raw, '\n')); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(hostReport{Status: "SAVED_PRIVATE_SYNTHETIC_PROOF", Bindings: proof.Bindings, StateBefore: app.state, StateAfter: app.state})
}

func verifyHistory(raw []byte, app *application) (hostReport, error) {
	before := *app
	proof, err := nativepolicy.DecodeAPIProof(raw)
	if err != nil {
		return hostReport{}, err
	}
	history, err := nativepolicy.VerifyAPIProof(proof)
	if err != nil {
		return hostReport{}, err
	}
	if app.current != before.current || app.state != before.state || app.providerCalls != before.providerCalls || app.effectCalls != before.effectCalls {
		return hostReport{}, fmt.Errorf("historical replay changed host state or invoked host operations")
	}
	return hostReport{Status: "VERIFIED_HISTORY_ONLY", Historical: &history, Bindings: history.Bindings, ProviderCalls: app.providerCalls, EffectCalls: app.effectCalls, StateBefore: before.state, StateAfter: app.state}, nil
}

func saveNewProof(path string, raw []byte) (err error) {
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("proof parent must be a caller-owned private directory")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		file.Close()
		if !complete {
			os.Remove(path)
		}
	}()
	if _, err = file.Write(raw); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

func readPrivateProof(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("proof must be a private regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, opened) || opened.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("proof changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, nativepolicy.MaxBatchBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > nativepolicy.MaxBatchBytes {
		return nil, fmt.Errorf("proof exceeds 4 MiB")
	}
	return raw, nil
}
