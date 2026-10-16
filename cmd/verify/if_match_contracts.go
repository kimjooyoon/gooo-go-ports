package main

const ifMatchPackage = "go/net/http/precondition"
const ifMatchActivity = "IfMatchCondition"
const ifMatchSource = "ports/net/http/precondition.gooo"
const ifMatchActivityID = "gooo://go/net/http/if-match-condition"
const ifMatchEntity = "gooo://go/net/http/if-match-result/v1"
const ifMatchScope = "Selected If-Match and current ETag are two unchanged decoded Unicode-scalar validUTF8 Text0..1024bytes each, including empty/NUL/control/invalid grammar. Private Go checkIfMatch compatibility returns original condNone0/condTrue1/condFalse2 as one required int64 field. Initial empty differs from whitespace-only; star suffix, adjacent tags and early strong match are preserved. No Header.Get or multiple-field selection, HTTP adapter, authenticated current version, authorization, effects or CAS guarantee."
const ifMatchProvenanceScope = "All284 ordered input occurrences are retained; fresh expectations come only from the pinned original Go enum/checkIfMatch adapter with three explicit lookup substitutions and unchanged scanETag/etagStrongMatch/TrimString. Fresh native execution and saved replay bind original two inputs, one ordered integer result field, current one-source closure, lowered body/record/generated ABI, exact explicit Go binary and raw saved parent. Independent inner canonical/typed-plan hashing remains NOT_CHECKED; historical private receipts do not establish current-source acceptance."

func isIfMatchSuite(s suite) bool {
	return s.Package == ifMatchPackage && s.Activity == ifMatchActivity
}
func isHTTPConditionalSuite(s suite) bool { return isETagSuite(s) || isIfMatchSuite(s) }
func etagRecordsFor(s suite) []any {
	if isIfMatchSuite(s) {
		return []any{headerAdditionOwnRecord("IfMatchResult", ifMatchEntity, []string{"condition"}, []string{"integer"})}
	}
	return etagRecords(etagReportConsumer(s))
}
