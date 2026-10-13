// Package apply automatically applies eligible changes to an in-memory reference store.
// It does not authenticate remote callers or persist state across a restart.
package apply

import "errors"

const (
	ApplySchema        = "gooo/conditional-apply-host-result/v1"
	ResolutionSchema   = "gooo/conditional-apply-host-resolution/v1"
	MaxOperations      = 1024
	MaxIdentityBytes   = 128
	MaxPolicyBytes     = 1024
	MaxGenerationBytes = 128
)

var (
	ErrInvalidHandle     = errors.New("invalid conditional-apply handle")
	ErrUnauthorized      = errors.New("writer is not currently authorized")
	ErrInvalidEnvelope   = errors.New("invalid bounded UTF-8 operation envelope")
	ErrRevisionConflict  = errors.New("owner update revision conflict")
	ErrRevisionExhausted = errors.New("store revision exhausted")
	ErrInvalidGeneration = errors.New("invalid local source generation")
)

// Epoch distinguishes store incarnations; possession of it grants no authority.
type Epoch [16]byte

// Config is trusted owner input. Policy text remains exact and is interpreted
// only by Gooo during Apply. Administrative changes always advance the revision.
type Config struct {
	Pattern       string `json:"pattern"`
	RequiredType  string `json:"required_type"`
	RequiredName  string `json:"required_name"`
	AllowedWriter string `json:"allowed_writer"`
}

type Snapshot struct {
	Epoch            Epoch  `json:"epoch"`
	Target           string `json:"target"`
	Revision         uint64 `json:"revision"`
	Pattern          string `json:"pattern"`
	RequiredType     string `json:"required_type"`
	RequiredName     string `json:"required_name"`
	GenerationSHA256 string `json:"generation_sha256"`
}

// Proposal contains no authoritative old policy, configuration, or proof.
// Its operation identity is scoped to Epoch and bound to every exact field.
type Proposal struct {
	Epoch            Epoch  `json:"epoch"`
	OperationID      string `json:"operation_id"`
	Target           string `json:"target"`
	ExpectedRevision uint64 `json:"expected_revision"`
	Candidate        string `json:"candidate"`
	GenerationSHA256 string `json:"generation_sha256"`
}

// ApplyResult is a value copy. Observed is the authorized snapshot used by this
// attempt, not a promise that the current state still equals it. HasDecision
// distinguishes a Gooo result from zero-value sentinels in an envelope conflict.
// REFUSED and CONFLICT describe this attempt, not some earlier different attempt.
type ApplyResult struct {
	Schema            string   `json:"schema"`
	Status            string   `json:"status"`
	Reason            string   `json:"reason"`
	Replay            bool     `json:"replay"`
	Recorded          bool     `json:"recorded"`
	Subject           string   `json:"subject"`
	Proposal          Proposal `json:"proposal"`
	Observed          Snapshot `json:"observed"`
	HasDecision       bool     `json:"has_decision"`
	Decision          Decision `json:"decision"`
	CommittedRevision uint64   `json:"committed_revision"`
}

// Resolution never evaluates policy or changes state. NOT_OBSERVED_AT_THIS_LOOKUP
// does not prove a concurrent operation cannot commit later. UNKNOWN_OUTCOME
// cannot settle an operation from another store incarnation.
type Resolution struct {
	Schema    string      `json:"schema"`
	Status    string      `json:"status"`
	Reason    string      `json:"reason"`
	Subject   string      `json:"subject"`
	Proposal  Proposal    `json:"proposal"`
	Observed  Snapshot    `json:"observed"`
	HasResult bool        `json:"has_result"`
	Result    ApplyResult `json:"result"`
}
