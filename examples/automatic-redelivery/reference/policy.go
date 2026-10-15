// Independently authored from frozen contract only; no Gooo candidate was read.
package main

import "fmt"

const ContractSHA256 = "e9dadc2b2bfe776648b216477ffd9cfe7b23ee0deaf2b22bd3ab27c268acdac0"

type Input struct {
	Method                       string `json:"method"`
	DeliveryState                string `json:"delivery_state"`
	BodyReplayable               bool   `json:"body_replayable"`
	IdempotencyKeyPresent        bool   `json:"idempotency_key_present"`
	ServerRepeatContractVerified bool   `json:"server_repeat_contract_verified"`
	RequestBindingCurrent        bool   `json:"request_binding_current"`
	CurrentAccessAllowed         bool   `json:"current_access_allowed"`
	AttemptsRemaining            int64  `json:"attempts_remaining"`
	ProviderRetryGuidance        string `json:"provider_retry_guidance"`
}

type Report struct {
	Input
	DomainStatus      string `json:"domain_status"`
	Action            string `json:"action"`
	Reason            string `json:"reason"`
	EligibleToClaim   bool   `json:"eligible_to_claim"`
	AttemptsIfClaimed int64  `json:"attempts_if_claimed"`
	DedupRequired     bool   `json:"dedup_required"`
}

var methods = []string{"GET", "HEAD", "OPTIONS", "TRACE", "POST", "PUT", "PATCH", "DELETE"}
var states = []string{"NOT_SENT", "OUTCOME_UNKNOWN", "APPLIED_CONFIRMED", "NOT_APPLIED_CONFIRMED"}
var guidances = []string{"ALLOW", "VETO", "ABSENT", "UNKNOWN"}

func member(s string, values []string) bool {
	for _, value := range values {
		if s == value {
			return true
		}
	}
	return false
}

// Evaluate uses mutually exclusive predicates, rather than a candidate if-chain.
// It evaluates supplied facts only. It does not authenticate, claim or send.
func Evaluate(in Input) (Report, error) {
	valid := member(in.Method, methods) && member(in.DeliveryState, states) && member(in.ProviderRetryGuidance, guidances) && in.AttemptsRemaining >= 0 && in.AttemptsRemaining <= 2
	read := member(in.Method, methods[:4])
	authorized := valid && in.CurrentAccessAllowed
	bound := authorized && in.RequestBindingCurrent
	unapplied := bound && in.DeliveryState != "APPLIED_CONFIRMED"
	notVetoed := unapplied && in.ProviderRetryGuidance != "VETO"
	budgeted := notVetoed && in.AttemptsRemaining > 0
	replayable := budgeted && in.BodyReplayable
	informed := replayable && (in.ProviderRetryGuidance == "ALLOW" || in.ProviderRetryGuidance == "ABSENT")
	uncertain := informed && in.DeliveryState == "OUTCOME_UNKNOWN"
	type branch struct {
		selected       bool
		action, reason string
	}
	choices := []branch{
		{!valid, "STOP", "INPUT_DOMAIN_INVALID"},
		{valid && !in.CurrentAccessAllowed, "STOP", "CURRENT_ACCESS_REFUSED"},
		{authorized && !in.RequestBindingCurrent, "STOP", "REQUEST_BINDING_CHANGED"},
		{bound && in.DeliveryState == "APPLIED_CONFIRMED", "LOOKUP_RECORDED_RESULT", "OPERATION_ALREADY_APPLIED"},
		{unapplied && in.ProviderRetryGuidance == "VETO", "STOP", "PROVIDER_RETRY_VETO"},
		{notVetoed && in.AttemptsRemaining == 0, "STOP", "ATTEMPT_BUDGET_EXHAUSTED"},
		{budgeted && !in.BodyReplayable, "STOP", "BODY_NOT_REPLAYABLE"},
		{replayable && in.ProviderRetryGuidance == "UNKNOWN", "REFRESH_OUTCOME", "PROVIDER_GUIDANCE_UNVERIFIED"},
		{informed && (in.DeliveryState == "NOT_SENT" || in.DeliveryState == "NOT_APPLIED_CONFIRMED"), "REDELIVER_ELIGIBLE", "POSITIVE_NONAPPLICATION_EVIDENCE"},
		{uncertain && !in.ServerRepeatContractVerified, "REFRESH_OUTCOME", "SERVER_REPEAT_CONTRACT_UNVERIFIED"},
		{uncertain && in.ServerRepeatContractVerified && read, "REDELIVER_ELIGIBLE", "VERIFIED_REPEAT_CONTRACT"},
		{uncertain && in.ServerRepeatContractVerified && !read && !in.IdempotencyKeyPresent, "REFRESH_OUTCOME", "DEDUP_KEY_NOT_BOUND"},
		{uncertain && in.ServerRepeatContractVerified && !read && in.IdempotencyKeyPresent, "REDELIVER_ELIGIBLE", "VERIFIED_SERVER_DEDUP"},
	}
	report := Report{Input: in, DomainStatus: "INVALID_INPUT", AttemptsIfClaimed: in.AttemptsRemaining}
	if valid {
		report.DomainStatus = "VALID_INPUT"
	}
	count := 0
	for _, item := range choices {
		if item.selected {
			count++
			report.Action = item.action
			report.Reason = item.reason
		}
	}
	if count != 1 {
		return Report{}, fmt.Errorf("independent predicate partition selected %d branches", count)
	}
	report.EligibleToClaim = report.Action == "REDELIVER_ELIGIBLE"
	// Invalid signed-int64 extremes never reach subtraction.
	if report.EligibleToClaim {
		report.AttemptsIfClaimed = in.AttemptsRemaining - 1
	}
	report.DedupRequired = valid && in.DeliveryState == "OUTCOME_UNKNOWN" && !read
	return report, nil
}

func ForEachValid(visit func(int, Input) error) error {
	index := 0
	for _, method := range methods {
		for _, state := range states {
			for _, body := range []bool{false, true} {
				for _, key := range []bool{false, true} {
					for _, contract := range []bool{false, true} {
						for _, binding := range []bool{false, true} {
							for _, access := range []bool{false, true} {
								for _, attempts := range []int64{0, 1, 2} {
									for _, guidance := range guidances {
										in := Input{method, state, body, key, contract, binding, access, attempts, guidance}
										if err := visit(index, in); err != nil {
											return err
										}
										index++
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if index != 12288 {
		return fmt.Errorf("wrong independent population: %d", index)
	}
	return nil
}

type Witness struct {
	Name   string `json:"name"`
	Inputs Input  `json:"inputs"`
}

func ManualWitnesses() []Witness {
	base := Input{"POST", "OUTCOME_UNKNOWN", true, true, true, true, true, 1, "ABSENT"}
	result := make([]Witness, 0, 16)
	add := func(name string, change func(*Input)) {
		in := base
		change(&in)
		result = append(result, Witness{name, in})
	}
	add("invalid-enum-before-access", func(in *Input) { in.Method = "post"; in.CurrentAccessAllowed = false })
	add("allow-does-not-bypass-access", func(in *Input) { in.ProviderRetryGuidance = "ALLOW"; in.CurrentAccessAllowed = false })
	add("allow-does-not-bypass-binding", func(in *Input) { in.ProviderRetryGuidance = "ALLOW"; in.RequestBindingCurrent = false })
	add("applied-veto-is-lookup-only", func(in *Input) {
		in.DeliveryState = "APPLIED_CONFIRMED"
		in.ProviderRetryGuidance = "VETO"
		in.AttemptsRemaining = 0
		in.BodyReplayable = false
	})
	add("veto-before-budget-and-body", func(in *Input) {
		in.ProviderRetryGuidance = "VETO"
		in.AttemptsRemaining = 0
		in.BodyReplayable = false
	})
	add("allow-does-not-bypass-budget", func(in *Input) { in.ProviderRetryGuidance = "ALLOW"; in.AttemptsRemaining = 0 })
	add("body-before-unverified-guidance", func(in *Input) { in.ProviderRetryGuidance = "UNKNOWN"; in.BodyReplayable = false })
	add("unknown-guidance-blocks-unsent", func(in *Input) { in.ProviderRetryGuidance = "UNKNOWN"; in.DeliveryState = "NOT_SENT" })
	add("terminal-whole-operation-nonapplication", func(in *Input) {
		in.DeliveryState = "NOT_APPLIED_CONFIRMED"
		in.ServerRepeatContractVerified = false
		in.IdempotencyKeyPresent = false
	})
	add("allow-does-not-prove-server-contract", func(in *Input) { in.ProviderRetryGuidance = "ALLOW"; in.ServerRepeatContractVerified = false })
	add("verified-read-repeat-without-key", func(in *Input) { in.Method = "GET"; in.IdempotencyKeyPresent = false })
	add("allow-does-not-supply-bound-key", func(in *Input) { in.ProviderRetryGuidance = "ALLOW"; in.IdempotencyKeyPresent = false })
	add("verified-write-dedup", func(in *Input) { in.ProviderRetryGuidance = "ALLOW" })
	add("signed-min-echo-no-arithmetic", func(in *Input) { in.AttemptsRemaining = -9223372036854775808; in.ProviderRetryGuidance = "VETO" })
	add("signed-max-echo-no-arithmetic", func(in *Input) { in.AttemptsRemaining = 9223372036854775807; in.ProviderRetryGuidance = "ALLOW" })
	add("verified-unsent-automatic-claim", func(in *Input) { in.DeliveryState = "NOT_SENT"; in.AttemptsRemaining = 2 })
	return result
}
