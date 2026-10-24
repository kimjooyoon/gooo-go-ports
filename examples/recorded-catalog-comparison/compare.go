// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
package recordedcatalog

import (
	_ "embed"
	"encoding/json"
	"reflect"

	"github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison/internal/kernel"
)

type Projection struct {
	Facts       AliasFacts   `json:"facts"`
	FactsSHA256 string       `json:"facts_sha256"`
	BeforePages []PageSource `json:"before_pages"`
	AfterPages  []PageSource `json:"after_pages"`
	BeforeRows  []RowSource  `json:"before_rows"`
	AfterRows   []RowSource  `json:"after_rows"`
}
type Report struct {
	Facts             Input                     `json:"facts"`
	InputSHA256       string                    `json:"input_sha256"`
	FactsSHA256       string                    `json:"facts_sha256"`
	Projection        Projection                `json:"projection"`
	AliasDecision     kernel.AliasDecision      `json:"alias_decision"`
	CollectionContext kernel.CollectionDecision `json:"collection_context"`
	Bindings          json.RawMessage           `json:"bindings"`
}

//go:embed generation.json
var generation []byte

// Empty parallel columns are real arrays even for an explicitly empty scope.
func emptyColumns[T any]() T {
	var result T
	v := reflect.ValueOf(&result).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Slice {
			f.Set(reflect.MakeSlice(f.Type(), 0, 0))
		}
	}
	return result
}

func columns(facts Input, before, after projectedObservation, factsHash string) (kernel.OriginInput, kernel.CollectionInput, Projection, error) {
	projected := AliasFacts{TargetAlias: facts.TargetAlias, Before: before.Entries, After: after.Entries}
	raw, err := json.Marshal(projected)
	if err != nil {
		return kernel.OriginInput{}, kernel.CollectionInput{}, Projection{}, err
	}
	aliasHash := digest(raw)
	origin := emptyColumns[kernel.OriginInput]()
	origin.ContextID = "gooo/alias-origin/v1/" + aliasHash
	origin.TargetAlias = facts.TargetAlias
	for _, e := range before.Entries {
		origin.BeforeServers = append(origin.BeforeServers, e.Server)
		origin.BeforeNames = append(origin.BeforeNames, e.OriginalName)
		origin.BeforeAliases = append(origin.BeforeAliases, e.ExposedName)
	}
	for _, e := range after.Entries {
		origin.AfterServers = append(origin.AfterServers, e.Server)
		origin.AfterNames = append(origin.AfterNames, e.OriginalName)
		origin.AfterAliases = append(origin.AfterAliases, e.ExposedName)
	}
	context := emptyColumns[kernel.CollectionInput]()
	context.AliasContextID = origin.ContextID
	bound, _ := json.Marshal([2]string{factsHash, aliasHash})
	context.ContextID = "gooo/recorded-catalog/v1/" + digest(bound)
	context.BeforeCatalogAuthorizationContextID = facts.Before.CatalogAuthorizationContextID
	context.AfterCatalogAuthorizationContextID = facts.After.CatalogAuthorizationContextID
	for si, o := range []Observation{facts.Before, facts.After} {
		side := []string{"BEFORE", "AFTER"}[si]
		decoded := []projectedObservation{before, after}[si]
		for _, d := range o.DeclaredConnections {
			context.DeclaredSides = append(context.DeclaredSides, side)
			context.DeclaredHostIDs = append(context.DeclaredHostIDs, d.HostID)
			context.DeclaredConnectionIDs = append(context.DeclaredConnectionIDs, d.ConnectionID)
		}
		for ci, c := range o.Captures {
			globalIndex := int64(len(context.CaptureSides))
			context.CaptureSides = append(context.CaptureSides, side)
			context.CaptureHostIDs = append(context.CaptureHostIDs, c.HostID)
			context.CaptureConnectionIDs = append(context.CaptureConnectionIDs, c.ConnectionID)
			context.CaptureRecordingBases = append(context.CaptureRecordingBases, c.ContextRecordingBasis)
			context.CaptureEvidenceKinds = append(context.CaptureEvidenceKinds, c.EvidenceKind)
			context.CaptureWatchCoverages = append(context.CaptureWatchCoverages, c.ConfigurationWatch.Coverage)
			context.CaptureRemoteAtomicities = append(context.CaptureRemoteAtomicities, c.RemoteAtomicity)
			context.CaptureBeginBindings = append(context.CaptureBeginBindings, c.Begin.BackendBindingRevision)
			context.CaptureEndBindings = append(context.CaptureEndBindings, c.End.BackendBindingRevision)
			context.CaptureBeginAuthContexts = append(context.CaptureBeginAuthContexts, c.Begin.AuthorizationContextID)
			context.CaptureEndAuthContexts = append(context.CaptureEndAuthContexts, c.End.AuthorizationContextID)
			context.CaptureBeginConfigRevisions = append(context.CaptureBeginConfigRevisions, c.Begin.RelevantConfigurationRevision)
			context.CaptureEndConfigRevisions = append(context.CaptureEndConfigRevisions, c.End.RelevantConfigurationRevision)
			context.CaptureBeginProtocols = append(context.CaptureBeginProtocols, c.Begin.ProtocolVersion)
			context.CaptureEndProtocols = append(context.CaptureEndProtocols, c.End.ProtocolVersion)
			context.CaptureRelevantChangeObserved = append(context.CaptureRelevantChangeObserved, c.ConfigurationWatch.RelevantChangeObserved)
			for pi, p := range c.Pages {
				page := decoded.Decoded[ci][pi]
				context.PageCaptureIndexes = append(context.PageCaptureIndexes, globalIndex)
				context.PageRequestCursorKinds = append(context.PageRequestCursorKinds, page.Source.RequestCursor.Tag)
				context.PageRequestCursorValues = append(context.PageRequestCursorValues, page.Source.RequestCursor.Value)
				context.PageNextCursorKinds = append(context.PageNextCursorKinds, page.Source.NextCursor.Tag)
				context.PageNextCursorValues = append(context.PageNextCursorValues, page.Source.NextCursor.Value)
				context.PageBindings = append(context.PageBindings, p.Context.BackendBindingRevision)
				context.PageAuthContexts = append(context.PageAuthContexts, p.Context.AuthorizationContextID)
				context.PageConfigRevisions = append(context.PageConfigRevisions, p.Context.RelevantConfigurationRevision)
				context.PageProtocols = append(context.PageProtocols, p.Context.ProtocolVersion)
			}
		}
	}
	return origin, context, Projection{Facts: projected, FactsSHA256: aliasHash, BeforePages: before.Pages, AfterPages: after.Pages, BeforeRows: before.Rows, AfterRows: after.Rows}, nil
}

// CompareJSON interprets only supplied saved observations. The composed Gooo
// activity owns every alias, pagination, configuration and authorization-scope
// decision. The importer performs no live collection or effects.
func CompareJSON(raw []byte) (report Report, returned error) {
	defer func() {
		if recover() != nil {
			report = Report{}
			returned = &Problem{State: "UNKNOWN", Stage: "evaluate", Code: "KERNEL_FAILURE"}
		}
	}()
	facts, err := decodeFacts(raw)
	if err != nil {
		return Report{}, err
	}
	before, err := projectObservation("BEFORE", facts.Before)
	if err != nil {
		return Report{}, err
	}
	after, err := projectObservation("AFTER", facts.After)
	if err != nil {
		return Report{}, err
	}
	encoded, err := json.Marshal(facts)
	if err != nil {
		return Report{}, err
	}
	factsHash := digest(encoded)
	origin, context, projection, err := columns(facts, before, after, factsHash)
	if err != nil {
		return Report{}, err
	}
	decision := kernel.Evaluate(origin, context)
	if decision.Alias.ContextID != origin.ContextID || decision.Collection.ContextID != context.ContextID || decision.Collection.AliasContextID != origin.ContextID || decision.Alias.ExecutionAuthorized || decision.Collection.ExecutionAuthorized || decision.Collection.LiveRemovalProven || decision.Collection.RemoteAtomicity != "UNKNOWN" {
		return Report{}, &Problem{State: "UNKNOWN", Stage: "bind_result", Code: "KERNEL_CONTEXT_OR_AUTHORITY_MISMATCH"}
	}
	report = Report{Facts: facts, InputSHA256: digest(raw), FactsSHA256: factsHash, Projection: projection, AliasDecision: decision.Alias, CollectionContext: decision.Collection, Bindings: append(json.RawMessage(nil), generation...)}
	encoded, err = json.Marshal(report)
	if err != nil {
		return Report{}, err
	}
	if len(encoded) > MaxReportBytes {
		return Report{}, &Problem{State: "INVALID_TRANSPORT", Stage: "encode_result", Code: "OUTPUT_LIMIT_EXCEEDED"}
	}
	return report, nil
}
