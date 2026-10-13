// This executable changes only an in-memory reference store owned by this process.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	policy "github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy/apply"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	owner, err := policy.NewInMemoryOwner("demo-service-grant", policy.Config{
		Pattern: "*", RequiredType: "mcp", RequiredName: "weather", AllowedWriter: "demo-automation",
	})
	if err != nil {
		return err
	}
	// The embedding application already established this local subject's identity.
	writer, err := owner.WriterForAuthenticated("demo-automation")
	if err != nil {
		return err
	}
	initial, err := writer.Snapshot()
	if err != nil {
		return err
	}
	proposal := func(s policy.Snapshot, id, candidate string) policy.Proposal {
		return policy.Proposal{Epoch: s.Epoch, OperationID: id, Target: s.Target,
			ExpectedRevision: s.Revision, Candidate: candidate, GenerationSHA256: s.GenerationSHA256}
	}
	encoder := json.NewEncoder(os.Stdout)
	show := func(name string, result policy.ApplyResult) error {
		return encoder.Encode(struct {
			Case              string `json:"case"`
			Status            string `json:"status"`
			Reason            string `json:"reason"`
			ObservedRevision  uint64 `json:"observed_revision"`
			CommittedRevision uint64 `json:"committed_revision"`
		}{name, result.Status, result.Reason, result.Observed.Revision, result.CommittedRevision})
	}
	safe, err := writer.Apply(proposal(initial, "tighten-type", "mcp://*"))
	if err != nil {
		return err
	}
	if safe.Status != "APPLIED" || safe.CommittedRevision != 2 {
		return fmt.Errorf("unexpected safe transition: %+v", safe)
	}
	if err := show("safe-tightening", safe); err != nil {
		return err
	}
	stale, err := writer.Apply(proposal(initial, "stale-request", "mcp://weather"))
	if err != nil {
		return err
	}
	if stale.Status != "CONFLICT" || stale.Reason != "STALE_REVISION" {
		return fmt.Errorf("unexpected stale result: %+v", stale)
	}
	if err := show("stale-proposal", stale); err != nil {
		return err
	}
	current, err := writer.Snapshot()
	if err != nil {
		return err
	}
	wider, err := writer.Apply(proposal(current, "widen", "*"))
	if err != nil {
		return err
	}
	if wider.Status != "REFUSED" || wider.Reason != "WIDENING_WITNESS" {
		return fmt.Errorf("unexpected widening result: %+v", wider)
	}
	if err := show("widening-refused", wider); err != nil {
		return err
	}
	lost := proposal(current, "tighten-name-response-dropped", "mcp://weather")
	// Simulate discarding a completed response. Do not invent a new operation ID.
	if _, err := writer.Apply(lost); err != nil {
		return err
	}
	resolved, err := writer.Resolve(lost)
	if err != nil {
		return err
	}
	if resolved.Status != "KNOWN" || !resolved.HasResult || resolved.Result.Status != "APPLIED" || resolved.Result.CommittedRevision != 3 {
		return fmt.Errorf("unexpected resolution: %+v", resolved)
	}
	if err := show("recover-dropped-response", resolved.Result); err != nil {
		return err
	}
	final, err := writer.Snapshot()
	if err != nil {
		return err
	}
	if final.Pattern != "mcp://weather" || final.Revision != 3 {
		return fmt.Errorf("unexpected final state: %+v", final)
	}
	return nil
}
