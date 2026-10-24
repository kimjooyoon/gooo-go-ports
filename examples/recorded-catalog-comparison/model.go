// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause; see LICENSE.
// Saved observations are declarations, not authenticated identity or authority.
package recordedcatalog

import "encoding/json"

const (
	MaxInputBytes  = 1 << 20
	MaxReportBytes = 4 << 20
	MaxFrameBytes  = 64 << 10
	MaxConnections = 8
	MaxPages       = 16
	MaxEntries     = 256
	MaxTextBytes   = 1024
)

type Connection struct {
	HostID       string `json:"host_id"`
	ConnectionID string `json:"connection_id"`
}
type Context struct {
	CapturedAt                    string `json:"captured_at"`
	BackendBindingRevision        string `json:"backend_binding_revision"`
	AuthorizationContextID        string `json:"authorization_context_id"`
	RelevantConfigurationRevision string `json:"relevant_configuration_revision"`
	ProtocolVersion               string `json:"protocol_version"`
}
type Watch struct {
	Coverage               string `json:"coverage"`
	RelevantChangeObserved bool   `json:"relevant_change_observed"`
}
type Page struct {
	Context              Context  `json:"context"`
	RequestJSON          string   `json:"request_json"`
	ResponseJSON         string   `json:"response_json"`
	DeclaredExposedNames []string `json:"declared_exposed_names"`
}
type Capture struct {
	HostID                string  `json:"host_id"`
	ConnectionID          string  `json:"connection_id"`
	ServerNameClaim       string  `json:"server_name_claim"`
	ContextRecordingBasis string  `json:"context_recording_basis"`
	EvidenceKind          string  `json:"evidence_kind"`
	Begin                 Context `json:"begin"`
	End                   Context `json:"end"`
	ConfigurationWatch    Watch   `json:"configuration_watch"`
	RemoteAtomicity       string  `json:"remote_atomicity"`
	Pages                 []Page  `json:"pages"`
}
type Observation struct {
	CatalogAuthorizationContextID string       `json:"catalog_authorization_context_id"`
	DeclaredConnections           []Connection `json:"declared_connections"`
	Captures                      []Capture    `json:"captures"`
}
type Input struct {
	TargetAlias string      `json:"target_alias"`
	Before      Observation `json:"before"`
	After       Observation `json:"after"`
}
type Entry struct {
	Server       string `json:"server"`
	OriginalName string `json:"original_name"`
	ExposedName  string `json:"exposed_name"`
}
type AliasFacts struct {
	TargetAlias string  `json:"target_alias"`
	Before      []Entry `json:"before"`
	After       []Entry `json:"after"`
}

// Cursor tags preserve the distinction between absent, null and empty string.
type Cursor struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}
type PageSource struct {
	CaptureIndex   int             `json:"capture_index"`
	PageIndex      int             `json:"page_index"`
	RequestSHA256  string          `json:"request_sha256"`
	ResponseSHA256 string          `json:"response_sha256"`
	RequestID      json.RawMessage `json:"request_id"`
	ResponseID     json.RawMessage `json:"response_id"`
	RequestCursor  Cursor          `json:"request_cursor"`
	NextCursor     Cursor          `json:"next_cursor"`
}
type RowSource struct {
	CaptureIndex   int       `json:"capture_index"`
	PageIndex      int       `json:"page_index"`
	ToolIndex      int       `json:"tool_index"`
	ResponseSHA256 string    `json:"response_sha256"`
	OriginTuple    [3]string `json:"origin_tuple"`
	OriginID       string    `json:"origin_id"`
}
type Problem struct {
	State          string `json:"state"`
	Stage          string `json:"stage"`
	Code           string `json:"code"`
	Side           string `json:"side,omitempty"`
	CaptureIndex   *int   `json:"capture_index,omitempty"`
	PageIndex      *int   `json:"page_index,omitempty"`
	ResponseJSON   string `json:"response_json,omitempty"`
	RequestSHA256  string `json:"request_sha256,omitempty"`
	ResponseSHA256 string `json:"response_sha256,omitempty"`
}

func (p *Problem) Error() string { return p.State + ":" + p.Stage + ":" + p.Code }

func invalid(code string) *Problem {
	return &Problem{State: "INVALID_TRANSPORT", Stage: "decode_json", Code: code}
}
